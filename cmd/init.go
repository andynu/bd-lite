package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"bd-lite/internal/output"
	"bd-lite/internal/store"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new .beads directory",
	RunE:  runInit,
}

var initPrefix string

func init() {
	initCmd.Flags().StringVar(&initPrefix, "prefix", "", "Issue ID prefix (default: directory name)")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	output.JSONMode = jsonFlag

	beadsDir := filepath.Join(".", ".beads")
	if _, err := os.Stat(beadsDir); err == nil {
		return fmt.Errorf(".beads directory already exists")
	}

	if err := os.MkdirAll(beadsDir, 0755); err != nil {
		return fmt.Errorf("create .beads: %w", err)
	}

	// Create empty issues.jsonl
	jsonlPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte{}, 0644); err != nil {
		return fmt.Errorf("create issues.jsonl: %w", err)
	}

	// Determine prefix
	prefix := initPrefix
	if prefix == "" {
		cwd, err := os.Getwd()
		if err != nil {
			prefix = "bd"
		} else {
			prefix = store.SanitizePrefix(filepath.Base(cwd))
			if prefix == "" {
				prefix = "bd"
			}
		}
	}

	// Write config.yaml
	configPath := filepath.Join(beadsDir, "config.yaml")
	configContent := fmt.Sprintf("issue-prefix: %s\n", prefix)
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("create config.yaml: %w", err)
	}

	output.PrintMessage(fmt.Sprintf("Initialized .beads with prefix '%s'", prefix))
	return nil
}
