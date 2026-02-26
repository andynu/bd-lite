package cmd

import (
	"fmt"
	"time"

	"bd-lite/internal/output"
	"bd-lite/internal/types"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Modify issue fields",
	Args:  cobra.ExactArgs(1),
	RunE:  runUpdate,
}

var (
	updateStatus      string
	updatePriority    int
	updateTitle       string
	updateDescription string
	updateAssignee    string
	updateType        string
	updateLabels      []string
)

func init() {
	updateCmd.Flags().StringVar(&updateStatus, "status", "", "New status")
	updateCmd.Flags().IntVar(&updatePriority, "priority", -1, "New priority (0-4)")
	updateCmd.Flags().StringVar(&updateTitle, "title", "", "New title")
	updateCmd.Flags().StringVar(&updateDescription, "description", "", "New description")
	updateCmd.Flags().StringVar(&updateAssignee, "assignee", "", "New assignee")
	updateCmd.Flags().StringVar(&updateType, "type", "", "New issue type")
	updateCmd.Flags().StringSliceVar(&updateLabels, "labels", nil, "Set labels")
	rootCmd.AddCommand(updateCmd)
}

func runUpdate(cmd *cobra.Command, args []string) error {
	id, err := st.ResolveID(args[0])
	if err != nil {
		return err
	}

	issue := st.Get(id)
	if issue == nil {
		return fmt.Errorf("issue %s not found", id)
	}

	changed := false

	if updateStatus != "" {
		newStatus := types.Status(updateStatus)
		if !newStatus.IsValid() {
			return fmt.Errorf("invalid status: %s", updateStatus)
		}

		// Enforce closed_at invariant
		if newStatus == types.StatusClosed && issue.Status != types.StatusClosed {
			now := time.Now()
			issue.ClosedAt = &now
		}
		if newStatus != types.StatusClosed && issue.Status == types.StatusClosed {
			issue.ClosedAt = nil
			issue.CloseReason = ""
		}

		issue.Status = newStatus
		changed = true
	}

	if updatePriority >= 0 {
		if updatePriority > 4 {
			return fmt.Errorf("priority must be between 0 and 4")
		}
		issue.Priority = updatePriority
		changed = true
	}

	if cmd.Flags().Changed("title") {
		issue.Title = updateTitle
		changed = true
	}

	if cmd.Flags().Changed("description") {
		issue.Description = updateDescription
		changed = true
	}

	if cmd.Flags().Changed("assignee") {
		issue.Assignee = updateAssignee
		changed = true
	}

	if updateType != "" {
		t := types.IssueType(updateType)
		if !t.IsValid() {
			return fmt.Errorf("invalid issue type: %s", updateType)
		}
		issue.IssueType = t
		changed = true
	}

	if cmd.Flags().Changed("labels") {
		issue.Labels = updateLabels
		changed = true
	}

	if !changed {
		return fmt.Errorf("no fields to update (use --status, --priority, --title, etc.)")
	}

	issue.UpdatedAt = time.Now()

	if err := issue.Validate(); err != nil {
		return err
	}

	st.Put(issue)
	if err := saveStore(); err != nil {
		return err
	}

	output.PrintUpdated(issue)
	return nil
}
