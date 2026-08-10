package secrets

import (
	"fmt"
	"sort"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
)

// NewListCmd creates the secrets list command
func NewListCmd() *cobra.Command {
	var showValues bool
	var appID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all secrets for the current project or a specific app",
		Long: `List all secrets configured for the current project or a specific app.

By default, secret values are hidden for security. Use --show-values to display them.

Examples:
  cerebrium secrets list                       # List project secrets (names only)
  cerebrium secrets list --show-values         # List project secrets with values
  cerebrium secrets list --app my-app          # List app-specific secrets
  cerebrium secrets list --app my-app --show-values
  cerebrium secrets list --output json`,
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, showValues, appID)
		},
	}

	cmd.Flags().BoolVar(&showValues, "show-values", false, "Show secret values (hidden by default)")
	cmd.Flags().StringVar(&appID, "app", "", "App ID to list secrets for (if not specified, lists project secrets)")
	ui.AddOutputFlag(cmd)

	return cmd
}

// jsonSecret omits Value unless --show-values was passed, so JSON output leaks no
// more than the table does.
type jsonSecret struct {
	Name  string  `json:"name"`
	Value *string `json:"value,omitempty"`
}

// newJSONSecrets builds the JSON payload in the given key order. Values are
// attached only when the caller asked to see them; without that, a name is all
// that leaves this function.
func newJSONSecrets(keys []string, secrets map[string]string, showValues bool) []jsonSecret {
	out := make([]jsonSecret, 0, len(keys))
	for _, key := range keys {
		entry := jsonSecret{Name: key}
		if showValues {
			value := secrets[key]
			entry.Value = &value
		}
		out = append(out, entry)
	}
	return out
}

func runList(cmd *cobra.Command, showValues bool, appID string) error {
	cmd.SilenceUsage = true

	outputFormat, err := ui.ParseOutputFormat(cmd)
	if err != nil {
		return err
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

	// Show spinner while fetching
	spinnerMsg := "Loading secrets..."
	if appID != "" {
		spinnerMsg = fmt.Sprintf("Loading secrets for app %s...", appID)
	}
	spinner := ui.NewSimpleSpinnerFor(outputFormat, spinnerMsg)
	spinner.Start()

	// Fetch secrets (project or app level)
	var secrets map[string]string
	fullAppID := expandAppID(appID, projectID)
	if fullAppID != "" {
		secrets, err = client.ListAppSecrets(cmd.Context(), projectID, fullAppID)
	} else {
		secrets, err = client.ListSecrets(cmd.Context(), projectID)
	}
	spinner.Stop()
	if err != nil {
		return ui.NewAPIError(err)
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if outputFormat == ui.OutputJSON {
		return ui.PrintJSON(newJSONSecrets(keys, secrets, showValues))
	}

	// Handle empty secrets
	if len(secrets) == 0 {
		fmt.Println("No secrets found")
		return nil
	}

	// Calculate max key width for alignment
	maxKeyWidth := len("NAME")
	for _, key := range keys {
		if len(key) > maxKeyWidth {
			maxKeyWidth = len(key)
		}
	}
	keyWidth := maxKeyWidth + 2

	// Print table
	if showValues {
		fmt.Printf("%-*s %s\n", keyWidth, "NAME", "VALUE")
		for _, key := range keys {
			fmt.Printf("%-*s %s\n", keyWidth, key, secrets[key])
		}
	} else {
		fmt.Println("NAME")
		for _, key := range keys {
			fmt.Println(key)
		}
		fmt.Printf("\nTotal: %d secret(s). Use --show-values to display values.\n", len(keys))
	}

	return nil
}
