package semanticmemory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const memorySchemaVersion = 2

type Manager struct {
	cfg      Config
	client   *QdrantClient
	embedder Embedder
}

type SearchHit struct {
	Text       string
	Kind       string
	SessionKey string
	Role       string
	Score      float64
	CreatedAt  string
}

type Status struct {
	Enabled    bool   `json:"enabled"`
	Ready      bool   `json:"ready"`
	URL        string `json:"url"`
	Collection string `json:"collection"`
	Provider   string `json:"provider"`
	Dimensions int    `json:"dimensions"`
	Count      int    `json:"count"`
	Error      string `json:"error,omitempty"`
}

func New(ctx context.Context, cfg Config) (*Manager, error) {
	cfg.normalize()
	if !cfg.Enabled {
		return nil, nil
	}
	if cfg.QdrantURL == "" {
		return nil, fmt.Errorf("qdrant_url is required when semantic memory is enabled")
	}
	embedder, err := newEmbedder(cfg)
	if err != nil {
		return nil, err
	}
	cfg.Dimensions = embedder.Dimensions()
	client := NewQdrantClient(cfg.QdrantURL, cfg.Collection, cfg.Dimensions)
	if err := client.EnsureCollection(ctx); err != nil {
		return nil, err
	}
	return &Manager{cfg: cfg, client: client, embedder: embedder}, nil
}

func (m *Manager) IndexMessage(ctx context.Context, sessionKey string, msg MessageInput) (int, error) {
	if m == nil || m.client == nil || m.embedder == nil {
		return 0, nil
	}
	candidates := ExtractCandidates(msg, m.cfg.MaxMemoryChars)
	if len(candidates) == 0 {
		return 0, nil
	}
	points := make([]QdrantPoint, 0, len(candidates))
	now := time.Now().UTC().Format(time.RFC3339)
	for _, candidate := range candidates {
		text := strings.TrimSpace(candidate.Text)
		if text == "" {
			continue
		}
		vector, err := m.embedder.Embed(ctx, text)
		if err != nil {
			return 0, err
		}
		id := stablePointID(sessionKey, msg.Role, candidate.Kind, text)
		points = append(points, QdrantPoint{
			ID:     id,
			Vector: vector,
			Payload: map[string]any{
				"text":           text,
				"kind":           candidate.Kind,
				"session_key":    sessionKey,
				"role":           msg.Role,
				"created_at":     now,
				"source":         "flyflor.semantic_memory",
				"schema_version": memorySchemaVersion,
			},
		})
	}
	if len(points) == 0 {
		return 0, nil
	}
	if err := m.client.Upsert(ctx, points); err != nil {
		return 0, err
	}
	return len(points), nil
}

func (m *Manager) Search(ctx context.Context, query string) ([]SearchHit, error) {
	if m == nil || m.client == nil || m.embedder == nil || strings.TrimSpace(query) == "" {
		return nil, nil
	}
	vector, err := m.embedder.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	results, err := m.client.Search(ctx, vector, m.cfg.TopK)
	if err != nil {
		return nil, err
	}
	hits := make([]SearchHit, 0, len(results))
	for _, result := range results {
		if result.Score < m.cfg.MinScore {
			continue
		}
		hit := SearchHit{
			Text:       payloadString(result.Payload, "text"),
			Kind:       payloadString(result.Payload, "kind"),
			SessionKey: payloadString(result.Payload, "session_key"),
			Role:       payloadString(result.Payload, "role"),
			CreatedAt:  payloadString(result.Payload, "created_at"),
			Score:      result.Score,
		}
		if strings.TrimSpace(hit.Text) != "" {
			hits = append(hits, hit)
		}
	}
	return hits, nil
}

func (m *Manager) DeleteSession(ctx context.Context, sessionKey string) error {
	if m == nil || m.client == nil || strings.TrimSpace(sessionKey) == "" {
		return nil
	}
	return m.client.DeleteSession(ctx, sessionKey)
}

func (m *Manager) Status(ctx context.Context) Status {
	status := Status{
		Enabled:    m != nil,
		Ready:      false,
		Collection: "",
		Provider:   "",
	}
	if m == nil {
		return status
	}
	status.URL = m.cfg.QdrantURL
	status.Collection = m.cfg.Collection
	status.Provider = m.cfg.Embedding.Provider
	status.Dimensions = m.cfg.Dimensions
	count, err := m.client.Count(ctx)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	status.Ready = true
	status.Count = count
	return status
}

func FormatHits(hits []SearchHit) string {
	if len(hits) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("VECTOR_MEMORY: Relevant long-term semantic memories from Qdrant. Treat them as useful hints, not as higher priority than the user's current instruction.\n")
	for _, hit := range hits {
		fmt.Fprintf(&sb, "- [%s score=%.2f] %s\n", hit.Kind, hit.Score, hit.Text)
	}
	return strings.TrimSpace(sb.String())
}

func stablePointID(parts ...string) string {
	joined := strings.Join(parts, "\x00")
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(joined)).String()
}

func payloadString(payload map[string]any, key string) string {
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprint(value)
}
