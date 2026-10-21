package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	apimock "github.com/cerebriumai/cerebrium/internal/api/mock"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uitesting "github.com/cerebriumai/cerebrium/internal/ui/testing"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

//go:generate go test -v -run TestLogsView -update

func TestLogsView(t *testing.T) {

	t.Run("initial state - after init", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		// Mock FetchAppLogs to return empty logs
		mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app", mock.Anything).
			Return(&api.AppLogsResponse{Logs: []api.AppLogEntry{}}, nil).
			Maybe()

		model := NewLogsView(ctx, LogsConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "test-app",
			AppName:   "test-app",
			Follow:    false, // Don't follow to avoid endless polling
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LogsView]{
				Name:       "initial_after_init",
				Msg:        nil,
				ViewGolden: "logs_initial",
				ModelAssert: func(t *testing.T, m *LogsView) {
					// After Init(), state should be LogsStatusStreaming
					assert.Equal(t, LogsStatusStreaming, m.state)
					assert.NotNil(t, m.logViewer)
				},
			}).
			Run(t)
	})

	t.Run("keyboard - handles scroll keys", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		// Mock FetchAppLogs
		mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app", mock.Anything).
			Return(&api.AppLogsResponse{Logs: []api.AppLogEntry{}}, nil).
			Maybe()

		model := NewLogsView(ctx, LogsConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "test-app",
			AppName:   "test-app",
			Follow:    false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LogsView]{
				Name:              "scroll_up_k",
				Msg:               tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}},
				SkipViewAssertion: true,
				ModelAssert: func(t *testing.T, m *LogsView) {
					// Key handler should not crash
					assert.Equal(t, LogsStatusStreaming, m.state)
				},
			}).
			Run(t)
	})

	t.Run("keyboard - ctrl+c cancels", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		// Mock FetchAppLogs
		mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app", mock.Anything).
			Return(&api.AppLogsResponse{Logs: []api.AppLogEntry{}}, nil).
			Maybe()

		model := NewLogsView(ctx, LogsConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "test-app",
			AppName:   "test-app",
			Follow:    false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LogsView]{
				Name:              "ctrl_c_cancel",
				Msg:               tea.KeyMsg{Type: tea.KeyCtrlC},
				SkipViewAssertion: true,
				ModelAssert: func(t *testing.T, m *LogsView) {
					assert.Equal(t, LogsStatusComplete, m.state)
					assert.NotNil(t, m.err)
					assert.True(t, m.err.SilentExit)
				},
			}).
			Run(t)
	})

	t.Run("signal cancel - stops streaming", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		// Mock FetchAppLogs
		mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app", mock.Anything).
			Return(&api.AppLogsResponse{Logs: []api.AppLogEntry{}}, nil).
			Maybe()

		model := NewLogsView(ctx, LogsConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "test-app",
			AppName:   "test-app",
			Follow:    false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LogsView]{
				Name:              "signal_cancel",
				Msg:               ui.SignalCancelMsg{},
				SkipViewAssertion: true,
				ModelAssert: func(t *testing.T, m *LogsView) {
					assert.Equal(t, LogsStatusComplete, m.state)
					assert.NotNil(t, m.err)
					assert.True(t, m.err.SilentExit)
				},
			}).
			Run(t)
	})

	t.Run("simple mode - ignores keyboard input", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		// Mock FetchAppLogs
		mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app", mock.Anything).
			Return(&api.AppLogsResponse{Logs: []api.AppLogEntry{}}, nil).
			Maybe()

		model := NewLogsView(ctx, LogsConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: true,
			},
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "test-app",
			AppName:   "test-app",
			Follow:    false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LogsView]{
				Name:              "ignore_keyboard",
				Msg:               tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}},
				SkipViewAssertion: true,
				ModelAssert: func(t *testing.T, m *LogsView) {
					// In simple mode, keyboard is ignored
					assert.Equal(t, LogsStatusStreaming, m.state)
				},
			}).
			Run(t)
	})

	t.Run("simple mode - view returns empty string", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		// Mock FetchAppLogs
		mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app", mock.Anything).
			Return(&api.AppLogsResponse{Logs: []api.AppLogEntry{}}, nil).
			Maybe()

		model := NewLogsView(ctx, LogsConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: true,
			},
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "test-app",
			AppName:   "test-app",
			Follow:    false,
		})

		// Init will be called by harness
		model.Init()

		view := model.View()
		assert.Empty(t, view, "View should return empty string in simple mode")
	})

	t.Run("GetError returns error", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewLogsView(ctx, LogsConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "test-app",
			AppName:   "test-app",
			Follow:    false,
		})

		// No error initially
		assert.Nil(t, model.GetError())

		// Set error
		testErr := ui.NewAPIError(errors.New("test error"))
		model.err = testErr

		assert.Equal(t, testErr, model.GetError())
	})
}

func TestLogsView_Init(t *testing.T) {
	t.Run("initializes viewer and sets state", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		// Mock FetchAppLogs to return empty logs
		mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app", mock.Anything).
			Return(&api.AppLogsResponse{Logs: []api.AppLogEntry{}}, nil).
			Maybe()

		model := NewLogsView(ctx, LogsConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "test-app",
			AppName:   "test-app",
			Follow:    false, // Don't follow to avoid endless polling
			SinceTime: "2024-01-15T10:00:00Z",
		})

		cmd := model.Init()

		// Should create viewer and set state to streaming
		assert.NotNil(t, model.logViewer)
		assert.Equal(t, LogsStatusStreaming, model.state)
		assert.NotNil(t, cmd)
	})

	t.Run("handles nil context", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		// Mock FetchAppLogs to return empty logs
		mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app", mock.Anything).
			Return(&api.AppLogsResponse{Logs: []api.AppLogEntry{}}, nil).
			Maybe()

		model := NewLogsView(context.TODO(), LogsConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "test-app",
			AppName:   "test-app",
			Follow:    false,
		})

		cmd := model.Init()

		// Should handle nil context gracefully
		assert.NotNil(t, model.logViewer)
		assert.NotNil(t, cmd)
	})
}

func TestLogsView_RenderHelpText(t *testing.T) {
	t.Run("follow mode - shows stop streaming hint", func(t *testing.T) {
		model := NewLogsView(t.Context(), LogsConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Client:    apimock.NewMockClient(t),
			ProjectID: "test-project",
			AppID:     "test-app",
			AppName:   "test-app",
			Follow:    true,
		})

		helpText := model.renderHelpText()
		assert.Contains(t, helpText, "ctrl+c: stop streaming")
	})

	t.Run("no follow mode - shows exit hint", func(t *testing.T) {
		model := NewLogsView(t.Context(), LogsConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Client:    apimock.NewMockClient(t),
			ProjectID: "test-project",
			AppID:     "test-app",
			AppName:   "test-app",
			Follow:    false,
		})

		helpText := model.renderHelpText()
		assert.Contains(t, helpText, "ctrl+c: exit")
	})

	t.Run("no logs - no scroll hints", func(t *testing.T) {
		model := NewLogsView(t.Context(), LogsConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Client:    apimock.NewMockClient(t),
			ProjectID: "test-project",
			AppID:     "test-app",
			AppName:   "test-app",
			Follow:    false,
		})

		// Manually setup state without calling Init
		model.state = LogsStatusStreaming

		helpText := model.renderHelpText()
		// With no logs or less than 40 logs, shouldn't show scroll hints
		assert.NotContains(t, helpText, "j/k: scroll")
	})
}

// TestLogsView_noFollowPrintsEveryLog guards the --no-follow path against losing
// logs. The provider signals completion on a different channel from the logs
// themselves, so the two race: if completion is handled first, LogsView quits on
// IsComplete and whatever the provider already produced is never printed.
//
// The race is won or lost per run, so a single iteration passes by luck. Looping
// makes a regression fail reliably instead of flaking in CI.
func TestLogsView_noFollowPrintsEveryLog(t *testing.T) {
	const (
		iterations = 50
		logCount   = 25
	)

	entries := make([]api.AppLogEntry, 0, logCount)
	for i := range logCount {
		entries = append(entries, api.AppLogEntry{
			LogID:     fmt.Sprintf("log-%d", i),
			Timestamp: time.Unix(int64(i), 0).UTC().Format(time.RFC3339),
			LogLine:   fmt.Sprintf("line %d", i),
			Stream:    "stdout",
		})
	}

	for i := range iterations {
		mockClient := apimock.NewMockClient(t)
		mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app", mock.Anything).
			Return(&api.AppLogsResponse{Logs: entries}, nil).
			Once()

		model := NewLogsView(t.Context(), LogsConfig{
			DisplayConfig: ui.DisplayConfig{IsInteractive: false, DisableAnimation: true},
			Client:        mockClient,
			ProjectID:     "test-project",
			AppID:         "test-app",
			AppName:       "test-app",
			Follow:        false,
		})

		stdout := captureStdout(t, func() {
			p := tea.NewProgram(model, tea.WithoutRenderer(), tea.WithInput(nil))
			_, err := p.Run()
			require.NoError(t, err)
		})

		printed := 0
		if trimmed := strings.TrimSpace(stdout); trimmed != "" {
			printed = len(strings.Split(trimmed, "\n"))
		}
		require.Equal(t, logCount, printed,
			"run %d printed %d of %d logs — the fetched batch was dropped on exit", i, printed, logCount)
	}
}

// captureStdout redirects os.Stdout for the duration of fn and returns what was
// written. The log viewer prints to os.Stdout directly in simple output mode.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	require.NoError(t, err)

	original := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = original }()

	// Drain concurrently so fn can't block on a full pipe buffer.
	captured := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		captured <- buf.String()
	}()

	fn()

	require.NoError(t, writer.Close())
	out := <-captured
	require.NoError(t, reader.Close())

	return out
}

// TestLogsView_noFollowPrintsOldestFirst pins the order of --no-follow output.
// A one-shot fetch pages backwards, so the API returns the batch newest-first,
// but logs have to read oldest-first like every other log tool.
func TestLogsView_noFollowPrintsOldestFirst(t *testing.T) {
	const logCount = 5

	// Newest-first, as the backend returns for a backward fetch.
	entries := make([]api.AppLogEntry, 0, logCount)
	for i := logCount - 1; i >= 0; i-- {
		entries = append(entries, api.AppLogEntry{
			LogID:     fmt.Sprintf("log-%d", i),
			Timestamp: time.Unix(int64(i), 0).UTC().Format(time.RFC3339),
			LogLine:   fmt.Sprintf("line %d", i),
			Stream:    "stdout",
		})
	}

	mockClient := apimock.NewMockClient(t)
	mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app", mock.Anything).
		Return(&api.AppLogsResponse{Logs: entries}, nil).
		Once()

	model := NewLogsView(t.Context(), LogsConfig{
		DisplayConfig: ui.DisplayConfig{IsInteractive: false, DisableAnimation: true},
		Client:        mockClient,
		ProjectID:     "test-project",
		AppID:         "test-app",
		AppName:       "test-app",
		Follow:        false,
	})

	stdout := captureStdout(t, func() {
		p := tea.NewProgram(model, tea.WithoutRenderer(), tea.WithInput(nil))
		_, err := p.Run()
		require.NoError(t, err)
	})

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	require.Len(t, lines, logCount)

	for i, line := range lines {
		assert.Contains(t, line, fmt.Sprintf("line %d", i),
			"output line %d is out of order — the batch was printed newest-first", i)
	}
}
