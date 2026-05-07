package semanticmemory

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

const (
	defaultCollection = "flyflor_memories"
	defaultDimensions = 256
	defaultTopK       = 6
	defaultMinScore   = 0.18
)

type Config struct {
	Enabled        bool            `json:"enabled"`
	QdrantURL      string          `json:"qdrant_url"`
	Collection     string          `json:"collection"`
	Dimensions     int             `json:"dimensions"`
	TopK           int             `json:"top_k"`
	MinScore       float64         `json:"min_score"`
	MaxMemoryChars int             `json:"max_memory_chars"`
	Embedding      EmbeddingConfig `json:"embedding"`
}

type EmbeddingConfig struct {
	Provider   string `json:"provider"`
	APIBase    string `json:"api_base"`
	APIKey     string `json:"api_key"`
	Model      string `json:"model"`
	Dimensions int    `json:"dimensions"`
}

func DefaultConfig() Config {
	url := firstNonEmpty(os.Getenv("FLYFLOR_QDRANT_URL"), os.Getenv("QDRANT_URL"))
	cfg := Config{
		Enabled:        strings.TrimSpace(url) != "",
		QdrantURL:      strings.TrimRight(strings.TrimSpace(url), "/"),
		Collection:     envOrDefault("FLYFLOR_QDRANT_COLLECTION", defaultCollection),
		Dimensions:     envInt("FLYFLOR_MEMORY_DIMENSIONS", defaultDimensions),
		TopK:           envInt("FLYFLOR_MEMORY_TOP_K", defaultTopK),
		MinScore:       envFloat("FLYFLOR_MEMORY_MIN_SCORE", defaultMinScore),
		MaxMemoryChars: envInt("FLYFLOR_MEMORY_MAX_CHARS", 900),
		Embedding: EmbeddingConfig{
			Provider:   strings.TrimSpace(os.Getenv("FLYFLOR_EMBEDDING_PROVIDER")),
			APIBase:    strings.TrimRight(strings.TrimSpace(os.Getenv("FLYFLOR_EMBEDDING_API_BASE")), "/"),
			APIKey:     strings.TrimSpace(os.Getenv("FLYFLOR_EMBEDDING_API_KEY")),
			Model:      strings.TrimSpace(os.Getenv("FLYFLOR_EMBEDDING_MODEL")),
			Dimensions: envInt("FLYFLOR_EMBEDDING_DIMENSIONS", 0),
		},
	}

	if enabled := strings.TrimSpace(os.Getenv("FLYFLOR_SEMANTIC_MEMORY_ENABLED")); enabled != "" {
		cfg.Enabled = parseBool(enabled, cfg.Enabled)
	}
	if cfg.Embedding.Provider == "" {
		if cfg.Embedding.APIBase != "" && cfg.Embedding.Model != "" {
			cfg.Embedding.Provider = "openai"
		} else {
			cfg.Embedding.Provider = "hash"
		}
	}
	cfg.normalize()
	return cfg
}

func ConfigFromRaw(raw json.RawMessage) Config {
	cfg := DefaultConfig()
	if len(raw) == 0 {
		return cfg
	}

	var wrapped struct {
		SemanticMemory *Config `json:"semantic_memory"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.SemanticMemory != nil {
		cfg.merge(*wrapped.SemanticMemory)
		return cfg
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil || !hasSemanticMemoryKeys(keys) {
		return cfg
	}

	var direct Config
	if err := json.Unmarshal(raw, &direct); err == nil {
		cfg.merge(direct)
	}
	return cfg
}

func (c *Config) merge(next Config) {
	c.Enabled = next.Enabled
	if next.QdrantURL != "" {
		c.QdrantURL = next.QdrantURL
	}
	if next.Collection != "" {
		c.Collection = next.Collection
	}
	if next.Dimensions > 0 {
		c.Dimensions = next.Dimensions
	}
	if next.TopK > 0 {
		c.TopK = next.TopK
	}
	if next.MinScore > 0 {
		c.MinScore = next.MinScore
	}
	if next.MaxMemoryChars > 0 {
		c.MaxMemoryChars = next.MaxMemoryChars
	}
	if next.Embedding.Provider != "" {
		c.Embedding.Provider = next.Embedding.Provider
	}
	if next.Embedding.APIBase != "" {
		c.Embedding.APIBase = next.Embedding.APIBase
	}
	if next.Embedding.APIKey != "" {
		c.Embedding.APIKey = next.Embedding.APIKey
	}
	if next.Embedding.Model != "" {
		c.Embedding.Model = next.Embedding.Model
	}
	if next.Embedding.Dimensions > 0 {
		c.Embedding.Dimensions = next.Embedding.Dimensions
	}
	c.normalize()
}

func (c *Config) normalize() {
	c.QdrantURL = strings.TrimRight(strings.TrimSpace(c.QdrantURL), "/")
	c.Collection = strings.TrimSpace(c.Collection)
	if c.Collection == "" {
		c.Collection = defaultCollection
	}
	if c.Dimensions <= 0 {
		c.Dimensions = defaultDimensions
	}
	if c.TopK <= 0 {
		c.TopK = defaultTopK
	}
	if c.MinScore <= 0 {
		c.MinScore = defaultMinScore
	}
	if c.MaxMemoryChars <= 0 {
		c.MaxMemoryChars = 900
	}
	c.Embedding.Provider = strings.ToLower(strings.TrimSpace(c.Embedding.Provider))
	if c.Embedding.Provider == "" {
		c.Embedding.Provider = "hash"
	}
	c.Embedding.APIBase = strings.TrimRight(strings.TrimSpace(c.Embedding.APIBase), "/")
	if c.Embedding.Dimensions <= 0 {
		c.Embedding.Dimensions = c.Dimensions
	}
	if c.Embedding.Provider == "hash" {
		c.Dimensions = c.Embedding.Dimensions
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func envFloat(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func parseBool(value string, fallback bool) bool {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func hasSemanticMemoryKeys(keys map[string]json.RawMessage) bool {
	for _, key := range []string{
		"enabled",
		"qdrant_url",
		"collection",
		"dimensions",
		"top_k",
		"min_score",
		"max_memory_chars",
		"embedding",
	} {
		if _, ok := keys[key]; ok {
			return true
		}
	}
	return false
}
