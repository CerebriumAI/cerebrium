package project

import (
	"github.com/spf13/cobra"
)

// NewProjectCmd creates the project command group
func NewProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project",
		Short: "Manage Cerebrium projects",
		Long:  "Commands for managing your Cerebrium projects and project context",
	}

	// Add subcommands
	cmd.AddCommand(newCurrentCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newSetCmd())

	return cmd
}
