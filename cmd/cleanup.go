package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bd-lite/internal/output"
	"bd-lite/internal/types"

	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	cleanupYes       bool
)

// isInteractive reports whether the process's stdin is an interactive
// terminal. Overridden in tests to force the non-interactive path without
// needing a real TTY.
//
// This must use a real ioctl-based TTY check (term.IsTerminal), not an
// os.ModeCharDevice test: /dev/null is a character device too, so that
// cheaper check misreads the most common non-interactive case -- stdin
// redirected from /dev/null by a background worker or hook -- as interactive.
var isInteractive = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func init() {
	cleanupCmd.Flags().IntVar(&cleanupOlderThan, "older-than", 0, "Only clean up issues closed more than N days ago")
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "Show what would happen without doing it")
	cleanupCmd.Flags().BoolVar(&cleanupNoArchive, "no-archive", false, "Delete without archiving")
	cleanupCmd.Flags().BoolVar(&cleanupYes, "yes", false, "Skip the confirmation prompt (required when stdin is not a terminal)")
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
		if len(toClean) > 0 {
			fmt.Printf("Backup would be written to %s\n", cleanupBackupPath(st.BeadsDir(), time.Now()))
		}
		return nil
	}

	if len(toClean) == 0 {
		output.PrintCleanupResult(0, cleanupNoArchive, false)
		return nil
	}

	backupPath := cleanupBackupPath(st.BeadsDir(), time.Now())

	if !cleanupYes {
		if !isInteractive() {
			return fmt.Errorf("refusing to delete %d closed issue(s): stdin is not a terminal; pass --yes to confirm non-interactively", len(toClean))
		}
		confirmed, err := confirmCleanupDeletion(cmd, len(toClean), backupPath)
		if err != nil {
			return err
		}
		if !confirmed {
			output.PrintMessage("Aborted; no issues deleted.")
			return nil
		}
	}

	issuesPath := filepath.Join(st.BeadsDir(), "issues.jsonl")
	if err := writeCleanupBackup(issuesPath, backupPath); err != nil {
		return fmt.Errorf("backup before cleanup: %w", err)
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

// confirmCleanupDeletion prints the pending deletion count and backup path,
// then reads a y/N answer from cmd's stdin. Only an explicit "y"/"yes"
// (case-insensitive) counts as confirmation.
func confirmCleanupDeletion(cmd *cobra.Command, count int, backupPath string) (bool, error) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "This will delete %d closed issue(s).\n", count)
	fmt.Fprintf(out, "Backup: %s\n", backupPath)
	fmt.Fprint(out, "Continue? [y/N] ")

	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// cleanupBackupPath returns where writeCleanupBackup would write a backup
// taken at ts. --no-archive skips the user-facing archive.jsonl, never this.
func cleanupBackupPath(beadsDir string, ts time.Time) string {
	return filepath.Join(beadsDir, ".cleanup-backups", fmt.Sprintf("issues-%s.jsonl", ts.UTC().Format(time.RFC3339)))
}

// writeCleanupBackup copies issuesPath byte-for-byte to backupPath, trash-style,
// so a bad cleanup can always be undone by restoring the copy.
func writeCleanupBackup(issuesPath, backupPath string) error {
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return err
	}

	src, err := os.Open(issuesPath)
	if err != nil {
		return err
	}
	defer src.Close()

	tmpPath := backupPath + ".tmp"
	dst, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := dst.Close(); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, backupPath)
}
