package terminal

import (
	"os"
	"testing"
)

func TestIsTerminalPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	if IsTerminal(int(r.Fd())) {
		t.Error("read end of pipe should not be a terminal")
	}
	if IsTerminal(int(w.Fd())) {
		t.Error("write end of pipe should not be a terminal")
	}
}

func TestIsTerminalFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "file")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer func() { _ = f.Close() }()

	if IsTerminal(int(f.Fd())) {
		t.Error("regular file should not be a terminal")
	}
}

func TestIsStdoutTTY(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	if IsStdoutTTY() {
		t.Error("stdout redirected to a pipe should not be a TTY")
	}
}

func TestIsStderrTTY(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer func() { _ = r.Close() }()
	defer func() { _ = w.Close() }()

	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()

	if IsStderrTTY() {
		t.Error("stderr redirected to a pipe should not be a TTY")
	}
}

func TestIsTerminalMatchesStdout(t *testing.T) {
	if got := IsStdoutTTY(); got != IsTerminal(int(os.Stdout.Fd())) {
		t.Errorf("IsStdoutTTY() = %v, IsTerminal(stdout) = %v", got, IsTerminal(int(os.Stdout.Fd())))
	}
}
