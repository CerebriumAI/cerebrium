package secrets

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
)

// isValidSecretKey checks if a secret key follows Kubernetes secret key naming conventions.
// Keys must be 1-253 characters and contain only alphanumeric characters, hyphens, underscores, or dots.
func isValidSecretKey(key string) bool {
	if len(key) == 0 || len(key) > 253 {
		return false
	}
	for _, c := range key {
		isLower := c >= 'a' && c <= 'z'
		isUpper := c >= 'A' && c <= 'Z'
		isDigit := c >= '0' && c <= '9'
		isAllowed := c == '_' || c == '-' || c == '.'
		if !isLower && !isUpper && !isDigit && !isAllowed {
			return false
		}
	}
	return true
}

// NewAddCmd creates the secrets add command
func NewAddCmd() *cobra.Command {
	var appID string

	cmd := &cobra.Command{
		Use:   "add <KEY>=<VALUE> [KEY2=VALUE2 ...]",
		Short: "Add or update secrets",
		Long: `Add or update one or more secrets for the current project or a specific app.

Secrets are specified as KEY=VALUE pairs. If a secret with the same key already exists, it will be updated.

Examples:
  cerebrium secrets add API_KEY=sk-123456                    # Add project secret
  cerebrium secrets add DB_HOST=localhost DB_PORT=5432       # Add multiple project secrets
  cerebrium secrets add --app my-app API_KEY=sk-123456       # Add app-specific secret
  cerebrium secrets add "MESSAGE=Hello World"                # Secret with spaces in value`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdd(cmd, args, appID)
		},
	}

	cmd.Flags().StringVar(&appID, "app", "", "App ID to add secrets for (if not specified, adds to project secrets)")

	return cmd
}

func runAdd(cmd *cobra.Command, args []string, appID string) error {
	cmd.SilenceUsage = true

	// Parse KEY=VALUE pairs
	newSecrets := make(map[string]string)
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			return ui.NewValidationError(fmt.Errorf("invalid format: %q. Expected KEY=VALUE", arg))
		}
		key := strings.TrimSpace(parts[0])
		value := parts[1]

		if key == "" {
			return ui.NewValidationError(fmt.Errorf("invalid format: %q. Key cannot be empty", arg))
		}

		if !isValidSecretKey(key) {
			return ui.NewValidationError(fmt.Errorf("invalid secret key %q. Keys must contain only alphanumeric characters, hyphens, underscores, or dots", key))
		}

		newSecrets[key] = value
	}

	// Load config
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to load config: %w", err))
	}

	// Get current project
	projectID, err := cfg.GetCurrentProject()
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get current project: %w", err))
	}

	// Create API client
	client, err := api.NewClient(cfg)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to create API client: %w", err))
	}

	// Expand app ID if provided
	fullAppID := expandAppID(appID, projectID)

	// Show spinner while adding secrets
	spinnerMsg := "Adding secrets..."
	if fullAppID != "" {
		spinnerMsg = fmt.Sprintf("Adding secrets to app %s...", appID)
	}
	spinner := ui.NewSimpleSpinner(spinnerMsg)
	spinner.Start()

	// Fetch existing secrets to merge (project or app level)
	var existingSecrets map[string]string
	if fullAppID != "" {
		existingSecrets, err = client.ListAppSecrets(cmd.Context(), projectID, fullAppID)
	} else {
		existingSecrets, err = client.ListSecrets(cmd.Context(), projectID)
	}
	if err != nil {
		spinner.Stop()
		return ui.NewAPIError(err)
	}

	// Merge new secrets with existing ones
	if existingSecrets == nil {
		existingSecrets = make(map[string]string)
	}
	for key, value := range newSecrets {
		existingSecrets[key] = value
	}

	// Update secrets (project or app level)
	if fullAppID != "" {
		err = client.UpdateAppSecrets(cmd.Context(), projectID, fullAppID, existingSecrets)
	} else {
		err = client.UpdateSecrets(cmd.Context(), projectID, existingSecrets)
	}
	spinner.Stop()
	if err != nil {
		return ui.NewAPIError(err)
	}

	// Print success message
	keys := make([]string, 0, len(newSecrets))
	for key := range newSecrets {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	target := "project"
	if appID != "" {
		target = fmt.Sprintf("app %q", appID)
	}

	if len(keys) == 1 {
		fmt.Println(ui.SuccessStyle.Render(fmt.Sprintf("Secret %q added to %s successfully.", keys[0], target)))
	} else {
		fmt.Println(ui.SuccessStyle.Render(fmt.Sprintf("%d secrets added to %s successfully: %s", len(keys), target, strings.Join(keys, ", "))))
	}

	return nil
}
