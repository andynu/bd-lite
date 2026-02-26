package cmd

import (
	"os/user"

	"bd-lite/internal/output"

	"github.com/spf13/cobra"
)

var commentCmd = &cobra.Command{
	Use:   "comment <id> <text>",
	Short: "Add a comment to an issue",
	Args:  cobra.ExactArgs(2),
	RunE:  runComment,
}

func init() {
	rootCmd.AddCommand(commentCmd)
}

func runComment(cmd *cobra.Command, args []string) error {
	id, err := st.ResolveID(args[0])
	if err != nil {
		return err
	}

	author := ""
	if u, err := user.Current(); err == nil {
		author = u.Username
	}

	if err := st.AddComment(id, args[1], author); err != nil {
		return err
	}

	if err := saveStore(); err != nil {
		return err
	}

	issue := st.Get(id)
	// Return the newly added comment (last in the list), matching beads behavior
	comment := issue.Comments[len(issue.Comments)-1]
	output.PrintComment(comment)
	return nil
}
