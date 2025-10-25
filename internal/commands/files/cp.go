package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uiFiles "github.com/cerebriumai/cerebrium/internal/ui/commands/files"
	"github.com/cerebriumai/cerebrium/pkg/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func NewCpCmd() *cobra.Command {
	var region string

	cmd := &cobra.Command{
		Use:   "cp <local_path> [remote_path]",
		Short: "Copy files to persistent storage",
		Long: `Upload files or directories to persistent storage.

Examples:
  cerebrium cp local_file.txt              # Upload to /local_file.txt
  cerebrium cp local_file.txt remote.txt   # Upload to /remote.txt
  cerebrium cp local_dir/ remote_dir/      # Upload directory
  cerebrium cp file.txt --region us-west-2 # Upload to specific region`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCp(cmd, args, region)
		},
	}

	cmd.Flags().StringVarP(&region, "region", "r", "", "Region for the storage volume")

	return cmd
}

func runCp(cmd *cobra.Command, args []string, region string) error {
	// Suppress Cobra's default error handling
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	localPath := args[0]
	remotePath := ""

	// Determine remote path
	if len(args) > 1 {
		remotePath = args[1]
	} else {
		// Default: if uploading a directory, use root; if file, use /filename
		info, err := os.Stat(localPath)
		if err != nil {
			return fmt.Errorf("failed to access source path: %w", err)
		}
		if info.IsDir() {
			remotePath = "/"
		} else {
			remotePath = "/" + filepath.Base(localPath)
		}
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
	model := uiFiles.NewFileUploadView(ctx, uiFiles.CpConfig{
		DisplayConfig: displayOpts,
		Client:        client,
		Config:        cfg,
		Region:        actualRegion,
		LocalPath:     localPath,
		RemotePath:    remotePath,
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
	m, ok := finalModel.(*uiFiles.FileUploadView)
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
