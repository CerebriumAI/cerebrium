package apps

import (
	"errors"
	"testing"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	apimock "github.com/cerebriumai/cerebrium/internal/api/mock"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uitesting "github.com/cerebriumai/cerebrium/internal/ui/testing"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

//go:generate go test -v -run TestAppsListView -update

func TestAppsListView(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	t.Run("success - interactive mode with multiple apps", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		// Apps are pre-sorted by UpdatedAt (most recent first) as they would be from the client
		apps := []api.App{
			{
				ID:        "app-2",
				Status:    "PENDING",
				CreatedAt: baseTime.Add(time.Hour),
				UpdatedAt: baseTime.Add(3 * time.Hour),
			},
			{
				ID:        "app-1",
				Status:    "ACTIVE",
				CreatedAt: baseTime,
				UpdatedAt: baseTime.Add(2 * time.Hour),
			},
			{
				ID:        "app-3",
				Status:    "FAILED",
				CreatedAt: baseTime.Add(30 * time.Minute),
				UpdatedAt: baseTime.Add(time.Hour),
			},
		}

		model := NewListView(ctx, ListConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "initial_loading",
				Msg:        nil,
				ViewGolden: "list_loading",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.True(t, m.loading)
					assert.Empty(t, m.apps)
				},
			}).
			Step(uitesting.TestStep[*ListView]{
				Name:       "apps_loaded",
				Msg:        appsLoadedMsg{apps: apps},
				ViewGolden: "list_success_multiple",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.False(t, m.loading)
					assert.Len(t, m.apps, 3)
					// Verify order is preserved from client (sorted by UpdatedAt, most recent first)
					assert.Equal(t, "app-2", m.apps[0].ID)
					assert.Equal(t, "app-1", m.apps[1].ID)
					assert.Equal(t, "app-3", m.apps[2].ID)
				},
			}).
			Run(t)
	})

	t.Run("success - single app", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		apps := []api.App{
			{
				ID:        "single-app",
				Status:    "ACTIVE",
				CreatedAt: baseTime,
				UpdatedAt: baseTime,
			},
		}

		model := NewListView(ctx, ListConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "single_app",
				Msg:        appsLoadedMsg{apps: apps},
				ViewGolden: "list_success_single",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.Len(t, m.apps, 1)
					assert.Equal(t, "single-app", m.apps[0].ID)
				},
			}).
			Run(t)
	})

	t.Run("success - empty list", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewListView(ctx, ListConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "empty_list",
				Msg:        appsLoadedMsg{apps: []api.App{}},
				ViewGolden: "list_empty",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.Empty(t, m.apps)
				},
			}).
			Run(t)
	})

	t.Run("success - simple mode with apps", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		// Apps are pre-sorted by UpdatedAt (most recent first) as they would be from the client
		apps := []api.App{
			{
				ID:        "simple-app-2",
				Status:    "BUILDING",
				CreatedAt: baseTime,
				UpdatedAt: baseTime.Add(2 * time.Hour),
			},
			{
				ID:        "simple-app-1",
				Status:    "ACTIVE",
				CreatedAt: baseTime,
				UpdatedAt: baseTime.Add(time.Hour),
			},
		}

		model := NewListView(ctx, ListConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: true,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name: "simple_mode_with_apps",
				Msg:  appsLoadedMsg{apps: apps},
				ViewAssert: func(t *testing.T, view string) {
					// In simple mode, View() returns empty string
					assert.Empty(t, view)
				},
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.Len(t, m.apps, 2)
					// Verify order is preserved from client
					assert.Equal(t, "simple-app-2", m.apps[0].ID)
					assert.Equal(t, "simple-app-1", m.apps[1].ID)
				},
			}).
			Run(t)
	})

	t.Run("success - simple mode empty", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewListView(ctx, ListConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: true,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name: "simple_mode_empty",
				Msg:  appsLoadedMsg{apps: []api.App{}},
				ViewAssert: func(t *testing.T, view string) {
					assert.Empty(t, view)
				},
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.Empty(t, m.apps)
				},
			}).
			Run(t)
	})

	t.Run("error - API error", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewListView(ctx, ListConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "api_error",
				Msg:        ui.NewAPIError(errors.New("API connection failed")),
				ViewGolden: "list_error",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.NotNil(t, m.Error())
					assert.Empty(t, m.apps)
				},
			}).
			Run(t)
	})

	t.Run("error - unauthorized", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewListView(ctx, ListConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "unauthorized_error",
				Msg:        ui.NewAPIError(errors.New("unauthorized: invalid token")),
				ViewGolden: "list_unauthorized",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.NotNil(t, m.Error())
					assert.Empty(t, m.apps)
				},
			}).
			Run(t)
	})

	t.Run("signal cancel - interactive mode", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewListView(ctx, ListConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name: "cancel_signal",
				Msg:  ui.SignalCancelMsg{},
				ViewAssert: func(t *testing.T, view string) {
					// Should still render loading status before quit
					uitesting.AssertContains(t, view, "Loading")
				},
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.True(t, m.loading)
				},
			}).
			Run(t)
	})

	t.Run("keyboard - ctrl+c in interactive mode", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewListView(ctx, ListConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		// First let it initialize
		model.Init()

		// Then send ctrl+c
		_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

		// Should quit
		assert.NotNil(t, cmd)
	})

	t.Run("apps with different statuses", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		apps := []api.App{
			{
				ID:        "active-app",
				Status:    "ACTIVE",
				CreatedAt: baseTime,
				UpdatedAt: baseTime.Add(5 * time.Hour),
			},
			{
				ID:        "building-app",
				Status:    "BUILDING",
				CreatedAt: baseTime,
				UpdatedAt: baseTime.Add(4 * time.Hour),
			},
			{
				ID:        "failed-app",
				Status:    "FAILED",
				CreatedAt: baseTime,
				UpdatedAt: baseTime.Add(3 * time.Hour),
			},
			{
				ID:        "pending-app",
				Status:    "PENDING",
				CreatedAt: baseTime,
				UpdatedAt: baseTime.Add(2 * time.Hour),
			},
			{
				ID:        "stopped-app",
				Status:    "STOPPED",
				CreatedAt: baseTime,
				UpdatedAt: baseTime.Add(time.Hour),
			},
		}

		model := NewListView(ctx, ListConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "multiple_statuses",
				Msg:        appsLoadedMsg{apps: apps},
				ViewGolden: "list_multiple_statuses",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.Len(t, m.apps, 5)
					// Verify sorting by UpdatedAt (most recent first)
					assert.Equal(t, "active-app", m.apps[0].ID)
					assert.Equal(t, "building-app", m.apps[1].ID)
					assert.Equal(t, "failed-app", m.apps[2].ID)
					assert.Equal(t, "pending-app", m.apps[3].ID)
					assert.Equal(t, "stopped-app", m.apps[4].ID)
				},
			}).
			Run(t)
	})
}

func Test_formatAppsTable(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	model := ListView{
		apps: []api.App{
			{
				ID:        "format-test-1",
				Status:    "ACTIVE",
				CreatedAt: baseTime,
				UpdatedAt: baseTime.Add(time.Hour),
			},
			{
				ID:        "format-test-2",
				Status:    "BUILDING",
				CreatedAt: baseTime.Add(30 * time.Minute),
				UpdatedAt: baseTime.Add(2 * time.Hour),
			},
		},
	}

	output := model.formatAppsTable()

	// Verify header is present
	assert.Contains(t, output, "ID")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "CREATED")
	assert.Contains(t, output, "UPDATED")

	// Verify apps are present
	assert.Contains(t, output, "format-test-1")
	assert.Contains(t, output, "format-test-2")
	assert.Contains(t, output, "ACTIVE")
	assert.Contains(t, output, "BUILDING")

	// Verify timestamps are formatted
	assert.Contains(t, output, "2024-01-15")
}

func TestAppsListView_View(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	t.Run("view during loading", func(t *testing.T) {
		model := ListView{
			ctx:     t.Context(),
			loading: true,
			spinner: ui.NewSpinner(),
			conf: ListConfig{
				ProjectID: "test-project",
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
			},
		}

		view := model.View()
		assert.Contains(t, view, "Loading apps")
	})

	t.Run("view with apps", func(t *testing.T) {
		model := ListView{
			ctx:     t.Context(),
			loading: false,
			apps: []api.App{
				{
					ID:        "view-test-app",
					Status:    "ACTIVE",
					CreatedAt: baseTime,
					UpdatedAt: baseTime,
				},
			},
			conf: ListConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
				ProjectID: "test-project",
			},
		}

		// Need to create table using table.Row type
		rows := []table.Row{
			{"view-test-app", "ACTIVE", "2024-01-15"},
		}
		model.table = newTable(rows)

		view := model.View()
		assert.Contains(t, view, "Apps")
	})

	t.Run("view with empty list", func(t *testing.T) {
		model := ListView{
			ctx:  t.Context(),
			apps: []api.App{},
			conf: ListConfig{
				ProjectID: "test-project",
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
			},
		}

		view := model.View()
		assert.Contains(t, view, "No apps found")
	})

	t.Run("view with error", func(t *testing.T) {
		model := ListView{
			ctx: t.Context(),
			err: ui.NewAPIError(errors.New("test error")),
			conf: ListConfig{
				ProjectID: "test-project",
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
			},
		}

		view := model.View()
		assert.Contains(t, view, "test error")
	})

	t.Run("view in simple mode", func(t *testing.T) {
		model := ListView{
			ctx:     t.Context(),
			loading: true,
			conf: ListConfig{
				ProjectID: "test-project",
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    false,
					DisableAnimation: true,
				},
			},
		}

		view := model.View()
		assert.Empty(t, view, "View should return empty string in simple mode")
	})
}
