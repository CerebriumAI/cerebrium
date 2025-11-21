package runs

import (
	"github.com/spf13/cobra"
)

// NewRunsCmd creates the runs command group
func NewRunsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runs",
		Short: "Manage app runs",
		Long:  "Commands for managing and viewing runs for your Cerebrium applications",
	}

	// Add subcommands
	cmd.AddCommand(newListCmd())

	return cmd
}
