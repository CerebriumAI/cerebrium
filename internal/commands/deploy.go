package commands

import (
	"fmt"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/files"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uiCommands "github.com/cerebriumai/cerebrium/internal/ui/commands"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/cerebriumai/cerebrium/pkg/projectconfig"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// NewDeployCmd creates a deploy command
func NewDeployCmd() *cobra.Command {
	var (
		name                string
		disableSyntaxCheck  bool
		logLevel            string
		configFile          string
		disableConfirmation bool
		disableBuildLogs    bool
		detach              bool
	)

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a Cerebrium app",
		Long: `Deploy a new Cortex app to Cerebrium.

This command will:
1. Load your cerebrium.toml configuration
2. Package your application files
3. Upload them to Cerebrium
4. Build and deploy your app

Example:
  cerebrium deploy
  cerebrium deploy --config-file ./custom-cerebrium.toml
  cerebrium deploy --no-color  # Disable animations and colors`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDeploy(cmd, deployOptions{
				name:               name,
				disableSyntaxCheck: disableSyntaxCheck,
				logLevel:           logLevel,
				configFile:         configFile,
				disableBuildLogs:   disableBuildLogs,
				detach:             detach,
			}, disableConfirmation)
		},
	}

	// Add flags (note: --no-color and --no-ansi are global flags from root command)
	cmd.Flags().StringVar(&name, "name", "", "Name of the App. Overrides the value in the TOML file if provided.")
	cmd.Flags().BoolVar(&disableSyntaxCheck, "disable-syntax-check", false, "Flag to disable syntax check")
	cmd.Flags().StringVar(&logLevel, "log-level", "INFO", "Log level for deployment (DEBUG or INFO)")
	cmd.Flags().StringVar(&configFile, "config-file", "./cerebrium.toml", "Path to the cerebrium config TOML file")
	cmd.Flags().BoolVarP(&disableConfirmation, "disable-confirmation", "y", false, "Disable confirmation prompt")
	cmd.Flags().BoolVar(&disableBuildLogs, "disable-build-logs", false, "Disable build logs during deployment")
	cmd.Flags().BoolVar(&detach, "detach", false, "Kick off deployment and exit without waiting for build completion. The build will continue on the server and Ctrl+C will not cancel it.")

	return cmd
}

type deployOptions struct {
	name               string
	disableSyntaxCheck bool
	logLevel           string
	configFile         string
	disableBuildLogs   bool
	detach             bool
}

func runDeploy(cmd *cobra.Command, opts deployOptions, disableConfirmation bool) error {
	cmd.SilenceUsage = true

	// Get display options from context (loaded once in root command)
	displayOpts, err := ui.GetDisplayConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get display options: %w", err))
	}

	// Get config from context (loaded once in root command)
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get config: %w", err))
	}

	// Check if user has a project configured
	if cfg.ProjectID == "" {
		return ui.NewValidationError(fmt.Errorf("no project configured. Please run 'cerebrium login' first"))
	}

	// Load project config from cerebrium.toml
	projectConfig, err := projectconfig.Load(opts.configFile)
	if err != nil {
		return ui.NewValidationError(err)
	}

	// Validate project config
	if err := projectconfig.Validate(projectConfig); err != nil {
		return ui.NewValidationError(fmt.Errorf("invalid configuration: %w", err))
	}

	// Override deployment name if provided
	if opts.name != "" {
		projectConfig.Deployment.Name = opts.name
	}

	// Early validation: Check if any files match the include/exclude patterns
	testFiles, err := files.DetermineIncludes(
		projectConfig.Deployment.Include,
		projectConfig.Deployment.Exclude,
	)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to determine files: %w", err))
	}
	if len(testFiles) == 0 {
		return ui.NewValidationError(fmt.Errorf("no files found matching include patterns. Please check your include/exclude patterns in cerebrium.toml"))
	}

	// Warn about development folders
	devFolders := files.DetectDevFolders(testFiles)
	if len(devFolders) > 0 {
		fmt.Printf("⚠️  Warning: Development folder(s) detected: %v\n", devFolders)
		fmt.Println("   Including development folders can significantly increase upload size.")
		fmt.Println("   Consider excluding them in your cerebrium.toml file.")
	}

	// Create API client
	client, err := api.NewClient(cfg)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to create API client: %w", err))
	}

	// Create Bubbletea model for deploy
	model := uiCommands.NewDeployView(cmd.Context(), uiCommands.DeployConfig{
		DisplayConfig:       displayOpts,
		Config:              projectConfig,
		ProjectID:           cfg.ProjectID,
		Client:              client,
		DisableBuildLogs:    opts.disableBuildLogs,
		DisableConfirmation: disableConfirmation,
		LogLevel:            opts.logLevel,
		Detach:              opts.detach,
	})

	// Configure Bubbletea based on display options
	var programOpts []tea.ProgramOption

	if !displayOpts.IsInteractive {
		// Non-TTY mode or animation disabled: disable renderer and input
		programOpts = append(programOpts,
			tea.WithoutRenderer(),
			tea.WithInput(nil),
		)
	}

	// Print a newline to preserve the command line in terminal history
	// This prevents Bubbletea's renderer from overwriting the "cerebrium deploy" line
	if displayOpts.IsInteractive {
		fmt.Println()
	}

	// Run Bubbletea program (it handles its own cleanup)
	p := tea.NewProgram(model, programOpts...)

	// Set up signal handling for graceful cancellation
	// This works for both TTY and non-TTY modes
	doneCh := ui.SetupSignalHandling(p, 5*time.Second)
	defer close(doneCh)

	finalModel, err := p.Run()
	if err != nil {
		return ui.NewInternalError(fmt.Errorf("internal error: %w", err))
	}

	// Extract model and check for errors
	m, ok := finalModel.(*uiCommands.DeployView)
	if !ok {
		return ui.NewInternalError(fmt.Errorf("unexpected model type"))
	}

	// Handle error from model
	if uiErr := m.GetError(); uiErr != nil {
		if uiErr.SilentExit {
			// Error was already shown in UI or should be silent - exit cleanly
			return nil
		}
		// Return error for Cobra/main.go to print
		return uiErr
	}

	return nil
}
