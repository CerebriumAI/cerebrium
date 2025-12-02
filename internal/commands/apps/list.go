package apps

import (
	"fmt"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all apps",
		Long: `List all apps under your current context.

Example:
  cerebrium apps list`,
		RunE: runList,
	}

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	// Get config from context
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get config: %w", err))
	}

	// Get current project
	projectID, err := cfg.GetCurrentProject()
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get current project: %w", err))
	}

	// Create API client
	client, err := api.NewClient(cfg)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to create API client: %w", err))
	}

	// Show spinner while fetching
	spinner := ui.NewSimpleSpinner("Loading apps...")
	spinner.Start()

	// Fetch apps
	apps, err := client.GetApps(cmd.Context(), projectID)
	spinner.Stop()
	if err != nil {
		return ui.NewAPIError(err)
	}

	// Print results
	if len(apps) == 0 {
		fmt.Printf("No apps found for project %s\n", projectID)
		return nil
	}

	// Print table header and rows
	fmt.Printf("%-50s %-10s %-20s %-20s\n", "ID", "STATUS", "CREATED", "UPDATED")
	for _, app := range apps {
		fmt.Printf("%-50s %-10s %-20s %-20s\n",
			app.ID,
			app.Status,
			app.CreatedAt.Format("2006-01-02 15:04:05"),
			app.UpdatedAt.Format("2006-01-02 15:04:05"),
		)
	}

	return nil
}
