package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestConfigPRFeedbackDisabledByEnv(t *testing.T) {
	t.Setenv("BCR_PR_FEEDBACK", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.PRFeedback {
		t.Errorf("expected PRFeedback disabled by env, got true")
	}
}

func TestConfigPRFeedbackDefaultEnabled(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !cfg.PRFeedback {
		t.Errorf("expected PRFeedback enabled by default, got false")
	}
}

func TestConfigPRFeedbackEnabledByEnv(t *testing.T) {
	t.Setenv("BCR_PR_FEEDBACK", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !cfg.PRFeedback {
		t.Errorf("expected PRFeedback enabled via env, got false")
	}
}

func TestConfigExcludeEnvOverride(t *testing.T) {
	t.Setenv("BCR_EXCLUDE", "generated/,TODO")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(cfg.Exclude) != 2 || cfg.Exclude[0] != "generated/" || cfg.Exclude[1] != "TODO" {
		t.Errorf("expected exclude list override, got %v", cfg.Exclude)
	}
}

func TestConfigExcludeDefaultEmpty(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(cfg.Exclude) != 0 {
		t.Errorf("expected empty exclude list by default, got %v", cfg.Exclude)
	}
}

func TestConfigGuidanceFields(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".bcr.yaml")
	err := os.WriteFile(configPath, []byte("guidance: follow the project style guide\nguidance_file: ./docs/review.md\n"), 0o600)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Chdir(dir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Guidance != "follow the project style guide" {
		t.Errorf("expected guidance parsed from YAML, got %q", cfg.Guidance)
	}
	if cfg.GuidanceFile != "./docs/review.md" {
		t.Errorf("expected guidance_file parsed from YAML, got %q", cfg.GuidanceFile)
	}
}

func TestGuidanceEnvOverride(t *testing.T) {
	t.Setenv("BCR_GUIDANCE", "env guidance")
	t.Setenv("BCR_GUIDANCE_FILE", "/tmp/env-guide.md")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Guidance != "env guidance" {
		t.Errorf("expected env guidance, got %q", cfg.Guidance)
	}
	if cfg.GuidanceFile != "/tmp/env-guide.md" {
		t.Errorf("expected env guidance_file, got %q", cfg.GuidanceFile)
	}
}

func TestResolveGuidanceEmptyWhenUnset(t *testing.T) {
	cfg := &Config{}
	got, err := cfg.ResolveGuidance()
	if err != nil {
		t.Fatalf("ResolveGuidance error: %v", err)
	}
	if got != "" {
		t.Errorf("expected empty guidance, got %q", got)
	}
}

func TestResolveGuidanceInlineAndFile(t *testing.T) {
	dir := t.TempDir()
	guide := filepath.Join(dir, "guide.md")
	if err := os.WriteFile(guide, []byte("# Review Guide\n\nPrefer explicit error handling.\n"), 0o600); err != nil {
		t.Fatalf("write guide: %v", err)
	}

	cfg := &Config{
		Guidance:     "Focus on correctness.",
		GuidanceFile: guide,
	}
	got, err := cfg.ResolveGuidance()
	if err != nil {
		t.Fatalf("ResolveGuidance error: %v", err)
	}

	if !strings.Contains(got, "# Review Guide") {
		t.Errorf("expected file content in resolved guidance, got %q", got)
	}
	if !strings.Contains(got, "Focus on correctness.") {
		t.Errorf("expected inline guidance in resolved guidance, got %q", got)
	}
	if !strings.Contains(got, "\n") {
		t.Errorf("expected file and inline guidance combined on separate lines, got %q", got)
	}
}

func TestResolveGuidanceMissingFileFails(t *testing.T) {
	cfg := &Config{GuidanceFile: filepath.Join(t.TempDir(), "does-not-exist.md")}
	_, err := cfg.ResolveGuidance()
	if err == nil {
		t.Fatal("expected error for missing guidance file")
	}
	if !strings.Contains(err.Error(), "guidance file") {
		t.Errorf("expected descriptive error mentioning guidance file, got %q", err)
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
