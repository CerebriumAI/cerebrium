package metrics

import (
	"github.com/spf13/cobra"
)

func NewMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Inspect resource usage for an app",
		Long:  "Commands for inspecting how much hardware your Cerebrium apps are actually using",
	}

	cmd.AddCommand(newResourcesCmd())

	return cmd
}
