package cmd

import (
	"fmt"
	"strings"
	"time"

	"bd-lite/internal/output"
	"bd-lite/internal/types"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <title>",
	Short: "Create a new issue",
	Args:  cobra.ExactArgs(1),
	RunE:  runAdd,
}

var (
	addDescription string
	addPriority    int
	addType        string
	addAssignee    string
	addLabels      []string
	addDeps        []string
)

func init() {
	addCmd.Flags().StringVarP(&addDescription, "description", "d", "", "Issue description")
	addCmd.Flags().IntVarP(&addPriority, "priority", "p", 2, "Priority (0-4)")
	addCmd.Flags().StringVarP(&addType, "type", "t", "task", "Issue type (bug/feature/task/epic/chore)")
	addCmd.Flags().StringVarP(&addAssignee, "assignee", "a", "", "Assignee")
	addCmd.Flags().StringSliceVarP(&addLabels, "labels", "l", nil, "Labels (comma-separated)")
	addCmd.Flags().StringSliceVar(&addDeps, "deps", nil, "Dependencies (type:id, e.g. blocks:bd-abc)")
	rootCmd.AddCommand(addCmd)
}

func runAdd(cmd *cobra.Command, args []string) error {
	now := time.Now()
	issue := &types.Issue{
		Title:       args[0],
		Description: addDescription,
		Status:      types.StatusOpen,
		Priority:    addPriority,
		IssueType:   types.IssueType(addType),
		Assignee:    addAssignee,
		Labels:      addLabels,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := issue.Validate(); err != nil {
		return err
	}

	st.Add(issue)

	// Parse and add dependencies
	for _, dep := range addDeps {
		parts := strings.SplitN(dep, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid dependency format '%s' (expected type:id)", dep)
		}
		depType := types.DependencyType(parts[0])
		if !depType.IsValid() {
			return fmt.Errorf("invalid dependency type '%s'", parts[0])
		}

		depID, err := st.ResolveID(parts[1])
		if err != nil {
			return fmt.Errorf("dependency target: %w", err)
		}

		issue.Dependencies = append(issue.Dependencies, &types.Dependency{
			IssueID:     issue.ID,
			DependsOnID: depID,
			Type:        depType,
			CreatedAt:   now,
		})
	}

	if err := saveStore(); err != nil {
		return err
	}

	output.PrintCreated(issue)
	return nil
}
