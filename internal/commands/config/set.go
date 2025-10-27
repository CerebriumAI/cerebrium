package config

import (
	"fmt"
	"strings"

	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a configuration value in ~/.cerebrium/config.yaml

Examples:
  cerebrium config set skip-version-check false
  cerebrium config set default-region us-west-2
  cerebrium config set dev-project dev-p-abc123`,
		Args: cobra.ExactArgs(2),
		RunE: runSet,
	}
}

func runSet(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	key := args[0]
	value := args[1]

	// Normalize the key (convert kebab-case to lowercase)
	normalizedKey := strings.ToLower(strings.ReplaceAll(key, "-", ""))

	// Validate that this is a recognized user-facing config key
	if !config.IsValidUserFacingKey(normalizedKey) {
		//nolint:errcheck // Writing to stderr, error not actionable
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: '%s' is not a recognized configuration key\n\n", key)
		//nolint:errcheck // Writing to stderr, error not actionable
		fmt.Fprintf(cmd.ErrOrStderr(), "Valid configuration keys:\n")

		// Show valid keys with descriptions
		for _, validKey := range config.GetUserFacingKeys() {
			normalized := strings.ToLower(strings.ReplaceAll(validKey, "-", ""))
			desc := config.GetConfigKeyDescription(normalized)
			if desc != "" {
				//nolint:errcheck // Writing to stderr, error not actionable
				fmt.Fprintf(cmd.ErrOrStderr(), "  %s - %s\n", validKey, desc)
			} else {
				//nolint:errcheck // Writing to stderr, error not actionable
				fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", validKey)
			}
		}

		//nolint:errcheck // Writing to stderr, error not actionable
		fmt.Fprintf(cmd.ErrOrStderr(), "\nNote: Tokens (access-token, refresh-token) are managed automatically via 'cerebrium login'\n")
		return fmt.Errorf("invalid configuration key")
	}

	env := config.GetEnvironment()
	actualKey := config.GetEnvironmentPrefixedKey(normalizedKey, env)

	// Convert string values to appropriate types
	var typedValue any
	switch strings.ToLower(value) {
	case "true":
		typedValue = true
	case "false":
		typedValue = false
	default:
		typedValue = value
	}

	viper.Set(actualKey, typedValue)

	// Save to config file
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Set %s = %v\n", key, typedValue)
	return nil
}
