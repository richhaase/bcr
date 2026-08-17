package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/richhaase/bcr/internal/domain"
)

func TestReviewReportsTokenAndCost(t *testing.T) {
	var buf bytes.Buffer
	run := &domain.ReviewRun{
		Models:           []string{"m1"},
		PromptTokens:     1200,
		CompletionTokens: 340,
		EstimatedCostUSD: 0.0023,
		Final: []domain.FinalFinding{{
			Rule:     "nil",
			Severity: "high",
			File:     "a.go",
			Line:     1,
			Message:  "nil deref",
			Keep:     true,
		}},
	}
	renderReport(&buf, run)
	out := buf.String()
	if !strings.Contains(out, "1200 prompt") {
		t.Errorf("expected prompt token count in report, got %q", out)
	}
	if !strings.Contains(out, "340 completion") {
		t.Errorf("expected completion token count in report, got %q", out)
	}
	if !strings.Contains(out, "Est Cost") {
		t.Errorf("expected estimated cost in report, got %q", out)
	}
}

func TestReviewReportsTokenAndCostLGTM(t *testing.T) {
	var buf bytes.Buffer
	run := &domain.ReviewRun{
		Models:           []string{"m1"},
		PromptTokens:     500,
		CompletionTokens: 80,
		EstimatedCostUSD: 0.0004,
	}
	renderReport(&buf, run)
	out := buf.String()
	if !strings.Contains(out, "500 prompt") {
		t.Errorf("expected prompt token count in LGTM report, got %q", out)
	}
	if !strings.Contains(out, "80 completion") {
		t.Errorf("expected completion token count in LGTM report, got %q", out)
	}
	if !strings.Contains(out, "Est Cost") {
		t.Errorf("expected estimated cost in LGTM report, got %q", out)
	}
}

func executeReview(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := NewRootCmd(BuildInfo{})
	root.SetArgs(args)
	var out bytes.Buffer
	var errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	err := root.Execute()
	return out.String(), errOut.String(), err
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func gitRepoWithCommit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init", "-b", "main")
	gitRun(t, dir, "config", "user.email", "test@example.com")
	gitRun(t, dir, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("line\n"), 0o600); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("change\n"), 0o600); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-m", "change")
	return dir
}

func isolateConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
}

func reviewServer(t *testing.T, content string) *httptest.Server {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"choices": []any{
			map[string]any{"message": map[string]any{"content": content}},
		},
		"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestReviewNoChanges(t *testing.T) {
	dir := gitRepoWithCommit(t)
	t.Chdir(dir)
	isolateConfig(t)

	out, _, err := executeReview(t, "review", "--base", "HEAD")
	if err != nil {
		t.Fatalf("review error: %v", err)
	}
	if !strings.Contains(out, "No changes detected in diff.") {
		t.Errorf("expected no-changes message, got %q", out)
	}
}

func TestReviewUnknownFlag(t *testing.T) {
	if _, _, err := executeReview(t, "review", "--no-such-flag"); err == nil {
		t.Fatal("expected error for unknown flag")
	}
}

func TestReviewNegativeFlagsIgnored(t *testing.T) {
	dir := gitRepoWithCommit(t)
	t.Chdir(dir)
	isolateConfig(t)

	out, _, err := executeReview(t, "review", "--base", "HEAD", "--concurrency", "-1", "--retries", "-2", "--exclude", "generated/")
	if err != nil {
		t.Fatalf("review error: %v", err)
	}
	if !strings.Contains(out, "No changes detected in diff.") {
		t.Errorf("expected no-changes message, got %q", out)
	}
}

func TestReviewInvalidGuidanceFile(t *testing.T) {
	dir := gitRepoWithCommit(t)
	t.Chdir(dir)
	isolateConfig(t)

	missing := filepath.Join(t.TempDir(), "missing.md")
	_, _, err := executeReview(t, "review", "--base", "HEAD", "--guidance-file", missing)
	if err == nil {
		t.Fatal("expected error for missing guidance file")
	}
	if !strings.Contains(err.Error(), "read guidance file") {
		t.Errorf("expected read guidance file error, got %v", err)
	}
}

func TestReviewEndToEndLGTM(t *testing.T) {
	dir := gitRepoWithCommit(t)
	t.Chdir(dir)
	isolateConfig(t)
	srv := reviewServer(t, `{"findings":[]}`)
	t.Setenv("BCR_BASE_URL", srv.URL)

	out, _, err := executeReview(t, "review", "--base", "HEAD~1", "--concurrency", "1")
	if err != nil {
		t.Fatalf("review error: %v", err)
	}
	if !strings.Contains(out, "LGTM: No actionable defects found.") {
		t.Errorf("expected LGTM output, got %q", out)
	}
}

func TestReviewExcludePatternBinding(t *testing.T) {
	dir := gitRepoWithCommit(t)
	t.Chdir(dir)
	isolateConfig(t)
	content := `{"findings":[{"rule":"generated-file","file":"generated/out.go","message":"generated code","severity":"info","line":1}]}`
	srv := reviewServer(t, content)
	t.Setenv("BCR_BASE_URL", srv.URL)

	out, _, err := executeReview(t, "review", "--base", "HEAD~1", "--exclude", "generated/")
	if err != nil {
		t.Fatalf("review error: %v", err)
	}
	if !strings.Contains(out, "LGTM: No actionable defects found.") {
		t.Errorf("expected LGTM output after exclusion, got %q", out)
	}
	if !strings.Contains(out, "excluded by regex patterns") {
		t.Errorf("expected excluded-by-patterns message, got %q", out)
	}
}

func TestReviewRetriesBinding(t *testing.T) {
	dir := gitRepoWithCommit(t)
	t.Chdir(dir)
	isolateConfig(t)
	payload, err := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": `{"findings":[]}`}}},
		"usage":   map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	var mu sync.Mutex
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		first := count == 1
		mu.Unlock()
		if first {
			http.Error(w, "overloaded", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	t.Setenv("BCR_BASE_URL", srv.URL)

	out, _, err := executeReview(t, "review", "--base", "HEAD~1", "--retries", "1")
	if err != nil {
		t.Fatalf("review error: %v", err)
	}
	if !strings.Contains(out, "LGTM: No actionable defects found.") {
		t.Errorf("expected LGTM output after retry, got %q", out)
	}
}
