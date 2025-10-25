package region

import (
	"fmt"

	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "Get the current default region",
		Long: `Get the current default region for deployments and storage operations.

If no region is set, defaults to 'us-east-1'.

Example:
  cerebrium region get`,
		Args: cobra.NoArgs,
		RunE: runGet,
	}
}

func runGet(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	// Get config from context (loaded once in root command)
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Get default region (with fallback to us-east-1)
	region := cfg.DefaultRegion
	if region == "" {
		region = "us-east-1"
	}

	fmt.Printf("Default region: %s\n", region)
	return nil
}
