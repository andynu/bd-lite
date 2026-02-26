package cmd

import (
	"bd-lite/internal/output"

	"github.com/spf13/cobra"
)

var readyCmd = &cobra.Command{
	Use:   "ready",
	Short: "Show issues ready to work on (open/in_progress, no blockers)",
	RunE:  runReady,
}

func init() {
	rootCmd.AddCommand(readyCmd)
}

func runReady(cmd *cobra.Command, args []string) error {
	issues := st.Ready()
	output.PrintIssueList(issues)
	return nil
}
