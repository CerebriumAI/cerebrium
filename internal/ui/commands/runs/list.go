package runs

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ListConfig configures the runs list view
type ListConfig struct {
	ui.DisplayConfig

	Client    api.Client
	AsyncOnly bool
	ProjectID string
	AppName   string
}

// ListView is the Bubbletea model for displaying runs
type ListView struct {
	ctx context.Context

	// State
	runs    []api.Run
	loading bool
	spinner *ui.SpinnerModel
	table   table.Model
	err     error

	conf ListConfig
}

// NewListView creates a new runs list view
func NewListView(ctx context.Context, conf ListConfig) *ListView {
	return &ListView{
		ctx:     ctx,
		loading: true,
		spinner: ui.NewSpinner(),
		conf:    conf,
	}
}

// Error returns the error if any occurred during execution
func (m *ListView) Error() error {
	return m.err
}

// Init starts the data fetch
func (m *ListView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.fetchRuns)
}

// Update handles messages
func (m *ListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.SignalCancelMsg:
		// Handle termination signal (SIGINT, SIGTERM)
		// Just exit cleanly - runs list doesn't need cleanup
		return m, tea.Quit

	case runsLoadedMsg:
		return m.onLoaded(msg.runs)

	case *ui.UIError:
		m.err = msg
		m.loading = false

		// In interactive mode, error will be shown in View()
		// In non-interactive mode, let main.go print it
		if !m.conf.SimpleOutput() {
			msg.SilentExit = true // Prevent double printing in interactive mode
		}

		return m, tea.Quit

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m.onKey(msg)

	default:
		// Update spinner only in interactive mode
		if !m.conf.SimpleOutput() && m.loading {
			var cmd tea.Cmd
			spinnerModel, cmd := m.spinner.Update(msg)
			m.spinner = spinnerModel.(*ui.SpinnerModel) //nolint:errcheck // Type assertion guaranteed by SpinnerModel structure
			return m, cmd
		}
	}

	return m, nil
}

// View renders the output
func (m *ListView) View() string {
	// Simple mode: output has already been printed directly
	if m.conf.SimpleOutput() {
		return ""
	}

	// Interactive mode: full UI experience with colors and animations
	if m.loading {
		loadingText := fmt.Sprintf(" Loading runs for %s", m.conf.AppName)
		if m.conf.AsyncOnly {
			loadingText += " (async only)"
		}
		loadingText += "..."
		return m.spinner.View() + loadingText
	}

	if m.err != nil {
		// Add multiple lines to see if the error is being overwritten
		return ui.FormatError(m.err)
	}

	if len(m.runs) == 0 {
		noRunsMsg := fmt.Sprintf("No runs found for app: %s", m.conf.AppName)
		if m.conf.AsyncOnly {
			noRunsMsg = fmt.Sprintf("No async runs found for app: %s", m.conf.AppName)
		}
		return ui.WarningStyle.Render(noRunsMsg)
	}

	// Render the table with a stylish title
	title := fmt.Sprintf("Runs for %s", m.conf.AppName)
	if m.conf.AsyncOnly {
		title += " (async only)"
	}

	// Render the table with a title
	var output strings.Builder
	output.WriteString(ui.TitleStyle.Render(title))
	output.WriteString("\n\n")
	output.WriteString(m.table.View())
	output.WriteString("\n\n")

	return output.String()
}

// Commands

func (m *ListView) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle quit keys only
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	}
	return m, nil
}

func (m *ListView) onLoaded(runs []api.Run) (tea.Model, tea.Cmd) {
	m.runs = runs
	m.loading = false

	// Sort by created date (most recent first)
	sort.Slice(m.runs, func(i, j int) bool {
		return m.runs[i].CreatedAt.After(m.runs[j].CreatedAt)
	})

	if m.conf.SimpleOutput() {
		// Simple mode: print directly with clean formatting
		if len(m.runs) == 0 {
			fmt.Printf("No runs found for app: %s\n", m.conf.AppName)
		} else {
			// Print simple table format
			fmt.Print(m.formatRunsTable())
		}
		return m, tea.Quit
	}

	// Interactive mode: create fancy table with colors and styling
	var rows []table.Row
	for _, run := range m.runs {
		rows = append(rows, table.Row{
			run.ID,
			run.FunctionName,
			strings.TrimSpace(m.colorizeRunStatus(run.GetDisplayStatus())),
			strings.TrimSpace(ui.FormatTimestamp(run.CreatedAt)),
			m.formatAsyncStatus(run.Async),
		})
	}

	// Create styled table and quit (non-interactive)
	m.table = newTable(rows)
	return m, tea.Quit
}

// formatRunsTable formats runs for non-TTY output
func (m *ListView) formatRunsTable() string {
	var output strings.Builder

	// Header
	output.WriteString(fmt.Sprintf("%-40s %-30s %-15s %-25s %-10s\n",
		"RUN ID", "FUNCTION NAME", "STATUS", "CREATED AT", "ASYNC"))

	// Runs
	for _, run := range m.runs {
		asyncStr := "No"
		if run.Async {
			asyncStr = "Yes"
		}

		output.WriteString(fmt.Sprintf("%-40s %-30s %-15s %-25s %-10s\n",
			run.ID,
			run.FunctionName,
			run.GetDisplayStatus(),
			run.CreatedAt.Format("2006-01-02 15:04:05 MST"),
			asyncStr,
		))
	}

	return output.String()
}

// colorizeRunStatus adds color to run status based on its value
func (m *ListView) colorizeRunStatus(status string) string {
	statusLower := strings.ToLower(status)

	var style lipgloss.Style
	switch {
	case strings.Contains(statusLower, "running"), strings.Contains(statusLower, "pending"):
		// Active/running states - blue
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	case strings.Contains(statusLower, "success"), strings.Contains(statusLower, "completed"):
		// Success states - green
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	case strings.Contains(statusLower, "fail"), strings.Contains(statusLower, "error"):
		// Error states - red
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	case strings.Contains(statusLower, "cancel"), strings.Contains(statusLower, "abort"):
		// Cancelled states - yellow
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	default:
		// Unknown states - default color
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	}

	return style.Render(status)
}

// formatAsyncStatus formats the async boolean as a styled string
func (m *ListView) formatAsyncStatus(isAsync bool) string {
	if isAsync {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			Bold(true).
			Render("Yes")
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Render("No")
}

// Messages

type runsLoadedMsg struct {
	runs []api.Run
}

// Commands (async operations)

func (m *ListView) fetchRuns() tea.Msg {
	// Construct the app ID, handling both formats:
	// - "5-dockerfile" -> "dev-p-0780791d-5-dockerfile"
	// - "dev-p-0780791d-5-dockerfile" -> "dev-p-0780791d-5-dockerfile"
	appID := normalizeAppID(m.conf.ProjectID, m.conf.AppName)

	// Call the API to fetch runs
	runs, err := m.conf.Client.GetRuns(m.ctx, m.conf.ProjectID, appID, m.conf.AsyncOnly)
	if err != nil {
		return ui.NewAPIError(err)
	}
	return runsLoadedMsg{runs: runs}
}

// Utils

// normalizeAppID ensures the app ID has the correct format.
// If the appName already starts with the projectID prefix, use it as-is.
// Otherwise, prepend the projectID.
func normalizeAppID(projectID, appName string) string {
	// Check if appName already has the project ID prefix
	expectedPrefix := projectID + "-"
	if strings.HasPrefix(appName, expectedPrefix) {
		return appName
	}
	// Prepend the project ID
	return fmt.Sprintf("%s-%s", projectID, appName)
}

func newTable(rows []table.Row) table.Model {
	// Calculate dynamic column widths based on content
	// Add padding for potential ANSI codes and better spacing
	const padding = 8

	// Initialize with header lengths
	widths := map[int]int{
		0: len("Run ID"),
		1: len("Function Name"),
		2: len("Status"),
		3: len("Created At"),
		4: len("Async"),
	}

	// Find max width for each column
	for _, row := range rows {
		for i, cell := range row {
			// Use lipgloss Width to get visual width (handles ANSI codes)
			cellWidth := lipgloss.Width(cell)
			if cellWidth > widths[i] {
				widths[i] = cellWidth
			}
		}
	}

	// Create table columns with dynamic widths plus padding
	columns := []table.Column{
		{Title: "Run ID", Width: widths[0] + padding},
		{Title: "Function Name", Width: widths[1] + padding},
		{Title: "Status", Width: widths[2] + padding},
		{Title: "Created At", Width: widths[3] + padding},
		{Title: "Async", Width: widths[4] + padding},
	}

	// Style the table (non-interactive, no selection highlighting)
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("11")).
		BorderBottom(true).
		Bold(true).
		Padding(0, 1)
	// Remove selection highlighting for non-interactive mode
	s.Selected = s.Selected.
		Foreground(lipgloss.NoColor{}).
		Background(lipgloss.NoColor{}).
		Bold(false)

	// Create table with styling - show all rows, not focused
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(len(rows)+1), // Show all rows
		table.WithFocused(false),
	)
	t.SetStyles(s)

	return t
}
