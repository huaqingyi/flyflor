package armsmemory

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultSpaceID              = "flyflor-methodologies"
	defaultDriver               = "local"
	defaultTopK                 = 5
	defaultMinScore             = 0.15
	defaultTimeout              = 4 * time.Second
	defaultMaxMethodologyChars  = 1600
	defaultMethodologyNamespace = "methodology"
	defaultDimensions           = 256
)

type Config struct {
	Enabled              bool    `json:"enabled"`
	Driver               string  `json:"driver"`
	Path                 string  `json:"path"`
	APIBase              string  `json:"api_base"`
	APIKey               string  `json:"api_key"`
	SpaceID              string  `json:"space_id"`
	Dimensions           int     `json:"dimensions"`
	TopK                 int     `json:"top_k"`
	MinScore             float64 `json:"min_score"`
	TimeoutSeconds       int     `json:"timeout_seconds"`
	MaxMethodologyChars  int     `json:"max_methodology_chars"`
	MethodologyNamespace string  `json:"methodology_namespace"`
}

func DefaultConfig() Config {
	apiBase := firstNonEmpty(os.Getenv("FLYFLOR_ARMS_API_BASE"), os.Getenv("ARMS_API_BASE"))
	path := firstNonEmpty(os.Getenv("FLYFLOR_ARMS_PATH"), os.Getenv("ARMS_PATH"))
	cfg := Config{
		Enabled:              strings.TrimSpace(apiBase) != "" || strings.TrimSpace(path) != "",
		Driver:               envOrDefault("FLYFLOR_ARMS_DRIVER", defaultDriver),
		Path:                 strings.TrimSpace(path),
		APIBase:              strings.TrimRight(strings.TrimSpace(apiBase), "/"),
		APIKey:               strings.TrimSpace(firstNonEmpty(os.Getenv("FLYFLOR_ARMS_API_KEY"), os.Getenv("ARMS_API_KEY"))),
		SpaceID:              envOrDefault("FLYFLOR_ARMS_SPACE_ID", defaultSpaceID),
		Dimensions:           envInt("FLYFLOR_ARMS_DIMENSIONS", defaultDimensions),
		TopK:                 envInt("FLYFLOR_ARMS_TOP_K", defaultTopK),
		MinScore:             envFloat("FLYFLOR_ARMS_MIN_SCORE", defaultMinScore),
		TimeoutSeconds:       envInt("FLYFLOR_ARMS_TIMEOUT_SECONDS", int(defaultTimeout.Seconds())),
		MaxMethodologyChars:  envInt("FLYFLOR_ARMS_MAX_CHARS", defaultMaxMethodologyChars),
		MethodologyNamespace: envOrDefault("FLYFLOR_ARMS_NAMESPACE", defaultMethodologyNamespace),
	}
	if enabled := strings.TrimSpace(os.Getenv("FLYFLOR_ARMS_ENABLED")); enabled != "" {
		cfg.Enabled = parseBool(enabled, cfg.Enabled)
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
		ARMS *Config `json:"arms"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.ARMS != nil {
		cfg.merge(*wrapped.ARMS)
		return cfg
	}

	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil || !hasARMSKeys(keys) {
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
	if next.APIBase != "" {
		c.APIBase = next.APIBase
	}
	if next.Driver != "" {
		c.Driver = next.Driver
	}
	if next.Path != "" {
		c.Path = next.Path
	}
	if next.APIKey != "" {
		c.APIKey = next.APIKey
	}
	if next.SpaceID != "" {
		c.SpaceID = next.SpaceID
	}
	if next.TopK > 0 {
		c.TopK = next.TopK
	}
	if next.Dimensions > 0 {
		c.Dimensions = next.Dimensions
	}
	if next.MinScore > 0 {
		c.MinScore = next.MinScore
	}
	if next.TimeoutSeconds > 0 {
		c.TimeoutSeconds = next.TimeoutSeconds
	}
	if next.MaxMethodologyChars > 0 {
		c.MaxMethodologyChars = next.MaxMethodologyChars
	}
	if next.MethodologyNamespace != "" {
		c.MethodologyNamespace = next.MethodologyNamespace
	}
	c.normalize()
}

func (c *Config) normalize() {
	c.Driver = strings.ToLower(strings.TrimSpace(c.Driver))
	if c.Driver == "" {
		c.Driver = defaultDriver
	}
	c.APIBase = strings.TrimRight(strings.TrimSpace(c.APIBase), "/")
	c.Path = strings.TrimSpace(c.Path)
	if c.Driver == "local" && c.Path == "" {
		c.Path = "~/.picoclaw/arms/arms.db"
	}
	c.SpaceID = strings.TrimSpace(c.SpaceID)
	if c.SpaceID == "" {
		c.SpaceID = defaultSpaceID
	}
	if c.TopK <= 0 {
		c.TopK = defaultTopK
	}
	if c.Dimensions <= 0 {
		c.Dimensions = defaultDimensions
	}
	if c.MinScore <= 0 {
		c.MinScore = defaultMinScore
	}
	if c.TimeoutSeconds <= 0 {
		c.TimeoutSeconds = int(defaultTimeout.Seconds())
	}
	if c.MaxMethodologyChars <= 0 {
		c.MaxMethodologyChars = defaultMaxMethodologyChars
	}
	c.MethodologyNamespace = strings.TrimSpace(c.MethodologyNamespace)
	if c.MethodologyNamespace == "" {
		c.MethodologyNamespace = defaultMethodologyNamespace
	}
}

func (c Config) timeout() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return defaultTimeout
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

func hasARMSKeys(keys map[string]json.RawMessage) bool {
	for _, key := range []string{
		"enabled",
		"driver",
		"path",
		"api_base",
		"api_key",
		"space_id",
		"dimensions",
		"top_k",
		"min_score",
		"timeout_seconds",
		"max_methodology_chars",
		"methodology_namespace",
	} {
		if _, ok := keys[key]; ok {
			return true
		}
	}
	return false
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
