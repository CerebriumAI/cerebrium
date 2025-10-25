package commands

import (
	"fmt"

	"github.com/cerebriumai/cerebrium/internal/version"
	"github.com/spf13/cobra"
)

// NewVersionCmd creates the version command
func NewVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Long:  "Print the version number of the Cerebrium CLI",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.GetFullVersion())
		},
	}
}
