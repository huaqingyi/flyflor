package semanticmemory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type QdrantClient struct {
	baseURL    string
	collection string
	dimensions int
	httpClient *http.Client
}

type QdrantPoint struct {
	ID      string         `json:"id"`
	Vector  []float32      `json:"vector"`
	Payload map[string]any `json:"payload"`
}

type QdrantSearchResult struct {
	Score   float64        `json:"score"`
	Payload map[string]any `json:"payload"`
}

func NewQdrantClient(baseURL, collection string, dimensions int) *QdrantClient {
	return &QdrantClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		collection: strings.TrimSpace(collection),
		dimensions: dimensions,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *QdrantClient) EnsureCollection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.collectionURL(), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("qdrant collection check failed: %s", resp.Status)
	}

	body := map[string]any{
		"vectors": map[string]any{
			"size":     c.dimensions,
			"distance": "Cosine",
		},
	}
	return c.doJSON(ctx, http.MethodPut, c.collectionURL(), body, nil)
}

func (c *QdrantClient) Upsert(ctx context.Context, points []QdrantPoint) error {
	if len(points) == 0 {
		return nil
	}
	body := map[string]any{"points": points}
	return c.doJSON(ctx, http.MethodPut, c.pointsURL("points")+"?wait=true", body, nil)
}

func (c *QdrantClient) Search(ctx context.Context, vector []float32, limit int) ([]QdrantSearchResult, error) {
	if limit <= 0 {
		limit = defaultTopK
	}
	body := map[string]any{
		"vector":       vector,
		"limit":        limit,
		"with_payload": true,
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key": "schema_version",
					"match": map[string]any{
						"value": memorySchemaVersion,
					},
				},
			},
		},
	}
	var out struct {
		Result []QdrantSearchResult `json:"result"`
	}
	if err := c.doJSON(ctx, http.MethodPost, c.pointsURL("points/search"), body, &out); err != nil {
		return nil, err
	}
	return out.Result, nil
}

func (c *QdrantClient) DeleteSession(ctx context.Context, sessionKey string) error {
	body := map[string]any{
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key": "session_key",
					"match": map[string]any{
						"value": sessionKey,
					},
				},
			},
		},
	}
	return c.doJSON(ctx, http.MethodPost, c.pointsURL("points/delete")+"?wait=true", body, nil)
}

func (c *QdrantClient) Count(ctx context.Context) (int, error) {
	body := map[string]any{
		"exact": true,
		"filter": map[string]any{
			"must": []map[string]any{
				{
					"key": "schema_version",
					"match": map[string]any{
						"value": memorySchemaVersion,
					},
				},
			},
		},
	}
	var out struct {
		Result struct {
			Count int `json:"count"`
		} `json:"result"`
	}
	if err := c.doJSON(ctx, http.MethodPost, c.pointsURL("points/count"), body, &out); err != nil {
		return 0, err
	}
	return out.Result.Count, nil
}

func (c *QdrantClient) collectionURL() string {
	return c.baseURL + "/collections/" + url.PathEscape(c.collection)
}

func (c *QdrantClient) pointsURL(path string) string {
	return c.collectionURL() + "/" + strings.TrimLeft(path, "/")
}

func (c *QdrantClient) doJSON(ctx context.Context, method, endpoint string, body any, out any) error {
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
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant %s %s failed: %s", method, endpoint, resp.Status)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
