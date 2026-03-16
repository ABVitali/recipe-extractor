package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	DatabaseURL    string
	MigrationsPath string

	LLMBaseURL   string
	LLMAPIKey    string
	LLMModel     string
	LLMMaxTokens int64

	MaxPagesPerChunk int
}

// Load reads configuration from environment variables and validates required fields.
// The required parameter controls which fields are mandatory: "all" requires
// everything including LLM settings, "db" requires only database settings.
func Load(scope string) (*Config, error) {
	cfg := &Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		MigrationsPath: envOrDefault("MIGRATIONS_PATH", "migrations"),
		LLMBaseURL:     os.Getenv("LLM_BASE_URL"),
		LLMAPIKey:      os.Getenv("LLM_API_KEY"),
		LLMModel:       envOrDefault("LLM_MODEL", "claude-opus-4-6"),
		LLMMaxTokens:   envOrDefaultInt64("LLM_MAX_TOKENS", 128000),
		MaxPagesPerChunk: envOrDefaultInt("MAX_PAGES_PER_CHUNK", 5),
	}

	if err := cfg.validate(scope); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate(scope string) error {
	var missing []string

	if c.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}

	if scope == "all" {
		if c.LLMBaseURL == "" {
			missing = append(missing, "LLM_BASE_URL")
		}
		if c.LLMAPIKey == "" {
			missing = append(missing, "LLM_API_KEY")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("required environment variables not set: %s", strings.Join(missing, ", "))
	}
	return nil
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func envOrDefaultInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		n, err := strconv.Atoi(val)
		if err != nil {
			// Return default on parse error; caller validates.
			return defaultVal
		}
		return n
	}
	return defaultVal
}

func envOrDefaultInt64(key string, defaultVal int64) int64 {
	if val := os.Getenv(key); val != "" {
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return defaultVal
		}
		return n
	}
	return defaultVal
}
