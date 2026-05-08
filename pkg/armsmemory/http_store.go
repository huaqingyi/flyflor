package armsmemory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type HTTPStore struct {
	cfg        Config
	httpClient *http.Client
}

func NewHTTPStore(cfg Config) *HTTPStore {
	cfg.normalize()
	return &HTTPStore{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: cfg.timeout()},
	}
}

func (s *HTTPStore) UpsertMethodologies(ctx context.Context, methodologies []Methodology) error {
	if len(methodologies) == 0 {
		return nil
	}
	body := map[string]any{
		"space_id":       s.cfg.SpaceID,
		"namespace":      s.cfg.MethodologyNamespace,
		"documents":      methodologies,
		"isolation_kind": "methodology_only",
	}
	return s.doJSON(ctx, http.MethodPost, s.endpoint("methodologies/upsert"), body, nil)
}

func (s *HTTPStore) SearchMethodologies(ctx context.Context, query string, limit int, minScore float64) ([]SearchHit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultTopK
	}
	if minScore <= 0 {
		minScore = defaultMinScore
	}
	body := map[string]any{
		"query":          query,
		"limit":          limit,
		"min_score":      minScore,
		"namespace":      s.cfg.MethodologyNamespace,
		"isolation_kind": "methodology_only",
		"filter": map[string]any{
			"source":     "flyflor.reflection",
			"namespace":  s.cfg.MethodologyNamespace,
			"schema":     methodologySchemaVersion,
			"memoryType": "methodology",
		},
	}
	var out struct {
		Results []SearchHit `json:"results"`
		Hits    []SearchHit `json:"hits"`
	}
	if err := s.doJSON(ctx, http.MethodPost, s.endpoint("methodologies/search"), body, &out); err != nil {
		return nil, err
	}
	if len(out.Results) > 0 {
		return filterHits(out.Results, minScore), nil
	}
	return filterHits(out.Hits, minScore), nil
}

func (s *HTTPStore) Status(ctx context.Context) (Status, error) {
	var out Status
	if err := s.doJSON(ctx, http.MethodGet, s.endpoint("status"), nil, &out); err != nil {
		return Status{}, err
	}
	out.Enabled = true
	if out.Driver == "" {
		out.Driver = "http"
	}
	if out.APIBase == "" {
		out.APIBase = s.cfg.APIBase
	}
	if out.SpaceID == "" {
		out.SpaceID = s.cfg.SpaceID
	}
	if out.Namespace == "" {
		out.Namespace = s.cfg.MethodologyNamespace
	}
	if out.Error == "" {
		out.Ready = true
	}
	return out, nil
}

func (s *HTTPStore) endpoint(path string) string {
	return strings.TrimRight(s.cfg.APIBase, "/") +
		"/v1/spaces/" +
		url.PathEscape(s.cfg.SpaceID) +
		"/" +
		strings.TrimLeft(path, "/")
}

func (s *HTTPStore) doJSON(ctx context.Context, method, endpoint string, body any, out any) error {
	var reader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if s.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("arms %s %s failed: %s", method, endpoint, resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func filterHits(hits []SearchHit, minScore float64) []SearchHit {
	if len(hits) == 0 {
		return nil
	}
	out := make([]SearchHit, 0, len(hits))
	for _, hit := range hits {
		if hit.Score < minScore {
			continue
		}
		if strings.TrimSpace(hit.Text) == "" {
			continue
		}
		out = append(out, hit)
	}
	return out
}
