package containers

import (
	"fmt"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list APP_NAME",
		Short: "List containers for an app and their states",
		Long: `List the recent containers for an app along with their state, including any
that are currently being torn down (shown as TERMINATING).

Example:
  cerebrium containers list my-app
  cerebrium containers list p-abc12345-my-app
  cerebrium containers list my-app --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, args[0])
		},
	}

	ui.AddOutputFlag(cmd)

	return cmd
}

func runList(cmd *cobra.Command, appName string) error {
	cmd.SilenceUsage = true

	outputFormat, err := ui.ParseOutputFormat(cmd)
	if err != nil {
		return err
	}

	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get config: %w", err))
	}

	projectID, err := cfg.GetCurrentProject()
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("no project selected: %w", err))
	}

	appID := api.NormalizeAppID(projectID, appName)

	client, err := api.NewClient(cfg)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to create API client: %w", err))
	}

	spinner := ui.NewSimpleSpinnerFor(outputFormat, "Loading containers...")
	spinner.Start()

	containers, err := client.ListContainers(cmd.Context(), projectID, appID)
	spinner.Stop()
	if err != nil {
		return ui.NewAPIError(err)
	}

	if outputFormat == ui.OutputJSON {
		return ui.PrintJSON(containers)
	}

	if len(containers) == 0 {
		fmt.Printf("No containers found for app: %s\n", appName)
		return nil
	}

	fmt.Printf("%-50s %-12s %-10s %-15s %s\n", "CONTAINER ID", "STATE", "RESTARTS", "REGION", "BUILD")
	for _, c := range containers {
		state := c.ContainerState
		if c.IsTerminating {
			state = "TERMINATING"
		}
		fmt.Printf("%-50s %-12s %-10d %-15s %s\n",
			c.ContainerID,
			state,
			c.ContainerRestartCount,
			c.Region,
			c.BuildID,
		)
	}

	return nil
}
