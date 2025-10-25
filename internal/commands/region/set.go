package region

import (
	"fmt"
	"strings"

	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
)

func newSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <region>",
		Short: "Set the default region",
		Long: `Set the default region for deployments and storage operations.

The region will be used when no region is explicitly specified in commands.

Examples:
  cerebrium region set us-west-2
  cerebrium region set eu-west-1
  cerebrium region set us-east-1`,
		Args: cobra.ExactArgs(1),
		RunE: runSet,
	}
}

func runSet(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	region := strings.TrimSpace(args[0])

	// Validate region is not empty
	if region == "" {
		return fmt.Errorf("region cannot be empty")
	}

	// Get config from context (loaded once in root command)
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Update region
	cfg.DefaultRegion = region

	// Save config
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Default region successfully set to: %s\n", region)
	return nil
}
