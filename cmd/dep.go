package cmd

import (
	"fmt"

	"bd-lite/internal/output"

	"github.com/spf13/cobra"
)

var depCmd = &cobra.Command{
	Use:   "dep",
	Short: "Manage dependencies",
}

var depAddCmd = &cobra.Command{
	Use:   "add <id> <depends-on-id>",
	Short: "Add a blocking dependency (id depends on depends-on-id)",
	Args:  cobra.ExactArgs(2),
	RunE:  runDepAdd,
}

var depRemoveCmd = &cobra.Command{
	Use:   "remove <id> <depends-on-id>",
	Short: "Remove a blocking dependency",
	Args:  cobra.ExactArgs(2),
	RunE:  runDepRemove,
}

var depTreeCmd = &cobra.Command{
	Use:   "tree <id>",
	Short: "Show dependency tree",
	Args:  cobra.ExactArgs(1),
	RunE:  runDepTree,
}

func init() {
	depCmd.AddCommand(depAddCmd)
	depCmd.AddCommand(depRemoveCmd)
	depCmd.AddCommand(depTreeCmd)
	rootCmd.AddCommand(depCmd)
}

func runDepAdd(cmd *cobra.Command, args []string) error {
	issueID, err := st.ResolveID(args[0])
	if err != nil {
		return err
	}
	dependsOnID, err := st.ResolveID(args[1])
	if err != nil {
		return err
	}

	if err := st.AddDependency(issueID, dependsOnID); err != nil {
		return err
	}

	if err := saveStore(); err != nil {
		return err
	}

	output.PrintMessage(fmt.Sprintf("Added dependency: %s depends on %s", issueID, dependsOnID))
	return nil
}

func runDepRemove(cmd *cobra.Command, args []string) error {
	issueID, err := st.ResolveID(args[0])
	if err != nil {
		return err
	}
	dependsOnID, err := st.ResolveID(args[1])
	if err != nil {
		return err
	}

	if err := st.RemoveDependency(issueID, dependsOnID); err != nil {
		return err
	}

	if err := saveStore(); err != nil {
		return err
	}

	output.PrintMessage(fmt.Sprintf("Removed dependency: %s no longer depends on %s", issueID, dependsOnID))
	return nil
}

func runDepTree(cmd *cobra.Command, args []string) error {
	id, err := st.ResolveID(args[0])
	if err != nil {
		return err
	}

	tree, err := st.DepTree(id)
	if err != nil {
		return err
	}

	output.PrintTree(tree, "", true)
	return nil
}
