package projects

import (
	"github.com/spf13/cobra"
)

// NewProjectsCmd creates the project command group
func NewProjectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "projects",
		Short:   "Manage Cerebrium projects (alias: `project`)",
		Long:    "Commands for managing your Cerebrium projects and project context",
		Aliases: []string{"project"}, // Legacy
	}

	// Add subcommands
	cmd.AddCommand(newCurrentCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newSetCmd())

	return cmd
}
