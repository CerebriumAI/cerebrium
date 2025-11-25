package commands

import (
	"fmt"
	"regexp"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/timeutil"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uiCommands "github.com/cerebriumai/cerebrium/internal/ui/commands"
	"github.com/cerebriumai/cerebrium/pkg/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func NewLogsCmd() *cobra.Command {
	var noFollow bool
	var since string

	cmd := &cobra.Command{
		Use:   "logs APP_NAME",
		Short: "View logs for an app",
		Long: `Fetch and display logs for the specified app, following by default.

Examples:
  # Follow logs continuously (default behavior)
  cerebrium logs app-name

  # Get logs once without following
  cerebrium logs app-name --no-follow

  # Get logs from the last hour
  cerebrium logs app-name --since "2d"

  # Get logs since a specific datetime
  cerebrium logs app-name --since "2023-12-01 10:00:00"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogsCommand(cmd, args[0], noFollow, since)
		},
	}

	cmd.Flags().BoolVar(&noFollow, "no-follow", false, "Don't follow log output (fetch once and exit)")
	cmd.Flags().StringVar(&since, "since", "", "Show logs since timestamp. Supports relative ('w|d|h|m|s') or absolute ('YYYY-MM-DD HH:mm:ss')")

	return cmd
}

func runLogsCommand(cmd *cobra.Command, appName string, noFollow bool, since string) error {
	cmd.SilenceUsage = true

	// Get config from context
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get config: %w", err))
	}

	// Get current project
	projectID, err := cfg.GetCurrentProject()
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("no project selected: %w", err))
	}

	// Determine the full appID
	// If appName already contains a project prefix (e.g., "p-abc123-myapp" or "dev-p-abc123-myapp"),
	// use it as-is. Otherwise, prepend the current project ID.
	appID := determineAppID(appName, projectID)

	// Parse --since flag if provided
	var sinceTime string
	if since != "" {
		sinceTime, err = timeutil.ParseSinceTime(since)
		if err != nil {
			return ui.NewValidationError(err)
		}
	}

	// Create API client
	client, err := api.NewClient(cfg)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to create API client: %w", err))
	}

	// Get display options
	displayOpts, err := ui.GetDisplayConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get display options: %w", err))
	}

	// Create Bubbletea model
	model := uiCommands.NewLogsView(cmd.Context(), uiCommands.LogsConfig{
		DisplayConfig: displayOpts,
		Client:        client,
		ProjectID:     projectID,
		AppID:         appID,
		AppName:       appName, // Keep original name for display
		Follow:        !noFollow,
		SinceTime:     sinceTime,
	})

	// Configure Bubbletea program
	var programOpts []tea.ProgramOption
	if !displayOpts.IsInteractive {
		programOpts = append(programOpts,
			tea.WithoutRenderer(),
			tea.WithInput(nil),
		)
	} else {
		programOpts = append(programOpts, tea.WithMouseCellMotion())
	}

	p := tea.NewProgram(model, programOpts...)

	// Setup signal handling
	ui.SetupSignalHandling(p, 0)

	// Run the program
	finalModel, err := p.Run()
	if err != nil {
		return ui.NewInternalError(fmt.Errorf("program error: %w", err))
	}

	// Check for errors from the model
	//nolint:errcheck // Type assertion guaranteed by Bubbletea model structure
	m := finalModel.(*uiCommands.LogsView)
	if uiErr := m.GetError(); uiErr != nil {
		if uiErr.SilentExit {
			return nil
		}
		return uiErr
	}

	return nil
}

// determineAppID determines the full app ID from the user input
// If the input already contains a project prefix (e.g., "p-abc123-myapp" or "dev-p-abc123-myapp"),
// it returns the input as-is. Otherwise, it prepends the current project ID.
func determineAppID(appName, currentProjectID string) string {
	// Check if appName already has a project prefix
	// Project ID formats: "p-{8+ chars}" or "dev-p-{8+ chars}" or "local-p-{8+ chars}"
	// The project ID part must be at least 8 alphanumeric characters to avoid false matches
	projectPrefixPattern := regexp.MustCompile(`^(dev-|local-)?p-[a-z0-9]{8,}-`)

	if projectPrefixPattern.MatchString(appName) {
		// Already has a project prefix - use as-is
		return appName
	}

	// No project prefix - prepend current project ID
	return currentProjectID + "-" + appName
}
