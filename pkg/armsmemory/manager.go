package armsmemory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Store interface {
	UpsertMethodologies(ctx context.Context, methodologies []Methodology) error
	SearchMethodologies(ctx context.Context, query string, limit int, minScore float64) ([]SearchHit, error)
	Status(ctx context.Context) (Status, error)
}

type Manager struct {
	cfg   Config
	store Store
}

type SearchHit struct {
	Title        string   `json:"title"`
	Text         string   `json:"text"`
	Situation    string   `json:"situation,omitempty"`
	Method       string   `json:"method,omitempty"`
	Avoid        string   `json:"avoid,omitempty"`
	NextTimeHint string   `json:"next_time_hint,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	SessionKey   string   `json:"session_key,omitempty"`
	Score        float64  `json:"score"`
	CreatedAt    string   `json:"created_at,omitempty"`
}

type Status struct {
	Enabled   bool   `json:"enabled"`
	Ready     bool   `json:"ready"`
	Driver    string `json:"driver"`
	APIBase   string `json:"api_base"`
	Path      string `json:"path,omitempty"`
	SpaceID   string `json:"space_id"`
	Namespace string `json:"namespace"`
	Count     int    `json:"count,omitempty"`
	Error     string `json:"error,omitempty"`
}

func New(cfg Config) (*Manager, error) {
	cfg.normalize()
	if !cfg.Enabled {
		return nil, nil
	}
	var store Store
	switch cfg.Driver {
	case "local", "":
		local, err := NewLocalStore(cfg)
		if err != nil {
			return nil, err
		}
		store = local
	case "http":
		if cfg.APIBase == "" {
			return nil, fmt.Errorf("arms api_base is required when arms http driver is enabled")
		}
		store = NewHTTPStore(cfg)
	default:
		return nil, fmt.Errorf("unsupported arms driver %q", cfg.Driver)
	}
	return &Manager{
		cfg:   cfg,
		store: store,
	}, nil
}

func NewWithStore(cfg Config, store Store) *Manager {
	cfg.normalize()
	if store == nil {
		return nil
	}
	return &Manager{cfg: cfg, store: store}
}

func (m *Manager) IndexMessage(ctx context.Context, sessionKey string, msg MessageInput) (int, error) {
	if m == nil || m.store == nil {
		return 0, nil
	}
	methodologies := ExtractMethodologies(msg, m.cfg.MaxMethodologyChars)
	if len(methodologies) == 0 {
		return 0, nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for i := range methodologies {
		methodologies[i].SessionKey = sessionKey
		methodologies[i].CreatedAt = now
		methodologies[i].Namespace = m.cfg.MethodologyNamespace
		methodologies[i].ID = stableMethodologyID(
			m.cfg.SpaceID,
			m.cfg.MethodologyNamespace,
			methodologies[i].Text,
		)
	}
	indexCtx, cancel := context.WithTimeout(ctx, m.cfg.timeout())
	defer cancel()
	if err := m.store.UpsertMethodologies(indexCtx, methodologies); err != nil {
		return 0, err
	}
	return len(methodologies), nil
}

func (m *Manager) Search(ctx context.Context, query string) ([]SearchHit, error) {
	if m == nil || m.store == nil || strings.TrimSpace(query) == "" {
		return nil, nil
	}
	searchCtx, cancel := context.WithTimeout(ctx, m.cfg.timeout())
	defer cancel()
	return m.store.SearchMethodologies(searchCtx, query, m.cfg.TopK, m.cfg.MinScore)
}

func (m *Manager) Status(ctx context.Context) Status {
	status := Status{
		Enabled:   m != nil,
		Ready:     false,
		Driver:    "",
		APIBase:   "",
		Path:      "",
		SpaceID:   "",
		Namespace: "",
	}
	if m == nil {
		return status
	}
	status.Driver = m.cfg.Driver
	status.APIBase = m.cfg.APIBase
	status.Path = m.cfg.Path
	status.SpaceID = m.cfg.SpaceID
	status.Namespace = m.cfg.MethodologyNamespace
	if m.store == nil {
		status.Error = "arms store is not configured"
		return status
	}
	checkCtx, cancel := context.WithTimeout(ctx, m.cfg.timeout())
	defer cancel()
	storeStatus, err := m.store.Status(checkCtx)
	if err != nil {
		status.Error = err.Error()
		return status
	}
	storeStatus.Enabled = true
	if storeStatus.APIBase == "" {
		storeStatus.APIBase = status.APIBase
	}
	if storeStatus.Driver == "" {
		storeStatus.Driver = status.Driver
	}
	if storeStatus.Path == "" {
		storeStatus.Path = status.Path
	}
	if storeStatus.SpaceID == "" {
		storeStatus.SpaceID = status.SpaceID
	}
	if storeStatus.Namespace == "" {
		storeStatus.Namespace = status.Namespace
	}
	return storeStatus
}

func FormatHits(hits []SearchHit) string {
	if len(hits) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("METHODOLOGY_MEMORY: Retrieved from ARMS. These are Flyflor self-growth methods only, not user facts, project facts, or general conversation memory. Use them as reusable process guidance before planning.\n")
	for _, hit := range hits {
		title := strings.TrimSpace(hit.Title)
		if title == "" {
			title = "methodology"
		}
		fmt.Fprintf(&sb, "- [%s score=%.2f] %s", title, hit.Score, strings.TrimSpace(hit.Text))
		if hint := strings.TrimSpace(hit.NextTimeHint); hint != "" {
			fmt.Fprintf(&sb, "\n  Next-time hint: %s", hint)
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func stableMethodologyID(parts ...string) string {
	joined := strings.Join(parts, "\x00")
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(joined)).String()
}
