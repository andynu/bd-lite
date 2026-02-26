package cmd

import (
	"bd-lite/internal/output"
	"bd-lite/internal/store"
	"bd-lite/internal/types"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List issues (excludes closed by default)",
	RunE:  runList,
}

var (
	listStatus   string
	listPriority int
	listType     string
	listAssignee string
	listContent  string
	listAll      bool
)

func init() {
	listCmd.Flags().StringVarP(&listStatus, "status", "s", "", "Filter by status")
	listCmd.Flags().IntVarP(&listPriority, "priority", "p", -1, "Filter by priority")
	listCmd.Flags().StringVarP(&listType, "type", "t", "", "Filter by type")
	listCmd.Flags().StringVarP(&listAssignee, "assignee", "a", "", "Filter by assignee")
	listCmd.Flags().StringVarP(&listContent, "content", "c", "", "Search in title and description")
	listCmd.Flags().BoolVar(&listAll, "all", false, "Include closed issues")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	opts := store.FilterOpts{}

	if listStatus != "" {
		s := types.Status(listStatus)
		opts.Status = &s
	} else if !listAll {
		// Default: exclude closed
		excludeClosed := true
		_ = excludeClosed
		// We'll filter manually since FilterOpts doesn't have "not" logic
	}

	if listPriority >= 0 {
		opts.Priority = &listPriority
	}
	if listType != "" {
		t := types.IssueType(listType)
		opts.IssueType = &t
	}
	if listAssignee != "" {
		opts.Assignee = &listAssignee
	}
	if listContent != "" {
		opts.Content = listContent
	}

	issues := st.Filter(opts)

	// Exclude closed unless --all or --status=closed
	if listStatus == "" && !listAll {
		filtered := make([]*types.Issue, 0, len(issues))
		for _, issue := range issues {
			if issue.Status != types.StatusClosed {
				filtered = append(filtered, issue)
			}
		}
		issues = filtered
	}

	output.PrintIssueList(issues)
	return nil
}
