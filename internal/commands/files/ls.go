package files

import (
	"errors"
	"fmt"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uiFiles "github.com/cerebriumai/cerebrium/internal/ui/commands/files"
	"github.com/cerebriumai/cerebrium/pkg/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func NewLsCmd() *cobra.Command {
	var region string

	cmd := &cobra.Command{
		Use:   "ls [path]",
		Short: "List contents of persistent storage",
		Long: `List files and directories in persistent storage.

Examples:
  cerebrium ls                    # List all files in the root directory
  cerebrium ls sub_folder/        # List all files in a specific directory
  cerebrium ls --region us-west-2 # List files in a specific region`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLs(cmd, args, region)
		},
	}

	cmd.Flags().StringVarP(&region, "region", "r", "", "Region for the storage volume")

	return cmd
}

func runLs(cmd *cobra.Command, args []string, region string) error {
	// Suppress Cobra's default error handling
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	// Default path is root
	path := "/"
	if len(args) > 0 {
		path = args[0]
	}

	// Get display options from context (loaded once in root command)
	displayOpts, err := ui.GetDisplayConfigFromContext(cmd)
	if err != nil {
		return fmt.Errorf("failed to get display options: %w", err)
	}

	// Use command context (not Background) for proper cancellation support
	ctx := cmd.Context()

	// Load config
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Verify we have a project configured
	if _, err := cfg.GetCurrentProject(); err != nil {
		return fmt.Errorf("failed to get current project: %w", err)
	}

	// Use provided region or fall back to default
	actualRegion := region
	if actualRegion == "" {
		actualRegion = cfg.GetDefaultRegion()
	}

	// Create API client
	client, err := api.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create API client: %w", err)
	}

	// Create Bubbletea model with display options
	model := uiFiles.NewListView(ctx, uiFiles.ListConfig{
		DisplayConfig: displayOpts,
		Client:        client,
		Config:        cfg,
		Path:          path,
		Region:        actualRegion,
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
	m, ok := finalModel.(*uiFiles.ListView)
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
