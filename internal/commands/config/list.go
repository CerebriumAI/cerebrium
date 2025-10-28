package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configuration",
		Long: `List all configuration keys and values from ~/.cerebrium/config.yaml

Example:
  cerebrium config list`,
		Args: cobra.NoArgs,
		RunE: runList,
	}
}

func runList(cmd *cobra.Command, args []string) error {
	cmd.SilenceUsage = true

	// Get current environment
	env := config.GetEnvironment()
	envPrefix := ""
	if env != config.EnvProd {
		envPrefix = string(env) + "-"
	}

	// Get all settings from viper
	settings := viper.AllSettings()

	if len(settings) == 0 {
		fmt.Println("No configuration found")
		return nil
	}

	// Filter to show only current environment keys + global keys
	var displayKeys []struct {
		userFacingKey string
		actualKey     string
		value         any
	}

	// Global keys
	for key, val := range settings {
		if key == "skipversioncheck" {
			displayKeys = append(displayKeys, struct {
				userFacingKey string
				actualKey     string
				value         any
			}{"skip-version-check", key, val})
		}
		if key == "loglevel" {
			displayKeys = append(displayKeys, struct {
				userFacingKey string
				actualKey     string
				value         any
			}{"log-level", key, val})
		}
	}

	// Environment-specific keys
	for key, val := range settings {
		// Only show keys for current environment
		if strings.HasPrefix(key, envPrefix) {
			// Strip the environment prefix for display
			userFacingKey := strings.TrimPrefix(key, envPrefix)

			// Convert to kebab-case for display
			displayKey := userFacingKey
			if userFacingKey == "defaultregion" {
				displayKey = "default-region"
			}

			// Skip tokens (managed by login)
			if userFacingKey == "accesstoken" || userFacingKey == "refreshtoken" {
				continue
			}

			displayKeys = append(displayKeys, struct {
				userFacingKey string
				actualKey     string
				value         any
			}{displayKey, key, val})
		}
	}

	if len(displayKeys) == 0 {
		fmt.Println("No configuration found for current environment")
		return nil
	}

	// Sort by user-facing key
	sort.Slice(displayKeys, func(i, j int) bool {
		return displayKeys[i].userFacingKey < displayKeys[j].userFacingKey
	})

	// Print key-value pairs
	for _, item := range displayKeys {
		fmt.Printf("%s: %v\n", item.userFacingKey, item.value)
	}

	return nil
}
