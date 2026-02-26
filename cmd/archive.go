package cmd

import (
	"path/filepath"
	"time"

	"bd-lite/internal/output"
	"bd-lite/internal/types"

	"github.com/spf13/cobra"
)

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Move closed issues to archive.jsonl",
	RunE:  runArchive,
}

var (
	archiveOlderThan int
	archiveDryRun    bool
)

func init() {
	archiveCmd.Flags().IntVar(&archiveOlderThan, "older-than", 0, "Only archive issues closed more than N days ago")
	archiveCmd.Flags().BoolVar(&archiveDryRun, "dry-run", false, "Show what would be archived without doing it")
	rootCmd.AddCommand(archiveCmd)
}

func runArchive(cmd *cobra.Command, args []string) error {
	cutoff := time.Time{}
	if archiveOlderThan > 0 {
		cutoff = time.Now().AddDate(0, 0, -archiveOlderThan)
	}

	// Find closed issues to archive
	var toArchive []*types.Issue
	for _, issue := range st.AllIssues() {
		if issue.Status != types.StatusClosed {
			continue
		}
		if !cutoff.IsZero() && issue.ClosedAt != nil && issue.ClosedAt.After(cutoff) {
			continue
		}
		toArchive = append(toArchive, issue)
	}

	if archiveDryRun {
		output.PrintArchiveResult(len(toArchive), true)
		return nil
	}

	if len(toArchive) == 0 {
		output.PrintArchiveResult(0, false)
		return nil
	}

	// Load existing archive
	existing, err := st.LoadArchive()
	if err != nil {
		return err
	}

	// Append to archive
	archived := append(existing, toArchive...)
	archivePath := filepath.Join(st.BeadsDir(), "archive.jsonl")
	if err := st.SaveToFile(archivePath, archived); err != nil {
		return err
	}

	// Remove from active issues
	for _, issue := range toArchive {
		st.Delete(issue.ID)
	}

	if err := saveStore(); err != nil {
		return err
	}

	output.PrintArchiveResult(len(toArchive), false)
	return nil
}
