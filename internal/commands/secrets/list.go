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

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all secrets for the current project",
		Long: `List all secrets configured for the current project.

By default, secret values are hidden for security. Use --show-values to display them.

Examples:
  cerebrium secrets list                  # List secret names only
  cerebrium secrets list --show-values    # List secrets with their values`,
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, showValues)
		},
	}

	cmd.Flags().BoolVar(&showValues, "show-values", false, "Show secret values (hidden by default)")

	return cmd
}

func runList(cmd *cobra.Command, showValues bool) error {
	cmd.SilenceUsage = true

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
	spinner := ui.NewSimpleSpinner("Loading secrets...")
	spinner.Start()

	// Fetch secrets
	secrets, err := client.ListSecrets(cmd.Context(), projectID)
	spinner.Stop()
	if err != nil {
		return ui.NewAPIError(err)
	}

	// Handle empty secrets
	if len(secrets) == 0 {
		fmt.Println("No secrets found")
		return nil
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

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
