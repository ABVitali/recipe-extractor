package config

import (
	"os"
	"testing"
)

func TestLoad_AllScope_MissingRequired(t *testing.T) {
	// Clear all env vars
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("LLM_BASE_URL")
	os.Unsetenv("LLM_API_KEY")

	_, err := Load("all")
	if err == nil {
		t.Fatal("expected error when required env vars are missing")
	}
}

func TestLoad_DBScope_OnlyNeedsDBURL(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test")
	defer os.Unsetenv("DATABASE_URL")
	os.Unsetenv("LLM_BASE_URL")
	os.Unsetenv("LLM_API_KEY")

	cfg, err := Load("db")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DatabaseURL != "postgres://test" {
		t.Errorf("expected DATABASE_URL=postgres://test, got %q", cfg.DatabaseURL)
	}
}

func TestLoad_Defaults(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://test")
	os.Setenv("LLM_BASE_URL", "http://localhost")
	os.Setenv("LLM_API_KEY", "key")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("LLM_BASE_URL")
		os.Unsetenv("LLM_API_KEY")
	}()

	cfg, err := Load("all")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LLMModel != "claude-opus-4-6" {
		t.Errorf("expected default model claude-opus-4-6, got %q", cfg.LLMModel)
	}
	if cfg.LLMMaxTokens != 128000 {
		t.Errorf("expected default max tokens 128000, got %d", cfg.LLMMaxTokens)
	}
	if cfg.MaxPagesPerChunk != 5 {
		t.Errorf("expected default chunk size 5, got %d", cfg.MaxPagesPerChunk)
	}
	if cfg.MigrationsPath != "migrations" {
		t.Errorf("expected default migrations path, got %q", cfg.MigrationsPath)
	}
}
