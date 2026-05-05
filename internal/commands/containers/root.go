package containers

import (
	"github.com/spf13/cobra"
)

func NewContainersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "containers",
		Short: "Manage and inspect app containers",
		Long:  "Commands for inspecting the containers (knative pods) running your Cerebrium apps",
	}

	cmd.AddCommand(newListCmd())

	return cmd
}
