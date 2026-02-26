package cmd

import (
	"fmt"
	"strings"
	"time"

	"bd-lite/internal/output"
	"bd-lite/internal/types"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:     "create <title>",
	Aliases: []string{"new"},
	Short:   "Create a new issue",
	Args:    cobra.ExactArgs(1),
	RunE:    runCreate,
}

var (
	createDescription string
	createPriority    int
	createType        string
	createAssignee    string
	createLabels      []string
	createDeps        []string
)

func init() {
	createCmd.Flags().StringVarP(&createDescription, "description", "d", "", "Issue description")
	createCmd.Flags().IntVarP(&createPriority, "priority", "p", 2, "Priority (0-4)")
	createCmd.Flags().StringVarP(&createType, "type", "t", "task", "Issue type (bug/feature/task/epic/chore)")
	createCmd.Flags().StringVarP(&createAssignee, "assignee", "a", "", "Assignee")
	createCmd.Flags().StringSliceVarP(&createLabels, "labels", "l", nil, "Labels (comma-separated)")
	createCmd.Flags().StringSliceVar(&createDeps, "deps", nil, "Dependencies (type:id, e.g. blocks:bd-abc)")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	now := time.Now()
	issue := &types.Issue{
		Title:       args[0],
		Description: createDescription,
		Status:      types.StatusOpen,
		Priority:    createPriority,
		IssueType:   types.IssueType(createType),
		Assignee:    createAssignee,
		Labels:      createLabels,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := issue.Validate(); err != nil {
		return err
	}

	st.Add(issue)

	// Parse and add dependencies
	for _, dep := range createDeps {
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
