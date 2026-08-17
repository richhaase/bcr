package port

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richhaase/bcr/internal/config"
)

func TestPortDetectsAndGenerates(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, ".acr.yaml"), "base: trunk\nconcurrency: 4\nretries: 5\npr_feedback: false\n")
	t.Chdir(dir)

	path, err := FindACRConfig()
	if err != nil {
		t.Fatalf("FindACRConfig error: %v", err)
	}
	if path == "" {
		t.Fatal("expected .acr.yaml to be detected")
	}

	res, err := Port(path)
	if err != nil {
		t.Fatalf("Port error: %v", err)
	}

	target := ".bcr.yaml"
	if err := Write(target, res.Config, false); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read generated config: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "concurrency: 4") {
		t.Errorf("expected concurrency in generated config, got %q", out)
	}
	if !strings.Contains(out, "pr_feedback: false") {
		t.Errorf("expected pr_feedback in generated config, got %q", out)
	}
}

func TestPortDirectFields(t *testing.T) {
	dir := t.TempDir()
	acrPath := filepath.Join(dir, ".acr.yaml")
	writeFile(t, acrPath, `
base: develop
concurrency: 6
retries: 7
pr_feedback: true
guidance_file: ./docs/review.md
filters:
  exclude_patterns:
    - "generated/(.*)"
    - "vendor/(.*)"
`)
	t.Chdir(dir)

	res, err := Port(acrPath)
	if err != nil {
		t.Fatalf("Port error: %v", err)
	}
	cfg := res.Config
	if cfg.Base != "develop" {
		t.Errorf("expected base develop, got %q", cfg.Base)
	}
	if cfg.Concurrency != 6 {
		t.Errorf("expected concurrency 6, got %d", cfg.Concurrency)
	}
	if cfg.Retries != 7 {
		t.Errorf("expected retries 7, got %d", cfg.Retries)
	}
	if !cfg.PRFeedback {
		t.Error("expected pr_feedback true")
	}
	if cfg.GuidanceFile != "./docs/review.md" {
		t.Errorf("expected guidance_file ./docs/review.md, got %q", cfg.GuidanceFile)
	}
	if len(cfg.Exclude) != 2 || cfg.Exclude[0] != "generated/(.*)" || cfg.Exclude[1] != "vendor/(.*)" {
		t.Errorf("expected exclude patterns ported, got %v", cfg.Exclude)
	}
}

func TestPortWarnsUnsupported(t *testing.T) {
	dir := t.TempDir()
	acrPath := filepath.Join(dir, ".acr.yaml")
	writeFile(t, acrPath, `
timeout: 30
fetch: 5
summarizer_timeout: 60
fp_filter_timeout: 10
fp_filter:
  threshold: 0.5
`)
	t.Chdir(dir)

	res, err := Port(acrPath)
	if err != nil {
		t.Fatalf("Port error: %v", err)
	}
	joined := strings.Join(res.Warnings, "\n")
	for _, want := range []string{"timeout", "fetch", "summarizer_timeout", "fp_filter_timeout", "fp_filter.threshold"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected warning mentioning %q, got %q", want, joined)
		}
	}
}

func TestPortAgentMapping(t *testing.T) {
	dir := t.TempDir()
	acrPath := filepath.Join(dir, ".acr.yaml")
	writeFile(t, acrPath, `
reviewer_agents:
  - claude
  - codex
  - custom-agent
`)
	t.Chdir(dir)

	res, err := Port(acrPath)
	if err != nil {
		t.Fatalf("Port error: %v", err)
	}
	want := []string{"anthropic/claude-sonnet-4-5", "openai/gpt-5.3-codex"}
	if len(res.Config.Models) != len(want) {
		t.Fatalf("expected models %v, got %v", want, res.Config.Models)
	}
	for i, m := range want {
		if res.Config.Models[i] != m {
			t.Errorf("expected models[%d] = %s, got %s", i, m, res.Config.Models[i])
		}
	}
	if !strings.Contains(strings.Join(res.Warnings, "\n"), "custom-agent") {
		t.Errorf("expected warning for unmapped agent, got %v", res.Warnings)
	}
}

func TestPortSummarizerAgentMapping(t *testing.T) {
	dir := t.TempDir()
	acrPath := filepath.Join(dir, ".acr.yaml")
	writeFile(t, acrPath, `
summarizer_agent: agy
`)
	t.Chdir(dir)

	res, err := Port(acrPath)
	if err != nil {
		t.Fatalf("Port error: %v", err)
	}
	if res.Config.SummarizerModel != "google/gemini-2.5-pro" {
		t.Errorf("expected gemini summarizer model, got %q", res.Config.SummarizerModel)
	}
}

func TestWriteRefusesOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, ".bcr.yaml")
	writeFile(t, target, "existing: true\n")

	if err := Write(target, config.DefaultConfig(), false); !errors.Is(err, ErrExists) {
		t.Fatalf("expected ErrExists without force, got %v", err)
	}

	if err := Write(target, config.DefaultConfig(), true); err != nil {
		t.Fatalf("Write with force error: %v", err)
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if strings.Contains(string(data), "existing: true") {
		t.Errorf("expected overwrite, got %q", string(data))
	}
}

func TestFindACRConfigMissing(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))

	path, err := FindACRConfig()
	if err != nil {
		t.Fatalf("FindACRConfig error: %v", err)
	}
	if path != "" {
		t.Errorf("expected empty path when no .acr.yaml, got %q", path)
	}
}

func TestFindACRConfigGlobalDir(t *testing.T) {
	dir := t.TempDir()
	xdg := filepath.Join(dir, "xdg")
	t.Chdir(dir)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	writeFile(t, filepath.Join(xdg, "acr", "config.yaml"), "base: main\n")

	path, err := FindACRConfig()
	if err != nil {
		t.Fatalf("FindACRConfig error: %v", err)
	}
	if path == "" {
		t.Fatal("expected acr config dir lookup to succeed")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
