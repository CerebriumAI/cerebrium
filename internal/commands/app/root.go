package app

import (
	"github.com/spf13/cobra"
)

// NewAppCmd creates the app command group
func NewAppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Manage Cerebrium apps",
		Long:  "Commands for managing your Cerebrium applications",
	}

	// Add subcommands
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newScaleCmd())

	return cmd
}
