package app

import (
	"errors"
	"testing"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	apimock "github.com/cerebriumai/cerebrium/internal/api/mock"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uitesting "github.com/cerebriumai/cerebrium/internal/ui/testing"
	"github.com/stretchr/testify/assert"
)

//go:generate go test -v -run TestAppGetView -update

func TestAppGetView(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	t.Run("success - interactive mode with GPU app", func(t *testing.T) {
		ctx := t.Context()

		appDetails := &api.AppDetails{
			ID:                         "test-app-gpu",
			CreatedAt:                  baseTime,
			UpdatedAt:                  baseTime.Add(time.Hour),
			Hardware:                   "GPU",
			CPU:                        "4",
			Memory:                     "16",
			GPUCount:                   "1",
			CooldownPeriodSeconds:      "60",
			MinReplicaCount:            "0",
			MaxReplicaCount:            "5",
			ResponseGracePeriodSeconds: "900",
			Status:                     "ACTIVE",
			LastBuildStatus:            "SUCCESS",
			LatestBuildID:              "build-123",
			Pods:                       []string{"pod-1", "pod-2"},
		}

		mockClient := apimock.NewMockClient(t)
		// Note: We don't set up expectations because the async command won't execute
		// due to spinner consuming the depth limit. We manually send the message instead.

		model := NewGetView(ctx, GetConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "test-app-gpu",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*GetView]{
				Name:       "initial_loading",
				ViewGolden: "get_loading",
				ModelAssert: func(t *testing.T, m *GetView) {
					assert.True(t, m.loading)
					assert.Nil(t, m.appDetails)
				},
			}).
			Step(uitesting.TestStep[*GetView]{
				Name: "success_loaded",
				// Manually trigger the loaded message (simulating what fetchAppDetails returns)
				Msg:        appDetailsLoadedMsg{appDetails: appDetails},
				ViewGolden: "get_success_gpu",
				ModelAssert: func(t *testing.T, m *GetView) {
					assert.False(t, m.loading)
					assert.NotNil(t, m.appDetails)
					assert.Equal(t, "test-app-gpu", m.appDetails.ID)
					assert.Equal(t, "GPU", m.appDetails.Hardware)
					assert.Equal(t, "1", m.appDetails.GPUCount)
					assert.Len(t, m.appDetails.Pods, 2)
				},
			}).
			Run(t)
	})

	t.Run("success - CPU only app", func(t *testing.T) {
		ctx := t.Context()

		appDetails := &api.AppDetails{
			ID:                         "test-app-cpu",
			CreatedAt:                  baseTime,
			UpdatedAt:                  baseTime,
			Hardware:                   "CPU",
			CPU:                        "2",
			Memory:                     "8",
			GPUCount:                   "0",
			CooldownPeriodSeconds:      "30",
			MinReplicaCount:            "1",
			MaxReplicaCount:            "3",
			ResponseGracePeriodSeconds: "600",
			Status:                     "PENDING",
			LastBuildStatus:            "BUILDING",
			LatestBuildID:              "",
			Pods:                       nil,
		}

		mockClient := apimock.NewMockClient(t)

		model := NewGetView(ctx, GetConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "test-app-cpu",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*GetView]{
				Name:       "success_cpu_only",
				Msg:        appDetailsLoadedMsg{appDetails: appDetails},
				ViewGolden: "get_success_cpu",
				ModelAssert: func(t *testing.T, m *GetView) {
					assert.NotNil(t, m.appDetails)
					assert.Equal(t, "test-app-cpu", m.appDetails.ID)
					assert.Equal(t, "CPU", m.appDetails.Hardware)
					assert.Equal(t, "0", m.appDetails.GPUCount)
					assert.Empty(t, m.appDetails.Pods)
				},
			}).
			Run(t)
	})

	t.Run("success - simple mode", func(t *testing.T) {
		ctx := t.Context()

		appDetails := &api.AppDetails{
			ID:                         "simple-app",
			CreatedAt:                  baseTime,
			UpdatedAt:                  baseTime,
			Hardware:                   "GPU",
			CPU:                        "4",
			Memory:                     "16",
			GPUCount:                   "2",
			CooldownPeriodSeconds:      "45",
			MinReplicaCount:            "0",
			MaxReplicaCount:            "10",
			ResponseGracePeriodSeconds: "1200",
			Status:                     "ACTIVE",
			LastBuildStatus:            "SUCCESS",
			LatestBuildID:              "build-456",
			Pods:                       []string{"pod-a"},
		}

		mockClient := apimock.NewMockClient(t)

		model := NewGetView(ctx, GetConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "simple-app",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: true,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*GetView]{
				Name: "simple_mode_initial",
				ViewAssert: func(t *testing.T, view string) {
					// In simple mode, View() returns empty string
					assert.Empty(t, view)
				},
				ModelAssert: func(t *testing.T, m *GetView) {
					// Initially loading
					assert.True(t, m.loading)
					assert.Nil(t, m.appDetails)
				},
			}).
			Step(uitesting.TestStep[*GetView]{
				Name: "simple_mode_loaded",
				Msg:  appDetailsLoadedMsg{appDetails: appDetails},
				ViewAssert: func(t *testing.T, view string) {
					// In simple mode, View() returns empty string
					assert.Empty(t, view)
				},
				ModelAssert: func(t *testing.T, m *GetView) {
					// After message, data should be loaded
					assert.NotNil(t, m.appDetails)
					assert.Equal(t, "simple-app", m.appDetails.ID)
					assert.False(t, m.loading)
				},
			}).
			Run(t)
	})

	t.Run("error - API error", func(t *testing.T) {
		ctx := t.Context()

		mockClient := apimock.NewMockClient(t)

		model := NewGetView(ctx, GetConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "error-app",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*GetView]{
				Name:       "api_error",
				Msg:        ui.NewAPIError(errors.New("API connection failed")),
				ViewGolden: "get_error",
				ModelAssert: func(t *testing.T, m *GetView) {
					assert.NotNil(t, m.Error())
					assert.Nil(t, m.appDetails)
				},
			}).
			Run(t)
	})

	t.Run("error - app not found", func(t *testing.T) {
		ctx := t.Context()

		mockClient := apimock.NewMockClient(t)

		model := NewGetView(ctx, GetConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "nonexistent-app",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*GetView]{
				Name:       "not_found_error",
				Msg:        ui.NewAPIError(errors.New("app not found")),
				ViewGolden: "get_not_found",
				ModelAssert: func(t *testing.T, m *GetView) {
					assert.NotNil(t, m.Error())
					assert.Nil(t, m.appDetails)
				},
			}).
			Run(t)
	})

	t.Run("signal cancel - interactive mode", func(t *testing.T) {
		ctx := t.Context()

		mockClient := apimock.NewMockClient(t)

		model := NewGetView(ctx, GetConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "cancel-app",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*GetView]{
				Name: "cancel_signal",
				Msg:  ui.SignalCancelMsg{},
				ViewAssert: func(t *testing.T, view string) {
					// Should still render loading status before quit
					uitesting.AssertContains(t, view, "Loading")
				},
				ModelAssert: func(t *testing.T, m *GetView) {
					assert.True(t, m.loading)
				},
			}).
			Run(t)
	})

	t.Run("parse errors - invalid string values", func(t *testing.T) {
		ctx := t.Context()

		appDetails := &api.AppDetails{
			ID:                         "parse-error-app",
			CreatedAt:                  baseTime,
			UpdatedAt:                  baseTime,
			Hardware:                   "GPU",
			CPU:                        "invalid",      // Invalid int
			Memory:                     "not-a-number", // Invalid int
			GPUCount:                   "bad",          // Invalid int
			CooldownPeriodSeconds:      "60",
			MinReplicaCount:            "0",
			MaxReplicaCount:            "5",
			ResponseGracePeriodSeconds: "900",
			Status:                     "ACTIVE",
			LastBuildStatus:            "SUCCESS",
			LatestBuildID:              "build-123",
			Pods:                       nil,
		}

		mockClient := apimock.NewMockClient(t)

		model := NewGetView(ctx, GetConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "parse-error-app",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*GetView]{
				Name:       "parse_errors",
				Msg:        appDetailsLoadedMsg{appDetails: appDetails},
				ViewGolden: "get_parse_errors",
				ViewAssert: func(t *testing.T, view string) {
					// parseErrors are populated during View() rendering
					uitesting.AssertContains(t, view, "[error parsing")
					uitesting.AssertContains(t, view, "Discord")
				},
				ModelAssert: func(t *testing.T, m *GetView) {
					// Verify the app details with invalid data were loaded
					assert.NotNil(t, m.appDetails)
					assert.Equal(t, "parse-error-app", m.appDetails.ID)
					// parseErrors is populated during View() call, not Update()
					// So we verify it in ViewAssert instead
				},
			}).
			Run(t)
	})

	t.Run("no data - empty hardware fields", func(t *testing.T) {
		ctx := t.Context()

		appDetails := &api.AppDetails{
			ID:                         "empty-app",
			CreatedAt:                  baseTime,
			UpdatedAt:                  baseTime,
			Hardware:                   "", // Empty
			CPU:                        "", // Empty
			Memory:                     "", // Empty
			GPUCount:                   "", // Empty
			CooldownPeriodSeconds:      "60",
			MinReplicaCount:            "0",
			MaxReplicaCount:            "1",
			ResponseGracePeriodSeconds: "", // Empty
			Status:                     "PENDING",
			LastBuildStatus:            "PENDING",
			LatestBuildID:              "",
			Pods:                       nil,
		}

		mockClient := apimock.NewMockClient(t)

		model := NewGetView(ctx, GetConfig{
			Client:    mockClient,
			ProjectID: "test-project",
			AppID:     "empty-app",
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		},
		)

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*GetView]{
				Name:       "empty_fields",
				Msg:        appDetailsLoadedMsg{appDetails: appDetails},
				ViewGolden: "get_empty_fields",
				ViewAssert: func(t *testing.T, view string) {
					uitesting.AssertContains(t, view, "Data Unavailable")
				},
				ModelAssert: func(t *testing.T, m *GetView) {
					assert.NotNil(t, m.appDetails)
					assert.Equal(t, "", m.appDetails.Hardware)
				},
			}).
			Run(t)
	})
}

func Test_formatAppDetailsSimple(t *testing.T) {
	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	model := GetView{
		appDetails: &api.AppDetails{
			ID:                         "format-test",
			CreatedAt:                  baseTime,
			UpdatedAt:                  baseTime.Add(time.Hour),
			Hardware:                   "GPU",
			CPU:                        "8",
			Memory:                     "32",
			GPUCount:                   "4",
			CooldownPeriodSeconds:      "120",
			MinReplicaCount:            "2",
			MaxReplicaCount:            "10",
			ResponseGracePeriodSeconds: "1800",
			Status:                     "ACTIVE",
			LastBuildStatus:            "SUCCESS",
			LatestBuildID:              "build-789",
			Pods:                       []string{"pod-1", "pod-2", "pod-3"},
		},
	}

	output := model.formatAppDetailsSimple()

	// Verify key sections are present
	assert.Contains(t, output, "App Details: format-test")
	assert.Contains(t, output, "APP")
	assert.Contains(t, output, "HARDWARE")
	assert.Contains(t, output, "SCALING PARAMETERS")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "LIVE PODS")

	// Verify specific values
	assert.Contains(t, output, "Compute: GPU")
	assert.Contains(t, output, "CPU: 8 cores")
	assert.Contains(t, output, "Memory: 32 GB")
	assert.Contains(t, output, "GPU Count: 4")
	assert.Contains(t, output, "Cooldown Period: 120s")
	assert.Contains(t, output, "Minimum Replicas: 2")
	assert.Contains(t, output, "Maximum Replicas: 10")
	assert.Contains(t, output, "Response Grace Period: 1800s")
	assert.Contains(t, output, "Status: ACTIVE")
	assert.Contains(t, output, "Last Build Status: SUCCESS")
	assert.Contains(t, output, "Last Build ID: build-789")
	assert.Contains(t, output, "Pod 1: pod-1")
	assert.Contains(t, output, "Pod 2: pod-2")
	assert.Contains(t, output, "Pod 3: pod-3")
}
