package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/richhaase/bcr/internal/domain"
)

func TestSaveReviewRecord(t *testing.T) {
	dir := t.TempDir()
	st := NewStoreAt(dir)
	prRef := "richhaase/bcr#8"
	rec := ReviewRecordV1{
		SchemaVersion:    CurrentSchemaVersion,
		Models:           []string{"m1"},
		Findings:         []domain.Finding{{Rule: "r", File: "a.go", Message: "msg"}},
		PromptTokens:     100,
		CompletionTokens: 50,
		EstimatedCostUSD: 0.001,
	}
	if err := st.SaveRun(prRef, rec); err != nil {
		t.Fatalf("SaveRun error: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, "prs", "richhaase", "bcr", "8"))
	if err != nil {
		t.Fatalf("ReadDir error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 record file, got %d", len(entries))
	}
}

func TestSaveReviewRecordUsesXDGDataHome(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdg)
	st := NewStore()
	prRef := "owner/repo#1"
	if err := st.SaveRun(prRef, ReviewRecordV1{SchemaVersion: CurrentSchemaVersion}); err != nil {
		t.Fatalf("SaveRun error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(xdg, "bcr", "prs", "owner", "repo", "1")); err != nil {
		t.Errorf("expected record under XDG data dir: %v", err)
	}
}

func TestReviewRecordContainsAllFields(t *testing.T) {
	dir := t.TempDir()
	st := NewStoreAt(dir)
	prRef := "owner/repo#3"
	rec := ReviewRecordV1{
		SchemaVersion:    CurrentSchemaVersion,
		Models:           []string{"deepseek/deepseek-chat", "qwen/qwen-2.5-coder-32b-instruct"},
		Findings:         []domain.Finding{{Rule: "nil", Severity: "high", File: "a.go", Line: 2, Message: "nil deref"}},
		Final:            []domain.FinalFinding{{Rule: "nil", Keep: true, File: "a.go", Message: "nil deref"}},
		PromptTokens:     1200,
		CompletionTokens: 340,
		EstimatedCostUSD: 0.0023,
	}
	if err := st.SaveRun(prRef, rec); err != nil {
		t.Fatalf("SaveRun error: %v", err)
	}
	records, err := st.ListRuns(prRef)
	if err != nil {
		t.Fatalf("ListRuns error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	got := records[0]
	if got.PRRef != prRef {
		t.Errorf("PRRef = %q, want %q", got.PRRef, prRef)
	}
	if got.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("schema version = %d, want %d", got.SchemaVersion, CurrentSchemaVersion)
	}
	if got.ID == "" {
		t.Error("expected non-empty ID")
	}
	if len(got.Models) != 2 {
		t.Errorf("expected 2 models, got %d", len(got.Models))
	}
	if len(got.Findings) != 1 || len(got.Final) != 1 {
		t.Errorf("expected findings and finals to be persisted")
	}
	if got.PromptTokens != 1200 || got.CompletionTokens != 340 {
		t.Errorf("token metrics not persisted: %d/%d", got.PromptTokens, got.CompletionTokens)
	}
	if got.EstimatedCostUSD != 0.0023 {
		t.Errorf("cost not persisted: %f", got.EstimatedCostUSD)
	}
}

func TestStoreIsAppendOnly(t *testing.T) {
	dir := t.TempDir()
	st := NewStoreAt(dir)
	prRef := "owner/repo#4"
	for i := 0; i < 3; i++ {
		if err := st.SaveRun(prRef, ReviewRecordV1{SchemaVersion: CurrentSchemaVersion}); err != nil {
			t.Fatalf("SaveRun %d error: %v", i, err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "prs", "owner", "repo", "4"))
	if err != nil {
		t.Fatalf("ReadDir error: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("expected 3 distinct record files, got %d", len(entries))
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if seen[e.Name()] {
			t.Errorf("duplicate filename %s; record overwritten", e.Name())
		}
		seen[e.Name()] = true
	}
}

func TestStoreSchemaVersionMismatch(t *testing.T) {
	dir := t.TempDir()
	st := NewStoreAt(dir)
	prRef := "owner/repo#5"
	prDir := filepath.Join(dir, "prs", "owner", "repo", "5")
	if err := os.MkdirAll(prDir, 0o750); err != nil {
		t.Fatalf("MkdirAll error: %v", err)
	}
	data := []byte(`{"schema_version": 99, "id": "old", "pr_ref": "owner/repo#5"}`)
	if err := os.WriteFile(filepath.Join(prDir, "old.json"), data, 0o600); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}
	if _, err := st.ListRuns(prRef); err == nil {
		t.Fatal("expected error on unsupported schema version")
	} else if !strings.Contains(err.Error(), "unsupported schema version") {
		t.Errorf("expected clear schema error, got %q", err)
	}
}

func TestDeskForgetCommand(t *testing.T) {
	dir := t.TempDir()
	st := NewStoreAt(dir)
	prRef := "owner/repo#6"
	for i := 0; i < 2; i++ {
		if err := st.SaveRun(prRef, ReviewRecordV1{SchemaVersion: CurrentSchemaVersion}); err != nil {
			t.Fatalf("SaveRun error: %v", err)
		}
	}
	removed, err := st.ForgetPR(prRef)
	if err != nil {
		t.Fatalf("ForgetPR error: %v", err)
	}
	if removed != 2 {
		t.Errorf("expected 2 removed records, got %d", removed)
	}
	records, err := st.ListRuns(prRef)
	if err != nil {
		t.Fatalf("ListRuns error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records after forget, got %d", len(records))
	}
}

func TestParsePRRef(t *testing.T) {
	tests := map[string]struct {
		ref    string
		owner  string
		repo   string
		number string
	}{
		"simple":        {"richhaase/bcr#8", "richhaase", "bcr", "8"},
		"numeric parts": {"repo1/owner2#123", "repo1", "owner2", "123"},
		"hyphens":       {"my-org/my-repo#42", "my-org", "my-repo", "42"},
		"underscores":   {"my_org/my_repo#7", "my_org", "my_repo", "7"},
		"dots allowed":  {"a.b/c.d#1", "a.b", "c.d", "1"},
		"number zeros":  {"o/r#000", "o", "r", "000"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			owner, repo, number, err := parsePRRef(tc.ref)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if owner != tc.owner || repo != tc.repo || number != tc.number {
				t.Errorf("got (%q, %q, %q), want (%q, %q, %q)", owner, repo, number, tc.owner, tc.repo, tc.number)
			}
		})
	}
}

func TestParsePRRejectsInvalid(t *testing.T) {
	tests := map[string]string{
		"empty":                "",
		"no number":            "owner/repo",
		"no hash":              "owner/repo",
		"empty owner":          "/repo#1",
		"empty repo":           "owner/#1",
		"empty number":         "owner/repo#",
		"non-numeric number":   "owner/repo#abc",
		"number with slash":    "owner/repo#1/2",
		"triple parts":         "a/b/c#1",
		"path traversal owner": "../repo#1",
		"path traversal repo":  "owner/..#1",
		"single dot owner":     "./repo#1",
		"single dot repo":      "owner/.#1",
		"slash in owner":       "own/er/rep#1",
		"backslash":            "own\\er/repo#1",
		"space in repo":        "owner/re po#1",
		"space in owner":       "ow ner/repo#1",
		"at sign":              "owner@x/repo#1",
		"colon":                "owner:repo#1",
	}
	for name, ref := range tests {
		t.Run(name, func(t *testing.T) {
			if _, _, _, err := parsePRRef(ref); err == nil {
				t.Errorf("expected error for ref %q", ref)
			}
		})
	}
}

func TestParsePRNoPathTraversal(t *testing.T) {
	refs := []string{
		"../repo#1",
		"repo/../../etc#1",
		"../.../repo#1",
		"owner/..#1",
		"..\\/evil#1",
	}
	for _, ref := range refs {
		owner, repo, number, err := parsePRRef(ref)
		if err == nil {
			t.Errorf("expected error for ref %q, got (%q, %q, %q)", ref, owner, repo, number)
		}
	}
}

func TestSaveRejectsInvalidPRRef(t *testing.T) {
	dir := t.TempDir()
	st := NewStoreAt(dir)
	if err := st.SaveRun("owner/../../etc#1", ReviewRecordV1{}); err == nil {
		t.Error("expected error saving under path-traversal PR ref")
	}
	if err := st.SaveRun("owner/repo#notanumber", ReviewRecordV1{}); err == nil {
		t.Error("expected error saving with non-numeric PR number")
	}
}

func TestAtomicWritePreventsPartialFiles(t *testing.T) {
	dir := t.TempDir()
	st := NewStoreAt(dir)
	prRef := "owner/repo#7"
	if err := st.SaveRun(prRef, ReviewRecordV1{SchemaVersion: CurrentSchemaVersion}); err != nil {
		t.Fatalf("SaveRun error: %v", err)
	}
	prDir := filepath.Join(dir, "prs", "owner", "repo", "7")
	entries, err := os.ReadDir(prDir)
	if err != nil {
		t.Fatalf("ReadDir error: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("leftover temp file %s; atomic write leaked", e.Name())
		}
	}
	records, err := st.ListRuns(prRef)
	if err != nil {
		t.Fatalf("ListRuns error: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected fully written record, got %d", len(records))
	}
}
