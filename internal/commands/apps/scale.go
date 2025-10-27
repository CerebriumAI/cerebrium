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

func newScaleCmd() *cobra.Command {
	var (
		cooldown            int
		minReplicas         int
		maxReplicas         int
		responseGracePeriod int
	)

	cmd := &cobra.Command{
		Use:   "scale APP_ID",
		Short: "Update scaling configuration for an app",
		Long: `Change the cooldown, min and max replicas of your app via the CLI.

Example:
  cerebrium apps scale p-abc123 --cooldown 60 --min-replicas 0 --max-replicas 5 --response-grace-period 30
  cerebrium apps scale p-abc123 --cooldown 120 --min-replicas 1 --max-replicas 10 --response-grace-period 60 --no-color`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScale(cmd, args, cooldown, minReplicas, maxReplicas, responseGracePeriod)
		},
	}

	cmd.Flags().IntVar(&cooldown, "cooldown", -1, "Update the cooldown period of your app (seconds before scaling down to min replicas)")
	cmd.Flags().IntVar(&minReplicas, "min-replicas", -1, "Update the minimum number of replicas to keep running")
	cmd.Flags().IntVar(&maxReplicas, "max-replicas", -1, "Update the maximum number of replicas to keep running")
	cmd.Flags().IntVar(&responseGracePeriod, "response-grace-period", -1, "Update the response grace period (seconds for app to respond or terminate)")

	// Mark at least one flag as required
	cmd.MarkFlagsOneRequired("cooldown", "min-replicas", "max-replicas", "response-grace-period")

	return cmd
}

func runScale(cmd *cobra.Command, args []string, cooldown, minReplicas, maxReplicas, responseGracePeriod int) error {
	// Suppress Cobra's default error handling
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	appID := args[0]

	// Validate app ID format
	if !strings.HasPrefix(appID, "p-") && !strings.HasPrefix(appID, "dev-p-") {
		return fmt.Errorf("invalid app ID format: '%s' should begin with 'p-'. Run 'cerebrium apps list' to get the correct app ID", appID)
	}

	// Build updates map from flags
	updates := make(map[string]any)
	if cooldown >= 0 {
		updates["cooldownPeriodSeconds"] = cooldown
	}
	if minReplicas >= 0 {
		updates["minReplicaCount"] = minReplicas
	}
	if maxReplicas >= 0 {
		updates["maxReplicaCount"] = maxReplicas
	}
	if responseGracePeriod >= 0 {
		updates["responseGracePeriodSeconds"] = responseGracePeriod
	}

	// Ensure at least one update is provided
	if len(updates) == 0 {
		return fmt.Errorf("no scaling parameters provided. Use --cooldown, --min-replicas, --max-replicas, or --response-grace-period")
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
	model := uiApp.NewScaleView(cmd.Context(), uiApp.ScaleConfig{
		DisplayConfig: displayOpts,
		Client:        client,
		ProjectID:     projectID,
		AppID:         appID,
		Updates:       updates,
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
	m, ok := finalModel.(*uiApp.ScaleView)
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
