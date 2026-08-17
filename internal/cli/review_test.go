package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/richhaase/bcr/internal/domain"
)

func TestReviewReportsTokenAndCost(t *testing.T) {
	var buf bytes.Buffer
	run := &domain.ReviewRun{
		Models:           []string{"m1"},
		PromptTokens:     1200,
		CompletionTokens: 340,
		EstimatedCostUSD: 0.0023,
		Final: []domain.FinalFinding{{
			Rule:     "nil",
			Severity: "high",
			File:     "a.go",
			Line:     1,
			Message:  "nil deref",
			Keep:     true,
		}},
	}
	renderReport(&buf, run)
	out := buf.String()
	if !strings.Contains(out, "1200 prompt") {
		t.Errorf("expected prompt token count in report, got %q", out)
	}
	if !strings.Contains(out, "340 completion") {
		t.Errorf("expected completion token count in report, got %q", out)
	}
	if !strings.Contains(out, "Est Cost") {
		t.Errorf("expected estimated cost in report, got %q", out)
	}
}

func TestReviewReportsTokenAndCostLGTM(t *testing.T) {
	var buf bytes.Buffer
	run := &domain.ReviewRun{
		Models:           []string{"m1"},
		PromptTokens:     500,
		CompletionTokens: 80,
		EstimatedCostUSD: 0.0004,
	}
	renderReport(&buf, run)
	out := buf.String()
	if !strings.Contains(out, "500 prompt") {
		t.Errorf("expected prompt token count in LGTM report, got %q", out)
	}
	if !strings.Contains(out, "80 completion") {
		t.Errorf("expected completion token count in LGTM report, got %q", out)
	}
	if !strings.Contains(out, "Est Cost") {
		t.Errorf("expected estimated cost in LGTM report, got %q", out)
	}
}
