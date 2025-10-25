package commands

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/internal/ui/logging"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	ctx             context.Context
	state           LogsStatus
	viewer          *logging.LogViewerModel
	err             *ui.UIError
	logScrollOffset int  // Current scroll position
	anchorBottom    bool // Auto-scroll to show latest logs

	conf LogsConfig
}

// NewLogsView creates a new logs view
func NewLogsView(ctx context.Context, conf LogsConfig) *LogsView {
	return &LogsView{
		ctx:          ctx,
		state:        LogsStatusLoading,
		anchorBottom: true, // Start anchored to bottom for latest logs
		conf:         conf,
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
	})
	m.viewer = viewer
	m.state = LogsStatusStreaming

	return m.viewer.Init()
}

// Update handles messages
func (m *LogsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.SignalCancelMsg:
		// Handle cancellation signal (Ctrl+C)
		if m.conf.SimpleOutput() {
			fmt.Fprintf(os.Stderr, "\nStopped watching logs\n")
		}
		m.err = ui.NewUserCancelledError()
		m.state = LogsStatusComplete
		return m, tea.Quit

	case tea.KeyMsg:
		// Only handle keyboard in interactive mode
		if m.conf.SimpleOutput() {
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			if m.conf.SimpleOutput() {
				fmt.Fprintf(os.Stderr, "\nStopped watching logs\n")
			}
			m.err = ui.NewUserCancelledError()
			m.state = LogsStatusComplete
			return m, tea.Quit

		case "j":
			// Scroll down one line
			if m.viewer != nil {
				totalLogs := len(m.viewer.GetLogs())
				maxVisible := 40
				maxOffset := max(0, totalLogs-maxVisible)
				m.logScrollOffset = min(maxOffset, m.logScrollOffset+1)
				// Always check if last log is visible
				m.anchorBottom = (m.logScrollOffset+maxVisible >= totalLogs)
			}

		case "J":
			// Scroll to bottom (Shift+J)
			if m.viewer != nil {
				totalLogs := len(m.viewer.GetLogs())
				maxVisible := 40
				m.logScrollOffset = max(0, totalLogs-maxVisible)
				// Always check if last log is visible
				m.anchorBottom = (m.logScrollOffset+maxVisible >= totalLogs)
			}

		case "k":
			// Scroll up one line
			m.logScrollOffset = max(0, m.logScrollOffset-1)
			// Always check if last log is visible
			if m.viewer != nil {
				totalLogs := len(m.viewer.GetLogs())
				maxVisible := 40
				m.anchorBottom = (m.logScrollOffset+maxVisible >= totalLogs)
			}

		case "K":
			// Scroll to top (Shift+K)
			m.logScrollOffset = 0
			// Always check if last log is visible (it won't be if we're at top)
			if m.viewer != nil {
				totalLogs := len(m.viewer.GetLogs())
				maxVisible := 40
				m.anchorBottom = (m.logScrollOffset+maxVisible >= totalLogs)
			}

		case "ctrl+u":
			// Page up - scroll up 10 lines
			m.logScrollOffset = max(0, m.logScrollOffset-10)
			// Always check if last log is visible
			if m.viewer != nil {
				totalLogs := len(m.viewer.GetLogs())
				maxVisible := 40
				m.anchorBottom = (m.logScrollOffset+maxVisible >= totalLogs)
			}

		case "ctrl+d":
			// Page down - scroll down 10 lines
			if m.viewer != nil {
				totalLogs := len(m.viewer.GetLogs())
				maxVisible := 40
				maxOffset := max(0, totalLogs-maxVisible)
				m.logScrollOffset = min(maxOffset, m.logScrollOffset+10)
				// Always check if last log is visible
				m.anchorBottom = (m.logScrollOffset+maxVisible >= totalLogs)
			}
		}

	default:
		// Delegate to log viewer
		if m.viewer != nil {
			updated, cmd := m.viewer.Update(msg)
			m.viewer = updated.(*logging.LogViewerModel) //nolint:errcheck // Type assertion guaranteed by LogViewerModel structure

			totalLogs := len(m.viewer.GetLogs())
			maxVisible := 40

			// Check if the last log is currently visible
			lastLogVisible := m.logScrollOffset+maxVisible >= totalLogs

			// If last log is visible OR we're anchored to bottom, keep scrolling to show new logs
			if lastLogVisible || m.anchorBottom {
				maxOffset := max(0, totalLogs-maxVisible)
				m.logScrollOffset = maxOffset
				m.anchorBottom = true // Keep anchored if last log is visible
			}

			// Check if log viewer is complete AND all logs have been processed
			// We need both conditions to avoid race condition where we quit before
			// all logs in the channel buffer have been displayed
			if m.viewer.IsComplete() {
				m.state = LogsStatusComplete
				if err := m.viewer.GetError(); err != nil {
					m.err = ui.NewAPIError(err)
					return m, tea.Quit
				}

				// In simple mode (--no-color or piped), quit when complete
				// But the LogViewerModel will handle draining all logs first
				if m.conf.SimpleOutput() {
					// Don't quit yet - let the viewer's tickMsg handle it
					// This ensures all buffered logs are printed
					return m, cmd
				}

				// In interactive mode with no-follow, stay on final view
				// This allows user to review logs and scroll
				if !m.conf.Follow {
					return m, nil // Stay on final view
				}

				// Follow mode completed (shouldn't normally reach here)
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

	// Interactive mode: render based on state
	switch m.state {
	case LogsStatusLoading:
		return fmt.Sprintf("%s Loading logs...", m.viewer.View())

	case LogsStatusStreaming:
		if m.viewer != nil {
			return m.renderLogViewer()
		}
		return "Streaming logs..."

	case LogsStatusError:
		if m.err != nil {
			return ui.FormatError(m.err)
		}
		return "An error occurred"

	case LogsStatusComplete:
		if m.err != nil && !m.err.SilentExit {
			return ui.FormatError(m.err)
		}
		// Show final logs if we have any
		if m.viewer != nil && len(m.viewer.GetLogs()) > 0 {
			return m.renderLogViewer()
		}
		return "" // Silent exit or normal completion
	}

	return ""
}

// renderLogViewer renders logs in a bordered box with title
func (m *LogsView) renderLogViewer() string {
	if m.viewer == nil {
		return ""
	}

	allLogs := m.viewer.GetLogs()
	totalLogs := len(allLogs)

	if totalLogs == 0 {
		// Show waiting message
		emptyContent := ui.PendingStyle.Render("Waiting for logs...")
		emptyBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("14")).
			Width(100).
			Height(3).
			Padding(0, 1).
			Render(emptyContent)
		return "\n" + emptyBox + "\n" + m.renderHelpText()
	}

	// Show 40 logs (increased from 20)
	maxVisible := 40
	start := m.logScrollOffset
	end := min(start+maxVisible, totalLogs)
	visibleLogs := allLogs[start:end]

	// Build log content with scroll indicators
	var content strings.Builder

	// Top indicator
	if start > 0 {
		content.WriteString(ui.PendingStyle.Render(fmt.Sprintf("↑ %d more lines above", start)))
		content.WriteString("\n")
	}

	// Log lines - format each log entry with milliseconds
	for i, log := range visibleLogs {
		timestamp := log.Timestamp.Local().Format("15:04:05.000")
		styledTimestamp := ui.TimestampStyle.Render(timestamp)
		content.WriteString(fmt.Sprintf("%s %s", styledTimestamp, log.Content))
		if i < len(visibleLogs)-1 {
			content.WriteString("\n")
		}
	}

	// Bottom indicator
	if end < totalLogs {
		content.WriteString("\n")
		content.WriteString(ui.PendingStyle.Render(fmt.Sprintf("↓ %d more lines below", totalLogs-end)))
	}

	// Dynamic height based on content (max 40 lines + indicators)
	height := min(len(visibleLogs)+2, 42) // +2 for padding/indicators
	if start > 0 {
		height++ // Extra line for top indicator
	}
	if end < totalLogs {
		height++ // Extra line for bottom indicator
	}

	// Render with border and title
	title := ui.CyanStyle.Render(fmt.Sprintf("App Logs (%d lines) - %s", totalLogs, m.conf.AppName))
	boxContent := title + "\n" + content.String()

	logBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("14")).
		Width(100).
		Height(height).
		Padding(0, 1).
		Render(boxContent)

	return "\n" + logBox + "\n" + m.renderHelpText()
}

// renderHelpText shows keyboard shortcuts
func (m *LogsView) renderHelpText() string {
	var hints []string

	if m.conf.Follow {
		hints = append(hints, "ctrl+c: stop streaming")
	} else {
		hints = append(hints, "ctrl+c: exit")
	}

	if m.viewer != nil && len(m.viewer.GetLogs()) > 40 {
		hints = append(hints, "j/k: scroll", "J/K: scroll to bottom/top", "ctrl+u/d: page up/down")
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
