package logging

import (
	"context"
	"fmt"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/charmbracelet/lipgloss"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type viewerState int

const (
	viewerStateInitialising viewerState = iota
	viewerStateRunning
	viewerStateFinished
)

// LogViewerConfig contains configuration for the log viewer
type LogViewerConfig struct {
	ui.DisplayConfig

	Provider     LogProvider
	TickInterval time.Duration // UI refresh interval (default: 500ms)
	ShowHelp     bool
	ViewSize     int
}

// LogViewerModel is a reusable component for viewing logs
type LogViewerModel struct {
	ctx    context.Context
	config LogViewerConfig

	state    viewerState
	spinner  *ui.SpinnerModel
	logChan  chan []Log
	doneChan chan error
	logs     []Log // Accumulated logs for display

	scrollOffset int
	anchorBottom bool // Auto-scroll to show latest logs

	err error
}

const (
	// maxLogsInMemory is the hard limit for logs stored in memory
	// When exceeded, oldest logs are evicted
	maxLogsInMemory = 10_000
)

// NewLogViewer creates a new log viewer
func NewLogViewer(ctx context.Context, config LogViewerConfig) *LogViewerModel {
	if config.TickInterval == 0 {
		config.TickInterval = 500 * time.Millisecond
	}
	if config.ViewSize == 0 {
		config.ViewSize = ui.MAX_LOGS_IN_VIEWER
	}
	// TODO: Allow 'infinite' size for --no-follow

	return &LogViewerModel{
		ctx:          ctx,
		config:       config,
		state:        viewerStateInitialising,
		spinner:      ui.NewSpinner(),
		logChan:      make(chan []Log, 10), // Buffered to prevent blocking provider
		doneChan:     make(chan error),
		anchorBottom: true, // Auto-scroll to bottom by default
	}
}

// Init initializes the log viewer and starts the provider
func (m *LogViewerModel) Init() tea.Cmd {
	// Start provider in background goroutine
	go func() {
		defer close(m.logChan)

		err := m.config.Provider.Collect(m.ctx, func(logs []Log) error {
			if m.ctx.Err() != nil {
				return m.ctx.Err()
			}

			// Write new logs to channel (non-blocking due to buffer)
			m.logChan <- logs
			return nil
		})
		// Provider closed - signal completion
		m.doneChan <- err
	}()

	m.state = viewerStateRunning

	return tea.Batch(
		m.spinner.Init(),
		waitForLogBatch(m.logChan),
		waitForProviderDone(m.doneChan),
		tick(m.config.TickInterval),
	)
}

func (m *LogViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case logBatchReceivedMsg:
		// Buffer logs silently (don't trigger render for every new log)
		for _, log := range msg.logs {
			m.logs = append(m.logs, log)

			// Direct output in simple mode
			if m.config.SimpleOutput() {
				fmt.Println(formatLogEntry(log))
			}
		}

		// Enforce memory limit - evict oldest logs if needed
		if len(m.logs) > maxLogsInMemory {
			numToEvict := len(m.logs) - maxLogsInMemory
			m.logs = m.logs[numToEvict:]
			m.scrollOffset -= numToEvict
		}

		if m.anchorBottom {
			m.scrollOffset = max(0, len(m.logs)-m.config.ViewSize)
		}

		sort.Slice(m.logs, func(i, j int) bool {
			return m.logs[i].Timestamp.Before(m.logs[j].Timestamp)
		})

		// Keep listening for more logs
		return m, waitForLogBatch(m.logChan)

	case providerDoneMsg:
		m.state = viewerStateFinished
		m.err = msg.err
		return m, nil

	case tickMsg:
		// Don't schedule another tick if we're done
		if m.state == viewerStateFinished && len(m.logChan) == 0 {
			return m, nil
		}

		// Schedule next tick
		return m, tick(m.config.TickInterval)

	case tea.KeyMsg:
		return m.onKey(msg)

	case ui.SignalCancelMsg:
		m.state = viewerStateFinished
		return m, tea.Quit

	default:
		// Update spinner only in interactive mode
		if !m.config.SimpleOutput() {
			updatedSpinner, cmd := m.spinner.Update(msg)
			m.spinner = updatedSpinner.(*ui.SpinnerModel) //nolint:errcheck // Type assertion guaranteed by SpinnerModel structure
			return m, cmd
		}
	}

	return m, nil
}

// View renders the log viewer
func (m *LogViewerModel) View() string {
	// Simple mode: output already printed directly, return empty
	if m.config.SimpleOutput() {
		return ""
	}
	// Show waiting message
	if len(m.logs) == 0 || m.state == viewerStateInitialising {
		emptyContent := ui.PendingStyle.Render("Waiting for logs...")
		emptyBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("14")).
			Width(100).
			Height(3).
			Padding(0, 1).
			Render(emptyContent)
		return "\n" + emptyBox + "\n"
	}

	var content strings.Builder

	// Note: Auto-scroll offset is calculated in Update() to keep View() pure
	var start, end int
	if m.anchorBottom {
		start = max(0, len(m.logs)-m.config.ViewSize)
		end = len(m.logs)
	} else {
		start = m.scrollOffset
		end = min(len(m.logs), start+m.config.ViewSize)
	}

	visibleLogs := m.logs[start:end]

	// Top indicator
	if start > 0 {
		content.WriteString(ui.PendingStyle.Render(fmt.Sprintf("↑ %d more lines above", start)))
		content.WriteString("\n")
	}

	// Log lines - format each log entry
	for i, log := range visibleLogs {
		timestamp := log.Timestamp.Local().Format("15:04:05")
		styledTimestamp := ui.TimestampStyle.Render(timestamp)
		content.WriteString(fmt.Sprintf("%s %s", styledTimestamp, log.Content))
		if i < len(visibleLogs)-1 {
			content.WriteString("\n")
		}
	}

	// Bottom indicator
	if end < len(m.logs) {
		content.WriteString("\n")
		content.WriteString(ui.PendingStyle.Render(fmt.Sprintf("↓ %d more lines below", len(m.logs)-end)))
	}

	// Dynamic height based on content (max 20 lines + indicators)
	height := min(len(visibleLogs)+2, m.config.ViewSize+2) // +2 for padding/indicators
	if start > 0 {
		height++ // Extra line for top indicator
	}
	if end < len(m.logs) {
		height++ // Extra line for bottom indicator
	}

	// Render with border and title
	title := ui.CyanStyle.Render(fmt.Sprintf("Logs (%d lines) offset %d", len(m.logs), m.scrollOffset))
	boxContent := title + "\n" + content.String()

	var output strings.Builder

	logBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("14")).
		Width(100).
		Height(height).
		Padding(0, 1).
		Render(boxContent)
	output.WriteString("\n")
	output.WriteString(logBox)
	output.WriteString("\n")

	if len(m.logs) > m.config.ViewSize {
		output.WriteString(ui.HelpStyle.Render(" j/k: scroll | J/K: scroll to bottom/top | ctrl+u/d: page up/down"))
		output.WriteString("\n")
	}

	return output.String()
}

func (m *LogViewerModel) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		m.state = viewerStateFinished
		return m, tea.Quit

	case "j":
		// Scroll down one line (only when expanded and logs exist)
		maxOffset := max(0, len(m.logs)-m.config.ViewSize)
		m.scrollOffset = min(maxOffset, m.scrollOffset+1)
		// Always check if last log is visible
		m.anchorBottom = m.scrollOffset+m.config.ViewSize >= len(m.logs)

	case "J":
		// Scroll to bottom (Shift+J)
		m.scrollOffset = max(0, len(m.logs)-m.config.ViewSize)
		// Always check if last log is visible
		m.anchorBottom = m.scrollOffset+m.config.ViewSize >= len(m.logs)

	case "k":
		// Scroll up one line (only when expanded)
		m.scrollOffset = max(0, m.scrollOffset-1)
		// Always check if last log is visible
		m.anchorBottom = m.scrollOffset+m.config.ViewSize >= len(m.logs)

	case "K":
		// Scroll to top (Shift+K)
		m.scrollOffset = 0
		// Always check if last log is visible
		m.anchorBottom = m.scrollOffset+m.config.ViewSize >= len(m.logs)

	case "ctrl+u":
		// Page up - scroll up 10 lines
		m.scrollOffset = max(0, m.scrollOffset-10)
		// Always check if last log is visible
		m.anchorBottom = m.scrollOffset+m.config.ViewSize >= len(m.logs)

	case "ctrl+d":
		// Page down - scroll down 10 lines
		maxOffset := max(0, len(m.logs)-m.config.ViewSize)
		m.scrollOffset = min(maxOffset, m.scrollOffset+10)
		// Always check if last log is visible
		m.anchorBottom = m.scrollOffset+m.config.ViewSize >= len(m.logs)
	}

	return m, nil
}

// Error returns the error if any occurred during execution
func (m *LogViewerModel) Error() error {
	return m.err
}

// GetLogs returns all accumulated logs
func (m *LogViewerModel) GetLogs() []Log {
	return m.logs
}

// GetError returns any error that occurred
func (m *LogViewerModel) GetError() error {
	return m.err
}

// IsComplete returns true if log collection has finished
func (m *LogViewerModel) IsComplete() bool {
	return m.state == viewerStateFinished
}

// Messages

type logBatchReceivedMsg struct {
	logs []Log
}

// providerDoneMsg signals that the log provider has finished collecting logs
// This is exported so tests can reference it when using Finally() in test harness
type providerDoneMsg struct {
	finalStatus string
	err         error
}

type tickMsg time.Time

// Commands

func waitForLogBatch(ch <-chan []Log) tea.Cmd {
	return func() tea.Msg {
		return logBatchReceivedMsg{logs: <-ch}
	}
}

func waitForProviderDone(ch <-chan error) tea.Cmd {
	return func() tea.Msg {
		return providerDoneMsg{err: <-ch}
	}
}

func tick(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
