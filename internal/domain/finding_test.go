package domain

import "testing"

func TestGroupFindings(t *testing.T) {
	findings := []Finding{
		{
			Rule:     "nil-check",
			Category: "correctness",
			Severity: "high",
			File:     "foo.go",
			Line:     10,
			Message:  "possible nil dereference",
			Agent:    "model-a",
		},
		{
			Rule:     "nil-check",
			Category: "correctness",
			Severity: "high",
			File:     "foo.go",
			Line:     10,
			Message:  "possible nil deref on variable x",
			Agent:    "model-b",
		},
		{
			Rule:     "race-condition",
			Category: "race",
			Severity: "critical",
			File:     "bar.go",
			Line:     50,
			Message:  "concurrent write without lock",
			Agent:    "model-a",
		},
	}

	groups := GroupFindings(findings)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	byRule := make(map[string]Group)
	for _, g := range groups {
		byRule[g.Rule] = g
	}

	if groups[0].Severity != "critical" {
		t.Errorf("expected critical first, got %s", groups[0].Severity)
	}

	nilGroup, ok := byRule["nil-check"]
	if !ok {
		t.Fatalf("missing nil-check group")
	}

	if nilGroup.Count != 2 {
		t.Errorf("expected count 2 for nil-check, got %d", nilGroup.Count)
	}

	if len(nilGroup.Agents) != 2 {
		t.Errorf("expected 2 agents, got %d", len(nilGroup.Agents))
	}
}

func TestParseFindingsMarkdown(t *testing.T) {
	raw := "Here is my review:\n\n```json\n{\n  \"findings\": [\n    {\n      \"rule\": \"mem-leak\",\n      \"file\": \"leak.go\",\n      \"line\": 15,\n      \"message\": \"fd not closed\"\n    }\n  ]\n}\n```\n"

	findings, err := ParseFindings(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Rule != "mem-leak" {
		t.Errorf("expected rule mem-leak, got %s", findings[0].Rule)
	}
}
