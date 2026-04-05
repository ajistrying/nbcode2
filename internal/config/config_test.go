package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DefaultProvider != "openai" {
		t.Errorf("expected default provider 'openai', got %q", cfg.DefaultProvider)
	}
	if cfg.Permissions != PermissionRiskTiered {
		t.Errorf("expected risk-tiered permissions, got %q", cfg.Permissions)
	}
	if cfg.Summarization.Threshold != 0.8 {
		t.Errorf("expected summarization threshold 0.8, got %f", cfg.Summarization.Threshold)
	}
	if cfg.Providers == nil {
		t.Error("expected non-nil providers map")
	}
}

func TestLoadFromYAML(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".nbcode")
	os.MkdirAll(configDir, 0755)

	configContent := `
providers:
  anthropic:
    api_key: test-key-123
    default_model: claude-sonnet-4-20250514
  openai:
    api_key: sk-test
    base_url: http://localhost:8000/v1
    default_model: llama3

default_provider: anthropic
permissions: yolo
summarization:
  threshold: 0.9
`
	configPath := filepath.Join(configDir, "config.yaml")
	os.WriteFile(configPath, []byte(configContent), 0644)

	// Override HOME to use our temp dir
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Clear env vars that would interfere
	for _, key := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OPENAI_BASE_URL", "OPENAI_MODEL", "ANTHROPIC_MODEL", "NBCODE_PROVIDER", "NBCODE_PERMISSIONS"} {
		orig := os.Getenv(key)
		os.Unsetenv(key)
		defer os.Setenv(key, orig)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DefaultProvider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", cfg.DefaultProvider)
	}
	if cfg.Permissions != PermissionYolo {
		t.Errorf("expected yolo permissions, got %q", cfg.Permissions)
	}
	if cfg.Providers["anthropic"].APIKey != "test-key-123" {
		t.Errorf("expected anthropic key 'test-key-123', got %q", cfg.Providers["anthropic"].APIKey)
	}
	if cfg.Providers["openai"].BaseURL != "http://localhost:8000/v1" {
		t.Errorf("expected openai base URL, got %q", cfg.Providers["openai"].BaseURL)
	}
	if cfg.Summarization.Threshold != 0.9 {
		t.Errorf("expected threshold 0.9, got %f", cfg.Summarization.Threshold)
	}
}

func TestEnvVarOverrides(t *testing.T) {
	// Use a temp dir with no config file
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Set env vars
	envVars := map[string]string{
		"ANTHROPIC_API_KEY": "env-anthropic-key",
		"ANTHROPIC_MODEL":   "claude-opus-4-20250514",
		"OPENAI_API_KEY":    "env-openai-key",
		"OPENAI_BASE_URL":   "http://vllm:8000/v1",
		"OPENAI_MODEL":      "meta-llama/Llama-3",
		"NBCODE_PROVIDER":   "anthropic",
		"NBCODE_PERMISSIONS": "always-ask",
	}

	for k, v := range envVars {
		orig := os.Getenv(k)
		os.Setenv(k, v)
		defer os.Setenv(k, orig)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Providers["anthropic"].APIKey != "env-anthropic-key" {
		t.Errorf("expected anthropic key from env, got %q", cfg.Providers["anthropic"].APIKey)
	}
	if cfg.Providers["anthropic"].DefaultModel != "claude-opus-4-20250514" {
		t.Errorf("expected anthropic model from env, got %q", cfg.Providers["anthropic"].DefaultModel)
	}
	if cfg.Providers["openai"].APIKey != "env-openai-key" {
		t.Errorf("expected openai key from env, got %q", cfg.Providers["openai"].APIKey)
	}
	if cfg.Providers["openai"].BaseURL != "http://vllm:8000/v1" {
		t.Errorf("expected openai base URL from env, got %q", cfg.Providers["openai"].BaseURL)
	}
	if cfg.DefaultProvider != "anthropic" {
		t.Errorf("expected provider from env, got %q", cfg.DefaultProvider)
	}
	if cfg.Permissions != PermissionAlwaysAsk {
		t.Errorf("expected always-ask permissions from env, got %q", cfg.Permissions)
	}
}

func TestEnvOverridesConfigFile(t *testing.T) {
	// Create config file with one key, env has a different one
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, ".nbcode")
	os.MkdirAll(configDir, 0755)

	configContent := `
providers:
  anthropic:
    api_key: config-file-key
    default_model: claude-haiku
`
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configContent), 0644)

	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Env var should override config file
	orig := os.Getenv("ANTHROPIC_API_KEY")
	os.Setenv("ANTHROPIC_API_KEY", "env-override-key")
	defer os.Setenv("ANTHROPIC_API_KEY", orig)

	// Clear other env vars
	for _, key := range []string{"OPENAI_API_KEY", "OPENAI_BASE_URL", "OPENAI_MODEL", "ANTHROPIC_MODEL", "NBCODE_PROVIDER", "NBCODE_PERMISSIONS"} {
		o := os.Getenv(key)
		os.Unsetenv(key)
		defer os.Setenv(key, o)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// API key should come from env
	if cfg.Providers["anthropic"].APIKey != "env-override-key" {
		t.Errorf("expected env override key, got %q", cfg.Providers["anthropic"].APIKey)
	}
}

func TestLoadNoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", origHome)

	// Clear all env vars
	for _, key := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OPENAI_BASE_URL", "OPENAI_MODEL", "ANTHROPIC_MODEL", "NBCODE_PROVIDER", "NBCODE_PERMISSIONS"} {
		orig := os.Getenv(key)
		os.Unsetenv(key)
		defer os.Setenv(key, orig)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should get defaults
	if cfg.DefaultProvider != "openai" {
		t.Errorf("expected default provider, got %q", cfg.DefaultProvider)
	}
}
