package apps

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/charmbracelet/lipgloss"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

type ListConfig struct {
	ui.DisplayConfig

	Client    api.Client
	ProjectID string
}

// ListView is the Bubbletea model for displaying apps
type ListView struct {
	ctx context.Context

	// State
	apps    []api.App
	loading bool
	spinner *ui.SpinnerModel
	table   table.Model
	err     error

	conf ListConfig
}

// NewListView creates a new apps list view
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
	return tea.Batch(m.spinner.Init(), m.fetchApps)
}

// Update handles messages
func (m *ListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.SignalCancelMsg:
		// Handle termination signal (SIGINT, SIGTERM)
		// Just exit cleanly - app list doesn't need cleanup
		return m, tea.Quit

	case appsLoadedMsg:
		return m.onLoaded(msg.apps)

	case *ui.UIError:
		msg.SilentExit = true // Will be shown in View()
		m.err = msg
		m.loading = false

		if m.conf.SimpleOutput() {
			fmt.Printf("Error: %s\n", msg.Error())
		}

		return m, tea.Quit

	case tea.KeyMsg:
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

	// Interactive mode: full UI experience
	if m.loading {
		return m.spinner.View() + " Loading apps..."
	}

	if m.err != nil {
		return ui.FormatError(m.err)
	}

	if len(m.apps) == 0 {
		return ui.WarningStyle.Render(fmt.Sprintf("No apps found for project %s", m.conf.ProjectID)) + "\n"
	}

	// Render the table with a title
	var output strings.Builder
	output.WriteString(ui.TitleStyle.Render("Apps"))
	output.WriteString("\n\n")
	output.WriteString(m.table.View())
	output.WriteString("\n\n")

	if ui.TableBiggerThanView(m.table) {
		// Add navigation help (indented by one space to distinguish from regular output)
		navHelp := " j/k scroll • J/K scroll to bottom/top • ctrl+d/ctrl+u page up/down • <esc> or q to quit"
		output.WriteString(ui.HelpStyle.Render(navHelp))
		output.WriteString("\n")
	}
	return output.String()
}

// formatAppsTable formats apps for non-TTY output
func (m *ListView) formatAppsTable() string {
	var output strings.Builder

	// Header
	output.WriteString(fmt.Sprintf("%-50s %-10s %-20s %-20s\n", "ID", "STATUS", "CREATED", "UPDATED"))

	// Apps
	for _, app := range m.apps {
		output.WriteString(fmt.Sprintf("%-50s %-10s %-20s %-20s\n",
			app.ID,
			app.Status,
			app.CreatedAt.Format("2006-01-02 15:04:05"),
			app.UpdatedAt.Format("2006-01-02 15:04:05"),
		))
	}

	return output.String()
}

// Messages

type appsLoadedMsg struct {
	apps []api.App
}

// Commands (async operations)

func (m *ListView) fetchApps() tea.Msg {
	// Call the real API
	apps, err := m.conf.Client.GetApps(m.ctx, m.conf.ProjectID)
	if err != nil {
		return ui.NewAPIError(err)
	}
	return appsLoadedMsg{apps}
}

func (m *ListView) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.conf.SimpleOutput() {
		return m, nil
	}
	slog.Debug("key pressed", "key", msg.String())

	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	case "J":
		return m.scrollToBottom()
	case "K":
		return m.scrollToTop()
	case "j":
		return m.scrollDown()
	case "k":
		return m.scrollUp()
	case "ctrl+d":
		return m.pageDown()
	case "ctrl+u":
		return m.pageUp()
	}

	// Let table handle navigation (j/k, arrows)
	return m.delegateToTable(msg)
}

// scrollToBottom scrolls the table to the bottom
func (m *ListView) scrollToBottom() (tea.Model, tea.Cmd) {
	if !m.loading && len(m.table.Rows()) > 0 {
		m.table.GotoBottom()
	}
	return m, nil
}

// scrollToTop scrolls the table to the top
func (m *ListView) scrollToTop() (tea.Model, tea.Cmd) {
	if !m.loading && len(m.table.Rows()) > 0 {
		m.table.GotoTop()
	}
	return m, nil
}

// scrollDown scrolls the table to the bottom
func (m *ListView) scrollDown() (tea.Model, tea.Cmd) {
	if !m.loading && len(m.table.Rows()) > 0 {
		m.table.MoveDown(1)
	}
	return m, nil
}

// scrollUp scrolls the table to the top
func (m *ListView) scrollUp() (tea.Model, tea.Cmd) {
	if !m.loading && len(m.table.Rows()) > 0 {
		m.table.MoveUp(1)
	}
	return m, nil
}

// pageDown scrolls the table to the bottom
func (m *ListView) pageDown() (tea.Model, tea.Cmd) {
	if !m.loading && len(m.table.Rows()) > 0 {
		m.table.MoveDown(10)
	}
	return m, nil
}

// pageUp scrolls the table to the top
func (m *ListView) pageUp() (tea.Model, tea.Cmd) {
	if !m.loading && len(m.table.Rows()) > 0 {
		m.table.MoveUp(10)
	}
	return m, nil
}

// delegateToTable passes navigation keys to the table
func (m *ListView) delegateToTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.loading && len(m.table.Rows()) > 0 {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *ListView) onLoaded(apps []api.App) (tea.Model, tea.Cmd) {
	m.apps = apps
	m.loading = false

	if m.conf.SimpleOutput() {
		// Simple mode: print directly
		if len(m.apps) == 0 {
			fmt.Println("No apps found")
		} else {
			// Print simple table format
			fmt.Print(m.formatAppsTable())
		}
		return m, tea.Quit
	}

	// Interactive mode: create fancy table
	var rows []table.Row
	for _, app := range m.apps {
		rows = append(rows, table.Row{
			app.ID,
			strings.TrimSpace(ui.ColorizeStatus(app.Status)),
			strings.TrimSpace(ui.FormatTimestamp(app.UpdatedAt)),
		})
	}
	m.table = newTable(rows)

	// Auto-quit if table fits on screen (no scrolling needed)
	if !ui.TableBiggerThanView(m.table) {
		return m, tea.Quit
	}
	return m, nil
}

// Utils

func newTable(rows []table.Row) table.Model {
	// Calculate dynamic column widths based on content
	// Add padding for potential ANSI codes and better spacing
	const padding = 8

	// Initialize with header lengths
	widths := map[int]int{
		0: len("ID"),
		1: len("Status"),
		2: len("Last Updated"),
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
		{Title: "ID", Width: widths[0] + padding},
		{Title: "Status", Width: widths[1] + padding},
		{Title: "Last Updated", Width: widths[2] + padding},
	}

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("11")).
		BorderBottom(true).
		Bold(true).
		Padding(0, 1)
	s.Selected = s.Selected.Bold(true)

	// Create table with styling
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(min(len(rows)+1, ui.MAX_TABLE_HEIGHT)), // Include header
		table.WithFocused(true),
	)
	t.SetStyles(s)

	return t
}
