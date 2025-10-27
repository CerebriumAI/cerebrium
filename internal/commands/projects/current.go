package projects

import (
	"fmt"

	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
)

func newCurrentCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Display current project context",
		Long: `Display the current project you are working in.

This shows the project ID that is currently set in your configuration.

Example:
  cerebrium projects current`,
		RunE: runCurrent,
	}

	return cmd
}

func runCurrent(cmd *cobra.Command, args []string) error {
	// Suppress Cobra's default error handling - we control it in main.go
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	// Get config from context (loaded once in root command)
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Get current project
	projectID, err := cfg.GetCurrentProject()
	if err != nil {
		return fmt.Errorf("failed to get current project: %w", err)
	}

	// Print the project ID
	fmt.Printf("projectId: %s\n", projectID)

	return nil
}
