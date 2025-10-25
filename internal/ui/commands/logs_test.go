package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/cerebriumai/cerebrium/internal/api"
	apimock "github.com/cerebriumai/cerebrium/internal/api/mock"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uitesting "github.com/cerebriumai/cerebrium/internal/ui/testing"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
					assert.True(t, m.anchorBottom)
					assert.NotNil(t, m.viewer)
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

	t.Run("anchor bottom - starts anchored", func(t *testing.T) {
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
			Follow:    true,
		})

		// Should start anchored to bottom for latest logs
		assert.True(t, model.anchorBottom)
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
		assert.NotNil(t, model.viewer)
		assert.Equal(t, LogsStatusStreaming, model.state)
		assert.NotNil(t, cmd)
	})

	t.Run("handles nil context", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		// Mock FetchAppLogs to return empty logs
		mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app", mock.Anything).
			Return(&api.AppLogsResponse{Logs: []api.AppLogEntry{}}, nil).
			Maybe()

		model := NewLogsView(nil, LogsConfig{
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
		assert.NotNil(t, model.viewer)
		assert.NotNil(t, cmd)
	})
}

func TestLogsView_RenderHelpText(t *testing.T) {
	ctx := context.Background()
	mockClient := apimock.NewMockClient(t)

	t.Run("follow mode - shows stop streaming hint", func(t *testing.T) {
		model := NewLogsView(ctx, LogsConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "test-app",
			AppName:   "test-app",
			Follow:    true,
		})

		helpText := model.renderHelpText()
		assert.Contains(t, helpText, "ctrl+c: stop streaming")
	})

	t.Run("no follow mode - shows exit hint", func(t *testing.T) {
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

		helpText := model.renderHelpText()
		assert.Contains(t, helpText, "ctrl+c: exit")
	})

	t.Run("no logs - no scroll hints", func(t *testing.T) {
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

		// Manually setup state without calling Init
		model.state = LogsStatusStreaming

		helpText := model.renderHelpText()
		// With no logs or less than 40 logs, shouldn't show scroll hints
		assert.NotContains(t, helpText, "j/k: scroll")
	})
}
