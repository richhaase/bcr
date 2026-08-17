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
