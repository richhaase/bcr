package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/richhaase/bcr/internal/config"
	"github.com/richhaase/bcr/internal/diff"
	"github.com/richhaase/bcr/internal/domain"
	"github.com/richhaase/bcr/internal/github"
	"github.com/richhaase/bcr/internal/pipeline"
	"github.com/richhaase/bcr/internal/watch"
)

type ghAdapter struct {
	owner string
	repo  string
	pr    int
}

func (g ghAdapter) Head(ctx context.Context) (string, bool, error) {
	sha, err := github.PRHeadSHA(ctx, g.owner, g.repo, g.pr)
	if err != nil {
		return "", false, err
	}
	open, err := github.PROpen(ctx, g.owner, g.repo, g.pr)
	if err != nil {
		return "", false, err
	}
	return sha, open, nil
}

func (g ghAdapter) CheckState(ctx context.Context) (string, error) {
	state, _, err := github.CIState(ctx, g.owner, g.repo, g.pr)
	if err != nil {
		return "", err
	}
	return state, nil
}

func (g ghAdapter) Submit(ctx context.Context, event, body string) error {
	return github.SubmitReview(ctx, g.owner, g.repo, g.pr, event, body)
}

func parseDuration(raw string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return d
}

func newWatchCmd() *cobra.Command {
	var (
		prNum        int
		postMode     string
		pollInterval string
		settleTime   string
		maxReviews   int
		maxDuration  string
	)

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Poll and re-review a pull request until it is clean or safety bounds are hit",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if prNum <= 0 {
				return fmt.Errorf("watch requires a positive --pr value")
			}
			if postMode != string(watch.PostModeComment) && postMode != string(watch.PostModeApprove) {
				return fmt.Errorf("invalid --post-mode %q (must be comment or approve)", postMode)
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			repoStr, err := github.RepoName(ctx)
			if err != nil {
				return fmt.Errorf("resolve github repo: %w", err)
			}
			owner, repo := splitOwnerRepo(repoStr)

			watchCfg := watch.Config{
				PR:           prNum,
				MaxReviews:   maxReviews,
				PostMode:     watch.PostMode(postMode),
				PollInterval: parseDuration(pollInterval, time.Minute),
				SettleTime:   parseDuration(settleTime, 10*time.Minute),
				MaxDuration:  parseDuration(maxDuration, 12*time.Hour),
			}

			reviewer := watch.ReviewFunc(func(rctx context.Context, _ string) (*domain.ReviewRun, error) {
				diffContent, diffErr := diff.PRDiff(rctx, prNum)
				if diffErr != nil {
					return nil, fmt.Errorf("failed getting diff: %w", diffErr)
				}

				var feedback string
				if cfg.PRFeedback {
					feedback, _ = github.FetchDiscussion(rctx, owner, repo, prNum)
				}

				runner := pipeline.NewRunner(pipeline.Config{
					Models:          cfg.Models,
					SummarizerModel: cfg.SummarizerModel,
					BaseURL:         cfg.BaseURL,
					APIKey:          cfg.APIKey,
					Diff:            diffContent,
					Extra:           cfg.Extra,
					Temperature:     cfg.Temperature,
					Concurrency:     cfg.Concurrency,
					Retries:         cfg.Retries,
					Feedback:        feedback,
					ExcludePatterns: cfg.Exclude,
				})
				run, runErr := runner.Run(rctx)
				if runErr != nil {
					return nil, fmt.Errorf("review failed: %w", runErr)
				}
				return run, nil
			})

			watcher := watch.New(watchCfg, watch.NewClock(), ghAdapter{owner: owner, repo: repo, pr: prNum}, reviewer)
			return watcher.Run(ctx)
		},
	}

	cmd.Flags().IntVarP(&prNum, "pr", "p", 0, "GitHub PR number to watch")
	cmd.Flags().StringVar(&postMode, "post-mode", "comment", "post mode: comment or approve")
	cmd.Flags().StringVar(&pollInterval, "poll-interval", "1m", "how often to poll for PR changes")
	cmd.Flags().StringVar(&settleTime, "settle-time", "10m", "quiet period after new commits before re-review")
	cmd.Flags().IntVar(&maxReviews, "max-reviews", 15, "maximum reviews before giving up")
	cmd.Flags().StringVar(&maxDuration, "max-duration", "12h", "maximum watch duration")

	return cmd
}
