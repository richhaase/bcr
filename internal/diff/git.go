package diff

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

func GitDiff(ctx context.Context, baseRef string) (string, error) {
	if baseRef == "" {
		baseRef = "HEAD~1"
	}

	mergeBaseCmd := exec.CommandContext(ctx, "git", "merge-base", baseRef, "HEAD")
	var mbOut bytes.Buffer
	mergeBaseCmd.Stdout = &mbOut
	if err := mergeBaseCmd.Run(); err == nil {
		mb := strings.TrimSpace(mbOut.String())
		if mb != "" {
			return runDiff(ctx, mb, "HEAD")
		}
	}

	return runDiff(ctx, baseRef, "HEAD")
}

func PRDiff(ctx context.Context, prNumber int) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "diff", fmt.Sprintf("%d", prNumber))
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("gh pr diff: %s: %w", strings.TrimSpace(errOut.String()), err)
	}

	return out.String(), nil
}

func runDiff(ctx context.Context, from, to string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", from+"..."+to)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		cmdFallback := exec.CommandContext(ctx, "git", "diff", from, to)
		var outFb bytes.Buffer
		cmdFallback.Stdout = &outFb
		if errFb := cmdFallback.Run(); errFb == nil {
			return outFb.String(), nil
		}
		return "", fmt.Errorf("git diff: %s: %w", strings.TrimSpace(errOut.String()), err)
	}

	return out.String(), nil
}
