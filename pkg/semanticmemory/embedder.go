package semanticmemory

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"strings"
	"unicode"
)

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimensions() int
}

func newEmbedder(cfg Config) (Embedder, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Embedding.Provider)) {
	case "", "hash", "local":
		return hashEmbedder{dimensions: cfg.Dimensions}, nil
	case "openai", "openai-compatible", "compatible":
		if cfg.Embedding.APIBase == "" || cfg.Embedding.Model == "" {
			return nil, fmt.Errorf("embedding api_base and model are required")
		}
		return &openAICompatibleEmbedder{
			apiBase:    cfg.Embedding.APIBase,
			apiKey:     cfg.Embedding.APIKey,
			model:      cfg.Embedding.Model,
			dimensions: cfg.Embedding.Dimensions,
			client:     &http.Client{},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider %q", cfg.Embedding.Provider)
	}
}

type hashEmbedder struct {
	dimensions int
}

func (e hashEmbedder) Dimensions() int {
	return e.dimensions
}

func (e hashEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	dim := e.dimensions
	if dim <= 0 {
		dim = defaultDimensions
	}
	vec := make([]float32, dim)
	for _, token := range memoryTokens(text) {
		h := fnv.New64a()
		_, _ = h.Write([]byte(strings.ToLower(token)))
		sum := h.Sum64()
		idx := int(sum % uint64(dim))
		sign := float32(1)
		if (sum>>63)&1 == 1 {
			sign = -1
		}
		vec[idx] += sign
	}
	normalize(vec)
	return vec, nil
}

func memoryTokens(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	words := strings.FieldsFunc(text, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsNumber(r))
	})
	tokens := make([]string, 0, len(words)+len([]rune(text))/3)
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word != "" {
			tokens = append(tokens, word)
		}
	}
	runes := []rune(text)
	for i := 0; i+2 < len(runes); i++ {
		if unicode.IsSpace(runes[i]) || unicode.IsSpace(runes[i+1]) || unicode.IsSpace(runes[i+2]) {
			continue
		}
		tokens = append(tokens, string(runes[i:i+3]))
	}
	return tokens
}

func normalize(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v * v)
	}
	if sum == 0 {
		return
	}
	scale := float32(1 / math.Sqrt(sum))
	for i := range vec {
		vec[i] *= scale
	}
}

type openAICompatibleEmbedder struct {
	apiBase    string
	apiKey     string
	model      string
	dimensions int
	client     *http.Client
}

func (e *openAICompatibleEmbedder) Dimensions() int {
	return e.dimensions
}

func (e *openAICompatibleEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body := map[string]any{
		"model": e.model,
		"input": text,
	}
	if e.dimensions > 0 {
		body["dimensions"] = e.dimensions
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.apiBase+"/embeddings", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding request failed: %s", resp.Status)
	}
	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("embedding response did not include a vector")
	}
	vec := out.Data[0].Embedding
	e.dimensions = len(vec)
	normalize(vec)
	return vec, nil
}
