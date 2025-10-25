package project

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
	// Don't quit - let user scroll and interact
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
	case "q", "esc":
		return m, tea.Quit
	case "J":
		return m.scrollToBottom()
	case "K":
		return m.scrollToTop()
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
	if !m.loading && len(m.projects) > 0 {
		m.table.GotoBottom()
	}
	return m, nil
}

// scrollToTop scrolls the table to the top
func (m *ListView) scrollToTop() (tea.Model, tea.Cmd) {
	if !m.loading && len(m.projects) > 0 {
		m.table.GotoTop()
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
	// Add helpful message about setting project context
	fmt.Println()
	fmt.Printf("You can set your current project context by running 'cerebrium project set %s'\n", m.projects[0].ID)
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

	// Add helpful messages (indented by one space to distinguish from regular output)
	helpText := fmt.Sprintf(" You can set your current project context by running 'cerebrium project set %s'", m.projects[0].ID)
	output.WriteString(ui.HelpStyle.Render(helpText))
	output.WriteString("\n")

	// Add navigation help
	navHelp := " Use ↑/↓ or j/k to scroll • J/K to scroll to bottom/top • <esc> or q to quit"
	output.WriteString(ui.HelpStyle.Render(navHelp))

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
	// Create table columns
	columns := []table.Column{
		{Title: "ID", Width: 50},
		{Title: "Name", Width: 50},
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
		Foreground(lipgloss.Color("11")).
		Bold(false)

	// Create table with styling
	// Set height to 15 rows for scrolling through projects
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(15),
		table.WithFocused(true), // Make it interactive/scrollable
	)
	t.SetStyles(s)

	return t
}
