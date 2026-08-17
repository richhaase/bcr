package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type PR struct {
	Owner  string
	Repo   string
	Number int
	Author string
	URL    string
}

func CurrentUser(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", "api", "user", "--jq", ".login")
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh api user: %s: %w", strings.TrimSpace(errOut.String()), err)
	}

	return strings.TrimSpace(out.String()), nil
}

func RepoName(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner")
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh repo view: %s: %w", strings.TrimSpace(errOut.String()), err)
	}

	return strings.TrimSpace(out.String()), nil
}

func PRInfo(ctx context.Context, owner, repo string, number int) (PR, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "view",
		fmt.Sprintf("%d", number), "--repo", owner+"/"+repo,
		"--json", "author,url,number", "--jq", "{author:.author.login,url:.url,number:.number}")
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return PR{}, fmt.Errorf("gh pr view: %s: %w", strings.TrimSpace(errOut.String()), err)
	}

	var parsed struct {
		Author string `json:"author"`
		URL    string `json:"url"`
		Number int    `json:"number"`
	}
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		return PR{}, fmt.Errorf("gh pr view parse: %w", err)
	}

	return PR{
		Owner:  owner,
		Repo:   repo,
		Number: parsed.Number,
		Author: parsed.Author,
		URL:    parsed.URL,
	}, nil
}

func PRHeadSHA(ctx context.Context, owner, repo string, number int) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "view",
		fmt.Sprintf("%d", number), "--repo", owner+"/"+repo,
		"--json", "headRefOid", "--jq", ".headRefOid")
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh pr view head: %s: %w", strings.TrimSpace(errOut.String()), err)
	}

	return strings.TrimSpace(out.String()), nil
}

func PROpen(ctx context.Context, owner, repo string, number int) (bool, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "view",
		fmt.Sprintf("%d", number), "--repo", owner+"/"+repo,
		"--json", "state", "--jq", ".state")
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return false, fmt.Errorf("gh pr view state: %s: %w", strings.TrimSpace(errOut.String()), err)
	}

	return strings.TrimSpace(out.String()) == "OPEN", nil
}

func CIState(ctx context.Context, owner, repo string, number int) (string, int, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "checks",
		fmt.Sprintf("%d", number), "--repo", owner+"/"+repo)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return "", 0, fmt.Errorf("gh pr checks: %s: %w", strings.TrimSpace(errOut.String()), err)
	}

	state, raw := ParseCheckState(out.String())
	return state, raw, nil
}

func ParseCheckState(output string) (string, int) {
	raw := 0
	header := true
	sawFailure := false
	sawPending := false

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		raw++
		if header {
			header = false
			continue
		}
		for _, field := range strings.Fields(trimmed) {
			switch strings.ToLower(field) {
			case "failure", "fail", "error", "canceled":
				sawFailure = true
			case "pending", "queued", "in_progress":
				sawPending = true
			}
		}
	}

	if raw <= 1 {
		return "unknown", raw
	}
	if sawFailure {
		return "failure", raw
	}
	if sawPending {
		return "pending", raw
	}
	return "success", raw
}

func validReviewEvent(event string) bool {
	switch event {
	case "request-changes", "comment", "approve":
		return true
	default:
		return false
	}
}

func SubmitReview(ctx context.Context, owner, repo string, number int, event, body string) error {
	if !validReviewEvent(event) {
		return fmt.Errorf("invalid review event %q (must be request-changes, comment, or approve)", event)
	}

	cmd := exec.CommandContext(ctx, "gh", "pr", "review",
		fmt.Sprintf("%d", number), "--repo", owner+"/"+repo,
		"--"+event, "--body", body)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh pr review: %s: %w", strings.TrimSpace(errOut.String()), err)
	}

	return nil
}
