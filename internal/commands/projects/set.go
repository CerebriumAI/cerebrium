package projects

import (
	"fmt"

	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
)

func newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <project_id>",
		Short: "Set the project context you are working in",
		Long: `Set the current project context to the specified project ID.

The project ID should start with 'p-'

Example:
  cerebrium projects set p-abcd1234`,
		Args: cobra.ExactArgs(1),
		RunE: runSet,
	}

	return cmd
}

func runSet(cmd *cobra.Command, args []string) error {
	// Suppress Cobra's default error handling - we control it in main.go
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	projectID := args[0]

	// Get config from context (loaded once in root command)
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// SetCurrentProject validates and saves the project ID
	if err := cfg.SetCurrentProject(projectID); err != nil {
		return err
	}

	// Print success message
	fmt.Printf("Project context successfully set to %s\n", projectID)

	return nil
}
