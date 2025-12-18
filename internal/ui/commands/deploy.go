package commands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bugsnag/bugsnag-go/v2"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/auth"
	"github.com/cerebriumai/cerebrium/internal/files"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/internal/ui/logging"
	"github.com/cerebriumai/cerebrium/internal/version"
	"github.com/cerebriumai/cerebrium/internal/wsapi"
	cerebrium_bugsnag "github.com/cerebriumai/cerebrium/pkg/bugsnag"
	"github.com/cerebriumai/cerebrium/pkg/projectconfig"
)

// DeployState represents the current state of the deployment
type DeployState int

const (
	StateConfirmation DeployState = iota
	StateLoadingFiles
	StateZippingFiles
	StateCreatingApp
	StateUploadingZip
	StateBuildingApp
	StateDrainingLogs // Waiting for remaining logs to arrive after build completes
	StateCancelling
	StateCancelled
	StateDeploySuccess
	StateDeployError
)

// DeployConfig contains deployment configuration
type DeployConfig struct {
	ui.DisplayConfig

	Config    *projectconfig.ProjectConfig
	ProjectID string
	Client    api.Client
	WSClient  wsapi.Client

	// Display config
	DisableBuildLogs    bool
	DisableConfirmation bool
	LogLevel            string
	Detach              bool // Exit after upload without waiting for build completion
}

// DeployView is the Bubbletea model for the deployment flow
type DeployView struct {
	ctx       context.Context
	ctxCancel context.CancelFunc

	state DeployState

	// State data
	isPartnerDeploy bool // Track if this is a partner app deployment
	fileList        []string
	zipPath         string
	zipSize         int64
	buildID         string
	appResponse     *api.CreateAppResponse
	logViewer       *logging.LogViewerModel
	idleMsgIdx      int
	buildStatus     string
	spinner         *ui.SpinnerModel
	message         string
	err             *ui.UIError

	// Upload progress tracking
	progressBar         progress.Model
	uploadedBytes       int64
	atomicBytesUploaded *atomic.Int64
	uploadStartTime     time.Time
	uploadSpeed         float64 // Cached upload speed in bytes/sec
	lastPrintedPercent  int     // Track last printed percentage for SimpleOutput

	// Viewport for scrollable confirmation screen
	viewport      viewport.Model
	viewportReady bool

	conf DeployConfig
}

// NewDeployView creates a new deploy view
func NewDeployView(ctx context.Context, conf DeployConfig) *DeployView {
	initialState := StateConfirmation
	isPartnerDeploy := conf.Config.PartnerService != nil

	if conf.DisableConfirmation {
		if isPartnerDeploy {
			initialState = StateCreatingApp
		} else {
			initialState = StateLoadingFiles
		}
	}

	prog := progress.New(
		progress.WithSolidFill("#EB3A6F"),
		progress.WithWidth(50),
		progress.WithoutPercentage(),
	)
	ctx, cancel := context.WithCancel(ctx)

	return &DeployView{
		ctx:             ctx,
		ctxCancel:       cancel,
		state:           initialState,
		isPartnerDeploy: isPartnerDeploy,
		spinner:         ui.NewSpinner(),
		progressBar:     prog,
		atomicBytesUploaded: &atomic.Int64{},
		conf:            conf,
	}
}

// Init starts the deployment flow

// Error returns the error if any occurred during execution
func (m *DeployView) Error() error {
	return m.err
}

// isPartnerService returns true if this is a partner service deployment
// (runtime is not cortex or custom) - these don't require file upload
func (m *DeployView) isPartnerService() bool {
	return m.isPartnerDeploy
}

func (m *DeployView) Init() tea.Cmd {
	if m.state == StateConfirmation {
		// In non-TTY mode, show confirmation prompt and wait for input
		if m.conf.SimpleOutput() {
			m.showDeploymentSummary()
			return m.waitForConfirmation
		}
		// No async operations for confirmation state in TTY mode
		return nil
	}

	// Confirmation disabled - start appropriate flow
	if m.isPartnerService() {
		// Partner services skip file loading/zipping, go directly to create app
		return tea.Batch(
			m.spinner.Init(),
			m.createApp,
		)
	}

	// Standard flow: start loading files
	return tea.Batch(
		m.spinner.Init(),
		m.loadFiles,
	)
}

// Update handles messages
func (m *DeployView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		viewportHeight := max(msg.Height-2, 1) // Reserve 2 lines for prompt

		if !m.viewportReady {
			// Viewport initialized here because terminal size is only known after
			// Bubbletea sends the first WindowSizeMsg (standard Bubbletea pattern)
			m.viewport = viewport.New(msg.Width, viewportHeight)
			m.viewportReady = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = viewportHeight
		}

		if m.state == StateConfirmation {
			m.viewport.SetContent(m.renderDeploymentSummary())
		}

		return m, nil

	case tea.KeyMsg:
		return m.onKey(msg)

	case tea.MouseMsg:
		if m.state == StateConfirmation && m.viewportReady {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		return m, nil

	case ui.SignalCancelMsg:
		// Handle termination signal (SIGINT, SIGTERM)
		// This is especially important for non-TTY environments

		// In detach mode, don't cancel the build - just exit cleanly
		if m.conf.Detach {
			if m.conf.SimpleOutput() {
				fmt.Printf("\nCtrl+C detected. Build continues in detached mode.\n")
				fmt.Println("Check the dashboard for build status.")
			}
			m.err = ui.NewUserCancelledError()
			return m, tea.Quit
		}

		if m.conf.SimpleOutput() {
			fmt.Printf("\nReceived termination signal, cancelling deployment...\n")
		}

		// Clean up if build is in progress
		if m.state >= StateBuildingApp && m.buildID != "" {
			m.state = StateCancelling

			// Keep log viewer expanded when cancelled (in interactive mode)
			err := m.conf.Client.CancelBuild(
				m.ctx,
				m.conf.ProjectID,
				m.conf.Config.Deployment.Name,
				m.buildID,
			)
			if err != nil {
				if m.conf.SimpleOutput() {
					fmt.Printf("⚠️  Warning: Failed to cancel build: %v\n", err)
				}
			} else {
				if m.conf.SimpleOutput() {
					fmt.Println("✓ Deployment cancelled")
				}
			}
			m.state = StateCancelled
		}

		// Exit after cancellation
		m.err = ui.NewUserCancelledError()
		m.ctxCancel() // Stop all subprocesses
		return m, tea.Quit

	case filesLoadedMsg:
		m.fileList = msg.fileList
		m.state = StateZippingFiles

		// Direct output in simple mode
		if m.conf.SimpleOutput() {
			fmt.Printf("✓ Loaded %d files\n", len(msg.fileList))
			return m, m.zipFiles
		}

		// Print completed step to scrollback in interactive mode
		return m, tea.Batch(
			tea.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓  Loaded %d files", len(msg.fileList)))),
			m.zipFiles,
		)

	case filesZippedMsg:
		m.zipPath = msg.zipPath
		m.zipSize = msg.zipSize
		m.state = StateCreatingApp

		if m.conf.SimpleOutput() {
			fmt.Printf("✓ Created zip (%s)\n", ui.FormatSize(msg.zipSize))
			return m, m.createApp
		}

		// Print completed step to scrollback in interactive mode
		return m, tea.Batch(
			tea.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓  Zipped files (%s)", ui.FormatSize(msg.zipSize)))),
			m.createApp,
		)

	case appCreatedMsg:
		m.appResponse = msg.response
		m.buildID = msg.response.BuildID
		m.buildStatus = msg.response.Status

		// Partner services skip upload - go directly to build monitoring
		if m.isPartnerDeploy {
			m.state = StateBuildingApp

			if m.conf.SimpleOutput() {
				fmt.Printf("✓ Created app (Build ID: %s)\n", msg.response.BuildID)
			}

			// Handle detach mode
			if m.conf.Detach {
				if m.conf.SimpleOutput() {
					fmt.Println("✓ Build started in detached mode")
					fmt.Printf("  Build ID: %s\n", m.buildID)
					fmt.Println("  Check the dashboard for build status.")
				} else {
					return m, tea.Sequence(
						tea.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓  Created app (Build ID: %s)", msg.response.BuildID))),
						tea.Println(ui.SuccessStyle.Render("✓  Build started in detached mode")),
						tea.Println(fmt.Sprintf("   Build ID: %s", m.buildID)),
						tea.Println("   Check the dashboard for build status."),
						tea.Quit,
					)
				}
				m.state = StateDeploySuccess
				return m, tea.Quit
			}

			if m.conf.SimpleOutput() {
				fmt.Println("Building app...")
			}

			// Initialize log viewer with streaming provider
			provider := logging.NewStreamingBuildLogProvider(logging.StreamingBuildLogProviderConfig{
				Client:    m.conf.WSClient,
				ProjectID: m.conf.ProjectID,
				BuildID:   m.buildID,
			})
			tickInterval := 50 * time.Millisecond

			m.logViewer = logging.NewLogViewer(m.ctx, logging.LogViewerConfig{
				DisplayConfig: m.conf.DisplayConfig,
				Provider:      provider,
				TickInterval:  tickInterval,
				ShowHelp:      true,
				AutoExpand:    true,
			})

			if !m.conf.SimpleOutput() {
				printCmds := tea.Sequence(
					tea.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓  Created app (Build ID: %s)", msg.response.BuildID))),
					tea.Println(""),
				)
				return m, tea.Batch(
					printCmds,
					m.logViewer.Init(),
					m.pollBuildStatus,
				)
			}

			return m, tea.Batch(
				m.logViewer.Init(),
				m.pollBuildStatus,
			)
		}

		// Standard flow: upload zip file
		m.state = StateUploadingZip
		m.uploadStartTime = time.Now()
		m.lastPrintedPercent = 0 // Reset progress tracking

		if m.conf.SimpleOutput() {
			fmt.Printf("✓ Created app (Build ID: %s)\n", msg.response.BuildID)
			fmt.Printf("Uploading to Cerebrium (%s)...\n", ui.FormatSize(m.zipSize))
			return m, tea.Batch(
				m.uploadZip,
				m.tickUploadProgress(),
			)
		}

		// Print completed step to scrollback in interactive mode
		return m, tea.Batch(
			tea.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓  Created app (Build ID: %s)", msg.response.BuildID))),
			m.uploadZip,
			m.tickUploadProgress(),
		)

	case uploadProgressTickMsg:
		if m.state == StateUploadingZip && m.atomicBytesUploaded != nil {
			m.uploadedBytes = m.atomicBytesUploaded.Load()

			// Calculate upload speed (only if we have uploaded data)
			if !m.uploadStartTime.IsZero() && m.uploadedBytes > 0 {
				elapsed := time.Since(m.uploadStartTime).Seconds()
				if elapsed > 0 {
					m.uploadSpeed = float64(m.uploadedBytes) / elapsed
				}
			}

			// In SimpleOutput mode, print progress every 10%
			if m.conf.SimpleOutput() && m.zipSize > 0 {
				currentPercent := int((float64(m.uploadedBytes) / float64(m.zipSize)) * 100)
				// Round down to nearest 10%
				percentDecile := (currentPercent / 10) * 10

				// Print if we've crossed a 10% threshold
				if percentDecile > m.lastPrintedPercent && percentDecile <= 100 {
					m.lastPrintedPercent = percentDecile
					m.printSimpleProgress(percentDecile)
				}
			}

			return m, m.tickUploadProgress()
		}
		return m, nil

	case zipUploadedMsg:
		m.state = StateBuildingApp

		if m.conf.SimpleOutput() {
			// Ensure we show 100% if we haven't already
			if m.lastPrintedPercent < 100 {
				m.uploadedBytes = m.zipSize
				m.printSimpleProgress(100)
			}
			fmt.Println("✓ Uploaded to Cerebrium")
		}

		// In detach mode, exit immediately after upload
		if m.conf.Detach {
			if m.conf.SimpleOutput() {
				fmt.Println("✓ Build started in detached mode")
				fmt.Printf("  Build ID: %s\n", m.buildID)
				fmt.Println("  Check the dashboard for build status.")
			} else {
				// Print detach message to scrollback in interactive mode
				return m, tea.Sequence(
					tea.Println(ui.SuccessStyle.Render("✓  Uploaded to Cerebrium")),
					tea.Println(ui.SuccessStyle.Render("✓  Build started in detached mode")),
					tea.Println(fmt.Sprintf("   Build ID: %s", m.buildID)),
					tea.Println("   Check the dashboard for build status."),
					tea.Quit,
				)
			}
			m.state = StateDeploySuccess
			return m, tea.Quit
		}

		if m.conf.SimpleOutput() {
			fmt.Println("Building app...")
		}

		// Initialize log viewer with streaming provider
		provider := logging.NewStreamingBuildLogProvider(logging.StreamingBuildLogProviderConfig{
			Client:    m.conf.WSClient,
			ProjectID: m.conf.ProjectID,
			BuildID:   m.buildID,
		})
		// Overly aggressive ticks will cause the CLI to re-render too often leading to a drop in performance
		tickInterval := 50 * time.Millisecond

		m.logViewer = logging.NewLogViewer(m.ctx, logging.LogViewerConfig{
			DisplayConfig: m.conf.DisplayConfig,
			Provider:      provider,
			TickInterval:  tickInterval,
			ShowHelp:      true,
			AutoExpand:    true, // Show all logs without box for deploy
		})

		// Start polling build status in parallel with log collection
		// Print upload completion to scrollback in interactive mode
		if !m.conf.SimpleOutput() {
			// Use tea.Sequence for ordered prints, then Batch with parallel commands
			printCmds := tea.Sequence(
				tea.Println(ui.SuccessStyle.Render("✓  Uploaded to Cerebrium")),
				tea.Println(""), // Empty line before logs
			)
			return m, tea.Batch(
				printCmds,
				m.logViewer.Init(),
				m.pollBuildStatus,
			)
		}

		return m, tea.Batch(
			m.logViewer.Init(),
			m.pollBuildStatus,
		)

	case buildStatusUpdateMsg:
		// Build status update from polling
		slog.Debug("Build status update", "status", msg.status, "buildId", msg.buildID)
		m.buildStatus = msg.status

		// Check if status is terminal
		if ui.IsTerminalStatus(msg.status) {
			// Terminal status detected, trigger completion
			return m, func() tea.Msg {
				return buildCompleteMsg{status: msg.status}
			}
		}

		// Continue polling if not terminal - schedule next poll
		return m, m.scheduleNextBuildPoll()

	case buildStatusPollErrorMsg:
		// Failed to fetch build status, retry after delay
		return m, m.scheduleNextBuildPoll()

	case buildCompleteMsg:
		// Build is complete, but we need to wait for logs to drain
		// Don't cancel context yet - let log viewer continue fetching
		m.buildStatus = msg.status
		m.state = StateDrainingLogs

		slog.Debug("Build complete, draining logs", "status", msg.status)

		// Wait 2 seconds to allow remaining logs to arrive
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return logDrainCompleteMsg(msg)
		})

	case logDrainCompleteMsg:
		// Logs have had time to drain, now complete the deployment
		m.ctxCancel() // Stop all subprocesses
		m.buildStatus = msg.status
		if msg.status == "success" || msg.status == "ready" {
			m.state = StateDeploySuccess

			if m.conf.SimpleOutput() {
				fmt.Println("✓ Build complete!")
				fmt.Println()
				fmt.Printf("✓ %s is now live!\n", m.conf.Config.Deployment.Name)
				fmt.Println()
				fmt.Printf("App Dashboard: %s\n", m.appResponse.DashboardURL)
				fmt.Println("\nEndpoint:")
				fmt.Printf("POST %s/{function_name}\n", m.appResponse.InternalEndpoint)
				return m, tea.Quit
			}

			// Print success message to scrollback in interactive mode
			return m, tea.Sequence(
				tea.Println(""),
				tea.Println(ui.SuccessStyle.Render("✓  Built app")),
				tea.Println(""),
				tea.Println(ui.GreenStyle.Render(fmt.Sprintf("✓ %s is now live!", m.conf.Config.Deployment.Name))),
				tea.Println(""),
				tea.Println(fmt.Sprintf("App Dashboard: %s", m.appResponse.DashboardURL)),
				tea.Println(""),
				tea.Println("Endpoint:"),
				tea.Println(ui.CyanStyle.Render("POST")+" "+m.appResponse.InternalEndpoint+"/{function_name}"),
				tea.Quit,
			)
		} else {
			m.state = StateDeployError
			err := ui.NewAPIError(fmt.Errorf("build failed with status: %s", msg.status))
			err.SilentExit = true // Will be shown in View()
			m.err = err

			// Report build failure to Bugsnag
			cerebrium_bugsnag.NotifyWithMetadata(
				m.ctx,
				fmt.Errorf("deployment build failed: %s", msg.status),
				bugsnag.SeverityError,
				bugsnag.MetaData{
					"deployment": {
						"build_id":     m.buildID,
						"build_status": msg.status,
						"project_id":   m.conf.ProjectID,
						"app_name":     m.conf.Config.Deployment.Name,
					},
				},
			)

			if m.conf.SimpleOutput() {
				fmt.Printf("✗ Build failed with status: %s\n", msg.status)
				return m, tea.Quit
			}

			// Print error message to scrollback in interactive mode
			return m, tea.Sequence(
				tea.Println(""),
				tea.Println(ui.ErrorStyle.Render(fmt.Sprintf("✗ Build failed with status: %s", msg.status))),
				tea.Quit,
			)
		}

	case confirmationResponseMsg:
		// Handle non-TTY confirmation response
		if msg.confirmed {
			if m.isPartnerService() {
				// Partner services skip file loading/zipping, go directly to create app
				m.state = StateCreatingApp
				return m, tea.Batch(
					m.spinner.Init(),
					m.createApp,
				)
			}
			m.state = StateLoadingFiles
			return m, tea.Batch(
				m.spinner.Init(),
				m.loadFiles,
			)
		}
		// User cancelled
		m.err = ui.NewUserCancelledError()
		return m, tea.Quit

	case buildCancelledMsg:
		// Build cancellation completed (from keyboard shortcut in interactive mode)
		m.ctxCancel() // Stop all subprocesses
		m.state = StateCancelled
		if msg.cancelErr != nil {
			// Show warning but still exit silently
			if m.conf.SimpleOutput() {
				fmt.Printf("⚠️  Warning: Failed to cancel build: %v\n", msg.cancelErr)
			} else {
				m.message = ui.WarningStyle.Render(fmt.Sprintf("⚠️  Warning: Failed to cancel build: %v", msg.cancelErr))
			}
		} else {
			if m.conf.SimpleOutput() {
				fmt.Println("✓ Deployment cancelled")
			}
		}
		m.err = ui.NewUserCancelledError()
		return m, tea.Quit

	case *ui.UIError:
		// Structured error from async operations
		m.ctxCancel()         // Stop all subprocesses
		msg.SilentExit = true // Will be shown below
		m.err = msg
		m.state = StateDeployError

		// Report errors to Bugsnag based on error type
		if msg.Type != ui.ErrorTypeUserCancelled {
			metadata := bugsnag.MetaData{
				"error": {
					"type":       fmt.Sprintf("%d", msg.Type),
					"state":      fmt.Sprintf("%d", m.state),
					"project_id": m.conf.ProjectID,
					"app_name":   m.conf.Config.Deployment.Name,
				},
			}

			if msg.Type == ui.ErrorTypeValidation {
				cerebrium_bugsnag.NotifyWithMetadata(m.ctx, msg.Err, bugsnag.SeverityWarning, metadata)
			} else {
				cerebrium_bugsnag.NotifyWithMetadata(m.ctx, msg.Err, bugsnag.SeverityError, metadata)
			}
		}

		if m.conf.SimpleOutput() {
			fmt.Printf("✗ %s\n", msg.Error())
			return m, tea.Quit
		}

		// Print error message to scrollback in interactive mode
		return m, tea.Sequence(
			tea.Println(""),
			tea.Println(ui.ErrorStyle.Render(fmt.Sprintf("✗ %s", msg.Error()))),
			tea.Quit,
		)

	default:
		// Update spinner only in interactive mode
		var cmds []tea.Cmd
		if !m.conf.SimpleOutput() {
			var spinnerCmd tea.Cmd
			spinnerModel, spinnerCmd := m.spinner.Update(msg)
			m.spinner = spinnerModel.(*ui.SpinnerModel) //nolint:errcheck // Type assertion guaranteed by SpinnerModel structure
			cmds = append(cmds, spinnerCmd)

		}

		// Update log viewer if active (during building or draining logs)
		if m.logViewer != nil && (m.state == StateBuildingApp || m.state == StateDrainingLogs) {
			updated, logCmd := m.logViewer.Update(msg)
			m.logViewer = updated.(*logging.LogViewerModel) //nolint:errcheck // Type assertion guaranteed by LogViewerModel structure

			cmds = append(cmds, logCmd)
		}

		return m, tea.Batch(cmds...)
	}
}

// View renders the output
func (m *DeployView) View() string {
	// Simple mode: output has already been printed directly
	if m.conf.SimpleOutput() {
		return ""
	}

	// Interactive mode: only render the ACTIVE state
	// Completed states are printed to scrollback via tea.Println in Update()
	var output strings.Builder

	if m.state == StateConfirmation {
		if m.viewportReady {
			output.WriteString(m.viewport.View())
		} else {
			output.WriteString(m.renderDeploymentSummary())
		}
		output.WriteString("\n")
		output.WriteString(ui.YellowStyle.Render("Do you want to deploy? (Y/n): "))
		return output.String()
	}

	// Helper function to format state line
	formatStateLine := func(icon string, text string, styleFunc func(...string) string) string {
		return fmt.Sprintf("%s  %s", icon, styleFunc(text))
	}

	// Show active state and pending states (completed states are printed via tea.Println)
	switch m.state {
	case StateLoadingFiles:
		output.WriteString(formatStateLine(m.spinner.View(), "Loading files...", ui.ActiveStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Zip files", ui.PendingStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Create app", ui.PendingStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Upload to Cerebrium", ui.PendingStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Build app", ui.PendingStyle.Render))
		output.WriteString("\n")

	case StateZippingFiles:
		output.WriteString(formatStateLine(m.spinner.View(), "Zipping files...", ui.ActiveStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Create app", ui.PendingStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Upload to Cerebrium", ui.PendingStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Build app", ui.PendingStyle.Render))
		output.WriteString("\n")

	case StateCreatingApp:
		output.WriteString(formatStateLine(m.spinner.View(), "Creating app...", ui.ActiveStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Upload to Cerebrium", ui.PendingStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Build app", ui.PendingStyle.Render))
		output.WriteString("\n")

	case StateUploadingZip:
		output.WriteString(formatStateLine(m.spinner.View(), "Uploading to Cerebrium...", ui.ActiveStyle.Render))
		output.WriteString("\n")
		output.WriteString(m.renderUploadProgress())
		output.WriteString(formatStateLine("-", "Build app", ui.PendingStyle.Render))
		output.WriteString("\n")

	case StateBuildingApp:
		// Show log viewer waiting message if no logs yet
		if m.logViewer != nil {
			logViewerOutput := m.logViewer.View()
			if logViewerOutput != "" {
				output.WriteString(logViewerOutput)
			}
		}
		// Add spacing between logs (printed above) and spinner
		output.WriteString("\n")
		// Show spinner message
		spinnerText := "Building app..."
		if m.idleMsgIdx > 0 && m.idleMsgIdx-1 < len(idleMessages) {
			spinnerText = idleMessages[m.idleMsgIdx-1]
		}
		output.WriteString(formatStateLine(m.spinner.View(), spinnerText, ui.ActiveStyle.Render))
		output.WriteString("\n")
		output.WriteString(m.renderHelpText())

	case StateDrainingLogs:
		output.WriteString(formatStateLine(m.spinner.View(), "Finishing up...", ui.ActiveStyle.Render))
		output.WriteString("\n")

	case StateCancelling:
		output.WriteString(formatStateLine(m.spinner.View(), "Cancelling build...", ui.YellowStyle.Render))
		output.WriteString("\n")

	case StateCancelled:
		output.WriteString(formatStateLine("⚠", "Build cancelled", ui.YellowStyle.Render))
		output.WriteString("\n")
		if m.message != "" {
			output.WriteString(m.message)
			output.WriteString("\n")
		}

	case StateDeploySuccess, StateDeployError:
		// These states print via tea.Println and then quit, so View() returns empty
		return ""
	}

	return output.String()
}

// Update helpers

func (m *DeployView) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle confirmation state first
	if m.state == StateConfirmation {
		switch msg.String() {
		case "y", "Y", "enter":
			// User confirmed deployment
			if m.isPartnerService() {
				// Partner services skip file loading/zipping, go directly to create app
				m.state = StateCreatingApp
				return m, tea.Batch(
					m.spinner.Init(),
					m.createApp,
				)
			}
			m.state = StateLoadingFiles
			return m, tea.Batch(
				m.spinner.Init(),
				m.loadFiles,
			)

		case "n", "N", "esc", "ctrl+c":
			m.ctxCancel() // Stop all subprocesses
			// User cancelled deployment
			m.err = ui.NewUserCancelledError()
			return m, tea.Quit

		case "up", "k", "down", "j", "pgup", "pgdown":
			if m.viewportReady {
				switch msg.String() {
				case "up", "k":
					m.viewport.ScrollUp(1)
				case "down", "j":
					m.viewport.ScrollDown(1)
				case "pgup":
					m.viewport.HalfPageUp()
				case "pgdown":
					m.viewport.HalfPageDown()
				}
			}
			return m, nil
		}
		return m, nil
	}

	if msg.String() == tea.KeyCtrlC.String() {
		// In detach mode, don't cancel the build - just exit cleanly
		if m.conf.Detach {
			m.err = ui.NewUserCancelledError()
			return m, tea.Quit
		}

		// User cancelled - clean up if build is in progress
		if m.state >= StateBuildingApp && m.buildID != "" {
			m.state = StateCancelling
			return m, m.cancelBuild
		}

		// No cleanup needed, just exit silently
		m.err = ui.NewUserCancelledError()
		return m, tea.Quit
	}

	// Only handle keyboard input in interactive mode
	if m.conf.SimpleOutput() || m.logViewer == nil {
		return m, nil
	}

	// Otherwise hand off to the log viewer and update it
	updatedViewer, cmd := m.logViewer.Update(msg)
	var ok bool
	m.logViewer, ok = updatedViewer.(*logging.LogViewerModel)
	if !ok {
		return m, nil
	}
	return m, cmd
}

// Messages

type filesLoadedMsg struct {
	fileList []string
}

type filesZippedMsg struct {
	zipPath string
	zipSize int64
}

type appCreatedMsg struct {
	response *api.CreateAppResponse
}

type zipUploadedMsg struct{}

type buildStatusUpdateMsg struct {
	buildID string
	status  string
}

type buildStatusPollErrorMsg struct {
	err error
}

type buildCompleteMsg struct {
	status string
}

type logDrainCompleteMsg struct {
	status string // Final build status to use for completion
}

type buildCancelledMsg struct {
	cancelErr error // nil if successful, error if cancellation failed
}

type confirmationResponseMsg struct {
	confirmed bool
}

type uploadProgressTickMsg time.Time

// Commands (async operations)

func (m *DeployView) loadFiles() tea.Msg {
	fileList, err := files.DetermineIncludes(
		m.conf.Config.Deployment.Include,
		m.conf.Config.Deployment.Exclude,
	)
	if err != nil {
		return ui.NewFileSystemError(fmt.Errorf("failed to load files: %w", err))
	}

	if len(fileList) == 0 {
		return ui.NewFileSystemError(fmt.Errorf("no files to upload. Please ensure you have files in your project"))
	}

	return filesLoadedMsg{fileList: fileList}
}

func (m *DeployView) zipFiles() tea.Msg {
	// Create temp directory for zip
	tmpDir, err := os.MkdirTemp("", "cerebrium-deploy-*")
	if err != nil {
		return ui.NewFileSystemError(fmt.Errorf("failed to create temp directory: %w", err))
	}
	// We clean this up later because it needs to be uploaded first

	zipPath := filepath.Join(tmpDir, "deployment.zip")

	// Create zip file with dependency files
	zipSize, err := files.CreateZip(m.fileList, zipPath, m.conf.Config)
	if err != nil {
		return ui.NewFileSystemError(fmt.Errorf("failed to create zip: %w", err))
	}

	// Validate zip size
	warning, err := files.ValidateZipSize(zipSize)
	if err != nil {
		return ui.NewFileSystemError(err)
	}
	if warning != "" {
		slog.Warn(warning)
	}

	return filesZippedMsg{zipPath: zipPath, zipSize: zipSize}
}

// isLikelyPrivateImage checks if the image URL suggests it's from a private registry
func isLikelyPrivateImage(imageURL string) bool {
	if imageURL == "" {
		return false
	}

	// Public images that don't need auth
	publicPrefixes := []string{
		"nvidia/cuda:",
		"debian:",
		"ubuntu:",
		"python:",
		"public.ecr.aws/", // Public ECR repos
	}

	for _, prefix := range publicPrefixes {
		if strings.HasPrefix(imageURL, prefix) {
			return false
		}
	}

	// Private registry indicators
	privateIndicators := []string{
		".dkr.ecr.",            // AWS ECR private
		".azurecr.io/",         // Azure Container Registry
		"gcr.io/",              // Google Container Registry
		".pkg.dev/",            // Google Artifact Registry
		"ghcr.io/",             // GitHub Container Registry
		"registry.gitlab.com/", // GitLab Container Registry
	}

	for _, indicator := range privateIndicators {
		if strings.Contains(imageURL, indicator) {
			return true
		}
	}

	// Docker Hub: if it has a namespace (user/org), it might be private
	// Format: [registry/]namespace/image:tag
	parts := strings.Split(imageURL, "/")
	if len(parts) >= 2 && !strings.Contains(parts[0], ".") {
		// Looks like namespace/image format (Docker Hub)
		// Could be private, worth checking for auth
		return true
	}

	// Custom domain registries (e.g., registry.company.com/image)
	if strings.Contains(strings.Split(imageURL, "/")[0], ".") {
		return true
	}

	return false
}

func (m *DeployView) createApp() tea.Msg {
	payload := m.conf.Config.ToPayload()
	payload["logLevel"] = m.conf.LogLevel
	payload["disableBuildLogs"] = m.conf.DisableBuildLogs
	payload["cliVersion"] = version.Version

	// Include Docker auth if available for private registry support
	baseImage := m.conf.Config.Deployment.DockerBaseImageURL
	dockerAuth, err := auth.GetDockerAuth()

	if err != nil {
		// Only warn if we expect this image might need auth
		if isLikelyPrivateImage(baseImage) {
			slog.Warn("Failed to read Docker auth (may be needed for private image)",
				"error", err,
				"image", baseImage)
		} else {
			// For public images, just debug log
			slog.Debug("Docker auth read error (not needed for public image)",
				"error", err,
				"image", baseImage)
		}
	} else if dockerAuth != "" {
		slog.Info("Docker auth detected", "length", len(dockerAuth))
		payload["dockerAuth"] = dockerAuth
	} else {
		// Only log if we might have expected auth
		if isLikelyPrivateImage(baseImage) {
			slog.Info("No Docker auth found (image may require authentication)",
				"image", baseImage)
		} else {
			slog.Debug("No Docker auth (using public image)",
				"image", baseImage)
		}
	}

	var response *api.CreateAppResponse

	if m.isPartnerDeploy {
		response, err = m.conf.Client.CreatePartnerApp(m.ctx, m.conf.ProjectID, payload)
	} else {
		response, err = m.conf.Client.CreateApp(m.ctx, m.conf.ProjectID, payload)
	}

	if err != nil {
		return ui.NewAPIError(fmt.Errorf("failed to create app: %w", err))
	}

	return appCreatedMsg{response: response}
}

func (m *DeployView) uploadZip() tea.Msg {
	// Clean up temp zip file even if the upload fails
	defer os.RemoveAll(m.zipPath) //nolint:errcheck // Best effort cleanup of temp file, error not actionable

	// Upload with progress tracking
	err := m.uploadZipWithProgress()
	if err != nil {
		return ui.NewAPIError(fmt.Errorf("failed to upload zip: %w", err))
	}

	return zipUploadedMsg{}
}

// uploadZipWithProgress uploads the zip file with progress tracking
func (m *DeployView) uploadZipWithProgress() error {
	// Open the zip file
	file, err := os.Open(m.zipPath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer file.Close() //nolint:errcheck // Deferred close, error not actionable

	// Get file info for size
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat zip file: %w", err)
	}

	// Create a reader that tracks progress
	progressReader := &progressReader{
		reader:  file,
		counter: m.atomicBytesUploaded,
	}

	// Create PUT request with context
	req, err := http.NewRequestWithContext(m.ctx, "PUT", m.appResponse.UploadURL, progressReader)
	if err != nil {
		return fmt.Errorf("failed to create upload request: %w", err)
	}

	req.Header.Set("Content-Type", "application/zip")
	req.ContentLength = fileInfo.Size()

	// Execute request using a simple HTTP client
	client := &http.Client{
		Timeout: 30 * time.Minute, // Allow up to 30 minutes for large uploads
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // Deferred close, error not actionable

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// progressReader wraps an io.Reader and tracks bytes read
type progressReader struct {
	reader  io.Reader
	counter *atomic.Int64
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 && pr.counter != nil {
		pr.counter.Add(int64(n))
	}
	return n, err
}

func (m *DeployView) tickUploadProgress() tea.Cmd {
	return tea.Tick(10*time.Millisecond, func(t time.Time) tea.Msg {
		return uploadProgressTickMsg(t)
	})
}

// printSimpleProgress prints a simple progress bar for non-TTY mode
func (m *DeployView) printSimpleProgress(percent int) {
	// Create a simple progress bar: [=====>     ] 50% (500 KB / 1.0 MB)
	barWidth := 20
	filledWidth := (percent * barWidth) / 100

	var bar strings.Builder
	bar.WriteString("[")
	for i := 0; i < barWidth; i++ {
		if i < filledWidth {
			bar.WriteString("=")
		} else if i == filledWidth {
			bar.WriteString(">")
		} else {
			bar.WriteString(" ")
		}
	}
	bar.WriteString("]")

	// Add stats
	uploaded := ui.FormatSize(m.uploadedBytes)
	total := ui.FormatSize(m.zipSize)
	stats := fmt.Sprintf("%d%% (%s / %s)", percent, uploaded, total)

	// Add speed if available (use cached speed)
	if m.uploadSpeed > 0 {
		stats += fmt.Sprintf(" • %s/s", ui.FormatSize(int64(m.uploadSpeed)))
	}

	fmt.Printf("%s %s\n", bar.String(), stats)
}

func (m *DeployView) cancelBuild() tea.Msg {
	// Attempt to cancel build
	err := m.conf.Client.CancelBuild(
		m.ctx,
		m.conf.ProjectID,
		m.conf.Config.Deployment.Name,
		m.buildID,
	)

	return buildCancelledMsg{cancelErr: err}
}

func (m *DeployView) pollBuildStatus() tea.Msg {
	// Check context first
	if m.ctx.Err() != nil {
		slog.Debug("Build status polling cancelled", "error", m.ctx.Err())
		return nil
	}

	// Fetch build status
	appID := fmt.Sprintf("%s-%s", m.conf.ProjectID, m.conf.Config.Deployment.Name)
	build, err := m.conf.Client.GetBuild(m.ctx, m.conf.ProjectID, appID, m.buildID)
	if err != nil {
		// Log error but don't fail - we'll keep trying
		slog.Warn("Failed to fetch build status", "error", err, "buildId", m.buildID)

		// Schedule retry after 1 second
		return buildStatusPollErrorMsg{err: err}
	}

	slog.Debug("Fetched build status", "status", build.Status, "buildId", build.Id)

	// Return status update message
	return buildStatusUpdateMsg{
		buildID: build.Id,
		status:  build.Status,
	}
}

func (m *DeployView) scheduleNextBuildPoll() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return m.pollBuildStatus()
	})
}

func (m *DeployView) renderUploadProgress() string {
	var output strings.Builder

	// Calculate progress percentage
	progressPercent := float64(0)
	if m.zipSize > 0 {
		progressPercent = float64(m.uploadedBytes) / float64(m.zipSize)
	}

	// Render progress bar
	progressView := m.progressBar.ViewAs(progressPercent)
	percentage := int(progressPercent * 100)

	percentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true)
	output.WriteString(fmt.Sprintf("   %s %s\n", progressView, percentStyle.Render(fmt.Sprintf("%3d%%", percentage))))

	// Render stats (uploaded/total, speed, ETA)
	statsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	var stats []string

	uploaded := ui.FormatSize(m.uploadedBytes)
	total := ui.FormatSize(m.zipSize)
	stats = append(stats, fmt.Sprintf("%s / %s", uploaded, total))

	// Use cached upload speed
	if m.uploadSpeed > 0 {
		stats = append(stats, fmt.Sprintf("%s/s", ui.FormatSize(int64(m.uploadSpeed))))

		if m.uploadedBytes < m.zipSize {
			remaining := float64(m.zipSize-m.uploadedBytes) / m.uploadSpeed
			eta := time.Duration(remaining) * time.Second
			stats = append(stats, fmt.Sprintf("ETA %s", eta.Round(time.Second)))
		}
	}

	output.WriteString("     " + statsStyle.Render(strings.Join(stats, " • ")) + "\n")

	return output.String()
}

// Idle messages shown during long builds
var idleMessages = []string{
	"Hang in there, still building!",
	"Still building, thanks for your patience!",
	"Almost there, please hold on!",
	"Thank you for waiting, we're nearly done!",
}

func (m *DeployView) renderHelpText() string {
	var hints []string

	hints = append(hints, "esc or ctrl+c: cancel build")

	helpText := strings.Join(hints, " | ")
	return ui.HelpStyle.Render(helpText)
}

// GetError returns any error that occurred during deployment
func (m *DeployView) GetError() *ui.UIError {
	return m.err
}

// showDeploymentSummary prints the deployment configuration for non-TTY mode
func (m *DeployView) showDeploymentSummary() {
	fmt.Println(m.renderDeploymentSummary())
	fmt.Print("Do you want to deploy? (Y/n): ")
}

// waitForConfirmation waits for user input in non-TTY mode
func (m *DeployView) waitForConfirmation() tea.Msg {
	// Read a single line from stdin
	var response string
	fmt.Scanln(&response) //nolint:errcheck,gosec // User input handling, errors handled by empty response default

	// Default to "yes" if empty (just Enter pressed)
	if response == "" || strings.ToLower(response) == "y" || strings.ToLower(response) == "yes" {
		return confirmationResponseMsg{confirmed: true}
	}
	return confirmationResponseMsg{confirmed: false}
}

// getRuntimeType returns a user-friendly runtime type name
func getRuntimeType(config *projectconfig.ProjectConfig) string {
	if config.PartnerService != nil {
		// Return partner service name (capitalize first letter)
		if config.PartnerService.Name != "" {
			return strings.ToUpper(string(config.PartnerService.Name[0])) + config.PartnerService.Name[1:]
		}
		return config.PartnerService.Name
	}

	if config.CustomRuntime != nil {
		if config.CustomRuntime.DockerfilePath != "" {
			return "Custom Docker"
		}
		return "Custom Python (ASGI/WSGI)"
	}

	return "Cortex"
}

// isCustomDocker returns true if this is a Custom Docker deployment
func isCustomDocker(config *projectconfig.ProjectConfig) bool {
	return config.CustomRuntime != nil && config.CustomRuntime.DockerfilePath != ""
}

// isCustomPython returns true if this is a Custom Python deployment
func isCustomPython(config *projectconfig.ProjectConfig) bool {
	return config.CustomRuntime != nil && config.CustomRuntime.DockerfilePath == ""
}

// isPartnerServiceRuntime returns true if this is a Partner Service deployment
func isPartnerServiceRuntime(config *projectconfig.ProjectConfig) bool {
	return config.PartnerService != nil
}

// isCortexRuntime returns true if this is a Cortex (default) deployment
func isCortexRuntime(config *projectconfig.ProjectConfig) bool {
	return config.CustomRuntime == nil && config.PartnerService == nil
}

// renderRuntimeSpecificSettings returns runtime-specific configuration as a string
func renderRuntimeSpecificSettings(config *projectconfig.ProjectConfig) string {
	if config.CustomRuntime != nil {
		// Custom Docker or Custom Python
		var settings []string

		if config.CustomRuntime.DockerfilePath != "" {
			settings = append(settings, fmt.Sprintf("Dockerfile: %s", config.CustomRuntime.DockerfilePath))
		}

		// Port is required for custom runtimes, always show it
		settings = append(settings, fmt.Sprintf("Port: %d", config.CustomRuntime.Port))

		if len(config.CustomRuntime.Entrypoint) > 0 {
			settings = append(settings, fmt.Sprintf("Entrypoint: %s", strings.Join(config.CustomRuntime.Entrypoint, " ")))
		}

		if config.CustomRuntime.HealthcheckEndpoint != "" {
			settings = append(settings, fmt.Sprintf("Healthcheck: %s", config.CustomRuntime.HealthcheckEndpoint))
		}

		if config.CustomRuntime.ReadycheckEndpoint != "" {
			settings = append(settings, fmt.Sprintf("Readycheck: %s", config.CustomRuntime.ReadycheckEndpoint))
		}

		return strings.Join(settings, "\n")
	}

	if config.PartnerService != nil && config.PartnerService.Port != nil {
		return fmt.Sprintf("Port: %d", *config.PartnerService.Port)
	}

	return "" // Cortex has no runtime-specific settings
}

// renderDeploymentSummary creates the deployment configuration summary
func (m *DeployView) renderDeploymentSummary() string {
	// For simple output mode, use plain text
	if m.conf.SimpleOutput() {
		var output strings.Builder
		output.WriteString("\n========== DEPLOYMENT CONFIGURATION ==========\n\n")

		// Helper function to format a section
		formatSection := func(header string, items []string) {
			output.WriteString(fmt.Sprintf("%s:\n", header))
			for _, item := range items {
				output.WriteString(fmt.Sprintf("  %s\n", item))
			}
			output.WriteString("\n")
		}

		// HARDWARE PARAMETERS
		var hardwareItems []string
		if m.conf.Config.Hardware.Compute != nil {
			hardwareItems = append(hardwareItems, fmt.Sprintf("Compute: %s", *m.conf.Config.Hardware.Compute))
		}
		if m.conf.Config.Hardware.CPU != nil {
			hardwareItems = append(hardwareItems, fmt.Sprintf("CPU: %.1f", *m.conf.Config.Hardware.CPU))
		}
		if m.conf.Config.Hardware.Memory != nil {
			hardwareItems = append(hardwareItems, fmt.Sprintf("Memory: %.0f GB", *m.conf.Config.Hardware.Memory))
		}
		if m.conf.Config.Hardware.GPUCount != nil && m.conf.Config.Hardware.Compute != nil && *m.conf.Config.Hardware.Compute != "CPU" {
			hardwareItems = append(hardwareItems, fmt.Sprintf("GPU Count: %d", *m.conf.Config.Hardware.GPUCount))
		}
		if m.conf.Config.Hardware.Region != nil {
			hardwareItems = append(hardwareItems, fmt.Sprintf("Region: %s", *m.conf.Config.Hardware.Region))
		}
		if m.conf.Config.Hardware.Provider != nil {
			hardwareItems = append(hardwareItems, fmt.Sprintf("Provider: %s", *m.conf.Config.Hardware.Provider))
		}
		if len(hardwareItems) > 0 {
			formatSection("HARDWARE PARAMETERS", hardwareItems)
		}

		// DEPLOYMENT PARAMETERS
		var deploymentItems []string
		deploymentItems = append(deploymentItems, fmt.Sprintf("Name: %s", m.conf.Config.Deployment.Name))

		// Display runtime type using the helper function
		deploymentItems = append(deploymentItems, fmt.Sprintf("Runtime: %s", getRuntimeType(m.conf.Config)))

		// Show Python Version, Docker Image, and Include/Exclude only for non-Custom Docker runtimes
		if !isCustomDocker(m.conf.Config) {
			if m.conf.Config.Deployment.PythonVersion != "" {
				deploymentItems = append(deploymentItems, fmt.Sprintf("Python Version: %s", m.conf.Config.Deployment.PythonVersion))
			}
			if m.conf.Config.Deployment.DockerBaseImageURL != "" {
				deploymentItems = append(deploymentItems, fmt.Sprintf("Docker Image: %s", m.conf.Config.Deployment.DockerBaseImageURL))
			}

			// Include/Exclude patterns
			if len(m.conf.Config.Deployment.Include) > 0 {
				deploymentItems = append(deploymentItems, fmt.Sprintf("Include: %s", strings.Join(m.conf.Config.Deployment.Include, ", ")))
			}
			if len(m.conf.Config.Deployment.Exclude) > 0 {
				deploymentItems = append(deploymentItems, fmt.Sprintf("Exclude: %s", strings.Join(m.conf.Config.Deployment.Exclude, ", ")))
			}
		}

		formatSection("DEPLOYMENT PARAMETERS", deploymentItems)

		// RUNTIME-SPECIFIC PARAMETERS (Custom Python, Custom Docker, or Partner Service)
		runtimeSettings := renderRuntimeSpecificSettings(m.conf.Config)
		if runtimeSettings != "" {
			runtimeItems := strings.Split(runtimeSettings, "\n")
			formatSection("RUNTIME PARAMETERS", runtimeItems)
		}

		// SCALING PARAMETERS
		var scalingItems []string
		if m.conf.Config.Scaling.Cooldown != nil {
			scalingItems = append(scalingItems, fmt.Sprintf("Cooldown: %ds", *m.conf.Config.Scaling.Cooldown))
		}
		if m.conf.Config.Scaling.MinReplicas != nil {
			scalingItems = append(scalingItems, fmt.Sprintf("Min Replicas: %d", *m.conf.Config.Scaling.MinReplicas))
		}
		if m.conf.Config.Scaling.MaxReplicas != nil {
			scalingItems = append(scalingItems, fmt.Sprintf("Max Replicas: %d", *m.conf.Config.Scaling.MaxReplicas))
		}
		if m.conf.Config.Scaling.ReplicaConcurrency != nil {
			concurrency := fmt.Sprintf("Replica Concurrency: %d", *m.conf.Config.Scaling.ReplicaConcurrency)

			// Add GPU warning if applicable
			if m.conf.Config.Hardware.Compute != nil && *m.conf.Config.Hardware.Compute != "CPU" && *m.conf.Config.Scaling.ReplicaConcurrency > 1 {
				concurrency += " ⚠️  (Multiple concurrent requests on GPU)"
			}
			scalingItems = append(scalingItems, concurrency)
		}
		if m.conf.Config.Scaling.EvaluationIntervalSeconds != nil {
			scalingItems = append(scalingItems, fmt.Sprintf("Evaluation Interval: %ds", *m.conf.Config.Scaling.EvaluationIntervalSeconds))
		}
		if m.conf.Config.Scaling.LoadBalancingAlgorithm != nil {
			scalingItems = append(scalingItems, fmt.Sprintf("Load Balancing Algorithm: %s", *m.conf.Config.Scaling.LoadBalancingAlgorithm))
		}
		if len(scalingItems) > 0 {
			formatSection("SCALING PARAMETERS", scalingItems)
		}

		// DEPENDENCIES (only show if not using Dockerfile)
		if m.conf.Config.CustomRuntime == nil || m.conf.Config.CustomRuntime.DockerfilePath == "" {
			var depItems []string

			// Pip packages
			if len(m.conf.Config.Dependencies.Pip) > 0 {
				var pkgs []string
				for pkg, ver := range m.conf.Config.Dependencies.Pip {
					if ver == "" {
						pkgs = append(pkgs, pkg)
					} else {
						pkgs = append(pkgs, fmt.Sprintf("%s==%s", pkg, ver))
					}
				}
				if len(pkgs) > 0 {
					sort.Strings(pkgs) // Sort for consistent output
					depItems = append(depItems, fmt.Sprintf("Pip: %s", strings.Join(pkgs, ", ")))
				}
			}

			// Apt packages
			if len(m.conf.Config.Dependencies.Apt) > 0 {
				var pkgs []string
				for pkg, ver := range m.conf.Config.Dependencies.Apt {
					if ver == "" {
						pkgs = append(pkgs, pkg)
					} else {
						pkgs = append(pkgs, fmt.Sprintf("%s=%s", pkg, ver))
					}
				}
				if len(pkgs) > 0 {
					sort.Strings(pkgs) // Sort for consistent output
					depItems = append(depItems, fmt.Sprintf("Apt: %s", strings.Join(pkgs, ", ")))
				}
			}

			// Conda packages
			if len(m.conf.Config.Dependencies.Conda) > 0 {
				var pkgs []string
				for pkg, ver := range m.conf.Config.Dependencies.Conda {
					if ver == "" {
						pkgs = append(pkgs, pkg)
					} else {
						pkgs = append(pkgs, fmt.Sprintf("%s==%s", pkg, ver))
					}
				}
				if len(pkgs) > 0 {
					sort.Strings(pkgs) // Sort for consistent output
					depItems = append(depItems, fmt.Sprintf("Conda: %s", strings.Join(pkgs, ", ")))
				}
			}

			if len(depItems) > 0 {
				formatSection("DEPENDENCIES", depItems)
			}
		}

		return output.String()
	}

	// For TTY mode, use the Panel with TableSections
	var sections []ui.TableSection

	// HARDWARE PARAMETERS
	var hardwareRows []ui.TableRow
	if m.conf.Config.Hardware.Compute != nil {
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "Compute:", Value: *m.conf.Config.Hardware.Compute})
	}
	if m.conf.Config.Hardware.CPU != nil {
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "CPU:", Value: fmt.Sprintf("%.1f", *m.conf.Config.Hardware.CPU)})
	}
	if m.conf.Config.Hardware.Memory != nil {
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "Memory:", Value: fmt.Sprintf("%.0f GB", *m.conf.Config.Hardware.Memory)})
	}
	if m.conf.Config.Hardware.GPUCount != nil && m.conf.Config.Hardware.Compute != nil && *m.conf.Config.Hardware.Compute != "CPU" {
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "GPU Count:", Value: fmt.Sprintf("%d", *m.conf.Config.Hardware.GPUCount)})
	}
	if m.conf.Config.Hardware.Region != nil {
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "Region:", Value: *m.conf.Config.Hardware.Region})
	}
	if m.conf.Config.Hardware.Provider != nil {
		hardwareRows = append(hardwareRows, ui.TableRow{Label: "Provider:", Value: *m.conf.Config.Hardware.Provider})
	}
	if len(hardwareRows) > 0 {
		sections = append(sections, ui.TableSection{
			Header: "HARDWARE PARAMETERS",
			Rows:   hardwareRows,
		})
	}

	// DEPLOYMENT PARAMETERS
	var deploymentRows []ui.TableRow
	deploymentRows = append(deploymentRows, ui.TableRow{Label: "Name:", Value: m.conf.Config.Deployment.Name})

	// Display runtime type using the helper function
	deploymentRows = append(deploymentRows, ui.TableRow{Label: "Runtime:", Value: getRuntimeType(m.conf.Config)})

	// Show Python Version, Docker Image, and Include/Exclude only for non-Custom Docker runtimes
	if !isCustomDocker(m.conf.Config) {
		if m.conf.Config.Deployment.PythonVersion != "" {
			deploymentRows = append(deploymentRows, ui.TableRow{Label: "Python Version:", Value: m.conf.Config.Deployment.PythonVersion})
		}
		if m.conf.Config.Deployment.DockerBaseImageURL != "" {
			deploymentRows = append(deploymentRows, ui.TableRow{Label: "Docker Image:", Value: m.conf.Config.Deployment.DockerBaseImageURL})
		}

		// Include/Exclude patterns
		if len(m.conf.Config.Deployment.Include) > 0 {
			deploymentRows = append(deploymentRows, ui.TableRow{Label: "Include:", Value: strings.Join(m.conf.Config.Deployment.Include, ", ")})
		}
		if len(m.conf.Config.Deployment.Exclude) > 0 {
			deploymentRows = append(deploymentRows, ui.TableRow{Label: "Exclude:", Value: strings.Join(m.conf.Config.Deployment.Exclude, ", ")})
		}
	}

	sections = append(sections, ui.TableSection{
		Header: "DEPLOYMENT PARAMETERS",
		Rows:   deploymentRows,
	})

	// RUNTIME-SPECIFIC PARAMETERS (Custom Python, Custom Docker, or Partner Service)
	runtimeSettings := renderRuntimeSpecificSettings(m.conf.Config)
	if runtimeSettings != "" {
		var runtimeRows []ui.TableRow
		for _, line := range strings.Split(runtimeSettings, "\n") {
			if line != "" {
				parts := strings.SplitN(line, ": ", 2)
				if len(parts) == 2 {
					runtimeRows = append(runtimeRows, ui.TableRow{Label: parts[0] + ":", Value: parts[1]})
				}
			}
		}
		if len(runtimeRows) > 0 {
			sections = append(sections, ui.TableSection{
				Header: "RUNTIME PARAMETERS",
				Rows:   runtimeRows,
			})
		}
	}

	// SCALING PARAMETERS
	var scalingRows []ui.TableRow
	if m.conf.Config.Scaling.Cooldown != nil {
		scalingRows = append(scalingRows, ui.TableRow{Label: "Cooldown:", Value: fmt.Sprintf("%ds", *m.conf.Config.Scaling.Cooldown)})
	}
	if m.conf.Config.Scaling.MinReplicas != nil {
		scalingRows = append(scalingRows, ui.TableRow{Label: "Min Replicas:", Value: fmt.Sprintf("%d", *m.conf.Config.Scaling.MinReplicas)})
	}
	if m.conf.Config.Scaling.MaxReplicas != nil {
		scalingRows = append(scalingRows, ui.TableRow{Label: "Max Replicas:", Value: fmt.Sprintf("%d", *m.conf.Config.Scaling.MaxReplicas)})
	}
	if m.conf.Config.Scaling.ReplicaConcurrency != nil {
		concurrency := fmt.Sprintf("%d", *m.conf.Config.Scaling.ReplicaConcurrency)

		// Add GPU warning if applicable
		if m.conf.Config.Hardware.Compute != nil && *m.conf.Config.Hardware.Compute != "CPU" && *m.conf.Config.Scaling.ReplicaConcurrency > 1 {
			concurrency += " ⚠️  (Multiple concurrent requests on GPU)"
		}
		scalingRows = append(scalingRows, ui.TableRow{Label: "Replica Concurrency:", Value: concurrency})
	}
	if m.conf.Config.Scaling.EvaluationIntervalSeconds != nil {
		scalingRows = append(scalingRows, ui.TableRow{Label: "Evaluation Interval:", Value: fmt.Sprintf("%ds", *m.conf.Config.Scaling.EvaluationIntervalSeconds)})
	}
	if m.conf.Config.Scaling.LoadBalancingAlgorithm != nil {
		scalingRows = append(scalingRows, ui.TableRow{Label: "Load Balancing Algorithm:", Value: *m.conf.Config.Scaling.LoadBalancingAlgorithm})
	}
	if len(scalingRows) > 0 {
		sections = append(sections, ui.TableSection{
			Header: "SCALING PARAMETERS",
			Rows:   scalingRows,
		})
	}

	// DEPENDENCIES (only show if not using Dockerfile)
	if m.conf.Config.CustomRuntime == nil || m.conf.Config.CustomRuntime.DockerfilePath == "" {
		var depRows []ui.TableRow

		// Pip packages
		if len(m.conf.Config.Dependencies.Pip) > 0 {
			var pkgs []string
			for pkg, ver := range m.conf.Config.Dependencies.Pip {
				if ver == "" {
					pkgs = append(pkgs, pkg)
				} else {
					pkgs = append(pkgs, fmt.Sprintf("%s==%s", pkg, ver))
				}
			}
			if len(pkgs) > 0 {
				sort.Strings(pkgs) // Sort for consistent output
				depRows = append(depRows, ui.TableRow{Label: "Pip:", Value: strings.Join(pkgs, ", ")})
			}
		}

		// Apt packages
		if len(m.conf.Config.Dependencies.Apt) > 0 {
			var pkgs []string
			for pkg, ver := range m.conf.Config.Dependencies.Apt {
				if ver == "" {
					pkgs = append(pkgs, pkg)
				} else {
					pkgs = append(pkgs, fmt.Sprintf("%s=%s", pkg, ver))
				}
			}
			if len(pkgs) > 0 {
				sort.Strings(pkgs) // Sort for consistent output
				depRows = append(depRows, ui.TableRow{Label: "Apt:", Value: strings.Join(pkgs, ", ")})
			}
		}

		// Conda packages
		if len(m.conf.Config.Dependencies.Conda) > 0 {
			var pkgs []string
			for pkg, ver := range m.conf.Config.Dependencies.Conda {
				if ver == "" {
					pkgs = append(pkgs, pkg)
				} else {
					pkgs = append(pkgs, fmt.Sprintf("%s==%s", pkg, ver))
				}
			}
			if len(pkgs) > 0 {
				sort.Strings(pkgs) // Sort for consistent output
				depRows = append(depRows, ui.TableRow{Label: "Conda:", Value: strings.Join(pkgs, ", ")})
			}
		}

		if len(depRows) > 0 {
			sections = append(sections, ui.TableSection{
				Header: "DEPENDENCIES",
				Rows:   depRows,
			})
		}
	}

	// Render table and wrap in Panel
	tableContent := ui.RenderDetailTable(sections)
	return ui.RenderPanel("DEPLOYMENT CONFIGURATION", tableContent)
}
