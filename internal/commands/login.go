package commands

import (
	"errors"
	"fmt"
	"os"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uiCommands "github.com/cerebriumai/cerebrium/internal/ui/commands"
	"github.com/cerebriumai/cerebrium/pkg/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// NewLoginCmd creates a direct login command at the root level for convenience
func NewLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Cerebrium",
		Long: `Authenticate user via OAuth and store token in the configuration.

Example:
  cerebrium login
  cerebrium login --no-color  # Disable animations and colors`,
		RunE: runLogin,
	}
}

// runLogin handles the login flow using bubbletea
func runLogin(cmd *cobra.Command, args []string) error {
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

	// For login, we need TTY for OAuth flow
	// In non-TTY environments, users should use service account tokens
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		return fmt.Errorf("login requires an interactive terminal. For non-interactive authentication, use service account tokens via the CEREBRIUM_SERVICE_ACCOUNT environment variable")
	}

	// Create API client for the login flow
	// Note: This client will be created with the config before tokens are saved
	// After tokens are saved, the client will be able to make authenticated requests
	client, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Create Bubbletea model for login
	model := uiCommands.NewLoginView(cmd.Context(), uiCommands.LoginConfig{
		DisplayConfig: displayOpts,
		Config:        cfg,
		Client:        client,
	})

	// Configure Bubbletea based on display options
	var programOpts []tea.ProgramOption
	if !displayOpts.IsInteractive {
		programOpts = append(programOpts, tea.WithoutRenderer())
	}

	// Run Bubbletea program
	p := tea.NewProgram(model, programOpts...)
	doneCh := ui.SetupSignalHandling(p, 0) // No state needed to clean up
	defer close(doneCh)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("ui error: %w", err)
	}

	// Check if there were any errors during execution
	if m, ok := finalModel.(*uiCommands.LoginView); ok {
		if uiErr := m.GetError(); uiErr != nil {
			// Handle UIError
			var uiErrTyped *ui.UIError
			if errors.As(uiErr, &uiErrTyped) && !uiErrTyped.SilentExit {
				// Error was already shown in UI or should be silent
				return uiErr
			}
			// Return error for printing
			return nil
		}
	}

	return nil
}
