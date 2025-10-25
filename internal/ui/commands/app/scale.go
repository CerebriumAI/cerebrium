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

type ScaleConfig struct {
	ui.DisplayConfig

	Client    api.Client
	ProjectID string
	AppID     string
	Updates   map[string]any
}

// ScaleView is the Bubbletea model for scaling an app
type ScaleView struct {
	ctx context.Context

	// State
	scaling bool
	scaled  bool
	spinner *ui.SpinnerModel
	err     error

	conf ScaleConfig
}

// NewScaleView creates a new app scale view
func NewScaleView(ctx context.Context, conf ScaleConfig) *ScaleView {
	return &ScaleView{
		ctx:     ctx,
		scaling: true,
		spinner: ui.NewSpinner(),
		conf:    conf,
	}
}

// Error returns the error if any occurred during execution
func (m *ScaleView) Error() error {
	return m.err
}

// Init starts the scaling operation
func (m *ScaleView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.scaleApp)
}

// Update handles messages
func (m *ScaleView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.SignalCancelMsg:
		// Handle termination signal
		if m.conf.SimpleOutput() {
			fmt.Fprintf(os.Stderr, "\nCancelled\n")
		}
		return m, tea.Quit

	case appScaledMsg:
		m.scaled = true
		m.scaling = false

		if m.conf.SimpleOutput() {
			fmt.Printf("App scaled successfully.\n")
		}

		return m, tea.Quit

	case *ui.UIError:
		msg.SilentExit = true
		m.err = msg
		m.scaling = false

		if m.conf.SimpleOutput() {
			fmt.Printf("Error: %s\n", msg.Error())
		}

		return m, tea.Quit

	case tea.KeyMsg:
		// Only handle keyboard input in interactive mode
		if !m.conf.SimpleOutput() && msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	default:
		// Update spinner only in interactive mode
		if !m.conf.SimpleOutput() && m.scaling {
			var cmd tea.Cmd
			spinnerModel, cmd := m.spinner.Update(msg)
			m.spinner = spinnerModel.(*ui.SpinnerModel) //nolint:errcheck // Type assertion guaranteed by SpinnerModel structure
			return m, cmd
		}
	}

	return m, nil
}

// View renders the output
func (m *ScaleView) View() string {
	// Simple mode: output has already been printed directly
	if m.conf.SimpleOutput() {
		return ""
	}

	// Interactive mode: full UI experience
	if m.scaling {
		var output strings.Builder
		output.WriteString(fmt.Sprintf("%s Updating scaling for app '%s'...\n\n", m.spinner.View(), m.conf.AppID))
		output.WriteString(m.formatUpdates())
		return output.String()
	}

	if m.err != nil {
		return ui.FormatError(m.err)
	}

	if m.scaled {
		var output strings.Builder
		output.WriteString(ui.SuccessStyle.Render(fmt.Sprintf("✓ App '%s' scaled successfully", m.conf.AppID)))
		output.WriteString("\n\n")
		output.WriteString(m.formatUpdates())
		return output.String()
	}

	return ""
}

// formatUpdates formats the updates being applied
func (m *ScaleView) formatUpdates() string {
	var output strings.Builder

	if cooldown, ok := m.conf.Updates["cooldownPeriodSeconds"].(int); ok {
		output.WriteString(fmt.Sprintf("  • Cooldown period: %d seconds\n", cooldown))
	}
	if minReplicas, ok := m.conf.Updates["minReplicaCount"].(int); ok {
		output.WriteString(fmt.Sprintf("  • Minimum replicas: %d\n", minReplicas))
	}
	if maxReplicas, ok := m.conf.Updates["maxReplicaCount"].(int); ok {
		output.WriteString(fmt.Sprintf("  • Maximum replicas: %d\n", maxReplicas))
	}
	if responseGracePeriod, ok := m.conf.Updates["responseGracePeriodSeconds"].(int); ok {
		output.WriteString(fmt.Sprintf("  • Response grace period: %d seconds\n", responseGracePeriod))
	}

	return output.String()
}

// Messages

type appScaledMsg struct{}

// Commands (async operations)

func (m *ScaleView) scaleApp() tea.Msg {
	err := m.conf.Client.UpdateApp(m.ctx, m.conf.ProjectID, m.conf.AppID, m.conf.Updates)
	if err != nil {
		return ui.NewAPIError(err)
	}
	return appScaledMsg{}
}
