package output

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"bd-lite/internal/store"
	"bd-lite/internal/types"
)

var JSONMode bool

// PrintIssue prints a single issue in human or JSON format.
func PrintIssue(issue *types.Issue) {
	if JSONMode {
		printJSON(issue)
		return
	}

	fmt.Printf("%s  %s\n", issue.ID, issue.Title)
	fmt.Printf("  Status:   %s\n", issue.Status)
	fmt.Printf("  Priority: %s\n", priorityLabel(issue.Priority))
	fmt.Printf("  Type:     %s\n", issue.IssueType)
	if issue.Assignee != "" {
		fmt.Printf("  Assignee: %s\n", issue.Assignee)
	}
	if issue.Description != "" {
		fmt.Printf("  Description:\n")
		for _, line := range strings.Split(issue.Description, "\n") {
			fmt.Printf("    %s\n", line)
		}
	}
	if len(issue.Labels) > 0 {
		fmt.Printf("  Labels:   %s\n", strings.Join(issue.Labels, ", "))
	}
	if len(issue.Dependencies) > 0 {
		fmt.Printf("  Dependencies:\n")
		for _, dep := range issue.Dependencies {
			fmt.Printf("    %s %s %s\n", dep.IssueID, dep.Type, dep.DependsOnID)
		}
	}
	if len(issue.Comments) > 0 {
		fmt.Printf("  Comments:\n")
		for _, c := range issue.Comments {
			author := c.Author
			if author == "" {
				author = "anonymous"
			}
			fmt.Printf("    [%s] %s: %s\n", c.CreatedAt.Format("2006-01-02 15:04"), author, c.Text)
		}
	}
	if issue.CloseReason != "" {
		fmt.Printf("  Closed:   %s (%s)\n", issue.ClosedAt.Format("2006-01-02"), issue.CloseReason)
	}
	fmt.Printf("  Created:  %s\n", issue.CreatedAt.Format("2006-01-02 15:04"))
	if issue.UpdatedAt != issue.CreatedAt {
		fmt.Printf("  Updated:  %s\n", issue.UpdatedAt.Format("2006-01-02 15:04"))
	}
}

// PrintIssueList prints a list of issues in table or JSON format.
func PrintIssueList(issues []*types.Issue) {
	if JSONMode {
		printJSON(issues)
		return
	}

	if len(issues) == 0 {
		fmt.Println("No issues found.")
		return
	}

	for _, issue := range issues {
		status := statusIcon(issue.Status)
		fmt.Printf("%s %s  P%d  %-12s  %s\n",
			status, issue.ID, issue.Priority, issue.IssueType, issue.Title)
	}
	fmt.Printf("\n%d issue(s)\n", len(issues))
}

// PrintTree prints a dependency tree.
func PrintTree(node *store.TreeNode, prefix string, isLast bool) {
	if JSONMode {
		printJSON(flattenTree(node))
		return
	}

	connector := "├── "
	if isLast {
		connector = "└── "
	}
	if prefix == "" {
		// Root node, no connector
		fmt.Printf("%s %s  [%s]\n", node.Issue.ID, node.Issue.Title, node.Issue.Status)
	} else {
		fmt.Printf("%s%s%s %s  [%s]\n", prefix, connector, node.Issue.ID, node.Issue.Title, node.Issue.Status)
	}

	childPrefix := prefix
	if prefix != "" {
		if isLast {
			childPrefix += "    "
		} else {
			childPrefix += "│   "
		}
	}

	for i, child := range node.Children {
		PrintTree(child, childPrefix, i == len(node.Children)-1)
	}
}

func flattenTree(node *store.TreeNode) []map[string]interface{} {
	var result []map[string]interface{}
	flattenTreeHelper(node, 0, &result)
	return result
}

func flattenTreeHelper(node *store.TreeNode, depth int, result *[]map[string]interface{}) {
	entry := map[string]interface{}{
		"id":     node.Issue.ID,
		"title":  node.Issue.Title,
		"status": node.Issue.Status,
		"depth":  depth,
	}
	*result = append(*result, entry)
	for _, child := range node.Children {
		flattenTreeHelper(child, depth+1, result)
	}
}

// PrintCreated prints a confirmation after creating an issue.
func PrintCreated(issue *types.Issue) {
	if JSONMode {
		printJSON(issue)
		return
	}
	fmt.Printf("Created %s: %s\n", issue.ID, issue.Title)
}

// PrintUpdated prints a confirmation after updating an issue.
func PrintUpdated(issue *types.Issue) {
	if JSONMode {
		printJSON(issue)
		return
	}
	fmt.Printf("Updated %s\n", issue.ID)
}

// PrintClosed prints a confirmation after closing an issue.
func PrintClosed(issue *types.Issue) {
	if JSONMode {
		printJSON(issue)
		return
	}
	reason := issue.CloseReason
	if reason == "" {
		reason = "closed"
	}
	fmt.Printf("Closed %s: %s\n", issue.ID, reason)
}

// PrintMessage prints a simple message (or JSON equivalent).
func PrintMessage(msg string) {
	if JSONMode {
		printJSON(map[string]string{"message": msg})
		return
	}
	fmt.Println(msg)
}

// PrintArchiveResult prints the result of an archive/cleanup operation.
func PrintArchiveResult(moved int, dryRun bool) {
	if JSONMode {
		printJSON(map[string]interface{}{
			"count":   moved,
			"dry_run": dryRun,
		})
		return
	}
	if dryRun {
		fmt.Printf("Would archive %d closed issue(s)\n", moved)
	} else {
		fmt.Printf("Archived %d closed issue(s)\n", moved)
	}
}

// PrintCleanupResult prints the result of a cleanup operation.
func PrintCleanupResult(deleted int, dryRun bool) {
	if JSONMode {
		printJSON(map[string]interface{}{
			"count":   deleted,
			"dry_run": dryRun,
		})
		return
	}
	if dryRun {
		fmt.Printf("Would delete %d closed issue(s)\n", deleted)
	} else {
		fmt.Printf("Deleted %d closed issue(s)\n", deleted)
	}
}

// Helpers

func printJSON(v interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	enc.Encode(v)
}

func priorityLabel(p int) string {
	switch p {
	case 0:
		return "P0 (critical)"
	case 1:
		return "P1 (high)"
	case 2:
		return "P2 (medium)"
	case 3:
		return "P3 (low)"
	case 4:
		return "P4 (backlog)"
	default:
		return fmt.Sprintf("P%d", p)
	}
}

func statusIcon(s types.Status) string {
	switch s {
	case types.StatusOpen:
		return "[ ]"
	case types.StatusInProgress:
		return "[>]"
	case types.StatusBlocked:
		return "[!]"
	case types.StatusClosed:
		return "[x]"
	default:
		return "[?]"
	}
}

// Age returns a human-readable age string.
func Age(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
