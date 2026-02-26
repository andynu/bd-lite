package cmd

import (
	"time"

	"bd-lite/internal/output"
	"bd-lite/internal/types"

	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Permanently delete closed issues",
	RunE:  runCleanup,
}

var (
	cleanupOlderThan int
	cleanupDryRun    bool
)

func init() {
	cleanupCmd.Flags().IntVar(&cleanupOlderThan, "older-than", 0, "Only delete issues closed more than N days ago")
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "Show what would be deleted without doing it")
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) error {
	cutoff := time.Time{}
	if cleanupOlderThan > 0 {
		cutoff = time.Now().AddDate(0, 0, -cleanupOlderThan)
	}

	var toDelete []string
	for _, issue := range st.AllIssues() {
		if issue.Status != types.StatusClosed {
			continue
		}
		if !cutoff.IsZero() && issue.ClosedAt != nil && issue.ClosedAt.After(cutoff) {
			continue
		}
		toDelete = append(toDelete, issue.ID)
	}

	if cleanupDryRun {
		output.PrintCleanupResult(len(toDelete), true)
		return nil
	}

	for _, id := range toDelete {
		st.Delete(id)
	}

	if err := saveStore(); err != nil {
		return err
	}

	output.PrintCleanupResult(len(toDelete), false)
	return nil
}
