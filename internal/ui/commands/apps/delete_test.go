package apps

import (
	"errors"
	"testing"

	apimock "github.com/cerebriumai/cerebrium/internal/api/mock"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uitesting "github.com/cerebriumai/cerebrium/internal/ui/testing"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

//go:generate go test -v -run TestAppDeleteView -update

func TestAppDeleteView(t *testing.T) {
	t.Run("success - interactive mode", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewDeleteView(
			ctx,
			DeleteConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
				Client:    mockClient,
				ProjectID: "test-project",
				AppID:     "test-app",
			},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeleteView]{
				Name:       "initial_deleting",
				ViewGolden: "delete_deleting",
				ModelAssert: func(t *testing.T, m *DeleteView) {
					assert.Equal(t, DeleteStateAppDeleting, m.status)
					assert.Nil(t, m.Error())
				},
			}).
			Step(uitesting.TestStep[*DeleteView]{
				Name:       "delete_success",
				Msg:        appDeletedMsg{},
				ViewGolden: "delete_success",
				ModelAssert: func(t *testing.T, m *DeleteView) {
					assert.Equal(t, DeleteStateAppDeleted, m.status)
					assert.Nil(t, m.Error())
				},
			}).
			Run(t)
	})

	t.Run("success - simple mode", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewDeleteView(
			ctx,
			DeleteConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    false,
					DisableAnimation: true,
				},
				Client:    mockClient,
				ProjectID: "test-project",
				AppID:     "simple-app",
			},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeleteView]{
				Name: "simple_mode_output",
				Msg:  appDeletedMsg{},
				ViewAssert: func(t *testing.T, view string) {
					// In simple mode, View() returns empty string
					assert.Empty(t, view)
				},
				ModelAssert: func(t *testing.T, m *DeleteView) {
					assert.Equal(t, DeleteStateAppDeleted, m.status)
					assert.Nil(t, m.Error())
				},
			}).
			Run(t)
	})

	t.Run("error - API error", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewDeleteView(
			ctx,
			DeleteConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
				Client:    mockClient,
				ProjectID: "test-project",
				AppID:     "error-app",
			},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeleteView]{
				Name:       "delete_error",
				Msg:        ui.NewAPIError(errors.New("API connection failed")),
				ViewGolden: "delete_error",
				ModelAssert: func(t *testing.T, m *DeleteView) {
					assert.Equal(t, DeleteStateAppDeleteError, m.status)
					assert.NotNil(t, m.Error())
				},
			}).
			Run(t)
	})

	t.Run("error - app not found", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewDeleteView(
			ctx,
			DeleteConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
				Client:    mockClient,
				ProjectID: "test-project",
				AppID:     "nonexistent-app",
			},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeleteView]{
				Name:       "not_found_error",
				Msg:        ui.NewAPIError(errors.New("app not found")),
				ViewGolden: "delete_not_found",
				ModelAssert: func(t *testing.T, m *DeleteView) {
					assert.Equal(t, DeleteStateAppDeleteError, m.status)
					assert.NotNil(t, m.Error())
				},
			}).
			Run(t)
	})

	t.Run("error - permission denied", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewDeleteView(
			ctx,
			DeleteConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
				Client:    mockClient,
				ProjectID: "test-project",
				AppID:     "forbidden-app",
			},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeleteView]{
				Name:       "permission_error",
				Msg:        ui.NewAPIError(errors.New("permission denied")),
				ViewGolden: "delete_permission_error",
				ModelAssert: func(t *testing.T, m *DeleteView) {
					assert.Equal(t, DeleteStateAppDeleteError, m.status)
					assert.NotNil(t, m.Error())
				},
			}).
			Run(t)
	})

	t.Run("signal cancel - interactive mode", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewDeleteView(
			ctx,
			DeleteConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
				Client:    mockClient,
				ProjectID: "test-project",
				AppID:     "cancel-app",
			},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeleteView]{
				Name: "cancel_signal",
				Msg:  ui.SignalCancelMsg{},
				ViewAssert: func(t *testing.T, view string) {
					// Should still render deleting status before quit
					uitesting.AssertContains(t, view, "Deleting")
				},
				ModelAssert: func(t *testing.T, m *DeleteView) {
					assert.Equal(t, DeleteStateAppDeleting, m.status)
				},
			}).
			Run(t)
	})

	t.Run("keyboard - ctrl+c in interactive mode", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewDeleteView(
			ctx,
			DeleteConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
				Client:    mockClient,
				ProjectID: "test-project",
				AppID:     "keyboard-app",
			},
		)

		// First let it initialize
		model.Init()

		// Then send ctrl+c
		_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

		// Should quit
		assert.NotNil(t, cmd)
	})

	t.Run("status transitions", func(t *testing.T) {
		ctx := t.Context()
		mockClient := apimock.NewMockClient(t)

		model := NewDeleteView(
			ctx,
			DeleteConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
				Client:    mockClient,
				ProjectID: "test-project",
				AppID:     "transition-app",
			},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeleteView]{
				Name: "initial_state",
				ModelAssert: func(t *testing.T, m *DeleteView) {
					assert.Equal(t, DeleteStateAppDeleting, m.status)
				},
			}).
			Step(uitesting.TestStep[*DeleteView]{
				Name: "after_delete",
				Msg:  appDeletedMsg{},
				ModelAssert: func(t *testing.T, m *DeleteView) {
					assert.Equal(t, DeleteStateAppDeleted, m.status)
				},
			}).
			Run(t)
	})
}

func TestAppDeleteView_View(t *testing.T) {
	t.Run("view during deletion", func(t *testing.T) {
		model := &DeleteView{
			ctx:     t.Context(),
			status:  DeleteStateAppDeleting,
			spinner: ui.NewSpinner(),
			conf: DeleteConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
				ProjectID: "test-project",
				AppID:     "test-app",
			},
		}

		view := model.View()
		assert.Contains(t, view, "Deleting app 'test-app'")
	})

	t.Run("view after success", func(t *testing.T) {
		model := &DeleteView{
			ctx:    t.Context(),
			status: DeleteStateAppDeleted,
			conf: DeleteConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
				ProjectID: "test-project",
				AppID:     "test-app",
			},
		}

		view := model.View()
		assert.Contains(t, view, "deleted successfully")
		assert.Contains(t, view, "test-app")
	})

	t.Run("view after error", func(t *testing.T) {
		model := &DeleteView{
			ctx:    t.Context(),
			status: DeleteStateAppDeleteError,
			err:    ui.NewAPIError(errors.New("test error")),
			conf: DeleteConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
				ProjectID: "test-project",
				AppID:     "test-app",
			},
		}

		view := model.View()
		// When m.Err is set, it uses ui.FormatError which produces "✗ Error: <message>"
		assert.Contains(t, view, "Error: test error")
	})

	t.Run("view in simple mode", func(t *testing.T) {
		model := &DeleteView{
			ctx:    t.Context(),
			status: DeleteStateAppDeleting,
			conf: DeleteConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    false,
					DisableAnimation: true,
				},
				ProjectID: "test-project",
				AppID:     "test-app",
			},
		}

		view := model.View()
		assert.Empty(t, view, "View should return empty string in simple mode")
	})
}
