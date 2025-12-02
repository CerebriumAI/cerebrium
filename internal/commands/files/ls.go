package files

import (
	"fmt"
	"sort"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
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
	cmd.SilenceUsage = true

	// Default path is root
	path := "/"
	if len(args) > 0 {
		path = args[0]
	}

	// Load config
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to load config: %w", err))
	}

	// Verify we have a project configured
	if _, err := cfg.GetCurrentProject(); err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get current project: %w", err))
	}

	// Use provided region or fall back to default
	actualRegion := region
	if actualRegion == "" {
		actualRegion = cfg.GetDefaultRegion()
	}

	// Create API client
	client, err := api.NewClient(cfg)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to create API client: %w", err))
	}

	// Show spinner while fetching
	spinner := ui.NewSimpleSpinner("Loading files...")
	spinner.Start()

	// Fetch files
	files, err := client.ListFiles(cmd.Context(), cfg.ProjectID, path, actualRegion)
	spinner.Stop()
	if err != nil {
		return ui.NewAPIError(err)
	}

	// Sort: directories first, then by name
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsFolder && !files[j].IsFolder {
			return true
		} else if !files[i].IsFolder && files[j].IsFolder {
			return false
		}
		return files[i].Name < files[j].Name
	})

	// Print results
	if len(files) == 0 {
		fmt.Println("No files found")
		return nil
	}

	// Calculate max name width
	maxNameWidth := len("NAME")
	for _, file := range files {
		if len(file.Name) > maxNameWidth {
			maxNameWidth = len(file.Name)
		}
	}
	nameWidth := maxNameWidth + 2

	// Print table header and rows
	fmt.Printf("%-*s %-15s %-20s\n", nameWidth, "NAME", "SIZE", "LAST MODIFIED")
	for _, file := range files {
		fmt.Printf("%-*s %-15s %-20s\n",
			nameWidth,
			file.Name,
			formatFileSize(file),
			formatLastModified(file.LastModified),
		)
	}

	return nil
}

// formatFileSize formats file size for display
func formatFileSize(file api.FileInfo) string {
	if file.IsFolder {
		return "Directory"
	}

	size := float64(file.SizeBytes)
	units := []string{"B", "KB", "MB", "GB", "TB"}
	unitIndex := 0

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	if unitIndex == 0 {
		return fmt.Sprintf("%d %s", int(size), units[unitIndex])
	}
	return fmt.Sprintf("%.2f %s", size, units[unitIndex])
}

// formatLastModified formats the last modified timestamp
func formatLastModified(timestamp string) string {
	if timestamp == "" || timestamp == "0001-01-01T00:00:00Z" {
		return "N/A"
	}

	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z", timestamp)
		if err != nil {
			return timestamp
		}
	}

	return t.Format("2006-01-02 15:04:05")
}
