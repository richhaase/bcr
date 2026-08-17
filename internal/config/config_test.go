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
	t.Setenv("BCR_API_KEY", "test-key-123")
	t.Setenv("BCR_MODELS", "model-one,model-two")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if cfg.APIKey != "test-key-123" {
		t.Errorf("expected APIKey override, got %s", cfg.APIKey)
	}

	if len(cfg.Models) != 2 || cfg.Models[0] != "model-one" {
		t.Errorf("expected models override, got %v", cfg.Models)
	}
}
