package projects

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
		Short: "List all projects",
		Long: `List all projects under your account.

Example:
  cerebrium projects list`,
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

	// Create API client
	client, err := api.NewClient(cfg)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to create API client: %w", err))
	}

	// Show spinner while fetching
	spinner := ui.NewSimpleSpinner("Loading projects...")
	spinner.Start()

	// Fetch projects
	projects, err := client.GetProjects(cmd.Context())
	spinner.Stop()
	if err != nil {
		return ui.NewAPIError(err)
	}

	// Print results
	if len(projects) == 0 {
		fmt.Println("No projects found")
		return nil
	}

	// Print table header and rows
	fmt.Printf("%-50s %-50s\n", "ID", "NAME")
	for _, project := range projects {
		fmt.Printf("%-50s %-50s\n", project.ID, project.Name)
	}

	fmt.Println()
	fmt.Println("You can set your current project by running `cerebrium projects set {project_id}`")

	return nil
}
