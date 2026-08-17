package pipeline

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/richhaase/bcr/internal/domain"
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

const summaryDismissedBody = `{"findings":[
  {"rule":"r","category":"c","severity":"high","file":"a.go","line":1,"message":"msg","keep":true},
  {"rule":"old","category":"c","severity":"medium","file":"b.go","line":3,"message":"already addressed","keep":false,"dismiss_reason":"already addressed in PR discussion"}
]}`

type capturingCompleter struct {
	mu                sync.Mutex
	summarizerMessage []provider.Message
}

func (c *capturingCompleter) Complete(_ context.Context, model string, messages []provider.Message, _ float64) (string, error) {
	if model == "summarizer" {
		c.mu.Lock()
		c.summarizerMessage = messages
		c.mu.Unlock()
		return summaryBody, nil
	}
	return reviewBody, nil
}

func TestPipelineInjectsFeedback(t *testing.T) {
	var f capturingCompleter
	runner := NewRunner(Config{
		Models:          []string{"m1"},
		SummarizerModel: "summarizer",
		Feedback:        "The timeout is intentional per discussion.",
	})
	runner.client = &f

	run, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if len(run.Final) != 1 {
		t.Fatalf("expected 1 final finding, got %d", len(run.Final))
	}

	f.mu.Lock()
	messages := append([]provider.Message(nil), f.summarizerMessage...)
	f.mu.Unlock()

	var userMsg string
	for _, m := range messages {
		if m.Role == "user" {
			userMsg = m.Content
		}
	}
	if !strings.Contains(userMsg, "Prior PR Discussion & Context:") {
		t.Errorf("expected feedback section in summarizer prompt, got %q", userMsg)
	}
	if !strings.Contains(userMsg, "The timeout is intentional per discussion.") {
		t.Errorf("expected feedback body in summarizer prompt, got %q", userMsg)
	}
}

func TestPipelineNoFeedbackWhenEmpty(t *testing.T) {
	var f capturingCompleter
	runner := NewRunner(Config{
		Models:          []string{"m1"},
		SummarizerModel: "summarizer",
	})
	runner.client = &f

	if _, err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run error: %v", err)
	}

	f.mu.Lock()
	messages := append([]provider.Message(nil), f.summarizerMessage...)
	f.mu.Unlock()

	for _, m := range messages {
		if m.Role == "user" && strings.Contains(m.Content, "Prior PR Discussion & Context:") {
			t.Errorf("did not expect feedback section without feedback config")
		}
	}
}

type dismissCompleter struct{}

func (dismissCompleter) Complete(_ context.Context, model string, _ []provider.Message, _ float64) (string, error) {
	if model == "summarizer" {
		return summaryDismissedBody, nil
	}
	return reviewBody, nil
}

func TestSynthesizerDropsDismissedFinding(t *testing.T) {
	runner := NewRunner(Config{
		Models:          []string{"m1"},
		SummarizerModel: "summarizer",
	})
	runner.client = dismissCompleter{}

	run, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if run.Dismissed != 1 {
		t.Errorf("expected 1 dismissed finding, got %d", run.Dismissed)
	}

	var kept []domain.FinalFinding
	for _, f := range run.Final {
		if f.Keep {
			kept = append(kept, f)
		}
	}
	if len(kept) != 1 {
		t.Errorf("expected 1 kept final finding, got %d", len(kept))
	}
}

const reviewBodyExcludeFile = `{"findings":[
  {"rule":"r","category":"c","severity":"high","file":"gen/gen.go","line":1,"message":"msg"},
  {"rule":"r2","category":"c","severity":"medium","file":"a.go","line":2,"message":"msg2"}
]}`

type excludeCompleter struct{}

func (excludeCompleter) Complete(_ context.Context, model string, _ []provider.Message, _ float64) (string, error) {
	if model == "summarizer" {
		return summaryBody, nil
	}
	return reviewBodyExcludeFile, nil
}

func TestRunAppliesExcludePatterns(t *testing.T) {
	runner := NewRunner(Config{
		Models:          []string{"m1"},
		SummarizerModel: "summarizer",
		ExcludePatterns: []string{`gen/`},
	})
	runner.client = excludeCompleter{}

	run, err := runner.Run(context.Background())
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if run.Excluded != 1 {
		t.Errorf("expected 1 excluded finding, got %d", run.Excluded)
	}
	if len(run.Findings) != 1 {
		t.Errorf("expected 1 post-exclusion finding, got %d", len(run.Findings))
	}
}

func TestRunRejectsMalformedExcludePattern(t *testing.T) {
	runner := NewRunner(Config{
		Models:          []string{"m1"},
		SummarizerModel: "summarizer",
		ExcludePatterns: []string{`[unclosed`},
	})
	runner.client = excludeCompleter{}

	_, err := runner.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for malformed exclude pattern")
	}
	if !strings.Contains(err.Error(), "invalid exclude pattern") {
		t.Errorf("expected clear error, got %q", err)
	}
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
