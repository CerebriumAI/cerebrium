package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/statuspage"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// StatusState represents the current state of status checking
type StatusState int

const (
	StateStatusLoading StatusState = iota
	StateStatusSuccess
	StateStatusError
)

// StatusConfig contains status command configuration
type StatusConfig struct {
	ui.DisplayConfig

	Client *statuspage.Client
}

// StatusView is the Bubbletea model for the status display
type StatusView struct {
	ctx context.Context

	state   StatusState
	spinner *ui.SpinnerModel
	err     *ui.UIError
	table   table.Model

	// Status data
	status *statuspage.StatusResponse

	conf StatusConfig
}

// NewStatusView creates a new status view
func NewStatusView(ctx context.Context, conf StatusConfig) *StatusView {
	return &StatusView{
		ctx:     ctx,
		state:   StateStatusLoading,
		spinner: ui.NewSpinner(),
		conf:    conf,
	}
}

// Init starts the status checking flow

// Error returns the error if any occurred during execution
func (m *StatusView) Error() error {
	return m.err
}

func (m *StatusView) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Init(),
		m.fetchStatus,
	)
}

// Update handles messages
func (m *StatusView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.SignalCancelMsg:
		// Handle termination signal (SIGINT, SIGTERM)
		if m.conf.SimpleOutput() {
			fmt.Printf("\nCancelled by user\n")
		}
		m.err = ui.NewUserCancelledError()
		return m, tea.Quit

	case tea.KeyMsg:
		// Only handle keyboard input in interactive mode
		if m.conf.SimpleOutput() {
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.err = ui.NewUserCancelledError()
			return m, tea.Quit
		}

	case statusLoadedMsg:
		m.status = msg.status
		m.state = StateStatusSuccess

		// Print directly in simple mode
		if m.conf.SimpleOutput() {
			m.printSimpleStatus()
		} else {
			// Create table for interactive mode
			m.createStatusTable()
		}

		return m, tea.Quit

	case *ui.UIError:
		// Structured error from async operations
		msg.SilentExit = true // Will be shown in View()
		m.err = msg
		m.state = StateStatusError

		if m.conf.SimpleOutput() {
			fmt.Printf("Error: %s\n", msg.Error())
		}

		return m, tea.Quit

	default:
		// Update spinner only in interactive mode
		if !m.conf.SimpleOutput() && m.state == StateStatusLoading {
			var cmd tea.Cmd
			spinnerModel, cmd := m.spinner.Update(msg)
			m.spinner = spinnerModel.(*ui.SpinnerModel) //nolint:errcheck // Type assertion guaranteed by SpinnerModel structure
			return m, cmd
		}
	}

	return m, nil
}

// View renders the output
func (m *StatusView) View() string {
	// Simple mode: output has already been printed directly
	if m.conf.SimpleOutput() {
		return ""
	}

	// Interactive mode: full UI experience
	switch m.state {
	case StateStatusLoading:
		return m.spinner.View() + " Checking Cerebrium service status..."

	case StateStatusError:
		if m.err != nil {
			return ui.FormatError(m.err)
		}
		return ui.ErrorStyle.Render("✗ Error checking status") + "\n"

	case StateStatusSuccess:
		if m.status == nil {
			return ui.WarningStyle.Render("No status data received") + "\n"
		}
		return m.renderInteractiveStatus()

	default:
		return ""
	}
}

// GetError returns any error that occurred during status checking
func (m *StatusView) GetError() *ui.UIError {
	return m.err
}

// Messages

type statusLoadedMsg struct {
	status *statuspage.StatusResponse
}

// Commands (async operations)

func (m *StatusView) fetchStatus() tea.Msg {
	status, err := m.conf.Client.GetStatus(m.ctx)
	if err != nil {
		return ui.NewAPIError(fmt.Errorf("failed to fetch status: %w", err))
	}
	return statusLoadedMsg{status: status}
}

// printSimpleStatus prints status for non-TTY mode
func (m *StatusView) printSimpleStatus() {
	if len(m.status.OngoingIncidents) == 0 {
		// All operational
		fmt.Println("Cerebrium Status: All Systems Operational")
		fmt.Println()
		for _, component := range m.status.AllComponents {
			fmt.Printf("%-20s %s\n", component.Name, strings.ToUpper(string(component.Status)))
		}
	} else {
		// There are incidents
		fmt.Printf("Cerebrium Status: %d Active Issue(s)\n", len(m.status.OngoingIncidents))
		fmt.Println()

		for _, incident := range m.status.OngoingIncidents {
			fmt.Printf("Issue: %s\n", incident.Name)
			fmt.Printf("Status: %s\n", strings.ToUpper(string(incident.Status)))
			if len(incident.AffectedComponents) > 0 {
				fmt.Printf("Affected: ")
				componentNames := make([]string, 0, len(incident.AffectedComponents))
				for _, comp := range incident.AffectedComponents {
					componentNames = append(componentNames, comp.Name)
				}
				fmt.Printf("%s\n", strings.Join(componentNames, ", "))
			}
			fmt.Printf("More info: %s\n", incident.URL)
			fmt.Println()
		}

		// Show all components status
		fmt.Println("Component Status:")
		for _, component := range m.status.AllComponents {
			fmt.Printf("%-20s %s\n", component.Name, strings.ToUpper(string(component.Status)))
		}
	}
}

// createStatusTable creates a beautiful table for status display
func (m *StatusView) createStatusTable() {
	if m.status == nil || len(m.status.AllComponents) == 0 {
		return
	}

	// Sort components by name for consistent display
	components := make([]statuspage.Component, len(m.status.AllComponents))
	copy(components, m.status.AllComponents)
	sort.Slice(components, func(i, j int) bool {
		return components[i].Name < components[j].Name
	})

	// Create table rows with status styling
	var rows []table.Row
	for _, component := range components {
		var statusText string
		switch component.Status {
		case "operational":
			statusText = ui.GreenStyle.Render("✓ Operational")
		case "degraded_performance", "degraded":
			statusText = ui.YellowStyle.Render("⚠ Degraded")
		case "downtime":
			statusText = ui.RedStyle.Render("✗ Down")
		case "maintenance":
			statusText = ui.CyanStyle.Render("🔧 Maintenance")
		default:
			caser := cases.Title(language.English)
			statusText = ui.PendingStyle.Render("? " + caser.String(string(component.Status)))
		}

		// Service name in explicit plain white text, status with color
		plainServiceName := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Render(component.Name)
		rows = append(rows, table.Row{
			plainServiceName,              // Explicitly plain white text for service names
			strings.TrimSpace(statusText), // Colored status only
		})
	}

	// Calculate dynamic column widths
	const padding = 8
	widths := map[int]int{
		0: len("Service"),
		1: len("Status"),
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

	// Create table columns with dynamic widths
	columns := []table.Column{
		{Title: "Service", Width: widths[0] + padding},
		{Title: "Status", Width: widths[1] + padding},
	}

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("11")).
		BorderBottom(true).
		Bold(true).
		Padding(0, 1)
	s.Selected = lipgloss.Style{} // No selection styling

	// Create table
	m.table = table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithHeight(min(len(rows)+1, 20)), // Include header, max 20 rows
		table.WithFocused(false),
	)
	m.table.SetStyles(s)
}

// renderInteractiveStatus renders status for interactive TTY mode
func (m *StatusView) renderInteractiveStatus() string {
	var output strings.Builder

	if len(m.status.OngoingIncidents) == 0 {
		// All operational - show green status with table
		output.WriteString(ui.GreenStyle.Render("✓ All Systems Operational") + "\n\n")

		// Show the beautiful status table
		if m.table.Columns() != nil {
			output.WriteString(ui.TitleStyle.Render("Service Status") + "\n\n")
			output.WriteString(m.table.View())
		}
	} else {
		// There are incidents - show them prominently
		incidentCount := len(m.status.OngoingIncidents)
		title := fmt.Sprintf("⚠ %d Active Issue", incidentCount)
		if incidentCount > 1 {
			title += "s"
		}
		output.WriteString(ui.YellowStyle.Render(title) + "\n\n")

		// Show incident details
		for _, incident := range m.status.OngoingIncidents {
			output.WriteString(ui.ErrorStyle.Render("Issue: ") + incident.Name + "\n")

			statusColor := ui.YellowStyle
			if incident.Status == "downtime" {
				statusColor = ui.RedStyle
			}
			caser := cases.Title(language.English)
			output.WriteString(
				ui.PendingStyle.Render("Status: ") +
					statusColor.Render(caser.String(strings.ReplaceAll(string(incident.Status), "_", " "))) + "\n")

			if len(incident.AffectedComponents) > 0 {
				componentNames := make([]string, 0, len(incident.AffectedComponents))
				for _, comp := range incident.AffectedComponents {
					componentNames = append(componentNames, comp.Name)
				}
				output.WriteString(ui.PendingStyle.Render("Affected: ") + strings.Join(componentNames, ", ") + "\n")
			}

			output.WriteString(ui.PendingStyle.Render("More info: ") + ui.URLStyle.Render(incident.URL) + "\n")
			output.WriteString("\n")
		}

		// Show status table even with incidents
		if m.table.Columns() != nil {
			output.WriteString(ui.TitleStyle.Render("Service Status") + "\n\n")
			output.WriteString(m.table.View())
		}
	}

	return output.String()
}
