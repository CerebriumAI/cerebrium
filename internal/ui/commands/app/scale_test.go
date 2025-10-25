package app

import (
	"errors"
	"testing"

	apimock "github.com/cerebriumai/cerebrium/internal/api/mock"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uitesting "github.com/cerebriumai/cerebrium/internal/ui/testing"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

//go:generate go test -v -run TestAppScaleView -update

func TestAppScaleView(t *testing.T) {
	t.Run("success - interactive mode with all parameters", func(t *testing.T) {
		ctx := t.Context()
		updates := map[string]any{
			"cooldownPeriodSeconds":      60,
			"minReplicaCount":            2,
			"maxReplicaCount":            10,
			"responseGracePeriodSeconds": 900,
		}

		mockClient := apimock.NewMockClient(t)

		model := NewScaleView(ctx, ScaleConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "test-app",
			Updates:   updates,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ScaleView]{
				Name:       "initial_scaling",
				Msg:        nil,
				ViewGolden: "scale_scaling_all",
				ModelAssert: func(t *testing.T, m *ScaleView) {
					assert.True(t, m.scaling)
					assert.False(t, m.scaled)
					assert.Nil(t, m.Error())
				},
			}).
			Step(uitesting.TestStep[*ScaleView]{
				Name:       "scale_success",
				Msg:        appScaledMsg{},
				ViewGolden: "scale_success_all",
				ModelAssert: func(t *testing.T, m *ScaleView) {
					assert.False(t, m.scaling)
					assert.True(t, m.scaled)
					assert.Nil(t, m.Error())
				},
			}).
			Run(t)
	})

	t.Run("success - cooldown only", func(t *testing.T) {
		ctx := t.Context()
		updates := map[string]any{
			"cooldownPeriodSeconds": 120,
		}

		mockClient := apimock.NewMockClient(t)

		model := NewScaleView(ctx, ScaleConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "cooldown-app",
			Updates:   updates,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ScaleView]{
				Name:       "cooldown_success",
				Msg:        appScaledMsg{},
				ViewGolden: "scale_success_cooldown",
				ModelAssert: func(t *testing.T, m *ScaleView) {
					assert.True(t, m.scaled)
					assert.Equal(t, 120, m.conf.Updates["cooldownPeriodSeconds"])
				},
			}).
			Run(t)
	})

	t.Run("success - min/max replicas only", func(t *testing.T) {
		ctx := t.Context()
		updates := map[string]any{
			"minReplicaCount": 1,
			"maxReplicaCount": 5,
		}

		mockClient := apimock.NewMockClient(t)

		model := NewScaleView(ctx, ScaleConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "replicas-app",
			Updates:   updates,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ScaleView]{
				Name:       "replicas_success",
				Msg:        appScaledMsg{},
				ViewGolden: "scale_success_replicas",
				ModelAssert: func(t *testing.T, m *ScaleView) {
					assert.True(t, m.scaled)
					assert.Equal(t, 1, m.conf.Updates["minReplicaCount"])
					assert.Equal(t, 5, m.conf.Updates["maxReplicaCount"])
				},
			}).
			Run(t)
	})

	t.Run("success - response grace period only", func(t *testing.T) {
		ctx := t.Context()
		updates := map[string]any{
			"responseGracePeriodSeconds": 1800,
		}

		mockClient := apimock.NewMockClient(t)

		model := NewScaleView(ctx, ScaleConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "grace-app",
			Updates:   updates,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ScaleView]{
				Name:       "grace_period_success",
				Msg:        appScaledMsg{},
				ViewGolden: "scale_success_grace",
				ModelAssert: func(t *testing.T, m *ScaleView) {
					assert.True(t, m.scaled)
					assert.Equal(t, 1800, m.conf.Updates["responseGracePeriodSeconds"])
				},
			}).
			Run(t)
	})

	t.Run("success - simple mode", func(t *testing.T) {
		ctx := t.Context()
		updates := map[string]any{
			"cooldownPeriodSeconds": 45,
			"minReplicaCount":       0,
			"maxReplicaCount":       3,
		}

		mockClient := apimock.NewMockClient(t)

		model := NewScaleView(ctx, ScaleConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "simple-app",
			Updates:   updates,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: true,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ScaleView]{
				Name: "simple_mode_output",
				Msg:  appScaledMsg{},
				ViewAssert: func(t *testing.T, view string) {
					// In simple mode, View() returns empty string
					assert.Empty(t, view)
				},
				ModelAssert: func(t *testing.T, m *ScaleView) {
					assert.True(t, m.scaled)
					assert.False(t, m.scaling)
				},
			}).
			Run(t)
	})

	t.Run("error - API error", func(t *testing.T) {
		ctx := t.Context()
		updates := map[string]any{
			"cooldownPeriodSeconds": 60,
		}

		mockClient := apimock.NewMockClient(t)

		model := NewScaleView(ctx, ScaleConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "error-app",
			Updates:   updates,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ScaleView]{
				Name:       "api_error",
				Msg:        ui.NewAPIError(errors.New("API connection failed")),
				ViewGolden: "scale_error",
				ModelAssert: func(t *testing.T, m *ScaleView) {
					assert.NotNil(t, m.Error())
					assert.False(t, m.scaling)
					assert.False(t, m.scaled)
				},
			}).
			Run(t)
	})

	t.Run("error - app not found", func(t *testing.T) {
		ctx := t.Context()
		updates := map[string]any{
			"minReplicaCount": 1,
		}

		mockClient := apimock.NewMockClient(t)

		model := NewScaleView(ctx, ScaleConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "nonexistent-app",
			Updates:   updates,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ScaleView]{
				Name:       "not_found_error",
				Msg:        ui.NewAPIError(errors.New("app not found")),
				ViewGolden: "scale_not_found",
				ModelAssert: func(t *testing.T, m *ScaleView) {
					assert.NotNil(t, m.Error())
					assert.False(t, m.scaled)
				},
			}).
			Run(t)
	})

	t.Run("error - invalid parameter", func(t *testing.T) {
		ctx := t.Context()
		updates := map[string]any{
			"minReplicaCount": 10,
			"maxReplicaCount": 5, // Invalid: min > max
		}

		mockClient := apimock.NewMockClient(t)

		model := NewScaleView(ctx, ScaleConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "invalid-app",
			Updates:   updates,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ScaleView]{
				Name:       "invalid_params_error",
				Msg:        ui.NewAPIError(errors.New("invalid parameters: minReplicaCount must be <= maxReplicaCount")),
				ViewGolden: "scale_invalid_params",
				ModelAssert: func(t *testing.T, m *ScaleView) {
					assert.NotNil(t, m.Error())
					assert.False(t, m.scaled)
				},
			}).
			Run(t)
	})

	t.Run("signal cancel - interactive mode", func(t *testing.T) {
		ctx := t.Context()
		updates := map[string]any{
			"cooldownPeriodSeconds": 60,
		}

		mockClient := apimock.NewMockClient(t)

		model := NewScaleView(ctx, ScaleConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "cancel-app",
			Updates:   updates,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ScaleView]{
				Name: "cancel_signal",
				Msg:  ui.SignalCancelMsg{},
				ViewAssert: func(t *testing.T, view string) {
					// Should still render scaling status before quit
					uitesting.AssertContains(t, view, "Updating scaling")
				},
				ModelAssert: func(t *testing.T, m *ScaleView) {
					assert.True(t, m.scaling)
				},
			}).
			Run(t)
	})

	t.Run("keyboard - ctrl+c in interactive mode", func(t *testing.T) {
		ctx := t.Context()
		updates := map[string]any{
			"cooldownPeriodSeconds": 60,
		}

		mockClient := apimock.NewMockClient(t)

		model := NewScaleView(ctx, ScaleConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "keyboard-app",
			Updates:   updates,
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

	t.Run("empty updates", func(t *testing.T) {
		ctx := t.Context()
		updates := map[string]any{}

		mockClient := apimock.NewMockClient(t)

		model := NewScaleView(ctx, ScaleConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "empty-updates-app",
			Updates:   updates,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*ScaleView]{
				Name:       "empty_updates",
				Msg:        appScaledMsg{},
				ViewGolden: "scale_empty_updates",
				ModelAssert: func(t *testing.T, m *ScaleView) {
					assert.True(t, m.scaled)
					assert.Empty(t, m.conf.Updates)
				},
			}).
			Run(t)
	})
}

func Test_formatUpdates(t *testing.T) {
	ctx := t.Context()

	t.Run("all parameters", func(t *testing.T) {
		model := ScaleView{
			ctx: ctx,
			conf: ScaleConfig{
				Updates: map[string]any{
					"cooldownPeriodSeconds":      120,
					"minReplicaCount":            2,
					"maxReplicaCount":            10,
					"responseGracePeriodSeconds": 1800,
				},
			},
		}

		output := model.formatUpdates()

		assert.Contains(t, output, "Cooldown period: 120 seconds")
		assert.Contains(t, output, "Minimum replicas: 2")
		assert.Contains(t, output, "Maximum replicas: 10")
		assert.Contains(t, output, "Response grace period: 1800 seconds")
	})

	t.Run("partial parameters", func(t *testing.T) {
		model := ScaleView{
			ctx: ctx,
			conf: ScaleConfig{
				Updates: map[string]any{
					"minReplicaCount": 1,
					"maxReplicaCount": 3,
				},
			},
		}

		output := model.formatUpdates()

		assert.Contains(t, output, "Minimum replicas: 1")
		assert.Contains(t, output, "Maximum replicas: 3")
		assert.NotContains(t, output, "Cooldown")
		assert.NotContains(t, output, "Response grace")
	})

	t.Run("empty updates", func(t *testing.T) {
		model := ScaleView{
			ctx: ctx,
			conf: ScaleConfig{
				Updates: map[string]any{},
			},
		}

		output := model.formatUpdates()

		assert.Empty(t, output)
	})
}

func TestAppScaleView_View(t *testing.T) {
	ctx := t.Context()
	updates := map[string]any{
		"cooldownPeriodSeconds": 60,
		"minReplicaCount":       1,
		"maxReplicaCount":       5,
	}

	t.Run("view during scaling", func(t *testing.T) {
		model := ScaleView{
			ctx:     ctx,
			scaling: true,
			spinner: ui.NewSpinner(),
			conf: ScaleConfig{
				ProjectID: "test-project",
				AppID:     "test-app",
				Updates:   updates,
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
			},
		}

		view := model.View()
		assert.Contains(t, view, "Updating scaling for app 'test-app'")
		assert.Contains(t, view, "Cooldown period: 60 seconds")
		assert.Contains(t, view, "Minimum replicas: 1")
		assert.Contains(t, view, "Maximum replicas: 5")
	})

	t.Run("view after success", func(t *testing.T) {
		model := ScaleView{
			ctx:     ctx,
			scaling: false,
			scaled:  true,
			conf: ScaleConfig{
				ProjectID: "test-project",
				AppID:     "test-app",
				Updates:   updates,
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
			},
		}

		view := model.View()
		assert.Contains(t, view, "scaled successfully")
		assert.Contains(t, view, "test-app")
		assert.Contains(t, view, "Cooldown period: 60 seconds")
	})

	t.Run("view after error", func(t *testing.T) {
		model := ScaleView{
			ctx:     ctx,
			scaling: false,
			err:     ui.NewAPIError(errors.New("test error")),
			conf: ScaleConfig{
				ProjectID: "test-project",
				AppID:     "test-app",
				Updates:   updates,
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
		model := ScaleView{
			ctx:     ctx,
			scaling: true,
			conf: ScaleConfig{
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    false,
					DisableAnimation: true,
				},
				ProjectID: "test-project",
				AppID:     "test-app",
				Updates:   updates,
			},
		}

		view := model.View()
		assert.Empty(t, view, "View should return empty string in simple mode")
	})
}
