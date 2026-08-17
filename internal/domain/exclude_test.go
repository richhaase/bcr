package domain

import (
	"strings"
	"testing"
)

func TestExcludeFindingsMatchesFields(t *testing.T) {
	findings := []Finding{
		{Rule: "nil-check", File: "a.go", Line: 1, Message: "possible nil deref"},
		{Rule: "go-generate", File: "gen/gen.go", Line: 2, Message: "generated file"},
		{Rule: "style", File: "b.go", Line: 3, Message: "TODO comment left behind"},
		{Rule: "perf", File: "c.go", Line: 4, Message: "allocates in loop"},
	}

	kept, excluded, err := ExcludeFindings(findings, []string{`gen/`, `TODO`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if excluded != 2 {
		t.Errorf("expected 2 excluded, got %d", excluded)
	}
	if len(kept) != 2 {
		t.Fatalf("expected 2 kept, got %d", len(kept))
	}
	for _, f := range kept {
		if strings.Contains(f.File, "generated") || strings.Contains(f.Message, "TODO") {
			t.Errorf("expected excluded finding in kept set: %+v", f)
		}
	}
}

func TestExcludeFindingsNoPatterns(t *testing.T) {
	findings := []Finding{{Rule: "r", File: "a.go", Message: "m"}}
	kept, excluded, err := ExcludeFindings(findings, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if excluded != 0 {
		t.Errorf("expected 0 excluded, got %d", excluded)
	}
	if len(kept) != 1 {
		t.Errorf("expected all findings kept, got %d", len(kept))
	}
}

func TestExcludeFindingsBlankPatternSkipped(t *testing.T) {
	findings := []Finding{{Rule: "r", File: "a.go", Message: "m"}}
	kept, excluded, err := ExcludeFindings(findings, []string{"   "})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if excluded != 0 || len(kept) != 1 {
		t.Errorf("expected no exclusion with blank pattern, got excluded=%d kept=%d", excluded, len(kept))
	}
}

func TestExcludeFindingsMalformedPattern(t *testing.T) {
	findings := []Finding{{Rule: "r", File: "a.go", Message: "m"}}
	_, _, err := ExcludeFindings(findings, []string{"[unclosed"})
	if err == nil {
		t.Fatal("expected error for malformed pattern")
	}
	if !strings.Contains(err.Error(), "invalid exclude pattern") {
		t.Errorf("expected clear error message, got %q", err)
	}
}
