package config

import (
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.BaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("expected default openrouter base url, got %s", cfg.BaseURL)
	}

	if len(cfg.Models) == 0 {
		t.Error("expected default models")
	}
}

func TestEnvOverride(t *testing.T) {
	t.Setenv("BCR_MODELS", "model-one,model-two")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(cfg.Models) != 2 || cfg.Models[0] != "model-one" {
		t.Errorf("expected models override, got %v", cfg.Models)
	}
}

func TestConcurrencyDefaultZero(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Concurrency != 0 {
		t.Errorf("expected default concurrency 0, got %d", cfg.Concurrency)
	}
}

func TestConcurrencyEnvOverride(t *testing.T) {
	t.Setenv("BCR_CONCURRENCY", "3")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Concurrency != 3 {
		t.Errorf("expected concurrency 3, got %d", cfg.Concurrency)
	}
}

func TestConcurrencyIgnoredWhenEmpty(t *testing.T) {
	t.Setenv("BCR_CONCURRENCY", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Concurrency != 0 {
		t.Errorf("expected concurrency 0, got %d", cfg.Concurrency)
	}
}

func TestRetriesDefaultThree(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Retries != 3 {
		t.Errorf("expected default retries 3, got %d", cfg.Retries)
	}
}

func TestRetriesEnvOverride(t *testing.T) {
	t.Setenv("BCR_RETRIES", "5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Retries != 5 {
		t.Errorf("expected retries 5, got %d", cfg.Retries)
	}
}

func TestRetriesIgnoredWhenEmpty(t *testing.T) {
	t.Setenv("BCR_RETRIES", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Retries != 3 {
		t.Errorf("expected retries 3, got %d", cfg.Retries)
	}
}

func TestProviderKeyDerivation(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	t.Setenv("OPENAI_API_KEY", "oa-key")
	t.Setenv("ANTHROPIC_API_KEY", "an-key")

	cases := []struct {
		baseURL string
		want    string
	}{
		{"https://openrouter.ai/api/v1", "or-key"},
		{"https://api.openai.com/v1", "oa-key"},
		{"https://api.anthropic.com/v1", "an-key"},
		{"http://localhost:11434/v1", ""},
	}

	for _, tc := range cases {
		if got := providerKeyFor(tc.baseURL); got != tc.want {
			t.Errorf("providerKeyFor(%q) = %q, want %q", tc.baseURL, got, tc.want)
		}
	}
}
