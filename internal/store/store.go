package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/richhaase/bcr/internal/domain"
)

const CurrentSchemaVersion = 1

type ReviewRecordV1 struct {
	SchemaVersion    int                   `json:"schema_version"`
	ID               string                `json:"id"`
	PRRef            string                `json:"pr_ref"`
	Timestamp        time.Time             `json:"timestamp"`
	Models           []string              `json:"models"`
	Findings         []domain.Finding      `json:"findings"`
	Final            []domain.FinalFinding `json:"final"`
	PromptTokens     int                   `json:"prompt_tokens"`
	CompletionTokens int                   `json:"completion_tokens"`
	EstimatedCostUSD float64               `json:"estimated_cost_usd"`
}

type Store struct {
	root string
}

func NewStoreAt(root string) *Store {
	return &Store{root: root}
}

func NewStore() *Store {
	return &Store{root: defaultRoot()}
}

func defaultRoot() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "bcr")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "bcr")
	}
	return filepath.Join(".", ".local", "share", "bcr")
}

func (s *Store) SaveRun(prRef string, record ReviewRecordV1) error {
	dir, err := s.prDir(prRef, true)
	if err != nil {
		return err
	}
	if record.SchemaVersion == 0 {
		record.SchemaVersion = CurrentSchemaVersion
	}
	if record.ID == "" {
		record.ID = newID()
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}
	record.PRRef = prRef
	filename := filepath.Join(dir, record.ID+".json")
	return atomicWriteJSON(filename, record)
}

func (s *Store) ListRuns(prRef string) ([]ReviewRecordV1, error) {
	dir, err := s.prDir(prRef, false)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var records []ReviewRecordV1
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		rec, err := readRecord(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, nil
}

func (s *Store) ForgetPR(prRef string) (int, error) {
	records, err := s.ListRuns(prRef)
	if err != nil {
		return 0, err
	}
	dir, err := s.prDir(prRef, false)
	if err != nil {
		return 0, err
	}
	for _, rec := range records {
		path := filepath.Join(dir, rec.ID+".json")
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return 0, err
		}
	}
	_ = os.Remove(dir)
	return len(records), nil
}

func (s *Store) prDir(prRef string, create bool) (string, error) {
	owner, repo, number, err := parsePRRef(prRef)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(s.root, "prs", owner, repo, number)
	if create {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func parsePRRef(prRef string) (owner, repo, number string, err error) {
	idx := strings.LastIndex(prRef, "#")
	if idx <= 0 || idx == len(prRef)-1 {
		return "", "", "", fmt.Errorf("invalid PR ref %q: expected owner/repo#number", prRef)
	}
	number = prRef[idx+1:]
	if !allDigits(number) {
		return "", "", "", fmt.Errorf("invalid PR ref %q: number must be digits", prRef)
	}
	ownerRepo := prRef[:idx]
	parts := strings.Split(ownerRepo, "/")
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid PR ref %q: expected owner/repo#number", prRef)
	}
	owner = parts[0]
	repo = parts[1]
	for _, p := range []string{owner, repo} {
		if !validComponent(p) {
			return "", "", "", fmt.Errorf("invalid PR ref %q: invalid owner/repo token", prRef)
		}
	}
	return owner, repo, number, nil
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validComponent(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	return true
}

func readRecord(path string) (ReviewRecordV1, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ReviewRecordV1{}, err
	}
	var rec ReviewRecordV1
	if err := json.Unmarshal(data, &rec); err != nil {
		return ReviewRecordV1{}, fmt.Errorf("decode store record %s: %w", path, err)
	}
	if rec.SchemaVersion != CurrentSchemaVersion {
		return ReviewRecordV1{}, fmt.Errorf("unsupported schema version %d in %s (current %d)", rec.SchemaVersion, path, CurrentSchemaVersion)
	}
	return rec, nil
}

func atomicWriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := syncFile(tmpName); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func syncFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return f.Sync()
}

func newID() string {
	return fmt.Sprintf("%x", time.Now().UTC().UnixNano())
}
