package runs

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
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
  cerebrium runs list myapp --async  # Only show async runs`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runList(cmd, args[0], asyncOnly)
		},
	}

	cmd.Flags().BoolVar(&asyncOnly, "async", false, "Only list runs that were executed asynchronously")

	return cmd
}

func runList(cmd *cobra.Command, appName string, asyncOnly bool) error {
	cmd.SilenceUsage = true

	// Get config from context
	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get config: %w", err))
	}

	// Get current project
	projectID, err := cfg.GetCurrentProject()
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("no project found. Please run 'cerebrium project set PROJECT_ID' to set the current project"))
	}

	// Create API client
	client, err := api.NewClient(cfg)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to create API client: %w", err))
	}

	// Construct the app ID
	appID := normalizeAppID(projectID, appName)

	// Show spinner while fetching
	spinner := ui.NewSimpleSpinner("Loading runs...")
	spinner.Start()

	// Fetch runs
	runs, err := client.GetRuns(cmd.Context(), projectID, appID, asyncOnly)
	spinner.Stop()
	if err != nil {
		return ui.NewAPIError(err)
	}

	// Sort by created date (most recent first)
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].CreatedAt.After(runs[j].CreatedAt)
	})

	// Print results
	if len(runs) == 0 {
		if asyncOnly {
			fmt.Printf("No async runs found for app: %s\n", appName)
		} else {
			fmt.Printf("No runs found for app: %s\n", appName)
		}
		return nil
	}

	// Print table header and rows
	fmt.Printf("%-40s %-30s %-15s %-25s %-10s\n", "RUN ID", "FUNCTION NAME", "STATUS", "CREATED AT", "ASYNC")
	for _, run := range runs {
		asyncStr := "No"
		if run.Async {
			asyncStr = "Yes"
		}
		fmt.Printf("%-40s %-30s %-15s %-25s %-10s\n",
			run.ID,
			run.FunctionName,
			run.GetDisplayStatus(),
			run.CreatedAt.Format("2006-01-02 15:04:05 MST"),
			asyncStr,
		)
	}

	return nil
}

// normalizeAppID ensures the app ID has the correct format.
func normalizeAppID(projectID, appName string) string {
	expectedPrefix := projectID + "-"
	if strings.HasPrefix(appName, expectedPrefix) {
		return appName
	}
	return fmt.Sprintf("%s-%s", projectID, appName)
}
