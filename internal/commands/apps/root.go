package apps

import (
	"github.com/spf13/cobra"
)

// NewAppsCmd creates the app command group
func NewAppsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "apps",
		Short:   "Manage Cerebrium apps (alias: `app`)",
		Long:    "Commands for managing your Cerebrium applications",
		Aliases: []string{"app"}, // Legacy
	}

	// Add subcommands
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newDeleteCmd())
	cmd.AddCommand(newScaleCmd())

	return cmd
}
