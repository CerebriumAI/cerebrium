package metrics

import (
	"fmt"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
)

type resourceFlags struct {
	since       time.Duration
	start       string
	end         string
	containerID string
	resolution  string
}

// seriesPeak is the highest value a single series reached over the window. Peak is
// nil when the series carried no samples at all.
type seriesPeak struct {
	Name string   `json:"name"`
	Peak *float64 `json:"peak"`
}

type metricSummary struct {
	Unit   string       `json:"unit"`
	Series []seriesPeak `json:"series"`
}

type metricsOutput struct {
	AppID       string                   `json:"appId"`
	Start       time.Time                `json:"start"`
	End         time.Time                `json:"end"`
	ContainerID string                   `json:"containerId,omitempty"`
	Summary     map[string]metricSummary `json:"summary"`
	Metrics     *api.ResourceMetrics     `json:"metrics"`
}

func newResourcesCmd() *cobra.Command {
	var flags resourceFlags

	cmd := &cobra.Command{
		Use:   "resources APP_NAME",
		Short: "Show CPU, memory and GPU memory usage for an app",
		Long: `Show how much CPU, memory and GPU memory (VRAM) an app actually used over a
time window, so you can right-size the hardware in your cerebrium.toml.

Values are peaks over the window: CPU in cores, memory and GPU memory in GB.

Examples:
  cerebrium metrics resources my-app
  cerebrium metrics resources my-app --since 24h
  cerebrium metrics resources my-app --start 2026-08-01T00:00:00Z --end 2026-08-02T00:00:00Z
  cerebrium metrics resources my-app --container-id <id>
  cerebrium metrics resources my-app --output json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResources(cmd, args[0], flags)
		},
	}

	cmd.Flags().DurationVar(&flags.since, "since", time.Hour, "Window ending now (ignored if --start is set)")
	cmd.Flags().StringVar(&flags.start, "start", "", "Start of the window as an RFC3339 timestamp")
	cmd.Flags().StringVar(&flags.end, "end", "", "End of the window as an RFC3339 timestamp (defaults to now)")
	cmd.Flags().StringVar(&flags.containerID, "container-id", "", "Scope metrics to a single container")
	cmd.Flags().StringVar(&flags.resolution, "resolution", "", "Data resolution: medium, high")
	ui.AddOutputFlag(cmd)

	return cmd
}

func runResources(cmd *cobra.Command, appName string, flags resourceFlags) error {
	cmd.SilenceUsage = true

	outputFormat, err := ui.ParseOutputFormat(cmd)
	if err != nil {
		return err
	}

	if flags.resolution != "" && flags.resolution != "medium" && flags.resolution != "high" {
		return ui.NewValidationError(fmt.Errorf("invalid resolution: %s (supported: medium, high)", flags.resolution))
	}

	start, end, err := resolveWindow(flags)
	if err != nil {
		return ui.NewValidationError(err)
	}

	cfg, err := config.GetConfigFromContext(cmd)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to get config: %w", err))
	}

	projectID, err := cfg.GetCurrentProject()
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("no project selected: %w", err))
	}

	client, err := api.NewClient(cfg)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to create API client: %w", err))
	}

	appID := api.NormalizeAppID(projectID, appName)

	spinner := ui.NewSimpleSpinnerFor(outputFormat, "Loading metrics...")
	spinner.Start()

	metrics, err := client.GetResourceMetrics(cmd.Context(), projectID, appID, api.ResourceMetricsOptions{
		Start:       start,
		End:         end,
		ContainerID: flags.containerID,
		Resolution:  flags.resolution,
	})
	spinner.Stop()
	if err != nil {
		return ui.NewAPIError(err)
	}

	summary := map[string]metricSummary{
		"cpu":    summarise(metrics.CPU, "cores"),
		"memory": summarise(metrics.Memory, "GB"),
		"gpu":    summarise(metrics.GPU, "GB"),
	}

	if outputFormat == ui.OutputJSON {
		return ui.PrintJSON(metricsOutput{
			AppID:       appID,
			Start:       start,
			End:         end,
			ContainerID: flags.containerID,
			Summary:     summary,
			Metrics:     metrics,
		})
	}

	printSummary(appID, start, end, flags.containerID, summary)
	return nil
}

// resolveWindow turns the time flags into a concrete range. --start wins over
// --since; --end defaults to now.
func resolveWindow(flags resourceFlags) (time.Time, time.Time, error) {
	end := time.Now().UTC()
	if flags.end != "" {
		parsed, err := time.Parse(time.RFC3339, flags.end)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --end timestamp %q: expected RFC3339 (e.g. 2026-08-01T00:00:00Z)", flags.end)
		}
		end = parsed.UTC()
	}

	var start time.Time
	if flags.start != "" {
		parsed, err := time.Parse(time.RFC3339, flags.start)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --start timestamp %q: expected RFC3339 (e.g. 2026-08-01T00:00:00Z)", flags.start)
		}
		start = parsed.UTC()
	} else {
		if flags.since <= 0 {
			return time.Time{}, time.Time{}, fmt.Errorf("--since must be positive, got %s", flags.since)
		}
		start = end.Add(-flags.since)
	}

	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("end of window (%s) must be after its start (%s)", end.Format(time.RFC3339), start.Format(time.RFC3339))
	}

	return start, end, nil
}

// summarise reduces each series to its peak over the window. Padded gaps are nil
// and are skipped, so a series that never reported stays nil rather than becoming 0.
func summarise(data api.ChartData, unit string) metricSummary {
	out := metricSummary{Unit: unit, Series: make([]seriesPeak, 0, len(data.Series))}

	for _, series := range data.Series {
		entry := seriesPeak{Name: series.Name}
		for _, sample := range series.Data {
			if !sample.Valid {
				continue
			}
			if entry.Peak == nil || sample.Value > *entry.Peak {
				peak := sample.Value
				entry.Peak = &peak
			}
		}
		out.Series = append(out.Series, entry)
	}

	return out
}

var metricRows = []struct {
	key   string
	label string
}{
	{"cpu", "CPU"},
	{"memory", "Memory"},
	{"gpu", "GPU Memory"},
}

func printSummary(appID string, start, end time.Time, containerID string, summary map[string]metricSummary) {
	scope := "app"
	if containerID != "" {
		scope = "container " + containerID
	}
	fmt.Printf("Peak usage for %s (%s) from %s to %s\n\n",
		appID, scope, start.Format(time.RFC3339), end.Format(time.RFC3339))

	columns := summaryColumns(summary)
	if len(columns) == 0 {
		fmt.Println("No metrics reported for this window. The app may not have run during it.")
		return
	}

	fmt.Printf("%-14s", "METRIC")
	for _, column := range columns {
		fmt.Printf("%10s", column)
	}
	fmt.Printf("  %s\n", "UNIT")

	for _, row := range metricRows {
		metric := summary[row.key]
		fmt.Printf("%-14s", row.label)
		for i := range columns {
			fmt.Printf("%10s", formatPeak(metric, i))
		}
		fmt.Printf("  %s\n", metric.Unit)
	}
}

// summaryColumns derives the header from the first metric that reported anything.
// Every metric carries the same series in the same order for a given scope, so
// cells are matched to columns by position.
func summaryColumns(summary map[string]metricSummary) []string {
	for _, row := range metricRows {
		series := summary[row.key].Series
		if len(series) == 0 {
			continue
		}

		// Scoped to a single container the API returns one series per metric, named
		// after the metric itself — a poor shared header, so label the column PEAK.
		if len(series) == 1 {
			return []string{"PEAK"}
		}

		columns := make([]string, 0, len(series))
		for _, entry := range series {
			columns = append(columns, entry.Name)
		}
		return columns
	}
	return nil
}

// formatPeak renders one cell. A metric that never reported shows "-" rather than
// 0.00, which would read as a measurement rather than an absence of one.
func formatPeak(metric metricSummary, column int) string {
	if column >= len(metric.Series) || metric.Series[column].Peak == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f", *metric.Series[column].Peak)
}
