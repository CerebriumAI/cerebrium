package runs

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	apimock "github.com/cerebriumai/cerebrium/internal/api/mock"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uitesting "github.com/cerebriumai/cerebrium/internal/ui/testing"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

//go:generate go test -v -run TestRunsListView -update

func TestRunsListView(t *testing.T) {
	t.Run("initial_state", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)
		model := NewListView(t.Context(), ListConfig{
			DisplayConfig: ui.DisplayConfig{IsInteractive: true},
			Client:        mockClient,
			ProjectID:     "test-project",
			AppName:       "test-app",
			AsyncOnly:     false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "initial",
				ViewGolden: "runs_list_initial",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.True(t, m.loading)
					assert.Nil(t, m.Error())
					assert.Empty(t, m.runs)
				},
			}).
			Run(t)
	})

	t.Run("loads_runs_successfully", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		// Sample runs data
		runs := []api.Run{
			{
				ID:           "run-1",
				FunctionName: "predict",
				Status:       "success",
				CreatedAt:    time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
				Async:        false,
			},
			{
				ID:           "run-2",
				FunctionName: "train",
				Status:       "running",
				CreatedAt:    time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC),
				Async:        true,
			},
		}

		model := NewListView(t.Context(), ListConfig{
			DisplayConfig: ui.DisplayConfig{IsInteractive: true},
			Client:        mockClient,
			ProjectID:     "test-project",
			AppName:       "test-app",
			AsyncOnly:     false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "fetch_runs",
				Msg:        runsLoadedMsg{runs: runs},
				ViewGolden: "runs_list_loaded",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.False(t, m.loading)
					assert.Nil(t, m.Error())
					assert.Len(t, m.runs, 2)
					// Check runs are sorted by date (most recent first)
					assert.Equal(t, "run-2", m.runs[0].ID)
					assert.Equal(t, "run-1", m.runs[1].ID)
				},
			}).
			Run(t)
	})

	t.Run("handles_api_error", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		apiError := errors.New("API request failed")

		model := NewListView(t.Context(), ListConfig{
			DisplayConfig: ui.DisplayConfig{IsInteractive: true},
			Client:        mockClient,
			ProjectID:     "test-project",
			AppName:       "test-app",
			AsyncOnly:     false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "api_error",
				Msg:        ui.NewAPIError(apiError),
				ViewGolden: "runs_list_error",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.False(t, m.loading)
					assert.NotNil(t, m.Error())
					assert.Contains(t, m.Error().Error(), "API request failed")
				},
			}).
			Run(t)
	})

	t.Run("handles_empty_runs_list", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		model := NewListView(t.Context(), ListConfig{
			DisplayConfig: ui.DisplayConfig{IsInteractive: true},
			Client:        mockClient,
			ProjectID:     "test-project",
			AppName:       "test-app",
			AsyncOnly:     false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "empty_runs",
				Msg:        runsLoadedMsg{runs: []api.Run{}},
				ViewGolden: "runs_list_empty",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.False(t, m.loading)
					assert.Nil(t, m.Error())
					assert.Empty(t, m.runs)
				},
			}).
			Run(t)
	})

	t.Run("handles_ctrl_c", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		model := NewListView(t.Context(), ListConfig{
			DisplayConfig: ui.DisplayConfig{IsInteractive: true},
			Client:        mockClient,
			ProjectID:     "test-project",
			AppName:       "test-app",
			AsyncOnly:     false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Finally(uitesting.TestStep[*ListView]{
				Name: "ctrl_c",
				Msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}, Alt: false},
				ModelAssert: func(t *testing.T, m *ListView) {
					// Model should still be loading as Ctrl+C doesn't cancel in list view
					assert.True(t, m.loading)
				},
			}).
			Run(t)
	})

	t.Run("async_only_filter", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		// Only async runs
		asyncRuns := []api.Run{
			{
				ID:           "async-run-1",
				FunctionName: "async_task",
				Status:       "success",
				CreatedAt:    time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
				Async:        true,
			},
		}

		model := NewListView(t.Context(), ListConfig{
			DisplayConfig: ui.DisplayConfig{IsInteractive: true},
			Client:        mockClient,
			ProjectID:     "test-project",
			AppName:       "test-app",
			AsyncOnly:     true,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "async_runs",
				Msg:        runsLoadedMsg{runs: asyncRuns},
				ViewGolden: "runs_list_async_only",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.False(t, m.loading)
					assert.Nil(t, m.Error())
					assert.Len(t, m.runs, 1)
					assert.True(t, m.runs[0].Async)
				},
			}).
			Run(t)
	})

	t.Run("displays_various_statuses_with_colors", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		// Runs with different statuses to test color coding
		runs := []api.Run{
			{
				ID:           "run-success",
				FunctionName: "predict",
				Status:       "success",
				CreatedAt:    time.Date(2025, 1, 5, 12, 0, 0, 0, time.UTC),
				Async:        false,
			},
			{
				ID:           "run-failure",
				FunctionName: "train",
				Status:       "failure",
				CreatedAt:    time.Date(2025, 1, 4, 12, 0, 0, 0, time.UTC),
				Async:        true,
			},
			{
				ID:           "run-running",
				FunctionName: "inference",
				Status:       "running",
				CreatedAt:    time.Date(2025, 1, 3, 12, 0, 0, 0, time.UTC),
				Async:        false,
			},
			{
				ID:           "run-pending",
				FunctionName: "process",
				Status:       "pending",
				CreatedAt:    time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC),
				Async:        true,
			},
			{
				ID:           "run-cancelled",
				FunctionName: "cleanup",
				Status:       "cancelled",
				CreatedAt:    time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
				Async:        false,
			},
		}

		model := NewListView(t.Context(), ListConfig{
			DisplayConfig: ui.DisplayConfig{IsInteractive: true},
			Client:        mockClient,
			ProjectID:     "test-project",
			AppName:       "my-app",
			AsyncOnly:     false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "various_statuses",
				Msg:        runsLoadedMsg{runs: runs},
				ViewGolden: "runs_list_various_statuses",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.False(t, m.loading)
					assert.Nil(t, m.Error())
					assert.Len(t, m.runs, 5)
					// Verify sorting by created date (most recent first)
					assert.Equal(t, "run-success", m.runs[0].ID)
					assert.Equal(t, "run-cancelled", m.runs[4].ID)
				},
			}).
			Run(t)
	})

	t.Run("displays_long_function_names", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		// Runs with long function names to test table formatting
		runs := []api.Run{
			{
				ID:           "run-with-very-long-id-12345678",
				FunctionName: "very_long_function_name_that_might_wrap",
				Status:       "success",
				CreatedAt:    time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
				Async:        true,
			},
			{
				ID:           "short-id",
				FunctionName: "fn",
				Status:       "running",
				CreatedAt:    time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC),
				Async:        false,
			},
		}

		model := NewListView(t.Context(), ListConfig{
			DisplayConfig: ui.DisplayConfig{IsInteractive: true},
			Client:        mockClient,
			ProjectID:     "test-project",
			AppName:       "test-app",
			AsyncOnly:     false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "long_names",
				Msg:        runsLoadedMsg{runs: runs},
				ViewGolden: "runs_list_long_names",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.False(t, m.loading)
					assert.Nil(t, m.Error())
					assert.Len(t, m.runs, 2)
				},
			}).
			Run(t)
	})

	t.Run("displays_many_runs_with_scrolling", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		// Create 25 runs to test table height limiting (max 20 rows)
		runs := make([]api.Run, 25)
		for i := 0; i < 25; i++ {
			runs[i] = api.Run{
				ID:           fmt.Sprintf("run-%02d", i+1),
				FunctionName: "predict",
				Status:       "success",
				CreatedAt:    time.Date(2025, 1, 1, 12, i, 0, 0, time.UTC),
				Async:        i%2 == 0,
			}
		}

		model := NewListView(t.Context(), ListConfig{
			DisplayConfig: ui.DisplayConfig{IsInteractive: true},
			Client:        mockClient,
			ProjectID:     "test-project",
			AppName:       "test-app",
			AsyncOnly:     false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ListView]{
				Name:       "many_runs",
				Msg:        runsLoadedMsg{runs: runs},
				ViewGolden: "runs_list_many_runs",
				ModelAssert: func(t *testing.T, m *ListView) {
					assert.False(t, m.loading)
					assert.Nil(t, m.Error())
					assert.Len(t, m.runs, 25)
				},
			}).
			Run(t)
	})
}

func Test_normalizeAppID(t *testing.T) {
	tcs := []struct {
		name      string
		projectID string
		appName   string
		expected  string
	}{
		{
			name:      "app name without project prefix",
			projectID: "dev-p-0780791d",
			appName:   "5-dockerfile",
			expected:  "dev-p-0780791d-5-dockerfile",
		},
		{
			name:      "app name with project prefix",
			projectID: "dev-p-0780791d",
			appName:   "dev-p-0780791d-5-dockerfile",
			expected:  "dev-p-0780791d-5-dockerfile",
		},
		{
			name:      "app name with partial match",
			projectID: "dev-p-0780791d",
			appName:   "dev-p-123-myapp",
			expected:  "dev-p-0780791d-dev-p-123-myapp",
		},
		{
			name:      "simple app name",
			projectID: "project-123",
			appName:   "myapp",
			expected:  "project-123-myapp",
		},
		{
			name:      "app name already has full ID",
			projectID: "project-123",
			appName:   "project-123-myapp-v2",
			expected:  "project-123-myapp-v2",
		},
		{
			name:      "edge case - empty app name",
			projectID: "project-123",
			appName:   "",
			expected:  "project-123-",
		},
		{
			name:      "edge case - app name with only dash",
			projectID: "project-123",
			appName:   "-test",
			expected:  "project-123--test",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizeAppID(tc.projectID, tc.appName)
			assert.Equal(t, tc.expected, result)
		})
	}
}
