package github

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseCheckState(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantState string
		wantRaw   int
	}{
		{
			name: "success",
			output: "NAME  DESCRIPTION STATE VIEW\n" +
				"ci/unit  CI Steps success nil\n" +
				"ci/lint  Lint  success nil\n",
			wantState: "success",
			wantRaw:   3,
		},
		{
			name: "failure",
			output: "NAME  DESCRIPTION STATE VIEW\n" +
				"ci/build  Build  failure nil\n",
			wantState: "failure",
			wantRaw:   2,
		},
		{
			name: "pending",
			output: "NAME  DESCRIPTION STATE VIEW\n" +
				"ci/test  Tests  pending nil\n",
			wantState: "pending",
			wantRaw:   2,
		},
		{
			name:      "empty",
			output:    "",
			wantState: "unknown",
			wantRaw:   0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state, raw := ParseCheckState(tc.output)
			if state != tc.wantState {
				t.Errorf("ParseCheckState() state = %q, want %q", state, tc.wantState)
			}
			if raw != tc.wantRaw {
				t.Errorf("ParseCheckState() raw = %d, want %d", raw, tc.wantRaw)
			}
		})
	}
}

func TestValidReviewEvent(t *testing.T) {
	valid := []string{"request-changes", "comment", "approve"}
	for _, e := range valid {
		if !validReviewEvent(e) {
			t.Errorf("expected %q to be a valid review event", e)
		}
	}

	invalid := []string{"", "Approve", "reject", "lgtm", "request_changes"}
	for _, e := range invalid {
		if validReviewEvent(e) {
			t.Errorf("expected %q to be an invalid review event", e)
		}
	}
}

const fakeGHScript = `#!/bin/sh
if [ -n "$FAKE_GH_EXIT" ]; then
	echo "fatal: forced command failure" >&2
	exit 1
fi
case "$1" in
	api)
		echo "octocat"
		exit 0
		;;
	repo)
		echo "acme/widget"
		exit 0
		;;
	pr)
		case "$2" in
			view)
				json=""
				for a in "$@"; do
					if [ "$json" = "pending" ]; then
						json="$a"
						break
					fi
					[ "$a" = "--json" ] && json="pending"
				done
				case "$json" in
					author,url,number)
						printf '%s\n' "$FAKE_GH_PR_JSON"
						;;
					headRefOid)
						echo "  abcdef123456  "
						;;
					state)
						echo "${FAKE_GH_STATE:-OPEN}"
						;;
					*)
						echo ""
						;;
				esac
				exit 0
				;;
			checks)
				echo "NAME STATUS"
				echo "ci/build success"
				exit 0
				;;
			review)
				exit 0
				;;
			*)
				exit 1
				;;
		esac
		;;
	*)
		;;
esac
echo "unhandled gh invocation: $*" >&2
exit 1
`

func withFakeGH(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "gh")
	if err := os.WriteFile(path, []byte(fakeGHScript), 0o600); err != nil {
		t.Fatalf("write fake gh: %v", err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod fake gh: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCurrentUser(t *testing.T) {
	withFakeGH(t)
	got, err := CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser error: %v", err)
	}
	if got != "octocat" {
		t.Errorf("CurrentUser = %q, want octocat", got)
	}
}

func TestCurrentUserError(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKE_GH_EXIT", "1")
	if _, err := CurrentUser(context.Background()); err == nil {
		t.Fatal("expected error from CurrentUser")
	} else if !strings.Contains(err.Error(), "gh api user") {
		t.Errorf("expected gh api user in error, got %v", err)
	}
}

func TestRepoName(t *testing.T) {
	withFakeGH(t)
	got, err := RepoName(context.Background())
	if err != nil {
		t.Fatalf("RepoName error: %v", err)
	}
	if got != "acme/widget" {
		t.Errorf("RepoName = %q, want acme/widget", got)
	}
}

func TestRepoNameError(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKE_GH_EXIT", "1")
	if _, err := RepoName(context.Background()); err == nil {
		t.Fatal("expected error from RepoName")
	} else if !strings.Contains(err.Error(), "gh repo view") {
		t.Errorf("expected gh repo view in error, got %v", err)
	}
}

func TestPRInfo(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKE_GH_PR_JSON", `{"author":"alice","url":"https://example.com/pull/7","number":7}`)
	pr, err := PRInfo(context.Background(), "acme", "widget", 7)
	if err != nil {
		t.Fatalf("PRInfo error: %v", err)
	}
	if pr.Owner != "acme" || pr.Repo != "widget" {
		t.Errorf("PR owner/repo = %s/%s, want acme/widget", pr.Owner, pr.Repo)
	}
	if pr.Number != 7 {
		t.Errorf("PR Number = %d, want 7", pr.Number)
	}
	if pr.Author != "alice" {
		t.Errorf("PR Author = %q, want alice", pr.Author)
	}
	if pr.URL != "https://example.com/pull/7" {
		t.Errorf("PR URL = %q, want example URL", pr.URL)
	}
}

func TestPRInfoMalformedJSON(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKE_GH_PR_JSON", "not json")
	if _, err := PRInfo(context.Background(), "acme", "widget", 7); err == nil {
		t.Fatal("expected parse error from PRInfo")
	} else if !strings.Contains(err.Error(), "gh pr view parse") {
		t.Errorf("expected parse error prefix, got %v", err)
	}
}

func TestPRInfoError(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKE_GH_EXIT", "1")
	if _, err := PRInfo(context.Background(), "acme", "widget", 7); err == nil {
		t.Fatal("expected error from PRInfo")
	} else if !strings.Contains(err.Error(), "gh pr view:") {
		t.Errorf("expected gh pr view in error, got %v", err)
	}
}

func TestPRHeadSHA(t *testing.T) {
	withFakeGH(t)
	sha, err := PRHeadSHA(context.Background(), "acme", "widget", 7)
	if err != nil {
		t.Fatalf("PRHeadSHA error: %v", err)
	}
	if sha != "abcdef123456" {
		t.Errorf("PRHeadSHA = %q, want abcdef123456", sha)
	}
}

func TestPRHeadSHAError(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKE_GH_EXIT", "1")
	if _, err := PRHeadSHA(context.Background(), "acme", "widget", 7); err == nil {
		t.Fatal("expected error from PRHeadSHA")
	} else if !strings.Contains(err.Error(), "gh pr view head") {
		t.Errorf("expected gh pr view head in error, got %v", err)
	}
}

func TestPROpenState(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  bool
	}{
		{"open", "OPEN", true},
		{"closed", "CLOSED", false},
		{"merged", "MERGED", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			withFakeGH(t)
			t.Setenv("FAKE_GH_STATE", tc.state)
			got, err := PROpen(context.Background(), "acme", "widget", 7)
			if err != nil {
				t.Fatalf("PROpen error: %v", err)
			}
			if got != tc.want {
				t.Errorf("PROpen = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPROpenError(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKE_GH_EXIT", "1")
	if _, err := PROpen(context.Background(), "acme", "widget", 7); err == nil {
		t.Fatal("expected error from PROpen")
	} else if !strings.Contains(err.Error(), "gh pr view state") {
		t.Errorf("expected gh pr view state in error, got %v", err)
	}
}

func TestSubmitReviewInvalidEvent(t *testing.T) {
	tests := []string{"", "Approve", "reject", "request_changes"}
	for _, e := range tests {
		err := SubmitReview(context.Background(), "acme", "widget", 7, e, "body")
		if err == nil {
			t.Errorf("expected error for invalid event %q", e)
		} else if !strings.Contains(err.Error(), "invalid review event") {
			t.Errorf("expected invalid review event in error, got %v", err)
		}
	}
}

func TestSubmitReview(t *testing.T) {
	withFakeGH(t)
	for _, e := range []string{"request-changes", "comment", "approve"} {
		if err := SubmitReview(context.Background(), "acme", "widget", 7, e, "review body"); err != nil {
			t.Errorf("SubmitReview(%q) error: %v", e, err)
		}
	}
}

func TestSubmitReviewError(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKE_GH_EXIT", "1")
	err := SubmitReview(context.Background(), "acme", "widget", 7, "approve", "body")
	if err == nil {
		t.Fatal("expected error from SubmitReview")
	} else if !strings.Contains(err.Error(), "gh pr review") {
		t.Errorf("expected gh pr review in error, got %v", err)
	}
}

func TestCIState(t *testing.T) {
	withFakeGH(t)
	state, raw, err := CIState(context.Background(), "acme", "widget", 7)
	if err != nil {
		t.Fatalf("CIState error: %v", err)
	}
	if state != "success" {
		t.Errorf("CIState state = %q, want success", state)
	}
	if raw != 2 {
		t.Errorf("CIState raw = %d, want 2", raw)
	}
}

func TestCIStateError(t *testing.T) {
	withFakeGH(t)
	t.Setenv("FAKE_GH_EXIT", "1")
	if _, _, err := CIState(context.Background(), "acme", "widget", 7); err == nil {
		t.Fatal("expected error from CIState")
	} else if !strings.Contains(err.Error(), "gh pr checks") {
		t.Errorf("expected gh pr checks in error, got %v", err)
	}
}
