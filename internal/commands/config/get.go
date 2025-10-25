package config

import (
	"fmt"
	"strings"

	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long: `Get a configuration value from ~/.cerebrium/config.yaml

Examples:
  cerebrium config get skip-version-check
  cerebrium config get default-region
  cerebrium config get dev-project`,
		Args: cobra.ExactArgs(1),
		RunE: runGet,
	}
}

func runGet(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	key := args[0]

	// Normalize the key (convert kebab-case to lowercase)
	normalizedKey := strings.ToLower(strings.ReplaceAll(key, "-", ""))

	// Validate that this is a recognized user-facing config key
	if !config.IsValidUserFacingKey(normalizedKey) {
		return fmt.Errorf("'%s' is not a recognized configuration key. Run 'cerebrium config set --help' for valid keys", key)
	}

	// Get the environment-prefixed key (e.g., "project" → "dev-project" if in dev)
	env := config.GetEnvironment()
	actualKey := config.GetEnvironmentPrefixedKey(normalizedKey, env)

	// Get the value from viper
	if !viper.IsSet(actualKey) {
		return fmt.Errorf("configuration key '%s' not set", key)
	}

	value := viper.Get(actualKey)
	fmt.Println(value)

	return nil
}
