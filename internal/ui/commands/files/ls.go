package files

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ListConfig struct {
	ui.DisplayConfig

	Client api.Client
	Config *config.Config
	Path   string
	Region string
}

// ListView is the Bubbletea model for displaying files
type ListView struct {
	ctx context.Context

	// State
	files   []api.FileInfo
	loading bool
	spinner *ui.SpinnerModel
	table   table.Model
	err     error

	conf ListConfig
}

// NewListView creates a new files list view
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
	return tea.Batch(m.spinner.Init(), m.fetchFiles)
}

// Update handles messages with minimal branching
func (m *ListView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case ui.SignalCancelMsg:
		return m, tea.Quit

	case filesLoadedMsg:
		return m.onLoaded(v.files)

	case *ui.UIError:
		return m.onError(v)

	case tea.KeyMsg:
		return m.onKey(v)

	default:
		return m.onDefault(msg)
	}
}

// onLoaded handles filesLoadedMsg
func (m *ListView) onLoaded(files []api.FileInfo) (tea.Model, tea.Cmd) {
	m.files = files
	m.loading = false

	if m.conf.SimpleOutput() {
		// Simple mode: print directly and quit
		m.printSimpleOutput()
		return m, tea.Quit
	}

	// Interactive mode: create styled table and quit (non-interactive)
	m.table = m.createTable()
	return m, tea.Quit
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
	// Handle quit keys only
	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m, tea.Quit
	}
	return m, nil
}

// onDefault handles default messages (e.g., spinner ticks)
func (m *ListView) onDefault(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Update spinner only in interactive mode while loading
	if m.conf.SimpleOutput() || !m.loading {
		return m, nil
	}

	var cmd tea.Cmd
	var spinnerModel tea.Model
	spinnerModel, cmd = m.spinner.Update(msg)
	m.spinner = spinnerModel.(*ui.SpinnerModel) //nolint:errcheck // Type assertion guaranteed by SpinnerModel structure
	return m, cmd
}

// createTable creates the interactive table from files
func (m *ListView) createTable() table.Model {
	sort.Slice(m.files, func(i, j int) bool {
		if m.files[i].IsFolder && !m.files[j].IsFolder {
			return true
		} else if !m.files[i].IsFolder && m.files[j].IsFolder {
			return false
		}
		return m.files[i].Name < m.files[j].Name
	})

	var rows []table.Row
	for _, file := range m.files {
		rows = append(rows, table.Row{
			file.Name,
			formatFileSize(file),
			formatLastModified(file.LastModified),
		})
	}

	return newTable(rows)
}

// printSimpleOutput prints files in non-TTY mode
func (m *ListView) printSimpleOutput() {
	if len(m.files) == 0 {
		fmt.Println("No files found")
		return
	}

	// Print simple table format
	fmt.Print(m.formatFilesTable())
}

// View renders the output
func (m *ListView) View() string {
	// Simple mode: output has already been printed directly
	if m.conf.SimpleOutput() {
		return ""
	}

	// Interactive mode: full UI experience
	if m.loading {
		return m.spinner.View() + " Loading files..."
	}

	if m.err != nil {
		return ui.FormatError(m.err)
	}

	if len(m.files) == 0 {
		return ui.WarningStyle.Render("No files found")
	}

	var output strings.Builder

	// Render the table with a title showing path
	title := fmt.Sprintf("Files: %s", m.conf.Path)
	output.WriteString(ui.TitleStyle.Render(title))
	output.WriteString("\n\n")
	output.WriteString(m.table.View())
	output.WriteString("\n")

	return output.String()
}

// formatFilesTable formats files for non-TTY output
func (m *ListView) formatFilesTable() string {
	var output strings.Builder

	// Calculate maximum name width from actual data
	maxNameWidth := len("NAME")
	for _, file := range m.files {
		if len(file.Name) > maxNameWidth {
			maxNameWidth = len(file.Name)
		}
	}

	// Add some padding for readability
	nameWidth := maxNameWidth + 2

	// Create format string with calculated width
	headerFormat := fmt.Sprintf("%%-%ds %%-15s %%-20s\n", nameWidth)
	rowFormat := fmt.Sprintf("%%-%ds %%-15s %%-20s\n", nameWidth)

	// Header
	output.WriteString(fmt.Sprintf(headerFormat, "NAME", "SIZE", "LAST MODIFIED"))

	// Files
	for _, file := range m.files {
		output.WriteString(fmt.Sprintf(rowFormat,
			file.Name,
			formatFileSize(file),
			formatLastModified(file.LastModified),
		))
	}

	return output.String()
}

// Messages

type filesLoadedMsg struct {
	files []api.FileInfo
}

// Commands (async operations)

func (m *ListView) fetchFiles() tea.Msg {
	// Call the real API
	files, err := m.conf.Client.ListFiles(m.ctx, m.conf.Config.ProjectID, m.conf.Path, m.conf.Region)
	if err != nil {
		return ui.NewAPIError(err)
	}
	return filesLoadedMsg{files}
}

// Utils

func newTable(rows []table.Row) table.Model {
	// Create table columns
	columns := []table.Column{
		{Title: "Name", Width: 50},
		{Title: "Size", Width: 15},
		{Title: "Last Modified", Width: 20},
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

// formatFileSize formats file size for display
func formatFileSize(file api.FileInfo) string {
	if file.IsFolder {
		return "Directory"
	}

	// Convert bytes to human readable format
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
	// Handle N/A or zero timestamps
	if timestamp == "" || timestamp == "0001-01-01T00:00:00Z" {
		return "N/A"
	}

	// Parse ISO timestamp
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		// Try alternative format
		t, err = time.Parse("2006-01-02T15:04:05Z", timestamp)
		if err != nil {
			return timestamp // Return as-is if parsing fails
		}
	}

	return t.Format("2006-01-02 15:04:05")
}
