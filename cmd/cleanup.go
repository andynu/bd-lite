package cmd

import (
	"path/filepath"
	"time"

	"bd-lite/internal/output"
	"bd-lite/internal/types"

	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Archive and delete closed issues",
	Long:  "Moves closed issues to archive.jsonl and removes them from the active store. Use --no-archive to delete without archiving.",
	RunE:  runCleanup,
}

var (
	cleanupOlderThan int
	cleanupDryRun    bool
	cleanupNoArchive bool
)

func init() {
	cleanupCmd.Flags().IntVar(&cleanupOlderThan, "older-than", 0, "Only clean up issues closed more than N days ago")
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "Show what would happen without doing it")
	cleanupCmd.Flags().BoolVar(&cleanupNoArchive, "no-archive", false, "Delete without archiving")
	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) error {
	cutoff := time.Time{}
	if cleanupOlderThan > 0 {
		cutoff = time.Now().AddDate(0, 0, -cleanupOlderThan)
	}

	var toClean []*types.Issue
	for _, issue := range st.AllIssues() {
		if issue.Status != types.StatusClosed {
			continue
		}
		if !cutoff.IsZero() && issue.ClosedAt != nil && issue.ClosedAt.After(cutoff) {
			continue
		}
		toClean = append(toClean, issue)
	}

	if cleanupDryRun {
		output.PrintCleanupResult(len(toClean), cleanupNoArchive, true)
		return nil
	}

	if len(toClean) == 0 {
		output.PrintCleanupResult(0, cleanupNoArchive, false)
		return nil
	}

	// Archive unless --no-archive
	if !cleanupNoArchive {
		existing, err := st.LoadArchive()
		if err != nil {
			return err
		}
		archived := append(existing, toClean...)
		archivePath := filepath.Join(st.BeadsDir(), "archive.jsonl")
		if err := st.SaveToFile(archivePath, archived); err != nil {
			return err
		}
	}

	// Delete from active store
	for _, issue := range toClean {
		st.Delete(issue.ID)
	}

	if err := saveStore(); err != nil {
		return err
	}

	output.PrintCleanupResult(len(toClean), cleanupNoArchive, false)
	return nil
}
