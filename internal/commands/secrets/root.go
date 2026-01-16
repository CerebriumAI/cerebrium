package secrets

import (
	"github.com/spf13/cobra"
)

// NewSecretsCmd creates the secrets command group
func NewSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Manage project secrets",
		Long:  `Manage secrets for your Cerebrium project. Secrets are environment variables that are securely stored and made available to your applications at runtime.`,
	}

	cmd.AddCommand(NewListCmd())
	cmd.AddCommand(NewAddCmd())

	return cmd
}
