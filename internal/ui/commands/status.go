package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/statuspage"
	"github.com/cerebriumai/cerebrium/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
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
		return m, tea.Quit

	case tea.KeyMsg:
		// Only handle keyboard input in interactive mode
		if m.conf.SimpleOutput() {
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.err = ui.NewUserCancelledError()
			return m, tea.Quit
		}

	case statusLoadedMsg:
		m.status = msg.status
		m.state = StateStatusSuccess

		// Print directly in simple mode
		if m.conf.SimpleOutput() {
			m.printSimpleStatus()
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

// formatStatusText returns styled status text for a component
func formatStatusText(status statuspage.Status) string {
	switch status {
	case statuspage.StatusOperational:
		return ui.GreenStyle.Render("✓ Operational")
	case statuspage.StatusDegradedPerformance, statuspage.StatusDegraded:
		return ui.YellowStyle.Render("⚠ Degraded")
	case statuspage.StatusDowntime:
		return ui.RedStyle.Render("✗ Down")
	case statuspage.StatusMaintenance:
		return ui.CyanStyle.Render("🔧 Maintenance")
	case statuspage.StatusNotMonitored:
		return ui.PendingStyle.Render("- Not Monitored")
	default:
		caser := cases.Title(language.English)
		return ui.PendingStyle.Render("? " + caser.String(string(status)))
	}
}

// renderInteractiveStatus renders status for interactive TTY mode
func (m *StatusView) renderInteractiveStatus() string {
	var output strings.Builder

	if len(m.status.OngoingIncidents) == 0 {
		// All operational - show green status with table
		output.WriteString(ui.GreenStyle.Render("✓ All Systems Operational") + "\n\n")
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
			if incident.Status == statuspage.StatusDowntime {
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
	}

	// Show service status table
	if len(m.status.AllComponents) > 0 {
		output.WriteString(ui.TitleStyle.Render("Service Status") + "\n\n")
		output.WriteString(m.renderStatusTable())
	}

	return output.String()
}

// renderStatusTable renders the component status table using RenderDetailTable
func (m *StatusView) renderStatusTable() string {
	// Sort components by name for consistent display
	components := make([]statuspage.Component, len(m.status.AllComponents))
	copy(components, m.status.AllComponents)
	sort.Slice(components, func(i, j int) bool {
		return components[i].Name < components[j].Name
	})

	// Build rows for the detail table
	rows := make([]ui.TableRow, 0, len(components))
	for _, component := range components {
		rows = append(rows, ui.TableRow{
			Label: component.Name,
			Value: formatStatusText(component.Status),
		})
	}

	return ui.RenderDetailTable([]ui.TableSection{
		{Rows: rows},
	})
}
