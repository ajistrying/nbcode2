package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ProviderConfig holds settings for a single LLM provider.
type ProviderConfig struct {
	APIKey       string `yaml:"api_key"`
	BaseURL      string `yaml:"base_url"`
	DefaultModel string `yaml:"default_model"`
}

// PermissionMode controls how tool execution is approved.
type PermissionMode string

const (
	PermissionRiskTiered PermissionMode = "risk-tiered"
	PermissionAlwaysAsk  PermissionMode = "always-ask"
	PermissionYolo       PermissionMode = "yolo"
)

// SummarizationConfig holds settings for context auto-summarization.
type SummarizationConfig struct {
	Provider  string  `yaml:"provider"`
	Model     string  `yaml:"model"`
	Threshold float64 `yaml:"threshold"` // 0.0-1.0, default 0.8
}

// Config is the top-level configuration for nbcode.
type Config struct {
	Providers       map[string]ProviderConfig `yaml:"providers"`
	DefaultProvider string                    `yaml:"default_provider"`
	Permissions     PermissionMode            `yaml:"permissions"`
	Summarization   SummarizationConfig       `yaml:"summarization"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Providers:       make(map[string]ProviderConfig),
		DefaultProvider: "openai",
		Permissions:     PermissionRiskTiered,
		Summarization: SummarizationConfig{
			Threshold: 0.8,
		},
	}
}

// loadDotEnv reads a .env file from the current directory and sets
// any variables that aren't already set in the environment.
// Existing env vars take precedence over .env values.
func loadDotEnv() {
	f, err := os.Open(".env")
	if err != nil {
		return // no .env file, that's fine
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		// Strip surrounding quotes from value
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}

		// Only set if not already in environment (env vars win)
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

// Load reads .env from the current directory, then ~/.nbcode/config.yaml,
// then applies env var overrides.
func Load() (*Config, error) {
	loadDotEnv()

	cfg := DefaultConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, nil // fall through to env vars
	}

	configPath := filepath.Join(home, ".nbcode", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnvOverrides(cfg)
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

// applyEnvOverrides applies environment variable overrides to the config.
// Env vars take precedence over config file values.
func applyEnvOverrides(cfg *Config) {
	// Anthropic
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		p := cfg.Providers["anthropic"]
		p.APIKey = key
		if p.BaseURL == "" {
			p.BaseURL = "https://api.anthropic.com"
		}
		cfg.Providers["anthropic"] = p
	}
	if model := os.Getenv("ANTHROPIC_MODEL"); model != "" {
		p := cfg.Providers["anthropic"]
		p.DefaultModel = model
		cfg.Providers["anthropic"] = p
	}

	// OpenAI-compatible (covers OpenAI, vLLM, HF)
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		p := cfg.Providers["openai"]
		p.APIKey = key
		cfg.Providers["openai"] = p
	}
	if url := os.Getenv("OPENAI_BASE_URL"); url != "" {
		p := cfg.Providers["openai"]
		p.BaseURL = url
		cfg.Providers["openai"] = p
	}
	if model := os.Getenv("OPENAI_MODEL"); model != "" {
		p := cfg.Providers["openai"]
		p.DefaultModel = model
		cfg.Providers["openai"] = p
	}

	// Default provider override
	if dp := os.Getenv("NBCODE_PROVIDER"); dp != "" {
		cfg.DefaultProvider = dp
	}

	// Permission mode override
	if pm := os.Getenv("NBCODE_PERMISSIONS"); pm != "" {
		cfg.Permissions = PermissionMode(pm)
	}
}
