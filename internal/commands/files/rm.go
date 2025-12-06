package files

import (
	"fmt"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
)

func NewRmCmd() *cobra.Command {
	var region string

	cmd := &cobra.Command{
		Use:   "rm <remote_path>",
		Short: "Remove files from persistent storage",
		Long: `Remove files or directories from persistent storage.

Note: Directory paths must end with a forward slash (/).

Examples:
  cerebrium rm /file_name.txt              # Remove a specific file
  cerebrium rm /sub_folder/file_name.txt   # Remove file in subdirectory
  cerebrium rm /sub_folder/                # Remove directory (must end with /)
  cerebrium rm --region us-west-2 /file    # Remove from specific region`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRm(cmd, args, region)
		},
	}

	cmd.Flags().StringVarP(&region, "region", "r", "", "Region for the storage volume")

	return cmd
}

func runRm(cmd *cobra.Command, args []string, region string) error {
	// Suppress Cobra's default error handling
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	remotePath := args[0]

	// Load config
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get current project
	projectID, err := cfg.GetCurrentProject()
	if err != nil {
		return fmt.Errorf("failed to get current project: %w", err)
	}

	// Use provided region or fall back to default
	actualRegion := region
	if actualRegion == "" {
		actualRegion = cfg.GetDefaultRegion()
	}

	// Create API client
	client, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Delete the file/directory
	if err := client.DeleteFile(cmd.Context(), projectID, remotePath, actualRegion); err != nil {
		return ui.NewAPIError(err)
	}

	// Print success message
	successMsg := fmt.Sprintf("%s removed successfully.", remotePath)
	fmt.Println(ui.SuccessStyle.Render(successMsg))

	return nil
}
