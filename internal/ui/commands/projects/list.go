package projects

import (
	"context"
	"fmt"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ListConfig struct {
	ui.DisplayConfig

	Client api.Client
}

// ListView is the Bubbletea model for displaying projects
type ListView struct {
	ctx context.Context

	// State
	projects []api.Project
	loading  bool
	spinner  *ui.SpinnerModel
	table    table.Model
	err      error

	conf ListConfig
}

// NewListView creates a new projects list view
func NewListView(ctx context.Context, conf ListConfig) *ListView {
	return &ListView{
		ctx:     ctx,
		loading: true,
		spinner: ui.NewSpinner(),
		conf:    conf,
	}
}

// Init starts the data fetch

// Error returns the error if any occurred during execution
func (m *ListView) Error() error {
	return m.err
}

func (m *ListView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.fetchProjects)
}

// Update handles messages with minimal branching
func (m *ListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case ui.SignalCancelMsg:
		return m, tea.Quit

	case projectsLoadedMsg:
		return m.onLoaded(v.projects)

	case *ui.UIError:
		return m.onError(v)

	case tea.KeyMsg:
		if v.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m.onKey(v)

	default:
		return m.onDefault(msg)
	}
}

// onLoaded handles projectsLoadedMsg
func (m *ListView) onLoaded(projects []api.Project) (tea.Model, tea.Cmd) {
	m.projects = projects
	m.loading = false

	if m.conf.SimpleOutput() {
		// Simple mode: print directly and quit
		m.printSimpleOutput()
		return m, tea.Quit
	}

	// Interactive mode: create fancy scrollable table
	m.table = m.createTable()

	// Auto-quit if table fits on screen (no scrolling needed)
	if !ui.TableBiggerThanView(m.table) {
		return m, tea.Quit
	}
	return m, nil
}

// onError handles *ui.UIError
func (m *ListView) onError(err *ui.UIError) (tea.Model, tea.Cmd) {
	err.SilentExit = true // Will be shown in View()
	m.err = err
	m.loading = false

	if m.conf.SimpleOutput() {
		fmt.Printf("Error: %s\n", err.Error())
	}

	return m, tea.Quit
}

// onKey handles tea.KeyMsg
func (m *ListView) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Only handle keyboard input in interactive mode
	if m.conf.SimpleOutput() {
		return m, nil
	}

	// Handle quit keys (ctrl+c is handled by SignalCancelMsg)
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

// onDefault handles default messages (e.g., spinner ticks)
func (m *ListView) onDefault(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Update spinner only in interactive mode while loading
	if !m.conf.SimpleOutput() && m.loading {
		var cmd tea.Cmd
		spinnerModel, cmd := m.spinner.Update(msg)
		m.spinner = spinnerModel.(*ui.SpinnerModel) //nolint:errcheck // Type assertion guaranteed by SpinnerModel structure
		return m, cmd
	}
	return m, nil
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

// scrollDown scrolls the table down
func (m *ListView) scrollDown() (tea.Model, tea.Cmd) {
	if !m.loading && len(m.table.Rows()) > 0 {
		m.table.MoveDown(1)
	}
	return m, nil
}

// scrollUp scrolls the table up
func (m *ListView) scrollUp() (tea.Model, tea.Cmd) {
	if !m.loading && len(m.table.Rows()) > 0 {
		m.table.MoveUp(1)
	}
	return m, nil
}

// pageDown scrolls the table down by page
func (m *ListView) pageDown() (tea.Model, tea.Cmd) {
	if !m.loading && len(m.table.Rows()) > 0 {
		m.table.MoveDown(10)
	}
	return m, nil
}

// pageUp scrolls the table up by page
func (m *ListView) pageUp() (tea.Model, tea.Cmd) {
	if !m.loading && len(m.table.Rows()) > 0 {
		m.table.MoveUp(10)
	}
	return m, nil
}

// delegateToTable passes navigation keys to the table
func (m *ListView) delegateToTable(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if !m.loading && len(m.projects) > 0 {
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		return m, cmd
	}
	return m, nil
}

// createTable creates the interactive table from projects
func (m *ListView) createTable() table.Model {
	var rows []table.Row
	for _, project := range m.projects {
		rows = append(rows, table.Row{
			project.ID,
			project.Name,
		})
	}
	return newTable(rows)
}

// printSimpleOutput prints projects in non-TTY mode
func (m *ListView) printSimpleOutput() {
	if len(m.projects) == 0 {
		fmt.Println("No projects found")
		return
	}

	// Print simple table format
	fmt.Print(m.formatProjectsTable())
}

// View renders the output
func (m *ListView) View() string {
	// Simple mode: output has already been printed directly
	if m.conf.SimpleOutput() {
		return ""
	}

	// Interactive mode: full UI experience
	if m.loading {
		return m.spinner.View() + " Loading projects..."
	}

	if m.err != nil {
		return ui.FormatError(m.err)
	}

	if len(m.projects) == 0 {
		return ui.WarningStyle.Render("No projects found")
	}

	var output strings.Builder

	// Render the table with a title
	output.WriteString(ui.TitleStyle.Render("Projects"))
	output.WriteString("\n\n")
	output.WriteString(m.table.View())
	output.WriteString("\n\n")

	if ui.TableBiggerThanView(m.table) {
		// Add navigation help (indented by one space to distinguish from regular output)
		navHelp := " j/k scroll • J/K scroll to bottom/top • ctrl+d/ctrl+u page up/down • <esc> or q to quit"
		output.WriteString(ui.HelpStyle.Render(navHelp))
		output.WriteString("\n")
	}
	output.WriteString(ui.HelpStyle.Render("You can set your current projects by running `cerebrium projects set {project_id}`\n"))

	return output.String()
}

// formatProjectsTable formats projects for non-TTY output
func (m *ListView) formatProjectsTable() string {
	var output strings.Builder

	// Header
	output.WriteString(fmt.Sprintf("%-50s %-50s\n", "ID", "NAME"))

	// Projects
	for _, project := range m.projects {
		output.WriteString(fmt.Sprintf("%-50s %-50s\n",
			project.ID,
			project.Name,
		))
	}

	return output.String()
}

// Messages

type projectsLoadedMsg struct {
	projects []api.Project
}

// Commands (async operations)

func (m *ListView) fetchProjects() tea.Msg {
	// Call the real API
	projects, err := m.conf.Client.GetProjects(m.ctx)
	if err != nil {
		return ui.NewAPIError(err)
	}
	return projectsLoadedMsg{projects}
}

// Utils

func newTable(rows []table.Row) table.Model {
	// Calculate dynamic column widths based on content
	// Add padding for better spacing
	const padding = 4

	// Initialize with header lengths
	widths := map[int]int{
		0: len("ID"),
		1: len("Name"),
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
		{Title: "Name", Width: widths[1] + padding},
	}

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("11")).
		BorderBottom(true).
		Bold(true).
		Padding(0, 1)
	// Keep selected row subtle since we're just browsing, not selecting
	s.Selected = s.Selected.
		Bold(false)

	// Create table with styling
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(min(len(rows)+1, ui.MAX_TABLE_HEIGHT)),
		table.WithFocused(true), // Make it interactive/scrollable
	)
	t.SetStyles(s)

	return t
}
