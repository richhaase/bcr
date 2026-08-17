package watch

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/richhaase/bcr/internal/domain"
)

type fakeClock struct {
	mu      sync.Mutex
	current time.Time
}

func (f *fakeClock) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.current
}

func (f *fakeClock) advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.current = f.current.Add(d)
}

type fakeGitHub struct {
	mu         sync.Mutex
	head       string
	open       bool
	state      string
	submitted  []string
	checkCalls int
}

func (f *fakeGitHub) Head(_ context.Context) (string, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.head, f.open, nil
}

func (f *fakeGitHub) CheckState(_ context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checkCalls++
	return f.state, nil
}

func (f *fakeGitHub) Submit(_ context.Context, event, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.submitted = append(f.submitted, event)
	return nil
}

func findingsRun() *domain.ReviewRun {
	return &domain.ReviewRun{
		Models: []string{"m"},
		Final: []domain.FinalFinding{
			{Keep: true, Rule: "r", File: "a.go", Line: 1, Severity: "high", Message: "found"},
		},
	}
}

func cleanRun() *domain.ReviewRun {
	return &domain.ReviewRun{Models: []string{"m"}}
}

func TestWatcherRunsInitialReview(t *testing.T) {
	gh := &fakeGitHub{head: "sha1", open: true, state: "success"}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reviewed := 0
	reviewer := ReviewFunc(func(_ context.Context, _ string) (*domain.ReviewRun, error) {
		reviewed++
		cancel()
		return findingsRun(), nil
	})

	w := New(Config{
		PR:           1,
		PollInterval: time.Millisecond,
		PostMode:     PostModeComment,
	}, &fakeClock{current: time.Unix(100, 0)}, gh, reviewer)

	_ = w.Run(ctx)

	if reviewed != 1 {
		t.Errorf("expected exactly 1 review, got %d", reviewed)
	}
	if len(gh.submitted) != 1 || gh.submitted[0] != "comment" {
		t.Errorf("expected a comment review to be posted, got %v", gh.submitted)
	}
}

func TestWatcherDefaultPollInterval(t *testing.T) {
	cfg := (Config{}).normalize()
	if cfg.PollInterval != time.Minute {
		t.Errorf("default poll interval = %v, want 1m", cfg.PollInterval)
	}
}

func TestWatcherSettleTimeQuietPeriod(t *testing.T) {
	gh := &fakeGitHub{head: "sha1", open: true}
	fc := &fakeClock{current: time.Unix(1000, 0)}

	var wg sync.WaitGroup
	reviewed := make(chan string, 1)

	review := ReviewFunc(func(_ context.Context, head string) (*domain.ReviewRun, error) {
		reviewed <- head
		return cleanRun(), nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := New(Config{
		PR:           1,
		PollInterval: time.Millisecond,
		SettleTime:   time.Hour,
		PostMode:     PostModeComment,
	}, fc, gh, review)

	wg.Add(1)
	var runErr error
	go func() {
		defer wg.Done()
		runErr = w.Run(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	select {
	case <-reviewed:
		t.Errorf("review ran before settle quiet period elapsed")
	default:
	}

	fc.advance(2 * time.Hour)
	select {
	case head := <-reviewed:
		if head != "sha1" {
			t.Errorf("reviewed head = %q, want sha1", head)
		}
	case <-time.After(500 * time.Millisecond):
		t.Errorf("review did not run after settle quiet period")
	}

	cancel()
	wg.Wait()
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		t.Errorf("unexpected run error: %v", runErr)
	}
}

func TestWatcherExitsOnTerminalLGTM(t *testing.T) {
	gh := &fakeGitHub{head: "sha1", open: true}
	reviewer := ReviewFunc(func(_ context.Context, _ string) (*domain.ReviewRun, error) {
		return cleanRun(), nil
	})

	w := New(Config{
		PR:           1,
		PollInterval: time.Millisecond,
		PostMode:     PostModeComment,
	}, &fakeClock{current: time.Unix(100, 0)}, gh, reviewer)

	err := w.Run(context.Background())
	if err != nil {
		t.Fatalf("watch exit code error: %v", err)
	}
	if len(gh.submitted) != 1 || gh.submitted[0] != "comment" {
		t.Errorf("expected a clean comment post, got %v", gh.submitted)
	}
}

func TestWatcherEnforcesSafetyBounds(t *testing.T) {
	gh := &fakeGitHub{head: "sha1", open: true}
	reviewer := ReviewFunc(func(_ context.Context, _ string) (*domain.ReviewRun, error) {
		return findingsRun(), nil
	})

	t.Run("max reviews", func(t *testing.T) {
		gh := &fakeGitHub{head: "sha1", open: true}
		w := New(Config{
			PR:           1,
			PollInterval: time.Millisecond,
			MaxReviews:   3,
			PostMode:     PostModeComment,
		}, &fakeClock{current: time.Unix(100, 0)}, gh, reviewer)
		err := w.Run(context.Background())
		if !errors.Is(err, ErrReviewLimitReached) {
			t.Errorf("expected ErrReviewLimitReached, got %v", err)
		}
		if len(gh.submitted) != 3 {
			t.Errorf("expected 3 reviews before giving up, got %v", gh.submitted)
		}
	})

	t.Run("max duration", func(t *testing.T) {
		fc := &fakeClock{current: time.Unix(100, 0)}
		review := ReviewFunc(func(_ context.Context, _ string) (*domain.ReviewRun, error) {
			fc.advance(2 * time.Second)
			return findingsRun(), nil
		})
		w := New(Config{
			PR:           1,
			PollInterval: time.Millisecond,
			MaxDuration:  time.Second,
			PostMode:     PostModeComment,
		}, fc, gh, review)
		err := w.Run(context.Background())
		if !errors.Is(err, ErrDurationLimitHit) {
			t.Errorf("expected ErrDurationLimitHit, got %v", err)
		}
	})
}

func TestWatcherDiscardsResultIfHeadMovedMidReview(t *testing.T) {
	gh := &fakeGitHub{head: "sha1", open: true}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reviewedHead := make(chan string, 2)
	review := ReviewFunc(func(_ context.Context, head string) (*domain.ReviewRun, error) {
		reviewedHead <- head
		if head == "sha1" {
			gh.mu.Lock()
			gh.head = "sha2"
			gh.mu.Unlock()
			return findingsRun(), nil
		}
		cancel()
		return cleanRun(), nil
	})

	w := New(Config{
		PR:           1,
		PollInterval: time.Millisecond,
		PostMode:     PostModeApprove,
	}, &fakeClock{current: time.Unix(100, 0)}, gh, review)

	_ = w.Run(ctx)

	if len(gh.submitted) != 0 {
		t.Errorf("no result should be posted for a moved head, got %v", gh.submitted)
	}
	if got := len(reviewedHead); got != 2 {
		t.Errorf("expected 2 review passes (initial discard + re-review), got %d", got)
	}
}

func TestWatcherPostModes(t *testing.T) {
	t.Run("comment clean", func(t *testing.T) {
		gh := &fakeGitHub{head: "sha1", open: true}
		reviewer := ReviewFunc(func(_ context.Context, _ string) (*domain.ReviewRun, error) {
			return cleanRun(), nil
		})
		w := New(Config{PR: 1, PollInterval: time.Millisecond, PostMode: PostModeComment},
			&fakeClock{current: time.Unix(100, 0)}, gh, reviewer)
		if err := w.Run(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(gh.submitted) != 1 || gh.submitted[0] != "comment" {
			t.Errorf("empty review should post a comment, got %v", gh.submitted)
		}
	})

	t.Run("approve clean ci-green", func(t *testing.T) {
		gh := &fakeGitHub{head: "sha1", open: true, state: "success"}
		reviewer := ReviewFunc(func(_ context.Context, _ string) (*domain.ReviewRun, error) {
			return cleanRun(), nil
		})
		w := New(Config{PR: 1, PollInterval: time.Millisecond, PostMode: PostModeApprove},
			&fakeClock{current: time.Unix(100, 0)}, gh, reviewer)
		if err := w.Run(context.Background()); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(gh.submitted) != 1 || gh.submitted[0] != "approve" {
			t.Errorf("expected an approve post with green CI, got %v", gh.submitted)
		}
		if gh.checkCalls == 0 {
			t.Errorf("CI should be checked before approving")
		}
	})

	t.Run("approve with findings", func(t *testing.T) {
		gh := &fakeGitHub{head: "sha1", open: true, state: "success"}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		review := ReviewFunc(func(_ context.Context, _ string) (*domain.ReviewRun, error) {
			cancel()
			return findingsRun(), nil
		})
		w := New(Config{PR: 1, PollInterval: time.Millisecond, PostMode: PostModeApprove},
			&fakeClock{current: time.Unix(100, 0)}, gh, review)
		_ = w.Run(ctx)
		if len(gh.submitted) != 1 || gh.submitted[0] != "request-changes" {
			t.Errorf("expected request-changes for findings, got %v", gh.submitted)
		}
	})
}
