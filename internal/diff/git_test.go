package diff

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", msg)
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	return dir
}

func chdirTo(t *testing.T, dir string) {
	t.Helper()
	t.Chdir(dir)
}

func TestGitDiffStandardCommit(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "one.txt", "one\n")
	commitAll(t, dir, "first")
	writeFile(t, dir, "two.txt", "two\n")
	commitAll(t, dir, "second")
	chdirTo(t, dir)

	out, err := GitDiff(context.Background(), "HEAD~1")
	if err != nil {
		t.Fatalf("GitDiff error: %v", err)
	}
	if !strings.Contains(out, "two.txt") {
		t.Errorf("expected diff to include two.txt, got:\n%s", out)
	}
	if !strings.Contains(out, "+two") {
		t.Errorf("expected diff to include added content, got:\n%s", out)
	}
}

func TestGitDiffNamedBranch(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "base.txt", "base\n")
	commitAll(t, dir, "main commit")
	runGit(t, dir, "checkout", "-b", "feature")
	writeFile(t, dir, "feat.txt", "feat\n")
	commitAll(t, dir, "feature commit")
	chdirTo(t, dir)

	out, err := GitDiff(context.Background(), "main")
	if err != nil {
		t.Fatalf("GitDiff error: %v", err)
	}
	if !strings.Contains(out, "feat.txt") {
		t.Errorf("expected diff against branch to include feat.txt, got:\n%s", out)
	}
	if strings.Contains(out, "base.txt") {
		t.Errorf("expected diff against branch to exclude base.txt, got:\n%s", out)
	}
}

func TestGitDiffEmpty(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "one.txt", "one\n")
	commitAll(t, dir, "first")
	chdirTo(t, dir)

	out, err := GitDiff(context.Background(), "HEAD")
	if err != nil {
		t.Fatalf("GitDiff error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty diff, got:\n%s", out)
	}
}

func TestGitDiffMergeBaseFallback(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "a.txt", "aaa\n")
	commitAll(t, dir, "root a")
	runGit(t, dir, "checkout", "--orphan", "alt")
	runGit(t, dir, "rm", "-rf", ".")
	writeFile(t, dir, "b.txt", "bbb\n")
	commitAll(t, dir, "root b")
	chdirTo(t, dir)

	out, err := GitDiff(context.Background(), "main")
	if err != nil {
		t.Fatalf("GitDiff error: %v", err)
	}
	if !strings.Contains(out, "b.txt") {
		t.Errorf("expected fallback diff to include b.txt, got:\n%s", out)
	}
}
