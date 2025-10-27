package apps

import (
	"errors"
	"fmt"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uiApp "github.com/cerebriumai/cerebrium/internal/ui/commands/apps"
	"github.com/cerebriumai/cerebrium/pkg/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get APP_ID",
		Short: "Get details for a specific app",
		Long: `Get specific details around an app.

Example:
  cerebrium apps get p-abc123
  cerebrium apps get p-abc123 --no-ansi  # Disable animations and colors`,
		Args: cobra.ExactArgs(1),
		RunE: runGet,
	}

	return cmd
}

func runGet(cmd *cobra.Command, args []string) error {
	// Suppress Cobra's default error handling
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	appID := args[0]

	// Get display options from context (loaded once in root command)
	displayOpts, err := ui.GetDisplayConfigFromContext(cmd)
	if err != nil {
		return fmt.Errorf("failed to get display options: %w", err)
	}

	// Get config from context (loaded once in root command)
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Get current project
	projectID, err := cfg.GetCurrentProject()
	if err != nil {
		return fmt.Errorf("failed to get current project: %w", err)
	}

	// Create API client
	client, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Create Bubbletea model
	model := uiApp.NewGetView(cmd.Context(), uiApp.GetConfig{
		Client:        client,
		ProjectID:     projectID,
		AppID:         appID,
		DisplayConfig: displayOpts,
	})

	// Configure Bubbletea based on display options
	var programOpts []tea.ProgramOption

	if !displayOpts.IsInteractive {
		// Non-interactive mode: disable renderer and input
		programOpts = append(programOpts,
			tea.WithoutRenderer(),
			tea.WithInput(nil),
		)
	}

	// Run Bubbletea program
	p := tea.NewProgram(model, programOpts...)
	doneCh := ui.SetupSignalHandling(p, 0)
	defer close(doneCh)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("ui error: %w", err)
	}

	// Extract model
	m, ok := finalModel.(*uiApp.GetView)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}

	// Check if there were any errors during execution
	// Handle UIError - check if it should be silent
	var uiErr *ui.UIError
	if errors.As(m.Error(), &uiErr) && !uiErr.SilentExit {
		return uiErr
	}

	return nil
}
