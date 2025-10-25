package logging

import (
	"context"
	"fmt"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// LogViewerConfig contains configuration for the log viewer
type LogViewerConfig struct {
	ui.DisplayConfig

	Provider     LogProvider
	TickInterval time.Duration // UI refresh interval (default: 500ms)
}

// LogViewerModel is a reusable component for viewing logs
type LogViewerModel struct {
	ctx           context.Context
	config        LogViewerConfig
	spinner       *ui.SpinnerModel
	logChan       chan []Log
	doneChan      chan error
	buildStatus   string
	lastLogTime   time.Time
	idleIndex     int
	logs          []Log // Accumulated logs for display
	isComplete    bool  // Provider has finished
	logChanClosed bool  // Log channel has been drained
	err           error
}

const (
	// MaxLogsInMemory is the hard limit for logs stored in memory
	// When exceeded, oldest logs are evicted
	MaxLogsInMemory = 10_000
)

// Idle messages shown during long builds
var idleIntervals = []time.Duration{20 * time.Second, 60 * time.Second, 120 * time.Second, 180 * time.Second}
var idleMessages = []string{
	"Hang in there, still building!",
	"Still building, thanks for your patience!",
	"Almost there, please hold on!",
	"Thank you for waiting, we're nearly done!",
}

// NewLogViewer creates a new log viewer
func NewLogViewer(ctx context.Context, config LogViewerConfig) *LogViewerModel {
	if config.TickInterval == 0 {
		config.TickInterval = 500 * time.Millisecond
	}

	return &LogViewerModel{
		ctx:         ctx,
		config:      config,
		spinner:     ui.NewSpinner(),
		logChan:     make(chan []Log, 10), // Buffered to prevent blocking provider
		doneChan:    make(chan error, 1),
		lastLogTime: time.Now(),
		idleIndex:   0,
	}
}

// Init initializes the log viewer and starts the provider

// Error returns the error if any occurred during execution
func (m *LogViewerModel) Error() error {
	return m.err
}

func (m *LogViewerModel) Init() tea.Cmd {
	// Start provider in background goroutine
	go func() {
		defer close(m.logChan)
		err := m.config.Provider.Collect(m.ctx, func(logs []Log) error {
			// Write logs to channel (non-blocking due to buffer)
			select {
			case m.logChan <- logs:
			case <-m.ctx.Done():
				return m.ctx.Err()
			}
			return nil
		})
		// Provider finished - signal completion
		m.doneChan <- err
	}()

	return tea.Batch(
		m.spinner.Init(),
		waitForLogBatch(m.logChan),
		waitForProviderDone(m.doneChan),
		tick(m.config.TickInterval),
	)
}

// Update handles messages
func (m *LogViewerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case logBatchReceivedMsg:
		// If logs is nil, the channel was closed
		if msg.logs == nil {
			m.logChanClosed = true
			return m, nil
		}

		// Buffer logs silently (don't trigger render for every new log)
		hasNewLogs := len(msg.logs) > 0
		for _, log := range msg.logs {
			m.logs = append(m.logs, log)

			// Direct output in simple mode
			if m.config.SimpleOutput() {
				fmt.Println(formatLogEntry(log))
			}

			// Update build status from metadata if available
			if status, ok := log.Metadata["buildStatus"].(string); ok {
				m.buildStatus = status
			}
		}

		// Enforce memory limit - evict oldest logs if needed
		if len(m.logs) > MaxLogsInMemory {
			numToEvict := len(m.logs) - MaxLogsInMemory
			m.logs = m.logs[numToEvict:]
		}

		// Reset idle tracking if we got new logs
		if hasNewLogs {
			m.lastLogTime = time.Now()
			m.idleIndex = 0
		}

		// Keep listening for more logs
		return m, waitForLogBatch(m.logChan)

	case providerDoneMsg:
		m.isComplete = true
		m.err = msg.err
		return m, nil

	case tickMsg:
		// Tick triggers render and checks idle time
		idleTime := time.Since(m.lastLogTime)
		for i, interval := range idleIntervals {
			if idleTime >= interval && i >= m.idleIndex {
				m.idleIndex = i + 1
				break
			}
		}

		// Only quit if both provider is complete AND log channel has been drained
		// This ensures all logs are processed before quitting
		if m.isComplete && m.logChanClosed {
			return m, tea.Quit
		}

		// Schedule next tick
		return m, tick(m.config.TickInterval)

	case ui.SignalCancelMsg:
		// Handle cancellation signal
		if m.config.SimpleOutput() {
			fmt.Fprintf(os.Stderr, "\nCancelled by user\n")
		}
		m.err = ui.NewUserCancelledError()
		m.isComplete = true
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

	var output strings.Builder

	// Show spinner message
	// Idle index is updated in Update() to keep View() pure
	spinnerText := "Building app..."

	// Determine spinner text based on current idle index
	if m.idleIndex > 0 && m.idleIndex-1 < len(idleMessages) {
		spinnerText = idleMessages[m.idleIndex-1]
	}

	if !m.isComplete {
		output.WriteString(m.spinner.View())
		output.WriteString(" ")
		output.WriteString(spinnerText)
		output.WriteString("\n\n")
	}

	// Show recent logs (last 20)
	startIdx := 0
	if len(m.logs) > 20 {
		startIdx = len(m.logs) - 20
	}

	for i := startIdx; i < len(m.logs); i++ {
		output.WriteString(formatLogEntry(m.logs[i]))
		output.WriteString("\n")
	}

	return output.String()
}

// GetStatus returns the current build status
func (m *LogViewerModel) GetStatus() string {
	return m.buildStatus
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
	return m.isComplete
}

// Messages

type logBatchReceivedMsg struct {
	logs []Log
}

type providerDoneMsg struct {
	err error
}

type tickMsg time.Time

// Commands

func waitForLogBatch(ch <-chan []Log) tea.Cmd {
	return func() tea.Msg {
		logs, ok := <-ch
		if !ok {
			// Channel closed - send message with nil logs to signal this
			return logBatchReceivedMsg{logs: nil}
		}
		return logBatchReceivedMsg{logs: logs}
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

// Helpers

func formatLogEntry(log Log) string {
	timestamp := log.Timestamp.Local().Format("15:04:05.000")
	styledTimestamp := ui.TimestampStyle.Render(timestamp)
	return fmt.Sprintf("%s %s", styledTimestamp, log.Content)
}
