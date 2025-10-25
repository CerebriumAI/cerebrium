package config

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open config file in editor",
		Long: `Open the configuration file in your default editor.

The editor is determined by (in order):
  1. $EDITOR environment variable
  2. $VISUAL environment variable
  3. Falls back to 'vi' on Unix, 'notepad' on Windows

Example:
  cerebrium config edit
  EDITOR=nano cerebrium config edit`,
		Args: cobra.NoArgs,
		RunE: runEdit,
	}
}

func runEdit(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	// Get config file path
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return fmt.Errorf("config file not found")
	}

	// Determine editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Default editor based on OS
		if runtime.GOOS == "windows" {
			editor = "notepad"
		} else {
			editor = "vi"
		}
	}

	fmt.Printf("Opening %s with %s...\n", configFile, editor)

	// Execute editor
	editorCmd := exec.CommandContext(cmd.Context(), editor, configFile) //nolint:gosec // Editor from user's environment variable
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return fmt.Errorf("failed to open editor: %w", err)
	}

	return nil
}
