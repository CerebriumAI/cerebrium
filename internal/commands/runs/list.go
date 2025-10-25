package runs

import (
	"errors"
	"fmt"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/internal/ui/commands/runs"
	"github.com/cerebriumai/cerebrium/pkg/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var asyncOnly bool

	cmd := &cobra.Command{
		Use:   "list APP_NAME",
		Short: "List all runs for a specific app",
		Long: `List all runs for the specified app in the current project.

Examples:
  cerebrium runs list myapp
  cerebrium runs list myapp --async  # Only show async runs
  cerebrium runs list myapp --no-color  # Disable animations and colors`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, args[0], asyncOnly)
		},
	}

	cmd.Flags().BoolVar(&asyncOnly, "async", false, "Only list runs that were executed asynchronously")

	return cmd
}

func runList(cmd *cobra.Command, appName string, asyncOnly bool) error {
	// Suppress Cobra's default error handling - we control it in main.go
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

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
		return fmt.Errorf("no project found. Please run 'cerebrium project set PROJECT_ID' to set the current project")
	}

	// Create API client
	client, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Create Bubbletea model with display options
	model := runs.NewListView(cmd.Context(), runs.ListConfig{
		DisplayConfig: displayOpts,
		Client:        client,
		ProjectID:     projectID,
		AppName:       appName,
		AsyncOnly:     asyncOnly,
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
	m, ok := finalModel.(*runs.ListView)
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
