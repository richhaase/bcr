package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	Concurrency     int               `yaml:"concurrency"`
	Retries         int               `yaml:"retries"`
	PRFeedback      bool              `yaml:"pr_feedback"`
	Exclude         []string          `yaml:"exclude"`
	Guidance        string            `yaml:"guidance"`
	GuidanceFile    string            `yaml:"guidance_file"`
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
		Concurrency:     0,
		Retries:         3,
		PRFeedback:      true,
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

	if val := os.Getenv("BCR_CONCURRENCY"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			cfg.Concurrency = n
		}
	}

	if val := os.Getenv("BCR_RETRIES"); val != "" {
		if n, err := strconv.Atoi(val); err == nil {
			cfg.Retries = n
		}
	}

	if val := os.Getenv("BCR_PR_FEEDBACK"); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			cfg.PRFeedback = b
		}
	}

	if val := os.Getenv("BCR_EXCLUDE"); val != "" {
		var list []string
		for _, p := range strings.Split(val, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				list = append(list, trimmed)
			}
		}
		cfg.Exclude = append(cfg.Exclude, list...)
	}

	if val := os.Getenv("BCR_GUIDANCE"); val != "" {
		cfg.Guidance = val
	}

	if val := os.Getenv("BCR_GUIDANCE_FILE"); val != "" {
		cfg.GuidanceFile = val
	}

	return cfg, nil
}

func (c *Config) ResolveGuidance() (string, error) {
	var parts []string

	if c.GuidanceFile != "" {
		data, err := os.ReadFile(c.GuidanceFile)
		if err != nil {
			return "", fmt.Errorf("read guidance file %q: %w", c.GuidanceFile, err)
		}
		parts = append(parts, strings.TrimSpace(string(data)))
	}

	if c.Guidance != "" {
		parts = append(parts, strings.TrimSpace(c.Guidance))
	}

	resolved := ""
	if len(parts) > 0 {
		resolved = strings.Join(parts, "\n")
	}

	return strings.TrimSpace(resolved), nil
}

func findConfigFile() string {
	if _, err := os.Stat(LocalConfigPath()); err == nil {
		return LocalConfigPath()
	}

	if path := GlobalConfigPath(); path != "" {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

func LocalConfigPath() string {
	return ".bcr.yaml"
}

func GlobalConfigPath() string {
	if dir := configDir(); dir != "" {
		return filepath.Join(dir, "bcr", "config.yaml")
	}
	return ""
}

const DefaultTemplate = `base_url: "https://openrouter.ai/api/v1"

models:
  - "deepseek/deepseek-chat"
  - "qwen/qwen-2.5-coder-32b-instruct"

summarizer_model: "anthropic/claude-3.7-sonnet"

base: "main"

temperature: 0.2

concurrency: 0

retries: 3

pr_feedback: true

exclude:
  - "generated/(.*)"
`

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
