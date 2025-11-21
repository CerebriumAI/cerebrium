package config

import (
	"fmt"

	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
)

// newTelemetryCmd creates the telemetry command
func newTelemetryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "telemetry",
		Short: "Manage telemetry settings",
		Long:  `Manage telemetry and error reporting settings for the Cerebrium CLI.`,
	}

	cmd.AddCommand(newTelemetryDisableCmd())
	cmd.AddCommand(newTelemetryEnableCmd())
	cmd.AddCommand(newTelemetryStatusCmd())

	return cmd
}

// newTelemetryDisableCmd creates the telemetry disable command
func newTelemetryDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable",
		Short: "Disable telemetry and error reporting",
		Long: `Disable telemetry and error reporting for the Cerebrium CLI.

This will prevent the CLI from sending crash reports and error information to help improve the product.
You can re-enable telemetry at any time using 'cerebrium config telemetry enable'.

You can also disable telemetry temporarily using the environment variable:
  export CEREBRIUM_TELEMETRY_DISABLED=true`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			cfg, err := config.Load()
			if err != nil {
				return ui.NewValidationError(fmt.Errorf("failed to load config: %w", err))
			}

			// Set telemetry to false
			telemetryEnabled := false
			cfg.TelemetryEnabled = &telemetryEnabled

			if err := config.Save(cfg); err != nil {
				return ui.NewFileSystemError(fmt.Errorf("failed to save config: %w", err))
			}

			fmt.Println("✓ Telemetry disabled")

			return nil
		},
	}
}

// newTelemetryEnableCmd creates the telemetry enable command
func newTelemetryEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable",
		Short: "Enable telemetry and error reporting",
		Long: `Enable telemetry and error reporting for the Cerebrium CLI.

This helps us improve the product by automatically reporting crashes and errors.
No personal data or code is ever transmitted - only error messages and system metadata.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			cfg, err := config.Load()
			if err != nil {
				return ui.NewValidationError(fmt.Errorf("failed to load config: %w", err))
			}

			// Set telemetry to true
			telemetryEnabled := true
			cfg.TelemetryEnabled = &telemetryEnabled

			if err := config.Save(cfg); err != nil {
				return ui.NewFileSystemError(fmt.Errorf("failed to save config: %w", err))
			}

			fmt.Println("✓ Telemetry enabled")

			return nil
		},
	}
}

// newTelemetryStatusCmd creates the telemetry status command
func newTelemetryStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show current telemetry status",
		Long:  `Display whether telemetry and error reporting is currently enabled or disabled.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

			cfg, err := config.Load()
			if err != nil {
				return ui.NewValidationError(fmt.Errorf("failed to load config: %w", err))
			}

			if cfg.IsTelemetryEnabled() {
				fmt.Println("Telemetry: enabled")
			} else {
				fmt.Println("Telemetry: disabled")
			}

			return nil
		},
	}
}
