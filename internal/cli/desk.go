package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/richhaase/bcr/internal/store"
)

func newDeskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "desk",
		Short: "Manage the persistent review workspace",
	}

	cmd.AddCommand(
		newDeskHistoryCmd(),
		newDeskForgetCmd(),
	)

	return cmd
}

func newDeskHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history <owner/repo#number>",
		Short: "Show chronological review history for a PR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prRef := args[0]
			out := cmd.OutOrStdout()
			st := store.NewStore()
			records, err := st.ListRuns(prRef)
			if err != nil {
				return err
			}
			if len(records) == 0 {
				fmt.Fprintf(out, "No review history for %s\n", prRef)
				return nil
			}

			sort.Slice(records, func(i, j int) bool {
				return records[i].Timestamp.Before(records[j].Timestamp)
			})

			fmt.Fprintf(out, "Review history for %s (%d run(s)):\n\n", prRef, len(records))
			for _, r := range records {
				kept := 0
				for _, f := range r.Final {
					if f.Keep {
						kept++
					}
				}
				fmt.Fprintf(out, "Run: %s\n", r.ID)
				fmt.Fprintf(out, "  Date:      %s\n", r.Timestamp.Format("2006-01-02 15:04:05 MST"))
				fmt.Fprintf(out, "  Models:    %s\n", joinStrings(r.Models))
				fmt.Fprintf(out, "  Findings:  %d total, %d kept\n", len(r.Final), kept)
				fmt.Fprintf(out, "  Tokens:    %d prompt + %d completion\n", r.PromptTokens, r.CompletionTokens)
				fmt.Fprintf(out, "  Est Cost:  $%.6f\n", r.EstimatedCostUSD)
				fmt.Fprintln(out)
			}
			return nil
		},
	}
}

func newDeskForgetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "forget <owner/repo#number>",
		Short: "Permanently delete stored history for a PR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prRef := args[0]
			out := cmd.OutOrStdout()
			st := store.NewStore()
			removed, err := st.ForgetPR(prRef)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "Removed %d stored record(s) for %s\n", removed, prRef)
			return nil
		},
	}
}

func joinStrings(in []string) string {
	out := ""
	for i, s := range in {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
