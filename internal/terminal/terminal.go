package terminal

import (
	"os"

	"golang.org/x/term"
)

func IsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}

func IsStdoutTTY() bool {
	return IsTerminal(int(os.Stdout.Fd()))
}

func IsStderrTTY() bool {
	return IsTerminal(int(os.Stderr.Fd()))
}
