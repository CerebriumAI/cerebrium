package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/internal/ui/logging"
	tea "github.com/charmbracelet/bubbletea"
)

// LogsStatus represents the current state of the logs command
type LogsStatus int

const (
	LogsStatusLoading LogsStatus = iota
	LogsStatusStreaming
	LogsStatusComplete
	LogsStatusError
)

// LogsConfig contains configuration for the logs command
type LogsConfig struct {
	ui.DisplayConfig

	Client    api.Client
	ProjectID string
	AppID     string // Full app ID (may include project prefix)
	AppName   string // Display name (original input)
	Follow    bool   // If false, fetch once and exit
	SinceTime string // ISO timestamp to start from
}

// LogsView is the Bubbletea model for the logs command
type LogsView struct {
	ctx       context.Context
	state     LogsStatus
	logViewer *logging.LogViewerModel
	err       *ui.UIError

	conf LogsConfig
}

// NewLogsView creates a new logs view
func NewLogsView(ctx context.Context, conf LogsConfig) *LogsView {
	return &LogsView{
		ctx:   ctx,
		state: LogsStatusLoading,
		conf:  conf,
	}
}

// Init initializes the logs view

// Error returns the error if any occurred during execution
func (m *LogsView) Error() error {
	return m.err
}

func (m *LogsView) Init() tea.Cmd {
	// Create log provider using the app ID (already determined in command)
	provider := logging.NewPollingAppLogProvider(logging.PollingAppLogProviderConfig{
		Client:    m.conf.Client,
		ProjectID: m.conf.ProjectID,
		AppID:     m.conf.AppID,
		Follow:    m.conf.Follow,
		SinceTime: m.conf.SinceTime,
	})

	// Create log viewer with the provider
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	viewer := logging.NewLogViewer(ctx, logging.LogViewerConfig{
		DisplayConfig: m.conf.DisplayConfig,
		Provider:      provider,
		ShowHelp:      m.conf.Follow,
		ViewSize:      40,
	})
	m.logViewer = viewer
	m.state = LogsStatusStreaming

	return m.logViewer.Init()
}

// Update handles messages
func (m *LogsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.SignalCancelMsg:
		// Handle cancellation signal (Ctrl+C)
		if m.conf.SimpleOutput() {
			fmt.Fprintf(os.Stderr, "\nStopped watching logs\n")
		}
		_, _ = m.logViewer.Update(msg)
		m.err = ui.NewUserCancelledError()
		m.state = LogsStatusComplete
		return m, tea.Quit

	case tea.KeyMsg:
		return m.onKey(msg)

	default:
		// Delegate to log viewer
		if m.logViewer != nil {
			updated, cmd := m.logViewer.Update(msg)
			m.logViewer = updated.(*logging.LogViewerModel) //nolint:errcheck // Type assertion guaranteed by LogViewerModel structure

			// Check if log viewer is complete AND all logs have been processed
			// We need both conditions to avoid race condition where we quit before
			// all logs in the channel buffer have been displayed
			if m.logViewer.IsComplete() {
				m.state = LogsStatusComplete
				if err := m.logViewer.GetError(); err != nil {
					m.err = ui.NewAPIError(err)
				}
				return m, tea.Quit
			}

			return m, cmd
		}
	}

	return m, nil
}

// View renders the logs view
func (m *LogsView) View() string {
	// Simple mode: output already printed directly by log viewer, return empty
	if m.conf.SimpleOutput() {
		return ""
	}

	var output strings.Builder

	if m.logViewer != nil {
		output.WriteString(m.logViewer.View())
	}

	// Interactive mode: render based on state
	switch m.state {
	case LogsStatusLoading:
		return fmt.Sprintf("Loading logs...") // TODO: Use spinner

	case LogsStatusStreaming:
		// Fall through, we just want to display the log viewer
		break

	case LogsStatusError:
		if m.err != nil {
			output.WriteString("\n")
			output.WriteString(ui.FormatError(m.err))
			output.WriteString("\n")
		} else {
			output.WriteString("\n")
			output.WriteString(ui.FormatError(errors.New("An unknown error occurred")))
			output.WriteString("\n")
		}

	case LogsStatusComplete:
		if m.err != nil && !m.err.SilentExit {
			output.WriteString("\n")
			output.WriteString(ui.FormatError(m.err))
			output.WriteString("\n")
		}
	}

	output.WriteString(m.renderHelpText())

	return output.String()
}

// renderHelpText shows keyboard shortcuts
func (m *LogsView) renderHelpText() string {
	var hints []string

	if m.conf.Follow {
		hints = append(hints, "ctrl+c: stop streaming")
	} else {
		hints = append(hints, "ctrl+c: exit")
	}

	if len(hints) == 0 {
		return ""
	}

	helpText := strings.Join(hints, " | ")
	return ui.HelpStyle.Render(helpText)
}

// GetError returns any error that occurred
func (m *LogsView) GetError() *ui.UIError {
	return m.err
}

func (m *LogsView) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == tea.KeyCtrlC.String() {
		// No cleanup needed, just exit silently
		m.err = ui.NewUserCancelledError()
		m.state = LogsStatusComplete
		return m, tea.Quit
	}

	// Only handle keyboard in interactive mode
	if m.conf.SimpleOutput() {
		return m, nil
	}

	// Otherwise hand off to the log viewer and update it
	updatedViewer, cmd := m.logViewer.Update(msg)
	m.logViewer = updatedViewer.(*logging.LogViewerModel)
	return m, cmd
}
