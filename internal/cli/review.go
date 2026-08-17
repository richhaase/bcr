package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/richhaase/bcr/internal/config"
	"github.com/richhaase/bcr/internal/diff"
	"github.com/richhaase/bcr/internal/domain"
	"github.com/richhaase/bcr/internal/github"
	"github.com/richhaase/bcr/internal/pipeline"
	"github.com/richhaase/bcr/internal/review"
	"github.com/richhaase/bcr/internal/terminal"
)

func newReviewCmd() *cobra.Command {
	var (
		baseRef         string
		prNum           int
		modelsFlag      string
		summarizerModel string
		yesFlag         bool
	)

	cmd := &cobra.Command{
		Use:   "review",
		Short: "Run parallel LLM code review on git diff",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if baseRef != "" {
				cfg.Base = baseRef
			}
			if modelsFlag != "" {
				var list []string
				for _, m := range strings.Split(modelsFlag, ",") {
					trimmed := strings.TrimSpace(m)
					if trimmed != "" {
						list = append(list, trimmed)
					}
				}
				if len(list) > 0 {
					cfg.Models = list
				}
			}
			if summarizerModel != "" {
				cfg.SummarizerModel = summarizerModel
			}

			var diffContent string
			if prNum > 0 {
				diffContent, err = diff.PRDiff(ctx, prNum)
			} else {
				diffContent, err = diff.GitDiff(ctx, cfg.Base)
			}
			if err != nil {
				return fmt.Errorf("failed getting diff: %w", err)
			}

			if strings.TrimSpace(diffContent) == "" {
				fmt.Fprintln(cmd.OutOrStdout(), "No changes detected in diff.")
				return nil
			}

			runner := pipeline.NewRunner(pipeline.Config{
				Models:          cfg.Models,
				SummarizerModel: cfg.SummarizerModel,
				BaseURL:         cfg.BaseURL,
				APIKey:          cfg.APIKey,
				Diff:            diffContent,
				Extra:           cfg.Extra,
				Temperature:     cfg.Temperature,
			})

			run, err := runner.Run(ctx)
			if err != nil {
				return fmt.Errorf("review failed: %w", err)
			}

			renderReport(cmd.OutOrStdout(), run)

			if prNum > 0 {
				if err := postReviewCmd(cmd, run, prNum, yesFlag); err != nil {
					return err
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&baseRef, "base", "b", "", "git base ref (default from config or main)")
	cmd.Flags().IntVarP(&prNum, "pr", "p", 0, "GitHub PR number to review via gh diff")
	cmd.Flags().StringVarP(&modelsFlag, "reviewers", "r", "", "comma-separated list of reviewer models")
	cmd.Flags().StringVarP(&summarizerModel, "summarizer", "s", "", "summarizer model")
	cmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "submit PR review non-interactively without prompting")

	return cmd
}

func postReviewCmd(cmd *cobra.Command, run *domain.ReviewRun, prNum int, yesFlag bool) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()

	repoStr, err := github.RepoName(ctx)
	if err != nil {
		return fmt.Errorf("resolve github repo: %w", err)
	}
	owner, repo := splitOwnerRepo(repoStr)

	prInfo, err := github.PRInfo(ctx, owner, repo, prNum)
	if err != nil {
		return fmt.Errorf("resolve pull request: %w", err)
	}

	currentUser, err := github.CurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("resolve current user: %w", err)
	}
	selfReview := prInfo.Author != "" && currentUser == prInfo.Author

	hasKept := review.HasKept(run)

	var ciOK bool
	if !selfReview && !hasKept {
		state, raw, derr := github.CIState(ctx, owner, repo, prNum)
		if derr != nil {
			slog.Warn("could not determine CI status; withholding approval", "err", derr)
		} else {
			slog.Debug("PR checks state", "state", state, "lines", raw)
			ciOK = state == "success"
		}
	}

	if yesFlag {
		event, reason := review.Disposition(run, ciOK, selfReview)
		if err := github.SubmitReview(ctx, owner, repo, prNum, event, review.PRBody(run)); err != nil {
			return err
		}
		slog.Info("posted PR review", "event", event, "reason", reason)
		return nil
	}

	return interactivePost(ctx, out, cmd.InOrStdin(), run, owner, repo, prNum, hasKept, ciOK, selfReview)
}

func interactivePost(ctx context.Context, out io.Writer, in io.Reader, run *domain.ReviewRun, owner, repo string, prNum int, hasKept, ciOK, selfReview bool) error {
	canApprove := ciOK && !selfReview

	var prompt string
	if hasKept {
		prompt = "Post review? [R]equest changes / [C]omment / [S]kip [R]: "
	} else {
		prompt = "Post LGTM? [A]pprove / [C]omment / [S]kip [A]: "
	}
	fmt.Fprint(out, prompt)

	scanner := bufio.NewScanner(in)
	if !scanner.Scan() {
		return scanner.Err()
	}
	choice := strings.ToUpper(strings.TrimSpace(scanner.Text()))

	var event string
	switch {
	case hasKept:
		switch choice {
		case "R", "":
			event = "request-changes"
		case "C":
			event = "comment"
		case "S":
			return nil
		default:
			fmt.Fprintln(out, "Invalid choice; skipping review post.")
			return nil
		}
	default:
		switch choice {
		case "A":
			if canApprove {
				event = "approve"
			} else {
				fmt.Fprintln(out, "Approval not allowed (self-review or CI not green); posting comment instead.")
				event = "comment"
			}
		case "C":
			event = "comment"
		case "S":
			return nil
		default:
			if canApprove {
				event = "approve"
			} else {
				event = "comment"
			}
		}
	}

	if err := github.SubmitReview(ctx, owner, repo, prNum, event, review.PRBody(run)); err != nil {
		return err
	}
	fmt.Fprintf(out, "Posted %s review to %s/%s#%d\n", event, owner, repo, prNum)
	return nil
}

func splitOwnerRepo(in string) (string, string) {
	parts := strings.Split(in, "/")
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], "/")
}

func renderReport(out io.Writer, run *domain.ReviewRun) {
	useColor := terminal.IsStdoutTTY()

	var kept []domain.FinalFinding
	for _, f := range run.Final {
		if f.Keep {
			kept = append(kept, f)
		}
	}

	if len(kept) == 0 {
		if useColor {
			fmt.Fprintln(out, "\033[32m✔ LGTM: No actionable defects found.\033[0m")
		} else {
			fmt.Fprintln(out, "✔ LGTM: No actionable defects found.")
		}
		if run.Dismissed > 0 {
			fmt.Fprintf(out, "(%d false positive / duplicate findings filtered out)\n", run.Dismissed)
		}
		return
	}

	fmt.Fprintf(out, "\nFound %d actionable finding(s) across %d models:\n\n", len(kept), len(run.Models))

	for i, f := range kept {
		sevColor := "\033[33m"
		if strings.ToLower(f.Severity) == "critical" || strings.ToLower(f.Severity) == "high" {
			sevColor = "\033[31m"
		}

		if useColor {
			fmt.Fprintf(out, "%d. %s[%s]\033[0m %s\n", i+1, sevColor, strings.ToUpper(f.Severity), f.Message)
		} else {
			fmt.Fprintf(out, "%d. [%s] %s\n", i+1, strings.ToUpper(f.Severity), f.Message)
		}

		fmt.Fprintf(out, "   File: %s:%d\n", f.File, f.Line)
		if f.Rule != "" {
			fmt.Fprintf(out, "   Rule: %s\n", f.Rule)
		}
		if f.Suggestion != "" {
			fmt.Fprintf(out, "   Fix:  %s\n", f.Suggestion)
		}
		if len(f.Agents) > 0 {
			fmt.Fprintf(out, "   Seen by: %s\n", strings.Join(f.Agents, ", "))
		}
		fmt.Fprintln(out)
	}

	if run.Dismissed > 0 {
		fmt.Fprintf(out, "(%d false positive / duplicate findings filtered out)\n\n", run.Dismissed)
	}
}
