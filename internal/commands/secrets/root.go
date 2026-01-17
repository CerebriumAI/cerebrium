package secrets

import (
	"strings"

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

// expandAppID expands a short app name to a full app ID if needed.
// If appID is empty, returns empty string.
// If appID already has the project prefix, returns as-is.
// Otherwise, prepends the project ID.
func expandAppID(appID, projectID string) string {
	if appID == "" {
		return ""
	}
	if strings.HasPrefix(appID, projectID+"-") {
		return appID
	}
	return projectID + "-" + appID
}
