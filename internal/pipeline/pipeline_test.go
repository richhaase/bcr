package pipeline

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/richhaase/bcr/internal/provider"
)

const reviewBody = `{"findings":[{"rule":"r","category":"c","severity":"high","file":"a.go","line":1,"message":"msg"}]}`

const summaryBody = `{"findings":[{"rule":"r","category":"c","severity":"high","file":"a.go","line":1,"message":"msg","keep":true}]}`

type fakeCompleter struct {
	mu        sync.Mutex
	active    int
	maxActive int
	reviews   int
}

func (f *fakeCompleter) Complete(_ context.Context, model string, _ []provider.Message, _ float64) (string, error) {
	f.mu.Lock()
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		f.active--
		f.mu.Unlock()
	}()

	if model == "summarizer" {
		return summaryBody, nil
	}

	f.mu.Lock()
	f.reviews++
	f.mu.Unlock()
	time.Sleep(30 * time.Millisecond)
	return reviewBody, nil
}

func TestRunBoundedConcurrency(t *testing.T) {
	var fd fakeCompleter
	runner := NewRunner(Config{
		Models:          []string{"m1", "m2", "m3", "m4"},
		SummarizerModel: "summarizer",
		Concurrency:     2,
	})
	runner.client = &fd

	run, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	fd.mu.Lock()
	defer fd.mu.Unlock()

	if fd.reviews != 4 {
		t.Errorf("expected 4 reviewer calls, got %d", fd.reviews)
	}
	if fd.maxActive > 2 {
		t.Errorf("expected max concurrency <= 2, got %d", fd.maxActive)
	}
	if len(run.Findings) != 4 {
		t.Errorf("expected 4 findings, got %d", len(run.Findings))
	}
	if len(run.Models) != 4 {
		t.Errorf("expected 4 models, got %d", len(run.Models))
	}
}

func TestRunUnboundedConcurrency(t *testing.T) {
	var fd fakeCompleter
	runner := NewRunner(Config{
		Models:          []string{"m1", "m2", "m3"},
		SummarizerModel: "summarizer",
		Concurrency:     0,
	})
	runner.client = &fd

	run, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	fd.mu.Lock()
	defer fd.mu.Unlock()

	if fd.reviews != 3 {
		t.Errorf("expected 3 reviewer calls, got %d", fd.reviews)
	}
	if fd.maxActive > 3 {
		t.Errorf("expected max concurrency <= 3, got %d", fd.maxActive)
	}
	if len(run.Findings) != 3 {
		t.Errorf("expected 3 findings, got %d", len(run.Findings))
	}
}

type failCompleter struct {
	mu      sync.Mutex
	calls   map[string]int
	failErr error
}

func (f *failCompleter) Complete(_ context.Context, model string, _ []provider.Message, _ float64) (string, error) {
	f.mu.Lock()
	if f.calls == nil {
		f.calls = make(map[string]int)
	}
	f.calls[model]++
	f.mu.Unlock()

	if model == "summarizer" {
		return summaryBody, nil
	}
	if model == "broken" {
		return "", f.failErr
	}
	return reviewBody, nil
}

func TestRunPreservesSuccessfulFindingsOnPermanentFailure(t *testing.T) {
	runner := NewRunner(Config{
		Models:          []string{"good", "broken", "also-good"},
		SummarizerModel: "summarizer",
		Concurrency:     0,
	})
	runner.client = &failCompleter{failErr: errors.New("invalid model")}

	run, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	if len(run.Findings) != 2 {
		t.Errorf("expected 2 preserved findings, got %d", len(run.Findings))
	}
	if len(run.Models) != 3 {
		t.Errorf("expected 3 models in report, got %d", len(run.Models))
	}
}
