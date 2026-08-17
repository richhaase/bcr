package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BaseURL         string            `yaml:"base_url"`
	APIKey          string            `yaml:"api_key"`
	Models          []string          `yaml:"models"`
	SummarizerModel string            `yaml:"summarizer_model"`
	Extra           map[string]string `yaml:"extra"`
	Base            string            `yaml:"base"`
	Temperature     float64           `yaml:"temperature"`
}

func DefaultConfig() *Config {
	return &Config{
		BaseURL: "https://openrouter.ai/api/v1",
		Models: []string{
			"deepseek/deepseek-chat",
			"qwen/qwen-2.5-coder-32b-instruct",
		},
		SummarizerModel: "anthropic/claude-3.7-sonnet",
		Extra:           make(map[string]string),
		Base:            "main",
		Temperature:     0.2,
	}
}

func Load() (*Config, error) {
	cfg := DefaultConfig()

	if path := findConfigFile(); path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			_ = yaml.Unmarshal(data, cfg)
		}
	}

	if val := os.Getenv("BCR_BASE_URL"); val != "" {
		cfg.BaseURL = val
	}
	cfg.APIKey = providerKeyFor(cfg.BaseURL)

	if val := os.Getenv("BCR_MODELS"); val != "" {
		var list []string
		for _, m := range strings.Split(val, ",") {
			trimmed := strings.TrimSpace(m)
			if trimmed != "" {
				list = append(list, trimmed)
			}
		}
		if len(list) > 0 {
			cfg.Models = list
		}
	}

	if val := os.Getenv("BCR_SUMMARIZER_MODEL"); val != "" {
		cfg.SummarizerModel = val
	}

	if val := os.Getenv("BCR_BASE"); val != "" {
		cfg.Base = val
	}

	return cfg, nil
}

func findConfigFile() string {
	if _, err := os.Stat(".bcr.yaml"); err == nil {
		return ".bcr.yaml"
	}

	if dir := configDir(); dir != "" {
		path := filepath.Join(dir, "bcr", "config.yaml")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func configDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return dir
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config")
	}
	return ""
}

func providerKeyFor(baseURL string) string {
	switch {
	case strings.Contains(baseURL, "openrouter"):
		return os.Getenv("OPENROUTER_API_KEY")
	case strings.Contains(baseURL, "openai.com"):
		return os.Getenv("OPENAI_API_KEY")
	case strings.Contains(baseURL, "anthropic.com"):
		return os.Getenv("ANTHROPIC_API_KEY")
	default:
		return ""
	}
}
