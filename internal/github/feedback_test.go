package github

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFetchDiscussion(t *testing.T) {
	prev := runDiscussionCmd
	defer func() { runDiscussionCmd = prev }()
	runDiscussionCmd = func(_ context.Context, args ...string) (string, error) {
		switch args[0] {
		case "pr":
			return "This PR fixes the timeout.", nil
		case "api":
			if strings.Contains(args[1], "/issues/") {
				return "The timeout is intentional.\n\nPlease document it.", nil
			}
			return "No leak here.\n", nil
		}
		return "", nil
	}

	out, err := FetchDiscussion(context.Background(), "acme", "widget", 7)
	if err != nil {
		t.Fatalf("FetchDiscussion error: %v", err)
	}

	if !strings.Contains(out, "PR Description:") {
		t.Errorf("expected PR description section, got %q", out)
	}
	if !strings.Contains(out, "This PR fixes the timeout.") {
		t.Errorf("expected PR description body, got %q", out)
	}
	if !strings.Contains(out, "The timeout is intentional.") {
		t.Errorf("expected issue comment, got %q", out)
	}
	if !strings.Contains(out, "No leak here.") {
		t.Errorf("expected review comment, got %q", out)
	}
}

func TestFetchDiscussionFormatsEmptyComments(t *testing.T) {
	prev := runDiscussionCmd
	defer func() { runDiscussionCmd = prev }()
	runDiscussionCmd = func(_ context.Context, args ...string) (string, error) {
		if args[0] == "pr" {
			return "Only a description.", nil
		}
		return "", nil
	}

	out, err := FetchDiscussion(context.Background(), "acme", "widget", 7)
	if err != nil {
		t.Fatalf("FetchDiscussion error: %v", err)
	}

	if strings.Contains(out, "Comments:") {
		t.Errorf("did not expect comments section, got %q", out)
	}
	if !strings.Contains(out, "Only a description.") {
		t.Errorf("expected description preserved, got %q", out)
	}
}

func TestFetchDiscussionGracefulDegradation(t *testing.T) {
	prev := runDiscussionCmd
	defer func() { runDiscussionCmd = prev }()
	runDiscussionCmd = func(context.Context, ...string) (string, error) {
		return "", errors.New("gh unavailable")
	}

	out, err := FetchDiscussion(context.Background(), "acme", "widget", 7)
	if err != nil {
		t.Fatalf("expected graceful degradation without error, got %v", err)
	}
	if out != "" {
		t.Errorf("expected empty string on subprocess error, got %q", out)
	}
}

func TestFetchDiscussionEmptyIsGraceful(t *testing.T) {
	prev := runDiscussionCmd
	defer func() { runDiscussionCmd = prev }()
	runDiscussionCmd = func(_ context.Context, args ...string) (string, error) {
		return "", nil
	}

	out, err := FetchDiscussion(context.Background(), "acme", "widget", 7)
	if err != nil {
		t.Fatalf("FetchDiscussion error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty discussion, got %q", out)
	}
}
