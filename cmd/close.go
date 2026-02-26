package cmd

import (
	"fmt"
	"time"

	"bd-lite/internal/output"
	"bd-lite/internal/types"

	"github.com/spf13/cobra"
)

var closeCmd = &cobra.Command{
	Use:   "close <id> [<id>...]",
	Short: "Close an issue",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runClose,
}

var closeReason string

func init() {
	closeCmd.Flags().StringVarP(&closeReason, "reason", "r", "", "Reason for closing")
	rootCmd.AddCommand(closeCmd)
}

func runClose(cmd *cobra.Command, args []string) error {
	ids, err := st.ResolveIDs(args)
	if err != nil {
		return err
	}

	for _, id := range ids {
		issue := st.Get(id)
		if issue == nil {
			return fmt.Errorf("issue %s not found", id)
		}

		if issue.Status == types.StatusClosed {
			return fmt.Errorf("issue %s is already closed", id)
		}

		now := time.Now()
		issue.Status = types.StatusClosed
		issue.ClosedAt = &now
		issue.CloseReason = closeReason
		issue.UpdatedAt = now

		st.Put(issue)
		output.PrintClosed(issue)
	}

	return saveStore()
}
