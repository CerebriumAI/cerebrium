package region

import (
	"github.com/spf13/cobra"
)

// NewRegionCmd creates the region command group
func NewRegionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "region",
		Short: "Manage default region",
		Long: `Manage default region for deployments and storage operations.

The default region is used when no region is explicitly specified in commands.

Examples:
  cerebrium region get
  cerebrium region set us-west-2`,
	}

	// Add subcommands
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newSetCmd())

	return cmd
}
