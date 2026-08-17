package review

import (
	"strings"
	"testing"

	"github.com/richhaase/bcr/internal/domain"
)

func TestPRBodyZeroKept(t *testing.T) {
	run := &domain.ReviewRun{
		Models:    []string{"m-a"},
		Dismissed: 3,
	}
	body := PRBody(run)
	if !strings.Contains(body, "LGTM") {
		t.Errorf("expected LGTM marker, got: %s", body)
	}
	if !strings.Contains(body, "3") {
		t.Errorf("expected filtered-out count present, got: %s", body)
	}
}

func TestPRBodyKeptAllFields(t *testing.T) {
	run := &domain.ReviewRun{
		Models: []string{"m-a", "m-b"},
		Final: []domain.FinalFinding{
			{
				Keep:       true,
				Rule:       "nil-check",
				Severity:   "high",
				File:       "foo.go",
				Line:       12,
				Message:    "possible nil deref",
				Suggestion: "guard against nil",
			},
			{
				Keep:     false,
				Rule:     "style",
				Severity: "low",
			},
		},
	}
	body := PRBody(run)
	for _, want := range []string{"foo.go:12", "nil-check", "possible nil deref", "guard against nil", "HIGH"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected body to contain %q, got:\n%s", want, body)
		}
	}
	if strings.Contains(body, "style") {
		t.Errorf("dropped finding should not appear in body:\n%s", body)
	}
}

func TestPRBodyNotesExcludedCount(t *testing.T) {
	run := &domain.ReviewRun{
		Models:    []string{"m-a"},
		Excluded:  2,
		Dismissed: 1,
		Final: []domain.FinalFinding{
			{
				Keep:     true,
				Rule:     "r",
				Severity: "high",
				File:     "foo.go",
				Line:     1,
				Message:  "m",
			},
		},
	}
	body := PRBody(run)
	if !strings.Contains(body, "2 finding(s) excluded by regex patterns") {
		t.Errorf("expected excluded-count note in body, got:\n%s", body)
	}
	if !strings.Contains(body, "1 false positive/duplicate finding(s) filtered out") {
		t.Errorf("expected dismissed-count note in body, got:\n%s", body)
	}
}

func TestPRBodySuggestionOmittedWhenEmpty(t *testing.T) {
	run := &domain.ReviewRun{
		Models: []string{"m-a"},
		Final: []domain.FinalFinding{
			{
				Keep:     true,
				Severity: "medium",
				File:     "bar.go",
				Line:     3,
				Message:  "cycle import risk",
			},
		},
	}
	body := PRBody(run)
	if strings.Contains(strings.ToLower(body), "suggestion") {
		t.Errorf("expected no suggestion section when empty, got:\n%s", body)
	}
}

func TestDisposition(t *testing.T) {
	clean := &domain.ReviewRun{Models: []string{"m-a"}}
	findings := &domain.ReviewRun{
		Models: []string{"m-a"},
		Final: []domain.FinalFinding{
			{Keep: true, Rule: "r", File: "f", Severity: "high", Message: "m"},
		},
	}

	tests := []struct {
		name       string
		run        *domain.ReviewRun
		ciOK       bool
		selfReview bool
		wantEvent  string
	}{
		{"self-review findings", findings, true, true, "request-changes"},
		{"self-review clean", clean, true, true, "comment"},
		{"findings present", findings, true, false, "request-changes"},
		{"clean ci-ok", clean, true, false, "approve"},
		{"clean ci-not-ok", clean, false, false, "comment"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			event, _ := Disposition(tc.run, tc.ciOK, tc.selfReview)
			if event != tc.wantEvent {
				t.Errorf("Disposition() = %q, want %q", event, tc.wantEvent)
			}
		})
	}
}
