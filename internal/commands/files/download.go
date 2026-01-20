package files

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uiFiles "github.com/cerebriumai/cerebrium/internal/ui/commands/files"
	"github.com/cerebriumai/cerebrium/pkg/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func NewDownloadCmd() *cobra.Command {
	var region string

	cmd := &cobra.Command{
		Use:   "download <remote_path> [local_path]",
		Short: "Download files from persistent storage",
		Long: `Download files or directories from persistent storage to your local machine.

Examples:
  cerebrium download remote_file.txt                  # Download to ./remote_file.txt
  cerebrium download remote.txt local.txt             # Download to ./local.txt
  cerebrium download file.txt --region us-west-2      # Download from specific region`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDownload(cmd, args, region)
		},
	}

	cmd.Flags().StringVarP(&region, "region", "r", "", "Region for the storage volume")

	return cmd
}

func runDownload(cmd *cobra.Command, args []string, region string) error {
	// Suppress Cobra's default error handling
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	// Normalize remote path to handle ./ prefixes and other path quirks
	// Preserve trailing slash as it indicates directory intent
	hasTrailingSlash := strings.HasSuffix(args[0], "/")
	remotePath := filepath.Clean(args[0])
	// Convert to forward slashes for API compatibility (Windows uses backslashes)
	remotePath = filepath.ToSlash(remotePath)
	if remotePath == "." {
		remotePath = "/"
	} else if hasTrailingSlash && !strings.HasSuffix(remotePath, "/") {
		remotePath += "/"
	}
	localPath := ""

	// Determine local path
	if len(args) > 1 {
		localPath = args[1]
	} else {
		// Default: use basename of remote path
		localPath = filepath.Base(remotePath)
	}

	// Get display options
	displayOpts, err := ui.GetDisplayConfigFromContext(cmd)
	if err != nil {
		return fmt.Errorf("failed to get display options: %w", err)
	}

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

	// Create Bubbletea model
	model := uiFiles.NewFileDownloadView(ctx, uiFiles.DownloadConfig{
		DisplayConfig: displayOpts,
		Client:        client,
		Config:        cfg,
		Region:        actualRegion,
		RemotePath:    remotePath,
		LocalPath:     localPath,
	})

	// Configure Bubbletea
	var programOpts []tea.ProgramOption
	if !displayOpts.IsInteractive {
		programOpts = append(programOpts,
			tea.WithoutRenderer(),
			tea.WithInput(nil),
		)
	}

	// Run Bubbletea program
	p := tea.NewProgram(model, programOpts...)
	ui.SetupSignalHandling(p, 0)

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("ui error: %w", err)
	}

	// Extract model
	m, ok := finalModel.(*uiFiles.FileDownloadView)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}

	// Check for errors
	// Handle UIError - check if it should be silent
	var uiErr *ui.UIError
	if errors.As(m.Error(), &uiErr) && !uiErr.SilentExit {
		return uiErr
	}

	return nil
}
