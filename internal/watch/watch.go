package watch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/richhaase/bcr/internal/domain"
	"github.com/richhaase/bcr/internal/review"
)

type PostMode string

const (
	PostModeComment PostMode = "comment"
	PostModeApprove PostMode = "approve"
)

var (
	ErrReviewLimitReached = errors.New("maximum review count reached without terminal LGTM")
	ErrDurationLimitHit   = errors.New("maximum watch duration reached without terminal LGTM")
)

type Config struct {
	PR           int
	PollInterval time.Duration
	SettleTime   time.Duration
	MaxReviews   int
	MaxDuration  time.Duration
	PostMode     PostMode
}

func (c Config) normalize() Config {
	if c.PollInterval <= 0 {
		c.PollInterval = time.Minute
	}
	if c.MaxReviews <= 0 {
		c.MaxReviews = 15
	}
	if c.MaxDuration <= 0 {
		c.MaxDuration = 12 * time.Hour
	}
	if c.PostMode == "" {
		c.PostMode = PostModeComment
	}
	return c
}

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func NewClock() Clock {
	return realClock{}
}

func (realClock) Now() time.Time {
	return time.Now()
}

type GitHub interface {
	Head(ctx context.Context) (string, bool, error)
	CheckState(ctx context.Context) (string, error)
	Submit(ctx context.Context, event, body string) error
}

type Reviewer interface {
	Review(ctx context.Context, headSHA string) (*domain.ReviewRun, error)
}

type ReviewFunc func(ctx context.Context, headSHA string) (*domain.ReviewRun, error)

func (f ReviewFunc) Review(ctx context.Context, headSHA string) (*domain.ReviewRun, error) {
	return f(ctx, headSHA)
}

type Watcher struct {
	cfg      Config
	clock    Clock
	gh       GitHub
	reviewer Reviewer
}

func New(cfg Config, clock Clock, gh GitHub, reviewer Reviewer) *Watcher {
	return &Watcher{
		cfg:      cfg.normalize(),
		clock:    clock,
		gh:       gh,
		reviewer: reviewer,
	}
}

func (w *Watcher) Run(ctx context.Context) error {
	cfg := w.cfg
	start := w.clock.Now()

	var lastKnownHead string
	var headChangeTime time.Time
	reviewCount := 0

	for {
		if err := sleepCtx(ctx, 0); err != nil {
			return err
		}

		if w.clock.Now().Sub(start) > cfg.MaxDuration {
			return fmt.Errorf("watch: %w", ErrDurationLimitHit)
		}
		if reviewCount >= cfg.MaxReviews {
			return fmt.Errorf("watch: %w", ErrReviewLimitReached)
		}

		head, open, err := w.gh.Head(ctx)
		if err != nil {
			return err
		}
		if !open {
			return nil
		}
		if head == "" {
			if err := sleepCtx(ctx, cfg.PollInterval); err != nil {
				return err
			}
			continue
		}

		if head != lastKnownHead {
			lastKnownHead = head
			headChangeTime = w.clock.Now()
		}

		if w.clock.Now().Sub(headChangeTime) < cfg.SettleTime {
			if err := sleepCtx(ctx, cfg.PollInterval); err != nil {
				return err
			}
			continue
		}

		run, err := w.reviewer.Review(ctx, lastKnownHead)
		if err != nil {
			return err
		}

		currentHead, openNow, err := w.gh.Head(ctx)
		if err != nil {
			return err
		}
		if currentHead != lastKnownHead {
			slog.Warn("PR head moved during review; discarding result", "previous", lastKnownHead, "current", currentHead)
			lastKnownHead = currentHead
			headChangeTime = w.clock.Now()
			continue
		}
		if !openNow {
			return nil
		}

		increment, terminal, err := w.post(ctx, run)
		if err != nil {
			return err
		}
		if terminal {
			return nil
		}
		if increment {
			reviewCount++
			headChangeTime = w.clock.Now()
		}

		if err := sleepCtx(ctx, cfg.PollInterval); err != nil {
			return err
		}
	}
}

func (w *Watcher) post(ctx context.Context, run *domain.ReviewRun) (increment bool, terminal bool, err error) {
	cfg := w.cfg
	hasKept := review.HasKept(run)
	body := review.PRBody(run)

	if cfg.PostMode != PostModeApprove {
		if hasKept {
			if err := w.gh.Submit(ctx, "comment", body); err != nil {
				return false, false, err
			}
			return true, false, nil
		}
		if err := w.gh.Submit(ctx, "comment", body); err != nil {
			return false, false, err
		}
		return false, true, nil
	}

	if hasKept {
		if err := w.gh.Submit(ctx, "request-changes", body); err != nil {
			return false, false, err
		}
		return true, false, nil
	}

	state, err := w.gh.CheckState(ctx)
	if err != nil {
		return false, false, err
	}
	if state != "success" {
		return false, false, nil
	}
	if err := w.gh.Submit(ctx, "approve", body); err != nil {
		return false, false, err
	}
	return false, true, nil
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
