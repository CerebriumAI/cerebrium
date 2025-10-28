package commands

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	appCmd "github.com/cerebriumai/cerebrium/internal/commands/apps"
	configCmd "github.com/cerebriumai/cerebrium/internal/commands/config"
	filesCmd "github.com/cerebriumai/cerebrium/internal/commands/files"
	projectCmd "github.com/cerebriumai/cerebrium/internal/commands/projects"
	regionCmd "github.com/cerebriumai/cerebrium/internal/commands/region"
	runsCmd "github.com/cerebriumai/cerebrium/internal/commands/runs"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/internal/version"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/cerebriumai/cerebrium/pkg/logrium"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "cerebrium",
		Short: "Cerebrium CLI",
		Long:  "Command line interface for the Cerebrium platform",
		// Silence errors - we handle them in main.go
		// Note: SilenceUsage is NOT set here so unknown commands show usage.
		// Individual commands set cmd.SilenceUsage = true to hide usage on errors.
		SilenceErrors: true,
		// Load config once and store in context for all subcommands
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			verbose, _ := cmd.Flags().GetBool("verbose")

			// Get display options for logger setup
			displayOpts, err := ui.NewDisplayConfig(cmd, verbose)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error getting display options: %v\n", err)
				os.Exit(1)
			}

			// Load config first (needed to get configured log level)
			cfg, err := config.Load()
			if err != nil {
				// Config loading failed - print error and exit
				// This is critical, we can't proceed without config
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}

			// Setup logger with configured log level
			if verbose {
				// Use configured log level (defaults to info if not set)
				logLevel := cfg.GetLogLevel()
				logFile, err := logrium.Setup(displayOpts.IsInteractive, logLevel)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error setting up logger: %v\n", err)
					os.Exit(1)
				}

				// Print log file location if logging to file
				if logFile != "" {
					fmt.Fprintf(os.Stderr, "Debug logs: %s\n", logFile)
				}
			} else {
				// Disable logging entirely when --verbose is not set
				logrium.Disable()
			}

			slog.Debug("Config loaded successfully")

			// Store config and display options in context so subcommands can access them
			ctx := context.WithValue(cmd.Context(), config.GetContextKey(), cfg)
			ctx = context.WithValue(ctx, ui.GetDisplayConfigContextKey(), displayOpts)
			cmd.SetContext(ctx)

			// Run version check (skip for version and config commands)
			if cmd.Name() != "version" && cmd.Name() != "config" {
				version.PrintUpdateNotification(cmd.Context(), cfg.SkipVersionCheck)
			}
		},
	}

	// Global flags (persistent flags are inherited by all subcommands)
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output and animations")
	rootCmd.PersistentFlags().Bool("no-ansi", false, "Disable colored output and animations (equivalent to --no-color)")

	// Add subcommands
	rootCmd.AddCommand(NewLoginCmd())
	rootCmd.AddCommand(NewInitCmd())
	rootCmd.AddCommand(NewDeployCmd())
	rootCmd.AddCommand(NewRunCmd())
	rootCmd.AddCommand(NewStatusCmd())
	rootCmd.AddCommand(NewLogsCmd())
	rootCmd.AddCommand(NewVersionCmd())
	rootCmd.AddCommand(configCmd.NewConfigCmd())
	rootCmd.AddCommand(appCmd.NewAppsCmd())
	rootCmd.AddCommand(projectCmd.NewProjectsCmd())
	rootCmd.AddCommand(regionCmd.NewRegionCmd())
	rootCmd.AddCommand(runsCmd.NewRunsCmd())

	// File operation commands (at root level for feature parity with Python CLI)
	rootCmd.AddCommand(filesCmd.NewLsCmd())
	rootCmd.AddCommand(filesCmd.NewCpCmd())
	rootCmd.AddCommand(filesCmd.NewDownloadCmd())
	rootCmd.AddCommand(filesCmd.NewRmCmd())

	return rootCmd
}
