package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/richhaase/bcr/internal/domain"
	"github.com/richhaase/bcr/internal/store"
)

func TestDeskHistoryCommand(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)
	st := store.NewStore()
	prRef := "owner/repo#12"
	if err := st.SaveRun(prRef, store.ReviewRecordV1{
		SchemaVersion:    store.CurrentSchemaVersion,
		Models:           []string{"m1"},
		PromptTokens:     100,
		CompletionTokens: 40,
		EstimatedCostUSD: 0.0001,
		Final:            []domain.FinalFinding{{Rule: "r", File: "a.go", Message: "msg", Keep: true}},
	}); err != nil {
		t.Fatalf("SaveRun error: %v", err)
	}

	out, errOut, err := executeCommand("desk", "history", prRef)
	if err != nil {
		t.Fatalf("desk history error: %v (stderr: %s)", err, errOut)
	}
	if !strings.Contains(out, "Review history for owner/repo#12") {
		t.Errorf("expected history header, got %q", out)
	}
	if !strings.Contains(out, "Tokens:") {
		t.Errorf("expected token economics in history, got %q", out)
	}
	if !strings.Contains(out, "Est Cost") {
		t.Errorf("expected cost economics in history, got %q", out)
	}
	if !strings.Contains(out, "1 total, 1 kept") {
		t.Errorf("expected finding counts in history, got %q", out)
	}
}

func TestDeskForgetCLICommand(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)
	st := store.NewStore()
	prRef := "owner/repo#13"
	if err := st.SaveRun(prRef, store.ReviewRecordV1{SchemaVersion: store.CurrentSchemaVersion}); err != nil {
		t.Fatalf("SaveRun error: %v", err)
	}
	if err := st.SaveRun(prRef, store.ReviewRecordV1{SchemaVersion: store.CurrentSchemaVersion}); err != nil {
		t.Fatalf("SaveRun error: %v", err)
	}

	out, _, err := executeCommand("desk", "forget", prRef)
	if err != nil {
		t.Fatalf("desk forget error: %v", err)
	}
	if !strings.Contains(out, "Removed 2 stored record(s)") {
		t.Errorf("expected removed count in output, got %q", out)
	}
	remaining, err := store.NewStoreAt(filepath.Join(xdg, "bcr")).ListRuns(prRef)
	if err != nil {
		t.Fatalf("ListRuns error: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected history deleted, got %d remaining records", len(remaining))
	}
}
