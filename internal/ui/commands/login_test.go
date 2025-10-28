package commands

import (
	"errors"
	"github.com/stretchr/testify/mock"
	"testing"

	"github.com/cerebriumai/cerebrium/internal/api"
	apimock "github.com/cerebriumai/cerebrium/internal/api/mock"
	"github.com/cerebriumai/cerebrium/internal/auth"
	"github.com/cerebriumai/cerebrium/internal/ui"
	uitesting "github.com/cerebriumai/cerebrium/internal/ui/testing"
	"github.com/cerebriumai/cerebrium/pkg/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

//go:generate go test -v -run TestLoginView -update

func TestLoginView(t *testing.T) {
	t.Run("initial state", func(t *testing.T) {
		cfg := &config.Config{}

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		// Test the view directly without running Init()
		// (Init() would trigger getDeviceCode which calls real auth API)
		view := model.View()
		assert.Contains(t, view, "Initialising authentication")
		assert.Equal(t, StateInitiating, model.state)
		assert.Nil(t, model.deviceAuth)
		assert.Nil(t, model.token)
	})

	t.Run("device code received", func(t *testing.T) {
		cfg := &config.Config{}

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		// Set to StateInitiating and skip Init() to avoid calling real auth API
		model.state = StateInitiating

		deviceAuth := &auth.DeviceAuthResponse{}
		deviceAuth.DeviceAuthResponsePayload.DeviceCode = "device-123"
		deviceAuth.DeviceAuthResponsePayload.UserCode = "USER-CODE"
		deviceAuth.DeviceAuthResponsePayload.VerificationURI = "https://auth.cerebrium.ai/activate"
		deviceAuth.DeviceAuthResponsePayload.VerificationURIComplete = "https://auth.cerebrium.ai/activate?code=USER-CODE"
		deviceAuth.DeviceAuthResponsePayload.ExpiresIn = 900

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Finally(uitesting.TestStep[*LoginView]{
				Name:            "device_code_received",
				Msg:             deviceCodeMsg{deviceAuth: deviceAuth},
				ExpectedMsgType: deviceCodeMsg{},
				ViewGolden:      "login_waiting_for_auth",
				ModelAssert: func(t *testing.T, m *LoginView) {
					assert.Equal(t, StateWaitingForAuth, m.state)
					assert.NotNil(t, m.deviceAuth)
					assert.Equal(t, "device-123", m.deviceAuth.DeviceAuthResponsePayload.DeviceCode)
				},
			}).
			Run(t)
	})

	t.Run("token received", func(t *testing.T) {
		cfg := &config.Config{}

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateWaitingForAuth
		model.deviceAuth = &auth.DeviceAuthResponse{}
		model.deviceAuth.DeviceAuthResponsePayload.VerificationURIComplete = "https://auth.cerebrium.ai/activate?code=USER-CODE"

		token := &auth.TokenResponse{
			AccessToken:  "access-token-abc",
			RefreshToken: "refresh-token-xyz",
		}

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Finally(uitesting.TestStep[*LoginView]{
				Name:            "token_received",
				Msg:             tokenReceivedMsg{token: token},
				ExpectedMsgType: tokenReceivedMsg{},
				ViewGolden:      "login_saving_token",
				ModelAssert: func(t *testing.T, m *LoginView) {
					assert.Equal(t, StateSavingToken, m.state)
					assert.NotNil(t, m.token)
					assert.Equal(t, "access-token-abc", m.token.AccessToken)
				},
			}).
			Run(t)
	})

	t.Run("token saved", func(t *testing.T) {
		cfg := &config.Config{}

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateSavingToken
		model.token = &auth.TokenResponse{
			AccessToken:  "saved-token",
			RefreshToken: "saved-refresh",
		}

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Finally(uitesting.TestStep[*LoginView]{
				Name:            "token_saved",
				Msg:             tokenSavedMsg{},
				ExpectedMsgType: tokenSavedMsg{},
				ViewGolden:      "login_setting_project",
				ModelAssert: func(t *testing.T, m *LoginView) {
					assert.Equal(t, StateSettingProject, m.state)
				},
			}).
			Run(t)
	})

	t.Run("project context set", func(t *testing.T) {
		// Set environment to prod so we don't see "Logging in to dev" in golden files
		t.Setenv("CEREBRIUM_ENV", "prod")

		cfg := &config.Config{}

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateSettingProject

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LoginView]{
				Name: "project_set",
				Msg: projectSetMsg{
					message: "Current project context set to ID: project-123",
				},
				ViewGolden: "login_success",
				ModelAssert: func(t *testing.T, m *LoginView) {
					assert.Equal(t, StateSuccess, m.state)
					assert.Contains(t, m.message, "project-123")
				},
			}).
			Run(t)
	})

	t.Run("error during auth", func(t *testing.T) {
		// Set environment to prod so we don't see "Logging in to dev" in golden files
		t.Setenv("CEREBRIUM_ENV", "prod")

		cfg := &config.Config{}

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LoginView]{
				Name:       "auth_error",
				Msg:        ui.NewAPIError(errors.New("authentication failed: invalid credentials")),
				ViewGolden: "login_error",
				ModelAssert: func(t *testing.T, m *LoginView) {
					assert.Equal(t, StateError, m.state)
					assert.NotNil(t, m.err)
				},
			}).
			Run(t)
	})

	t.Run("dev environment shows environment indicator", func(t *testing.T) {
		// Set environment to dev to verify we see "Logging in to dev"
		t.Setenv("CEREBRIUM_ENV", "dev")

		cfg := &config.Config{}

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateSettingProject

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LoginView]{
				Name: "dev_environment",
				Msg: projectSetMsg{
					message: "Current project context set to ID: project-123",
				},
				ViewGolden: "login_success_dev",
				ModelAssert: func(t *testing.T, m *LoginView) {
					assert.Equal(t, StateSuccess, m.state)
					assert.Contains(t, m.message, "project-123")
				},
			}).
			Run(t)
	})

	t.Run("signal cancel", func(t *testing.T) {
		cfg := &config.Config{}

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LoginView]{
				Name: "cancel_signal",
				Msg:  ui.SignalCancelMsg{},
				ViewAssert: func(t *testing.T, view string) {
					// Should still show initial state
					uitesting.AssertContains(t, view, "Initialising")
				},
			}).
			Run(t)
	})

	t.Run("keyboard ctrl+c", func(t *testing.T) {
		cfg := &config.Config{}

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateWaitingForAuth

		// Test Update() directly
		_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

		// Should quit
		assert.NotNil(t, cmd)
	})

	t.Run("keyboard q", func(t *testing.T) {
		cfg := &config.Config{}

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateWaitingForAuth

		// Test Update() directly
		_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

		// Should quit
		assert.NotNil(t, cmd)
	})

	t.Run("simple mode - success flow", func(t *testing.T) {
		cfg := &config.Config{}

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: true,
			},
		})

		model.state = StateSettingProject

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LoginView]{
				Name: "simple_mode_success",
				Msg: projectSetMsg{
					message: "Login successful!",
				},
				ViewAssert: func(t *testing.T, view string) {
					// Simple mode returns empty view
					assert.Empty(t, view)
				},
				ModelAssert: func(t *testing.T, m *LoginView) {
					assert.Equal(t, StateSuccess, m.state)
				},
			}).
			Run(t)
	})

	t.Run("simple mode - error", func(t *testing.T) {
		cfg := &config.Config{}

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: true,
			},
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LoginView]{
				Name: "simple_mode_error",
				Msg:  ui.NewAPIError(errors.New("login failed")),
				ViewAssert: func(t *testing.T, view string) {
					// Simple mode returns empty view
					assert.Empty(t, view)
				},
				ModelAssert: func(t *testing.T, m *LoginView) {
					assert.Equal(t, StateError, m.state)
					assert.NotNil(t, m.err)
				},
			}).
			Run(t)
	})
}

func TestLoginView_View(t *testing.T) {
	cfg := &config.Config{}

	t.Run("view during initiating", func(t *testing.T) {
		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateInitiating

		view := model.View()
		assert.Contains(t, view, "Initialising authentication")
	})

	t.Run("view during waiting for auth", func(t *testing.T) {
		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateWaitingForAuth
		model.deviceAuth = &auth.DeviceAuthResponse{}
		model.deviceAuth.DeviceAuthResponsePayload.VerificationURIComplete = "https://auth.cerebrium.ai/activate?code=TEST"

		view := model.View()
		assert.Contains(t, view, "Waiting for credentials")
		assert.Contains(t, view, "https://auth.cerebrium.ai/activate?code=TEST")
	})

	t.Run("view during saving token", func(t *testing.T) {
		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateSavingToken

		view := model.View()
		assert.Contains(t, view, "Saving auth token")
	})

	t.Run("view during setting project", func(t *testing.T) {
		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateSettingProject

		view := model.View()
		assert.Contains(t, view, "Setting project context")
	})

	t.Run("view on success", func(t *testing.T) {
		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateSuccess
		model.message = "Login successful! Project ID: test-123"

		view := model.View()
		assert.Contains(t, view, "Set project context")
		assert.Contains(t, view, "Login successful")
		assert.Contains(t, view, "test-123")
	})

	t.Run("view on error", func(t *testing.T) {
		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateError
		model.err = ui.NewAPIError(errors.New("authentication failed"))

		view := model.View()
		assert.Contains(t, view, "authentication failed")
	})

	t.Run("view in simple mode", func(t *testing.T) {
		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: true,
			},
		})

		model.state = StateWaitingForAuth

		view := model.View()
		assert.Empty(t, view, "View should return empty string in simple mode")
	})
}

func TestLoginView_GetError(t *testing.T) {
	cfg := &config.Config{}

	t.Run("returns nil when no error", func(t *testing.T) {
		model := NewLoginView(t.Context(), LoginConfig{
			Config:        cfg,
			DisplayConfig: ui.DisplayConfig{IsInteractive: true},
		})

		assert.Nil(t, model.GetError())
	})

	t.Run("returns error when set", func(t *testing.T) {
		model := NewLoginView(t.Context(), LoginConfig{
			Config:        cfg,
			DisplayConfig: ui.DisplayConfig{IsInteractive: true},
		})

		testErr := ui.NewAPIError(errors.New("test error"))
		model.err = testErr

		assert.Equal(t, testErr, model.GetError())
	})
}

func TestLoginView_StateTransitions(t *testing.T) {
	cfg := &config.Config{}

	t.Run("state transitions step by step", func(t *testing.T) {
		// Test each transition individually with Finally to avoid async chains

		t.Run("initiating to waiting", func(t *testing.T) {
			model := NewLoginView(t.Context(), LoginConfig{
				Config: cfg,
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
			})

			da := &auth.DeviceAuthResponse{}
			da.DeviceAuthResponsePayload.DeviceCode = "dev-code"

			harness := uitesting.NewTestHarness(t, model)
			harness.
				Finally(uitesting.TestStep[*LoginView]{
					Name:            "device_code",
					Msg:             deviceCodeMsg{deviceAuth: da},
					ExpectedMsgType: deviceCodeMsg{},
					ModelAssert: func(t *testing.T, m *LoginView) {
						assert.Equal(t, StateWaitingForAuth, m.state)
					},
				}).
				Run(t)
		})

		t.Run("waiting to saving", func(t *testing.T) {
			model := NewLoginView(t.Context(), LoginConfig{
				Config: cfg,
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
			})
			model.state = StateWaitingForAuth

			harness := uitesting.NewTestHarness(t, model)
			harness.
				Finally(uitesting.TestStep[*LoginView]{
					Name: "token_received",
					Msg: tokenReceivedMsg{
						token: &auth.TokenResponse{
							AccessToken:  "access",
							RefreshToken: "refresh",
						},
					},
					ExpectedMsgType: tokenReceivedMsg{},
					ModelAssert: func(t *testing.T, m *LoginView) {
						assert.Equal(t, StateSavingToken, m.state)
					},
				}).
				Run(t)
		})

		t.Run("saving to setting project", func(t *testing.T) {
			model := NewLoginView(t.Context(), LoginConfig{
				Config: cfg,
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
			})
			model.state = StateSavingToken

			harness := uitesting.NewTestHarness(t, model)
			harness.
				Finally(uitesting.TestStep[*LoginView]{
					Name:            "token_saved",
					Msg:             tokenSavedMsg{},
					ExpectedMsgType: tokenSavedMsg{},
					ModelAssert: func(t *testing.T, m *LoginView) {
						assert.Equal(t, StateSettingProject, m.state)
					},
				}).
				Run(t)
		})

		t.Run("setting project to success", func(t *testing.T) {
			model := NewLoginView(t.Context(), LoginConfig{
				Config: cfg,
				DisplayConfig: ui.DisplayConfig{
					IsInteractive:    true,
					DisableAnimation: false,
				},
			})
			model.state = StateSettingProject

			harness := uitesting.NewTestHarness(t, model)
			harness.
				Step(uitesting.TestStep[*LoginView]{
					Name: "project_set",
					Msg: projectSetMsg{
						message: "Success!",
					},
					ModelAssert: func(t *testing.T, m *LoginView) {
						assert.Equal(t, StateSuccess, m.state)
						assert.Equal(t, "Success!", m.message)
					},
				}).
				Run(t)
		})
	})

	t.Run("error during device code", func(t *testing.T) {
		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LoginView]{
				Name: "device_code_error",
				Msg:  ui.NewAPIError(errors.New("failed to get device code")),
				ModelAssert: func(t *testing.T, m *LoginView) {
					assert.Equal(t, StateError, m.state)
					assert.NotNil(t, m.err)
				},
			}).
			Run(t)
	})

	t.Run("error during token polling", func(t *testing.T) {
		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateWaitingForAuth

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LoginView]{
				Name: "polling_error",
				Msg:  ui.NewAPIError(errors.New("authentication timeout")),
				ModelAssert: func(t *testing.T, m *LoginView) {
					assert.Equal(t, StateError, m.state)
					assert.NotNil(t, m.err)
				},
			}).
			Run(t)
	})

	t.Run("error during token save", func(t *testing.T) {
		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateSavingToken

		harness := uitesting.NewTestHarness(t, model)
		harness.
			Step(uitesting.TestStep[*LoginView]{
				Name: "save_error",
				Msg:  ui.NewConfigurationError(errors.New("failed to save config")),
				ModelAssert: func(t *testing.T, m *LoginView) {
					assert.Equal(t, StateError, m.state)
					assert.NotNil(t, m.err)
				},
			}).
			Run(t)
	})
}

func TestLoginView_WithClient(t *testing.T) {
	t.Run("uses client from config when provided", func(t *testing.T) {
		cfg := &config.Config{
			// Set an existing project ID to avoid config.Save() being called
			ProjectID: "existing-project",
		}
		mockClient := apimock.NewMockClient(t)

		// Mock GetProjects to return test projects including the existing one
		mockClient.EXPECT().
			GetProjects(mock.Anything).
			Return([]api.Project{
				{ID: "existing-project", Name: "Existing Project"},
				{ID: "project-123", Name: "Test Project"},
			}, nil).
			Once()

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			Client: mockClient,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		// Manually set state and call setProjectContext
		model.state = StateSettingProject
		model.token = &auth.TokenResponse{
			AccessToken:  "test-access-token",
			RefreshToken: "test-refresh-token",
		}
		cfg.AccessToken = model.token.AccessToken
		cfg.RefreshToken = model.token.RefreshToken

		// Call setProjectContext which should use the mocked client
		msg := model.setProjectContext()

		// Verify the message is correct
		projectMsg, ok := msg.(projectSetMsg)
		assert.True(t, ok, "Expected projectSetMsg")
		assert.Contains(t, projectMsg.message, "existing-project")

		// Verify the mock was called
		mockClient.AssertExpectations(t)
	})

	t.Run("handles nil client gracefully", func(t *testing.T) {
		cfg := &config.Config{}

		model := NewLoginView(t.Context(), LoginConfig{
			Config: cfg,
			Client: nil, // No client provided
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateSettingProject

		// Call setProjectContext which should handle nil client gracefully
		msg := model.setProjectContext()

		// Should return empty projectSetMsg
		projectMsg, ok := msg.(projectSetMsg)
		assert.True(t, ok, "Expected projectSetMsg")
		assert.Equal(t, "", projectMsg.message)
	})

	t.Run("handles GetProjects error gracefully", func(t *testing.T) {
		ctx := t.Context()
		cfg := &config.Config{}
		mockClient := apimock.NewMockClient(t)

		// Mock GetProjects to return error
		mockClient.EXPECT().
			GetProjects(mock.Anything).
			Return(nil, errors.New("network error")).
			Once()

		model := NewLoginView(ctx, LoginConfig{
			Config: cfg,
			Client: mockClient,
			DisplayConfig: ui.DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
		})

		model.state = StateSettingProject

		// Call setProjectContext which should handle error gracefully
		msg := model.setProjectContext()

		// Should return empty projectSetMsg (error is non-fatal)
		projectMsg, ok := msg.(projectSetMsg)
		assert.True(t, ok, "Expected projectSetMsg")
		assert.Equal(t, "", projectMsg.message)

		// Verify the mock was called
		mockClient.AssertExpectations(t)
	})
}
