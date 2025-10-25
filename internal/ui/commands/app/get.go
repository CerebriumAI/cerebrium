package app

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

type GetConfig struct {
	ui.DisplayConfig

	Client    api.Client
	AppID     string
	ProjectID string
}

// GetView is the Bubbletea model for displaying app details
type GetView struct {
	ctx context.Context

	// State
	appDetails  *api.AppDetails
	loading     bool
	spinner     *ui.SpinnerModel
	err         error
	parseErrors []string // Track parsing errors to show helpful message

	// Display options
	conf GetConfig
}

// NewGetView creates a new app get view
func NewGetView(ctx context.Context, conf GetConfig) *GetView {
	return &GetView{
		ctx:     ctx,
		loading: true,
		spinner: ui.NewSpinner(),
		conf:    conf,
	}
}

// Error returns the error if any occurred during execution
func (m *GetView) Error() error {
	return m.err
}

// Init starts the data fetch
func (m *GetView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.fetchAppDetails)
}

// Update handles messages
func (m *GetView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.SignalCancelMsg:
		// Handle termination signal
		if m.conf.SimpleOutput() {
			fmt.Fprintf(os.Stderr, "\nCancelled\n")
		}
		return m, tea.Quit

	case appDetailsLoadedMsg:
		m.appDetails = msg.appDetails
		m.loading = false

		if m.conf.SimpleOutput() {
			// Simple mode: print directly
			fmt.Print(m.formatAppDetailsSimple())
		}

		return m, tea.Quit

	case *ui.UIError:
		msg.SilentExit = true
		m.err = msg
		m.loading = false

		if m.conf.SimpleOutput() {
			fmt.Printf("Error: %s\n", msg.Error())
		}

		return m, tea.Quit

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
func (m *GetView) View() string {
	// Simple mode: output has already been printed directly
	if m.conf.SimpleOutput() {
		return ""
	}

	// Interactive mode: full UI experience
	if m.loading {
		return m.spinner.View() + " Loading app details..."
	}

	if m.err != nil {
		return ui.FormatError(m.err)
	}

	if m.appDetails == nil {
		return ui.WarningStyle.Render("No app details found")
	}

	// Render detailed table
	return m.formatAppDetailsTable()
}

// formatAppDetailsTable formats app details for interactive display
func (m *GetView) formatAppDetailsTable() string {
	app := m.appDetails

	// Build sections
	var sections []ui.TableSection

	// APP Section
	appRows := []ui.TableRow{
		{Label: "ID", Value: app.ID},
		{Label: "Created At", Value: ui.FormatTimestamp(app.CreatedAt)},
	}
	if !app.UpdatedAt.Equal(app.CreatedAt) {
		appRows = append(appRows, ui.TableRow{Label: "Updated At", Value: ui.FormatTimestamp(app.UpdatedAt)})
	}
	sections = append(sections, ui.TableSection{Header: "APP", Rows: appRows})

	// HARDWARE Section
	hardwareRows := []ui.TableRow{}
	if app.Hardware != "" {
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "Compute", Value: app.Hardware})
	} else {
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "Compute", Value: "Data Unavailable"})
	}
	cpu, err := app.GetCPU()
	if err != nil {
		m.parseErrors = append(m.parseErrors, "CPU")
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "CPU", Value: ui.ErrorStyle.Render("[error parsing CPU]")})
	} else if cpu == 0 {
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "CPU", Value: "Data Unavailable"})
	} else {
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "CPU", Value: fmt.Sprintf("%d cores", cpu)})
	}
	memory, err := app.GetMemory()
	if err != nil {
		m.parseErrors = append(m.parseErrors, "Memory")
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "Memory", Value: ui.ErrorStyle.Render("[error parsing Memory]")})
	} else if memory == 0 {
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "Memory", Value: "Data Unavailable"})
	} else {
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "Memory", Value: fmt.Sprintf("%d GB", memory)})
	}
	gpuCount, err := app.GetGPUCount()
	if err != nil {
		m.parseErrors = append(m.parseErrors, "GPU Count")
		if app.Hardware != "CPU" {
			hardwareRows = append(hardwareRows, ui.TableRow{Label: "GPU Count", Value: ui.ErrorStyle.Render("[error parsing GPU Count]")})
		}
	} else if app.Hardware != "CPU" && gpuCount > 0 {
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "GPU Count", Value: fmt.Sprintf("%d", gpuCount)})
	}
	sections = append(sections, ui.TableSection{Header: "HARDWARE", Rows: hardwareRows})

	// SCALING PARAMETERS Section
	scalingRows := []ui.TableRow{}
	if cooldown, err := app.GetCooldownPeriodSeconds(); err != nil {
		m.parseErrors = append(m.parseErrors, "Cooldown Period")
		scalingRows = append(scalingRows, ui.TableRow{Label: "Cooldown Period", Value: ui.ErrorStyle.Render("[error parsing Cooldown Period]")})
	} else {
		scalingRows = append(scalingRows, ui.TableRow{Label: "Cooldown Period", Value: fmt.Sprintf("%ds", cooldown)})
	}
	if minReplicas, err := app.GetMinReplicaCount(); err != nil {
		m.parseErrors = append(m.parseErrors, "Minimum Replicas")
		scalingRows = append(scalingRows, ui.TableRow{Label: "Minimum Replicas", Value: ui.ErrorStyle.Render("[error parsing Minimum Replicas]")})
	} else {
		scalingRows = append(scalingRows, ui.TableRow{Label: "Minimum Replicas", Value: fmt.Sprintf("%d", minReplicas)})
	}
	if maxReplicas, err := app.GetMaxReplicaCount(); err != nil {
		m.parseErrors = append(m.parseErrors, "Maximum Replicas")
		scalingRows = append(scalingRows, ui.TableRow{Label: "Maximum Replicas", Value: ui.ErrorStyle.Render("[error parsing Maximum Replicas]")})
	} else {
		scalingRows = append(scalingRows, ui.TableRow{Label: "Maximum Replicas", Value: fmt.Sprintf("%d", maxReplicas)})
	}
	if responsePeriod, err := app.GetResponseGracePeriodSeconds(); err != nil {
		m.parseErrors = append(m.parseErrors, "Response Grace Period")
	} else if responsePeriod > 0 {
		scalingRows = append(scalingRows, ui.TableRow{Label: "Response Grace Period", Value: fmt.Sprintf("%ds", responsePeriod)})
	}
	sections = append(sections, ui.TableSection{Header: "SCALING PARAMETERS", Rows: scalingRows})

	// STATUS Section
	statusRows := []ui.TableRow{
		{Label: "Status", Value: ui.ColorizeStatus(app.Status)},
		{Label: "Last Build Status", Value: ui.ColorizeStatus(app.LastBuildStatus)},
	}
	if app.LatestBuildID != "" {
		statusRows = append(statusRows, ui.TableRow{Label: "Last Build ID", Value: app.LatestBuildID})
	}
	sections = append(sections, ui.TableSection{Header: "STATUS", Rows: statusRows})

	// LIVE PODS Section
	if len(app.Pods) > 0 {
		podRows := []ui.TableRow{}
		for i, pod := range app.Pods {
			podRows = append(podRows, ui.TableRow{Label: fmt.Sprintf("Pod %d", i+1), Value: pod})
		}
		sections = append(sections, ui.TableSection{Header: "LIVE PODS", Rows: podRows})
	}

	// Render table with panel
	tableContent := ui.RenderDetailTable(sections)
	panel := ui.RenderPanel(fmt.Sprintf("App Details for %s", app.ID), tableContent)

	// If there were parse errors, add a helpful message
	if len(m.parseErrors) > 0 {
		var helpMsg strings.Builder
		helpMsg.WriteString("\n")
		helpMsg.WriteString(ui.WarningStyle.Render("⚠ Some fields could not be parsed correctly."))
		helpMsg.WriteString("\n")
		helpMsg.WriteString("This may indicate a CLI version mismatch with the API.\n")
		helpMsg.WriteString("Try updating your CLI or reach out to Cerebrium on Discord for support:\n")
		helpMsg.WriteString(ui.URLStyle.Render("https://discord.gg/ATj6USmeE2"))
		helpMsg.WriteString("\n")
		return panel + helpMsg.String()
	}

	return panel
}

// formatAppDetailsSimple formats app details for non-TTY output
func (m *GetView) formatAppDetailsSimple() string {
	app := m.appDetails
	var output strings.Builder

	header := fmt.Sprintf("App Details: %s", app.ID)
	output.WriteString(header + "\n")
	output.WriteString(strings.Repeat("=", len(header)) + "\n\n")

	output.WriteString("APP\n")
	output.WriteString(fmt.Sprintf("  ID: %s\n", app.ID))
	output.WriteString(fmt.Sprintf("  Created At: %s\n", ui.FormatTimestamp(app.CreatedAt)))
	if !app.UpdatedAt.Equal(app.CreatedAt) {
		output.WriteString(fmt.Sprintf("  Updated At: %s\n", ui.FormatTimestamp(app.UpdatedAt)))
	}
	output.WriteString("\n")

	output.WriteString("HARDWARE\n")
	if app.Hardware != "" {
		output.WriteString(fmt.Sprintf("  Compute: %s\n", app.Hardware))
	} else {
		output.WriteString("  Compute: Data Unavailable\n")
	}
	if cpu, err := app.GetCPU(); err != nil {
		m.parseErrors = append(m.parseErrors, "CPU")
		output.WriteString("  CPU: [error parsing CPU]\n")
	} else if cpu == 0 {
		output.WriteString("  CPU: Data Unavailable\n")
	} else {
		output.WriteString(fmt.Sprintf("  CPU: %d cores\n", cpu))
	}
	if memory, err := app.GetMemory(); err != nil {
		m.parseErrors = append(m.parseErrors, "Memory")
		output.WriteString("  Memory: [error parsing Memory]\n")
	} else if memory == 0 {
		output.WriteString("  Memory: Data Unavailable\n")
	} else {
		output.WriteString(fmt.Sprintf("  Memory: %d GB\n", memory))
	}
	if gpuCount, err := app.GetGPUCount(); err != nil {
		m.parseErrors = append(m.parseErrors, "GPU Count")
		if app.Hardware != "CPU" {
			output.WriteString("  GPU Count: [error parsing GPU Count]\n")
		}
	} else if app.Hardware != "CPU" && gpuCount > 0 {
		output.WriteString(fmt.Sprintf("  GPU Count: %d\n", gpuCount))
	}
	output.WriteString("\n")

	output.WriteString("SCALING PARAMETERS\n")
	if cooldown, err := app.GetCooldownPeriodSeconds(); err != nil {
		m.parseErrors = append(m.parseErrors, "Cooldown Period")
		output.WriteString("  Cooldown Period: [error parsing Cooldown Period]\n")
	} else {
		output.WriteString(fmt.Sprintf("  Cooldown Period: %ds\n", cooldown))
	}
	if minReplicas, err := app.GetMinReplicaCount(); err != nil {
		m.parseErrors = append(m.parseErrors, "Minimum Replicas")
		output.WriteString("  Minimum Replicas: [error parsing Minimum Replicas]\n")
	} else {
		output.WriteString(fmt.Sprintf("  Minimum Replicas: %d\n", minReplicas))
	}
	if maxReplicas, err := app.GetMaxReplicaCount(); err != nil {
		m.parseErrors = append(m.parseErrors, "Maximum Replicas")
		output.WriteString("  Maximum Replicas: [error parsing Maximum Replicas]\n")
	} else {
		output.WriteString(fmt.Sprintf("  Maximum Replicas: %d\n", maxReplicas))
	}
	if responsePeriod, err := app.GetResponseGracePeriodSeconds(); err != nil {
		m.parseErrors = append(m.parseErrors, "Response Grace Period")
	} else if responsePeriod > 0 {
		output.WriteString(fmt.Sprintf("  Response Grace Period: %ds\n", responsePeriod))
	}
	output.WriteString("\n")

	output.WriteString("STATUS\n")
	output.WriteString(fmt.Sprintf("  Status: %s\n", app.Status))
	output.WriteString(fmt.Sprintf("  Last Build Status: %s\n", app.LastBuildStatus))
	if app.LatestBuildID != "" {
		output.WriteString(fmt.Sprintf("  Last Build ID: %s\n", app.LatestBuildID))
	}
	output.WriteString("\n")

	if len(app.Pods) > 0 {
		output.WriteString("LIVE PODS\n")
		for i, pod := range app.Pods {
			output.WriteString(fmt.Sprintf("  Pod %d: %s\n", i+1, pod))
		}
		output.WriteString("\n")
	}

	// If there were parse errors, add a helpful message
	if len(m.parseErrors) > 0 {
		output.WriteString("\n")
		output.WriteString("⚠ Some fields could not be parsed correctly.\n")
		output.WriteString("This may indicate a CLI version mismatch with the API.\n")
		output.WriteString("Try updating your CLI or reach out to Cerebrium on Discord for support:\n")
		output.WriteString("https://discord.gg/ATj6USmeE2\n")
	}

	return output.String()
}

// Messages

type appDetailsLoadedMsg struct {
	appDetails *api.AppDetails
}

// Commands (async operations)

func (m *GetView) fetchAppDetails() tea.Msg {
	appDetails, err := m.conf.Client.GetApp(m.ctx, m.conf.ProjectID, m.conf.AppID)
	if err != nil {
		return ui.NewAPIError(err)
	}
	return appDetailsLoadedMsg{appDetails}
}
