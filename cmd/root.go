package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bd-lite/internal/output"
	"bd-lite/internal/store"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "bd",
	Short:         "Lightweight JSONL issue tracker",
	SilenceErrors: true,
	SilenceUsage:  true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip store loading for init command
		if cmd.Name() == "init" {
			return nil
		}
		return loadStore()
	},
}

var (
	jsonFlag bool
	st       *store.Store
)

func init() {
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "Output in JSON format")
}

func Execute() error {
	err := rootCmd.Execute()
	if err == nil {
		return nil
	}

	// If cobra doesn't recognize the command, try bd-<cmd> in PATH
	msg := err.Error()
	if strings.HasPrefix(msg, "unknown command") {
		args := os.Args[1:]
		if len(args) > 0 {
			ext, lookErr := exec.LookPath("bd-" + args[0])
			if lookErr == nil {
				cmd := exec.Command(ext, args[1:]...)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if runErr := cmd.Run(); runErr != nil {
					if exitErr, ok := runErr.(*exec.ExitError); ok {
						os.Exit(exitErr.ExitCode())
					}
					return runErr
				}
				return nil
			}
		}
	}

	return err
}

func loadStore() error {
	output.JSONMode = jsonFlag

	beadsDir, err := findBeadsDir()
	if err != nil {
		return err
	}

	st, err = store.Load(beadsDir)
	if err != nil {
		return fmt.Errorf("failed to load store: %w", err)
	}
	return nil
}

func findBeadsDir() (string, error) {
	if envDir := os.Getenv("BEADS_DIR"); envDir != "" {
		if _, err := os.Stat(envDir); err == nil {
			return envDir, nil
		}
	}

	// Walk up from cwd looking for .beads/
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		candidate := filepath.Join(dir, ".beads")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no .beads directory found (hint: run 'bd init' first)")
}

func saveStore() error {
	return st.Save()
}
