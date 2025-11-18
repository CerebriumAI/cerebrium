package commands

import (
	"errors"
	"testing"

	"github.com/cerebriumai/cerebrium/internal/api"
	apimock "github.com/cerebriumai/cerebrium/internal/api/mock"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uitesting "github.com/cerebriumai/cerebrium/internal/ui/testing"
	"github.com/cerebriumai/cerebrium/pkg/projectconfig"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

//go:generate go test -v -run TestDeployView -update

func TestDeployView(t *testing.T) {
	t.Run("confirmation state when not disabled", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "test-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:              config,
			ProjectID:           "test-project",
			Client:              mockClient,
			DisableBuildLogs:    false,
			DisableConfirmation: false, // Confirmation enabled
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name:       "initial_confirmation_state",
				ViewGolden: "deploy_confirmation",
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateConfirmation, m.state)
				},
			}).
			Run(t)
	})

	t.Run("confirmation - user accepts", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "test-app",
			},
			Hardware: projectconfig.HardwareConfig{
				CPU:    func(v float64) *float64 { return &v }(1.0),
				Memory: func(v float64) *float64 { return &v }(2.0),
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:              config,
			ProjectID:           "test-project",
			Client:              mockClient,
			DisableBuildLogs:    false,
			DisableConfirmation: false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name:       "confirm_yes",
				Msg:        tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}},
				ViewGolden: "deploy_confirm_yes_transition",
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateLoadingFiles, m.state)
				},
			}).
			Run(t)
	})

	t.Run("confirmation - user cancels", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "test-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:              config,
			ProjectID:           "test-project",
			Client:              mockClient,
			DisableBuildLogs:    false,
			DisableConfirmation: false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name: "confirm_no",
				Msg:  tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}},
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.NotNil(t, m.err)
					assert.Contains(t, m.err.Error(), "cancelled by user")
				},
			}).
			Run(t)
	})

	t.Run("confirmation state with full config", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		// Create a comprehensive config to test the display
		compute := "NVIDIA_A10"
		cpu := 2.0
		memory := 8.0
		gpuCount := 1
		region := "us-east-1"
		provider := "aws"
		minReplicas := 0
		maxReplicas := 5
		cooldown := 60
		replicaConcurrency := 2

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name:          "comprehensive-app",
				PythonVersion: "3.11",
				Include:       []string{"*.py", "requirements.txt"},
				Exclude:       []string{"__pycache__", "*.pyc"},
			},
			Hardware: projectconfig.HardwareConfig{
				Compute:  &compute,
				CPU:      &cpu,
				Memory:   &memory,
				GPUCount: &gpuCount,
				Region:   &region,
				Provider: &provider,
			},
			Scaling: projectconfig.ScalingConfig{
				MinReplicas:        &minReplicas,
				MaxReplicas:        &maxReplicas,
				Cooldown:           &cooldown,
				ReplicaConcurrency: &replicaConcurrency,
			},
			Dependencies: projectconfig.DependenciesConfig{
				Pip: map[string]string{
					"torch":        "2.0.0",
					"transformers": "4.30.0",
					"numpy":        "",
				},
				Apt: map[string]string{
					"ffmpeg": "",
				},
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:              config,
			ProjectID:           "test-project",
			Client:              mockClient,
			DisableBuildLogs:    false,
			DisableConfirmation: false,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name:       "full_config_confirmation",
				ViewGolden: "deploy_full_config_confirmation",
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateConfirmation, m.state)
				},
			}).
			Run(t)
	})

	t.Run("initial state", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "test-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:              config,
			ProjectID:           "test-project",
			Client:              mockClient,
			DisableBuildLogs:    false,
			DisableConfirmation: true,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name:       "initial_loading_files",
				ViewGolden: "deploy_loading_files",
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateLoadingFiles, m.state)
					assert.Empty(t, m.fileList)
				},
			}).
			Run(t)
	})

	t.Run("files loaded with real zip", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "test-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		// Use actual test files from testdata/test-app
		testFiles := []string{
			"testdata/test-app/main.py",
			"testdata/test-app/requirements.txt",
			"testdata/test-app/config.yaml",
		}

		// Using the new Expect/Finally API:
		// 1. Step sends filesLoadedMsg
		// 2. Harness executes zipFiles() command (actually zips the test files!)
		// 3. Finally intercepts filesZippedMsg, validates message, and stops
		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name: "files_loaded",
				Msg:  filesLoadedMsg{fileList: testFiles},
				// Don't assert state here - it will be checked after async processing
				SkipViewAssertion: true,
			}).
			Finally(uitesting.TestStep[*DeployView]{
				Name:            "files_zipped",
				ExpectedMsgType: filesZippedMsg{}, // Explicitly expect this message type
				MessageAssert: func(t *testing.T, msg tea.Msg) {
					// Validate the actual message from zipFiles() BEFORE Update()
					fzm, ok := msg.(filesZippedMsg)
					assert.True(t, ok, "should be filesZippedMsg")
					assert.NotEmpty(t, fzm.zipPath, "zip path should be set in message")
					assert.Greater(t, fzm.zipSize, int64(0), "zip size should be > 0 in message")
				},
				ViewGolden: "deploy_files_loaded_real_zip",
				ModelAssert: func(t *testing.T, m *DeployView) {
					// After filesZippedMsg is processed by Update():
					assert.Equal(t, StateCreatingApp, m.state)
					assert.Len(t, m.fileList, 3, "files should be preserved")
					assert.NotEmpty(t, m.zipPath, "zip path should be in model")
					assert.Greater(t, m.zipSize, int64(0), "zip size should be in model")
				},
			}).
			Run(t)
	})

	t.Run("files zipped transition", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "test-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		// Start from StateZippingFiles to avoid Init() processing
		model.state = StateZippingFiles
		model.fileList = []string{"testdata/test-app/main.py"}

		harness := uitesting.NewTestHarness(t, model)
		harness.
			// filesZippedMsg triggers createApp command, so use Finally to stop before it executes
			Finally(uitesting.TestStep[*DeployView]{
				Name: "files_zipped",
				Msg: filesZippedMsg{
					zipPath: "/tmp/cerebrium-deploy-123/app.zip",
					zipSize: 1024000, // 1MB
				},
				ViewGolden: "deploy_files_zipped",
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateCreatingApp, m.state)
					assert.Equal(t, "/tmp/cerebrium-deploy-123/app.zip", m.zipPath)
					assert.Equal(t, int64(1024000), m.zipSize)
				},
			}).
			Run(t)
	})

	t.Run("app created transition", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "test-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		model.state = StateCreatingApp
		model.zipPath = "/tmp/test.zip"

		response := &api.CreateAppResponse{
			BuildID:          "build-abc123",
			Status:           "building",
			UploadURL:        "https://s3.amazonaws.com/upload",
			KeyName:          "test-app",
			InternalEndpoint: "https://test-app.internal",
			DashboardURL:     "https://dashboard.cerebrium.ai/app/test-app",
		}

		harness := uitesting.NewTestHarness(t, model)
		harness.
			// appCreatedMsg triggers uploadZip command, so use Finally to stop before it executes
			Finally(uitesting.TestStep[*DeployView]{
				Name:       "app_created",
				Msg:        appCreatedMsg{response: response},
				ViewGolden: "deploy_app_created",
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateUploadingZip, m.state)
					assert.Equal(t, "build-abc123", m.buildID)
					assert.Equal(t, "building", m.buildStatus)
					assert.NotNil(t, m.appResponse)
				},
			}).
			Run(t)
	})

	t.Run("zip uploaded transition", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		// Mock FetchBuildLogs since zipUploadedMsg transitions to StateBuildingApp
		// which initializes the log viewer and starts fetching logs
		mockClient.On("FetchBuildLogs", mock.Anything, "test-project", "test-app", "build-123").
			Return(&api.BuildLogsResponse{Logs: []api.BuildLog{}}, nil).
			Maybe()

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "test-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		model.state = StateUploadingZip
		model.buildID = "build-123"

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name:       "zip_uploaded",
				Msg:        zipUploadedMsg{},
				ViewGolden: "deploy_zip_uploaded",
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateBuildingApp, m.state)
					assert.NotNil(t, m.logViewer, "LogViewer should be initialized")
				},
			}).
			Run(t)
	})

	t.Run("build complete - success", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "test-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		model.state = StateBuildingApp
		model.buildID = "build-success"
		model.appResponse = &api.CreateAppResponse{
			BuildID:          "build-success",
			Status:           "building",
			DashboardURL:     "https://dashboard.cerebrium.ai/app/test-app",
			InternalEndpoint: "https://test-app.internal",
		}

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name:       "build_complete_success",
				Msg:        buildCompleteMsg{status: "success"},
				ViewGolden: "deploy_build_success",
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateDeploySuccess, m.state)
					assert.Equal(t, "success", m.buildStatus)
				},
			}).
			Run(t)
	})

	t.Run("build complete - ready status", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "ready-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		model.state = StateBuildingApp
		model.buildID = "build-ready"
		model.appResponse = &api.CreateAppResponse{
			BuildID:      "build-ready",
			DashboardURL: "https://dashboard.cerebrium.ai",
		}

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name:       "build_ready",
				Msg:        buildCompleteMsg{status: "ready"},
				ViewGolden: "deploy_build_ready",
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateDeploySuccess, m.state)
					assert.Equal(t, "ready", m.buildStatus)
				},
			}).
			Run(t)
	})

	t.Run("build complete - failed", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "fail-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		model.state = StateBuildingApp
		model.buildID = "build-fail"

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name:       "build_failed",
				Msg:        buildCompleteMsg{status: "failed"},
				ViewGolden: "deploy_build_failed",
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateDeployError, m.state)
					assert.NotNil(t, m.err)
					assert.Equal(t, "failed", m.buildStatus)
				},
			}).
			Run(t)
	})

	t.Run("error - UIError message", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "error-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name:       "api_error",
				Msg:        ui.NewAPIError(errors.New("API connection failed")),
				ViewGolden: "deploy_error",
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateDeployError, m.state)
					assert.NotNil(t, m.err)
					assert.True(t, m.err.SilentExit)
				},
			}).
			Run(t)
	})

	t.Run("cancellation - signal during loading", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "cancel-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:              config,
			ProjectID:           "test-project",
			Client:              mockClient,
			DisableConfirmation: true, // Skip confirmation to test loading state
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name: "cancel_early",
				Msg:  ui.SignalCancelMsg{},
				ViewAssert: func(t *testing.T, view string) {
					uitesting.AssertContains(t, view, "Loading")
				},
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.NotNil(t, m.err)
					assert.Contains(t, m.err.Error(), "cancelled by user")
				},
			}).
			Run(t)
	})

	t.Run("cancellation - keyboard ctrl+c early", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "cancel-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		model.state = StateZippingFiles

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name: "keyboard_cancel",
				Msg:  tea.KeyMsg{Type: tea.KeyCtrlC},
				ViewAssert: func(t *testing.T, view string) {
					// View shows the progress so far, with Cancelled message
					uitesting.AssertContains(t, view, "Zipping files")
				},
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.NotNil(t, m.err)
				},
			}).
			Run(t)
	})

	t.Run("cancellation - build cancel success", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "cancel-build-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		model.state = StateBuildingApp
		model.buildID = "build-to-cancel"

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name:       "build_cancelled",
				Msg:        buildCancelledMsg{cancelErr: nil},
				ViewGolden: "deploy_cancelled",
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateCancelled, m.state)
					assert.NotNil(t, m.err)
				},
			}).
			Run(t)
	})

	t.Run("cancellation - build cancel failed", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "cancel-fail-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		model.state = StateBuildingApp
		model.buildID = "build-cancel-fail"

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name:       "cancel_with_error",
				Msg:        buildCancelledMsg{cancelErr: errors.New("cancellation API error")},
				ViewGolden: "deploy_cancel_error",
				ViewAssert: func(t *testing.T, view string) {
					uitesting.AssertContains(t, view, "Warning")
					uitesting.AssertContains(t, view, "Failed to cancel")
				},
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateCancelled, m.state)
					assert.NotNil(t, m.err)
					assert.NotEmpty(t, m.message)
				},
			}).
			Run(t)
	})

	t.Run("simple mode - success", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "simple-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: true,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		model.state = StateBuildingApp
		model.buildID = "build-simple"
		model.appResponse = &api.CreateAppResponse{
			BuildID:      "build-simple",
			DashboardURL: "https://dashboard.cerebrium.ai",
		}

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name: "simple_mode_success",
				Msg:  buildCompleteMsg{status: "success"},
				ViewAssert: func(t *testing.T, view string) {
					assert.Empty(t, view, "Simple mode should have empty View()")
				},
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateDeploySuccess, m.state)
				},
			}).
			Run(t)
	})

	t.Run("simple mode - error", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "simple-error-app",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: true,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*DeployView]{
				Name: "simple_mode_error",
				Msg:  ui.NewAPIError(errors.New("deployment failed")),
				ViewAssert: func(t *testing.T, view string) {
					assert.Empty(t, view, "Simple mode should have empty View()")
				},
				ModelAssert: func(t *testing.T, m *DeployView) {
					assert.Equal(t, StateDeployError, m.state)
					assert.NotNil(t, m.err)
				},
			}).
			Run(t)
	})

	t.Run("app created with docker auth present", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		// Mock CreateApp to verify dockerAuth is included in payload
		mockClient.On("CreateApp", mock.Anything, "test-project", mock.MatchedBy(func(payload map[string]any) bool {
			// Verify dockerAuth is present in the payload
			_, hasAuth := payload["dockerAuth"]
			return hasAuth && payload["dockerAuth"] != ""
		})).Return(&api.CreateAppResponse{
			BuildID:          "build-with-auth",
			Status:           "building",
			UploadURL:        "https://s3.amazonaws.com/upload",
			KeyName:          "test-app",
			InternalEndpoint: "https://test-app.internal",
			DashboardURL:     "https://dashboard.cerebrium.ai/app/test-app",
		}, nil).Once()

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "test-app-with-auth",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		// Simulate having Docker auth by mocking the auth.GetDockerAuth response
		// Note: In real tests, you'd mock the auth package or use dependency injection
		// For this test, we're verifying the CreateApp call includes dockerAuth when available

		// Start from StateCreatingApp to trigger createApp command
		model.state = StateCreatingApp
		model.zipPath = "/tmp/test.zip"
		model.zipSize = 1024

		// The actual Docker auth inclusion happens in createApp() method
		// which calls auth.GetDockerAuth() internally
		// This test verifies the mock expectation that dockerAuth is included
		
		harness := uitesting.NewTestHarness(t, model)
		harness.
			Finally(uitesting.TestStep[*DeployView]{
				Name: "app_created_with_auth",
				ExpectedMsgType: appCreatedMsg{},
				ModelAssert: func(t *testing.T, m *DeployView) {
					// Verify the mock was called with dockerAuth in payload
					mockClient.AssertExpectations(t)
				},
			}).
			Run(t)
	})

	t.Run("app created without docker auth", func(t *testing.T) {
		mockClient := apimock.NewMockClient(t)

		// Mock CreateApp to verify dockerAuth is NOT included when empty
		mockClient.On("CreateApp", mock.Anything, "test-project", mock.MatchedBy(func(payload map[string]any) bool {
			// Verify dockerAuth is either absent or empty
			dockerAuth, hasAuth := payload["dockerAuth"]
			return !hasAuth || dockerAuth == ""
		})).Return(&api.CreateAppResponse{
			BuildID:          "build-without-auth",
			Status:           "building",
			UploadURL:        "https://s3.amazonaws.com/upload",
			KeyName:          "test-app",
			InternalEndpoint: "https://test-app.internal",
			DashboardURL:     "https://dashboard.cerebrium.ai/app/test-app",
		}, nil).Once()

		config := &projectconfig.ProjectConfig{
			Deployment: projectconfig.DeploymentConfig{
				Name: "test-app-no-auth",
			},
		}

		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    mockClient,
		})

		// Start from StateCreatingApp to trigger createApp command
		model.state = StateCreatingApp
		model.zipPath = "/tmp/test.zip"
		model.zipSize = 1024

		// When auth.GetDockerAuth() returns empty, dockerAuth should not be in payload
		
		harness := uitesting.NewTestHarness(t, model)
		harness.
			Finally(uitesting.TestStep[*DeployView]{
				Name: "app_created_without_auth",
				ExpectedMsgType: appCreatedMsg{},
				ModelAssert: func(t *testing.T, m *DeployView) {
					// Verify the mock was called without dockerAuth in payload
					mockClient.AssertExpectations(t)
				},
			}).
			Run(t)
	})
}

func TestDeployView_GetError(t *testing.T) {
	config := &projectconfig.ProjectConfig{
		Deployment: projectconfig.DeploymentConfig{
			Name: "error-test",
		},
	}

	t.Run("returns nil when no error", func(t *testing.T) {
		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{IsInteractive: true},
			Config:        config,
			ProjectID:     "test-project",
			Client:        apimock.NewMockClient(t),
		})

		assert.Nil(t, model.GetError())
	})

	t.Run("returns error when set", func(t *testing.T) {
		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{IsInteractive: true},
			Config:        config,
			ProjectID:     "test-project",
			Client:        apimock.NewMockClient(t),
		})

		testErr := ui.NewAPIError(errors.New("test error"))
		model.err = testErr

		assert.Equal(t, testErr, model.GetError())
	})
}

func TestDeployView_View(t *testing.T) {

	config := &projectconfig.ProjectConfig{
		Deployment: projectconfig.DeploymentConfig{
			Name: "view-test",
		},
	}

	t.Run("view during loading files", func(t *testing.T) {
		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    apimock.NewMockClient(t),
		})

		model.state = StateLoadingFiles

		view := model.View()
		assert.Contains(t, view, "Loading files")
	})

	t.Run("view during zipping", func(t *testing.T) {
		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    apimock.NewMockClient(t),
		})

		model.state = StateZippingFiles
		model.fileList = []string{"file1.py", "file2.py"}

		view := model.View()
		assert.Contains(t, view, "Zipping files")
		assert.Contains(t, view, "2 files")
	})

	t.Run("view during app creation", func(t *testing.T) {
		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    apimock.NewMockClient(t),
		})

		model.state = StateCreatingApp
		model.zipSize = 2048000

		view := model.View()
		assert.Contains(t, view, "Creating app")
	})

	t.Run("view during upload", func(t *testing.T) {
		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    apimock.NewMockClient(t),
		})

		model.state = StateUploadingZip
		model.zipSize = 5242880 // 5MB

		view := model.View()
		assert.Contains(t, view, "Uploading")
	})

	t.Run("view during build", func(t *testing.T) {
		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    apimock.NewMockClient(t),
		})

		model.state = StateBuildingApp
		model.buildID = "build-123"

		view := model.View()
		assert.Contains(t, view, "Building")
	})

	t.Run("view on error", func(t *testing.T) {
		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    apimock.NewMockClient(t),
		})

		model.state = StateDeployError
		model.err = ui.NewAPIError(errors.New("deployment failed"))

		view := model.View()
		assert.Contains(t, view, "deployment failed")
	})

	t.Run("view in simple mode", func(t *testing.T) {
		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: true,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    apimock.NewMockClient(t),
		})

		model.state = StateZippingFiles

		view := model.View()
		assert.Empty(t, view, "View should return empty string in simple mode")
	})

	t.Run("view during cancellation", func(t *testing.T) {
		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    apimock.NewMockClient(t),
		})

		model.state = StateCancelling
		model.buildID = "build-cancel"

		view := model.View()
		assert.Contains(t, view, "Cancelling")
	})

	t.Run("view after cancelled", func(t *testing.T) {
		model := NewDeployView(t.Context(), DeployConfig{
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			Config:    config,
			ProjectID: "test-project",
			Client:    apimock.NewMockClient(t),
		})

		model.state = StateCancelled
		model.err = ui.NewUserCancelledError()

		view := model.View()
		// When cancelled without error, view shows "Deployment cancelled"
		assert.Contains(t, view, "cancelled")
	})
}
