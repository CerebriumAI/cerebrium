package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/auth"
	"github.com/cerebriumai/cerebrium/internal/ui"
	cerebriumBugsnag "github.com/cerebriumai/cerebrium/pkg/bugsnag"
	"github.com/cerebriumai/cerebrium/pkg/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// LoginState represents the current state of the login flow
type LoginState int

const (
	StateInitiating LoginState = iota
	StateWaitingForAuth
	StateSavingToken
	StateSettingProject
	StateSuccess
	StateError
)

type LoginConfig struct {
	ui.DisplayConfig

	Config *config.Config
	Client api.Client
}

// LoginView is the Bubbletea model for the login flow
type LoginView struct {
	ctx     context.Context
	state   LoginState
	spinner *ui.SpinnerModel
	err     error
	message string

	// Auth data
	deviceAuth *auth.DeviceAuthResponse
	token      *auth.TokenResponse

	conf LoginConfig
}

// NewLoginView creates a new login view
func NewLoginView(ctx context.Context, conf LoginConfig) *LoginView {
	return &LoginView{
		ctx:     ctx,
		state:   StateInitiating,
		spinner: ui.NewSpinner(),
		conf:    conf,
	}
}

// Init starts the login flow

// Error returns the error if any occurred during execution
func (m *LoginView) Error() error {
	return m.err
}

func (m *LoginView) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Init(),
		m.getDeviceCode,
	)
}

// Update handles messages
func (m *LoginView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.SignalCancelMsg:
		// Handle termination signal (SIGINT, SIGTERM)
		// Just exit cleanly - login doesn't need cleanup
		return m, tea.Quit

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}

	case deviceCodeMsg:
		m.deviceAuth = msg.deviceAuth
		m.state = StateWaitingForAuth

		// Direct output in simple mode
		if m.conf.SimpleOutput() {
			fmt.Println("✓ Initialised authentication")
		}

		// Open browser
		//nolint:staticcheck // Empty branch intentional - browser opening is best-effort, user can open manually
		if err := auth.OpenBrowser(m.ctx, m.deviceAuth.DeviceAuthResponsePayload.VerificationURIComplete); err != nil {
			// Non-fatal, user can still open manually
		}

		// Show URL in simple mode
		if m.conf.SimpleOutput() {
			fmt.Println("\nClick here if your browser does not automatically open:")
			fmt.Println(m.deviceAuth.DeviceAuthResponsePayload.VerificationURIComplete)
			fmt.Println("\nWaiting for credentials...")
		}

		// Start polling for token
		return m, m.pollForToken

	case tokenReceivedMsg:
		m.token = msg.token
		m.state = StateSavingToken

		if m.conf.SimpleOutput() {
			fmt.Println("✓ Received credentials")
			fmt.Println("Saving auth token...")
		}

		return m, m.saveToken

	case tokenSavedMsg:
		m.state = StateSettingProject

		if m.conf.SimpleOutput() {
			fmt.Println("✓ Saved auth token")
			fmt.Println("Setting project context...")
		}

		return m, m.setProjectContext

	case projectSetMsg:
		m.state = StateSuccess
		m.message = msg.message

		if m.conf.SimpleOutput() {
			fmt.Println("✓ Set project context")
			if msg.message != "" {
				fmt.Println()
				fmt.Println(msg.message)
			}
		}

		return m, tea.Quit

	case *ui.UIError:
		msg.SilentExit = true // Will be shown in View()
		m.err = msg
		m.state = StateError

		// Report login errors to Bugsnag
		// Don't report user cancellations
		if msg.Type != ui.ErrorTypeUserCancelled && !cerebriumBugsnag.IsUserCancellation(msg.Err) {
			metadata := bugsnag.MetaData{
				"login": {
					"error_type": fmt.Sprintf("%d", msg.Type),
					"state":      fmt.Sprintf("%d", m.state),
				},
			}

			if msg.Type == ui.ErrorTypeValidation {
				cerebriumBugsnag.NotifyWithMetadata(msg.Err, bugsnag.SeverityWarning, metadata, m.ctx)
			} else {
				cerebriumBugsnag.NotifyWithMetadata(msg.Err, bugsnag.SeverityError, metadata, m.ctx)
			}
		}

		if m.conf.SimpleOutput() {
			fmt.Printf("Error: %s\n", msg.Error())
		}

		return m, tea.Quit

	default:
		// Update spinner only in interactive mode
		if !m.conf.SimpleOutput() {
			var cmd tea.Cmd
			spinnerModel, cmd := m.spinner.Update(msg)
			m.spinner = spinnerModel.(*ui.SpinnerModel) //nolint:errcheck // Type assertion guaranteed by SpinnerModel structure
			return m, cmd
		}
	}

	return m, nil
}

// View renders the output
func (m *LoginView) View() string {
	// Simple mode: output has already been printed directly
	if m.conf.SimpleOutput() {
		return ""
	}

	// Interactive mode: full UI experience
	var output strings.Builder
	if config.GetEnvironment() == config.EnvDev || config.GetEnvironment() == config.EnvLocal {
		// Show environment indicator if not prod
		output.WriteString(strings.TrimSpace(
			ui.PendingStyle.Bold(true).Render(fmt.Sprintf("Logging in to %s\n", config.GetEnvironment()))))
	}

	// Helper function to format state line
	formatStateLine := func(icon string, text string, style lipgloss.Style) string {
		return fmt.Sprintf("%s  %s", icon, style.Render(text))
	}

	// State 1: Initializing
	switch {
	case m.state == StateInitiating:
		output.WriteString(formatStateLine(m.spinner.View(), "Initialising authentication...", ui.ActiveStyle))
	case m.state > StateInitiating:
		output.WriteString(formatStateLine("✓", "Initialised authentication", ui.SuccessStyle))
	default:
		output.WriteString(formatStateLine("-", "Initialise authentication", ui.PendingStyle))
	}
	output.WriteString("\n")

	// State 2: Waiting for auth
	switch {
	case m.state == StateWaitingForAuth:
		output.WriteString(formatStateLine(m.spinner.View(), "Waiting for credentials...", ui.ActiveStyle))
	case m.state > StateWaitingForAuth:
		output.WriteString(formatStateLine("✓", "Received credentials", ui.SuccessStyle))
	default:
		output.WriteString(formatStateLine("-", "Waiting for credentials", ui.PendingStyle))
	}
	output.WriteString("\n")

	// State 3: Saving token
	switch {
	case m.state == StateSavingToken:
		output.WriteString(formatStateLine(m.spinner.View(), "Saving auth token...", ui.ActiveStyle))
	case m.state > StateSavingToken:
		output.WriteString(formatStateLine("✓", "Saved auth token", ui.SuccessStyle))
	default:
		output.WriteString(formatStateLine("-", "Saving auth token", ui.PendingStyle))
	}
	output.WriteString("\n")

	// State 4: Setting project context
	switch {
	case m.state == StateSettingProject:
		output.WriteString(formatStateLine(m.spinner.View(), "Setting project context...", ui.ActiveStyle))
	case m.state > StateSettingProject:
		output.WriteString(formatStateLine("✓", "Set project context", ui.SuccessStyle))
	default:
		output.WriteString(formatStateLine("-", "Setting project context", ui.PendingStyle))
	}
	output.WriteString("\n")

	// Add URL display when waiting for auth
	if m.state == StateWaitingForAuth && m.deviceAuth != nil {
		output.WriteString("\n")
		output.WriteString("Click here if your browser does not automatically open:\n")
		output.WriteString(ui.URLStyle.Render(m.deviceAuth.DeviceAuthResponsePayload.VerificationURIComplete))
		output.WriteString("\n")
	}

	// Show success message
	if m.state == StateSuccess && m.message != "" {
		output.WriteString("\n")
		output.WriteString(m.message)
		output.WriteString("\n")
	}

	// Show error
	if m.state == StateError {
		output.WriteString("\n")
		output.WriteString(ui.FormatError(m.err))
	}

	return output.String()
}

// Messages
type deviceCodeMsg struct {
	deviceAuth *auth.DeviceAuthResponse
}

type tokenReceivedMsg struct {
	token *auth.TokenResponse
}

type tokenSavedMsg struct{}

type projectSetMsg struct {
	message string
}

// Commands (async operations)

func (m *LoginView) getDeviceCode() tea.Msg {
	envConfig := m.conf.Config.GetEnvConfig()

	deviceAuth, err := auth.RequestDeviceCode(m.ctx, envConfig.APIV1Url)
	if err != nil {
		return ui.NewAPIError(fmt.Errorf("failed to get device code: %w", err))
	}

	return deviceCodeMsg{deviceAuth: deviceAuth}
}

func (m *LoginView) pollForToken() tea.Msg {
	envConfig := m.conf.Config.GetEnvConfig()
	token, err := auth.PollForToken(m.ctx, envConfig.APIV1Url, m.deviceAuth.DeviceAuthResponsePayload.DeviceCode)
	if err != nil {
		return ui.NewAPIError(fmt.Errorf("authentication failed: %w", err))
	}

	return tokenReceivedMsg{token: token}
}

func (m *LoginView) saveToken() tea.Msg {
	m.conf.Config.AccessToken = m.token.AccessToken
	m.conf.Config.RefreshToken = m.token.RefreshToken

	if err := config.Save(m.conf.Config); err != nil {
		return ui.NewConfigurationError(fmt.Errorf("failed to save configuration: %w", err))
	}

	return tokenSavedMsg{}
}

func (m *LoginView) setProjectContext() tea.Msg {
	// Use client from config
	if m.conf.Client == nil {
		// Non-fatal, just return success
		return projectSetMsg{}
	}

	// Get projects
	ctx := context.Background()
	projects, err := m.conf.Client.GetProjects(ctx)
	if err != nil {
		// Non-fatal, just return success
		return projectSetMsg{}
	}

	if len(projects) == 0 {
		return projectSetMsg{}
	}

	currentProjectID := m.conf.Config.ProjectID

	// Check if current project still exists
	projectExists := false
	for _, p := range projects {
		if p.ID == currentProjectID {
			projectExists = true
			break
		}
	}

	var message string
	if currentProjectID != "" && projectExists {
		message = fmt.Sprintf("Using existing project context ID: %s", currentProjectID)
	} else {
		// Use first project as default
		m.conf.Config.ProjectID = projects[0].ID
		if err := config.Save(m.conf.Config); err != nil {
			// Non-fatal
			return projectSetMsg{}
		}

		if currentProjectID != "" {
			message = fmt.Sprintf("Updated project context to ID: %s (previous project not found)", m.conf.Config.ProjectID)
		} else {
			message = fmt.Sprintf("Current project context set to ID: %s", m.conf.Config.ProjectID)
		}
	}

	// Set default region if not set
	if m.conf.Config.DefaultRegion == "" {
		m.conf.Config.DefaultRegion = "us-east-1"
		//nolint:errcheck,gosec // Best effort save of default region, error not actionable
		config.Save(m.conf.Config)
	}

	return projectSetMsg{message: message}
}

// GetError returns any error that occurred during login
func (m *LoginView) GetError() error {
	return m.err
}
