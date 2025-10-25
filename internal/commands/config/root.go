package config

import (
	"github.com/spf13/cobra"
)

// NewConfigCmd creates the config command group
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage CLI configuration",
		Long: `Manage CLI configuration settings.

Configuration is stored in ~/.cerebrium/config.yaml

Available subcommands:
  set   - Set a configuration value
  get   - Get a configuration value
  list  - List all configuration
  edit  - Open config file in editor`,
	}

	// Add subcommands
	cmd.AddCommand(newSetCmd())
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newEditCmd())

	return cmd
}
