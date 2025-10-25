package ui

import (
	"testing"

	uitesting "github.com/cerebriumai/cerebrium/internal/ui/testing"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/stretchr/testify/assert"
)

//go:generate go test -v -run TestSpinnerModel -update

func TestSpinnerModel(t *testing.T) {
	t.Run("initial state", func(t *testing.T) {
		model := NewSpinner()

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*SpinnerModel]{
				Name:       "initial_view",
				Msg:        nil, // Just test initial state
				ViewGolden: "spinner_initial",
				ModelAssert: func(t *testing.T, m *SpinnerModel) {
					assert.NotNil(t, m.spinner)
				},
			}).
			Run(t)
	})

	t.Run("handles tick messages", func(t *testing.T) {
		model := NewSpinner()

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*SpinnerModel]{
				Name:       "first_tick",
				Msg:        spinner.TickMsg{},
				ViewGolden: "spinner_first_tick",
				ViewAssert: func(t *testing.T, view string) {
					// View should contain spinner frame
					assert.NotEmpty(t, view)
				},
				ModelAssert: func(t *testing.T, m *SpinnerModel) {
					assert.NotNil(t, m.spinner)
				},
			}).
			Step(uitesting.TestStep[*SpinnerModel]{
				Name:       "second_tick",
				Msg:        spinner.TickMsg{},
				ViewGolden: "spinner_second_tick",
				ViewAssert: func(t *testing.T, view string) {
					// View should still contain spinner frame
					assert.NotEmpty(t, view)
				},
			}).
			Run(t)
	})
}

func TestSpinnerModel_View(t *testing.T) {
	model := NewSpinner()

	// The View should delegate to the internal spinner
	view := model.View()
	assert.NotEmpty(t, view, "View should return spinner frame")
}

func TestSpinnerModel_Update(t *testing.T) {
	model := NewSpinner()

	// Update with tick message
	updatedModel, cmd := model.Update(spinner.TickMsg{})

	assert.NotNil(t, updatedModel, "Update should return model")
	assert.NotNil(t, cmd, "Update should return next tick command")
}
