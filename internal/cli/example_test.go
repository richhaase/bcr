package cli

import (
	"bytes"
)

func executeCommand(args ...string) (string, string, error) {
	root := NewRootCmd(BuildInfo{})
	root.SetArgs(args)
	var out bytes.Buffer
	var errOut bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errOut)
	err := root.Execute()
	return out.String(), errOut.String(), err
}
