package github

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const maxDiscussionChars = 8000

type PRDiscussion struct {
	Body     string
	Comments []string
}

type discussionRunner func(ctx context.Context, args ...string) (string, error)

var runDiscussionCmd discussionRunner = func(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh: %s: %w", strings.TrimSpace(errOut.String()), err)
	}

	return out.String(), nil
}

func FetchDiscussion(ctx context.Context, owner, repo string, number int) (string, error) {
	res, err := fetchDiscussion(ctx, owner, repo, number, runDiscussionCmd)
	if err != nil {
		return "", nil
	}
	return res, nil
}

func fetchDiscussion(ctx context.Context, owner, repo string, number int, runner discussionRunner) (string, error) {
	prNum := fmt.Sprintf("%d", number)
	repoStr := owner + "/" + repo

	body, err := runner(ctx, "pr", "view", prNum, "--repo", repoStr, "--json", "body", "--jq", ".body")
	if err != nil {
		return "", fmt.Errorf("fetch PR description: %w", err)
	}

	issueComments, err := runner(ctx, "api", "repos/"+repoStr+"/issues/"+prNum+"/comments", "--jq", ".[].body")
	if err != nil {
		return "", fmt.Errorf("fetch issue comments: %w", err)
	}

	reviewComments, err := runner(ctx, "api", "repos/"+repoStr+"/pulls/"+prNum+"/comments", "--jq", ".[].body")
	if err != nil {
		return "", fmt.Errorf("fetch review comments: %w", err)
	}

	comments := collectComments(issueComments, reviewComments)
	if strings.TrimSpace(body) == "" && len(comments) == 0 {
		return "", nil
	}

	return renderDiscussion(PRDiscussion{
		Body:     strings.TrimSpace(body),
		Comments: comments,
	}), nil
}

func collectComments(collections ...string) []string {
	var comments []string
	for _, collection := range collections {
		for _, line := range strings.Split(collection, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				comments = append(comments, trimmed)
			}
		}
	}
	return comments
}

func renderDiscussion(d PRDiscussion) string {
	var builder strings.Builder
	if d.Body != "" {
		fmt.Fprintf(&builder, "PR Description:\n%s", d.Body)
	}
	if len(d.Comments) > 0 {
		if builder.Len() > 0 {
			builder.WriteString("\n\n")
		}
		fmt.Fprintf(&builder, "Comments:\n- %s", strings.Join(d.Comments, "\n- "))
	}
	return truncateDiscussion(builder.String())
}

func truncateDiscussion(s string) string {
	if len(s) <= maxDiscussionChars {
		return s
	}
	return s[:maxDiscussionChars]
}
