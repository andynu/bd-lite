package cmd

import (
	"fmt"

	"bd-lite/internal/output"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show <id> [<id>...]",
	Short: "Display full issue details",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	ids, err := st.ResolveIDs(args)
	if err != nil {
		return err
	}

	for i, id := range ids {
		issue := st.Get(id)
		if issue == nil {
			return fmt.Errorf("issue %s not found", id)
		}
		if i > 0 && !output.JSONMode {
			fmt.Println()
		}
		output.PrintIssue(issue)
	}
	return nil
}
