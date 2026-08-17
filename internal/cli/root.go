package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

func NewRootCmd(build BuildInfo) *cobra.Command {
	var verbose bool

	root := &cobra.Command{
		Use:           "bcr",
		Short:         "Bare Code Reviewer - lightweight multi-model LLM review",
		Version:       build.Version,
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			level := slog.LevelInfo
			if verbose {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{
				Level: level,
			})))
		},
	}

	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose (debug) logging")

	root.AddCommand(
		newReviewCmd(),
		newWatchCmd(),
		newConfigCmd(),
		newVersionCmd(build),
	)

	return root
}

func Execute(ctx context.Context, version, commit, date string) int {
	root := NewRootCmd(BuildInfo{Version: version, Commit: commit, Date: date})

	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}
