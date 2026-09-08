package projects

import (
	"github.com/cerebriumai/cerebrium/internal/authsession"
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
	cmd.AddCommand(authsession.WithoutAuth(newCurrentCmd()))
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(authsession.WithoutAuth(newSetCmd()))

	return cmd
}
