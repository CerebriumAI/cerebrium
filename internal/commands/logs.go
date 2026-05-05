package commands

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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
	var containerID string

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
  cerebrium logs app-name --since "2023-12-01 10:00:00"

  # Only show logs from a specific container
  cerebrium logs app-name --container-id <container-id>`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogsCommand(cmd, args[0], noFollow, since, containerID)
		},
	}

	cmd.Flags().BoolVar(&noFollow, "no-follow", false, "Don't follow log output (fetch once and exit)")
	cmd.Flags().StringVar(&since, "since", "", "Show logs since timestamp. Supports relative ('w|d|h|m|s') or absolute ('YYYY-MM-DD HH:mm:ss')")
	cmd.Flags().StringVar(&containerID, "container-id", "", "Only show logs from the specified container ID")

	return cmd
}

func runLogsCommand(cmd *cobra.Command, appName string, noFollow bool, since, containerID string) error {
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

	// The backend exact-matches container_id against the full pod name. If the
	// user gave us anything shorter (e.g. just the "<replicaset>-<pod>" suffix
	// they copied from `containers list`), look it up and expand to the full id.
	if containerID != "" {
		containerID, err = resolveContainerID(cmd.Context(), client, projectID, appID, containerID)
		if err != nil {
			return ui.NewValidationError(err)
		}
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
		ContainerID:   containerID,
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
		return uiErr
	}

	return nil
}

// resolveContainerID expands a partial container id (e.g. "74d8f9d9cf-nlc5n")
// to the full pod name the backend stores. If the supplied value already matches
// a container id exactly, it is returned unchanged. Otherwise we list the app's
// containers and look for any whose id has the supplied value as a suffix.
func resolveContainerID(ctx context.Context, client api.Client, projectID, appID, supplied string) (string, error) {
	containers, err := client.ListContainers(ctx, projectID, appID)
	if err != nil {
		return "", fmt.Errorf("look up containers for %s: %w", appID, err)
	}

	var matches []string
	for _, c := range containers {
		if c.ContainerID == supplied {
			return supplied, nil
		}
		if strings.HasSuffix(c.ContainerID, supplied) {
			matches = append(matches, c.ContainerID)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", fmt.Errorf("no container matching %q found for app %s — run `cerebrium containers list %s` to see available containers", supplied, appID, appID)
	default:
		return "", fmt.Errorf("container id %q is ambiguous, matches multiple containers: %s", supplied, strings.Join(matches, ", "))
	}
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
