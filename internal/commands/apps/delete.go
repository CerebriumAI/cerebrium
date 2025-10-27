package apps

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uiApp "github.com/cerebriumai/cerebrium/internal/ui/commands/apps"
	"github.com/cerebriumai/cerebrium/pkg/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func newDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete APP_ID",
		Short: "Delete an app",
		Long: `Delete an app from Cerebrium.

Example:
  cerebrium apps delete p-abc123
  cerebrium apps delete p-abc123 --no-color  # Disable animations and colors`,
		Args: cobra.ExactArgs(1),
		RunE: runDelete,
	}

	return cmd
}

func runDelete(cmd *cobra.Command, args []string) error {
	// Suppress Cobra's default error handling
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	appID := args[0]

	// Validate app ID format
	if !strings.HasPrefix(appID, "p-") && !strings.HasPrefix(appID, "dev-p-") {
		return fmt.Errorf("invalid apps ID format: '%s' should begin with 'p-'. Run 'cerebrium apps list' to get the correct apps ID", appID)
	}

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
	model := uiApp.NewDeleteView(cmd.Context(), uiApp.DeleteConfig{
		DisplayConfig: displayOpts,
		Client:        client,
		ProjectID:     projectID,
		AppID:         appID,
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
	m, ok := finalModel.(*uiApp.DeleteView)
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
