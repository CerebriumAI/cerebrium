package commands

import (
	"archive/tar"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/files"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/internal/ui/logging"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/cerebriumai/cerebrium/pkg/projectconfig"
	tea "github.com/charmbracelet/bubbletea"
)

// RunState represents the current state of the run execution
type RunState int

const (
	RunStatePreparingFiles RunState = iota
	RunStateCheckingDependencies
	RunStateCreatingBaseImage
	RunStateCreatingApp
	RunStateCreatingTar
	RunStateUploadingRun
	RunStateExecuting
	RunStatePollingLogs
	RunStateDrainingLogs // Waiting for remaining logs to arrive after run completes
	RunStateSuccess
	RunStateError
)

const (
	// maxTarSize is the maximum allowed tar file size for cerebrium run (4MB)
	maxTarSize = 4 * 1024 * 1024
	// maxDepsSize is the maximum allowed dependencies size (380KB)
	maxDepsSize = 380 * 1024
)

// RunConfig contains run configuration
type RunConfig struct {
	ui.DisplayConfig

	Config       *projectconfig.ProjectConfig
	ProjectID    string
	Client       api.Client
	Filename     string
	FunctionName *string
	Region       string
	DataMap      map[string]any
}

// RunView is the Bubbletea model for the run flow
type RunView struct {
	ctx context.Context

	state   RunState
	spinner *ui.SpinnerModel
	err     *ui.UIError

	fileList       []string
	tarPath        string
	tarSize        int64
	imageDigest    *string
	runID          string
	appName        string
	appID          string
	logViewer      *logging.LogViewerModel
	runStatus      string
	runCompleted   bool // Track if we've already handled completion
	needsInjection bool
	needsBaseImage bool // Track if we need to create a base image for dependencies
	region         string
	hardwareInfo   string
	jsonData       map[string]any

	conf RunConfig
}

// NewRunView creates a new run view
func NewRunView(ctx context.Context, conf RunConfig) *RunView {
	return &RunView{
		ctx:     ctx,
		state:   RunStatePreparingFiles,
		spinner: ui.NewSpinner(),
		conf:    conf,
	}
}

// Init starts the run flow

// Error returns the error if any occurred during execution
func (m *RunView) Error() error {
	return m.err
}

func (m *RunView) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Init(),
		m.prepareRun,
	)
}

// Update handles messages
func (m *RunView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ui.SignalCancelMsg:
		return m.handleSignalCancel()

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case runPreparedMsg:
		return m.handleRunPrepared(msg)

	case baseImageCreatedMsg:
		return m.handleBaseImageCreated(msg)

	case runAppCreatedMsg:
		return m.handleAppCreated()

	case tarCreatedMsg:
		return m.handleTarCreated(msg)

	case runUploadedMsg:
		return m.handleRunUploaded(msg)

	case runStatusMsg:
		return m.handleRunStatus(msg)

	case runLogDrainCompleteMsg:
		return m.handleLogDrainComplete(msg)

	case pollLogsTickMsg:
		return m.handlePollLogsTick()

	case *ui.UIError:
		return m.handleError(msg)

	default:
		return m.handleDefault(msg)
	}
}

func (m *RunView) handleSignalCancel() (tea.Model, tea.Cmd) {
	if m.conf.SimpleOutput() {
		fmt.Fprintf(os.Stderr, "\nReceived termination signal, exiting...\n")
	}
	m.err = ui.NewUserCancelledError()
	return m, tea.Quit
}

func (m *RunView) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.conf.SimpleOutput() {
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "q":
		m.err = ui.NewUserCancelledError()
		return m, tea.Quit
	}

	return m, nil
}

func (m *RunView) handleRunPrepared(msg runPreparedMsg) (tea.Model, tea.Cmd) {
	m.fileList = msg.fileList
	m.appName = msg.appName
	m.appID = msg.appID
	m.region = msg.region
	m.hardwareInfo = msg.hardwareInfo
	m.jsonData = msg.jsonData
	m.needsInjection = msg.needsInjection

	// Check if we need to create a base image for dependencies
	// We track this here but create the app first (required for base-image API auth)
	m.needsBaseImage = m.conf.Config != nil && (len(m.conf.Config.Dependencies.Pip) > 0 ||
		len(m.conf.Config.Dependencies.Conda) > 0 ||
		len(m.conf.Config.Dependencies.Apt) > 0)

	if m.conf.SimpleOutput() {
		fmt.Printf("✓ Prepared %d files\n", len(msg.fileList))
	}

	// Always create the app first - the base-image endpoint requires the app to exist
	// for authorization (app-authorizer checks APPS_TABLE)
	m.state = RunStateCreatingApp

	if m.conf.SimpleOutput() {
		return m, m.createApp
	}

	return m, tea.Sequence(
		tea.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓  Prepared %d files", len(msg.fileList)))),
		m.createApp,
	)
}

func (m *RunView) handleBaseImageCreated(msg baseImageCreatedMsg) (tea.Model, tea.Cmd) {
	m.imageDigest = &msg.imageDigest
	m.state = RunStateCreatingTar

	if m.conf.SimpleOutput() {
		fmt.Printf("✓ Base image ready: %s\n", msg.imageDigest)
		return m, m.createTar
	}

	return m, tea.Sequence(
		tea.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓  Base image ready: %s", msg.imageDigest))),
		m.createTar,
	)
}

func (m *RunView) handleAppCreated() (tea.Model, tea.Cmd) {
	if m.conf.SimpleOutput() {
		fmt.Printf("✓ Created run app: %s%s\n", m.appName, m.hardwareInfo)
	}

	// If we need a base image for dependencies, create it now (after app exists)
	if m.needsBaseImage {
		m.state = RunStateCheckingDependencies

		if m.conf.SimpleOutput() {
			fmt.Println("Checking dependencies...")
			return m, m.createBaseImage
		}

		return m, tea.Sequence(
			tea.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓  Created run app: %s%s", m.appName, m.hardwareInfo))),
			m.createBaseImage,
		)
	}

	// No dependencies, proceed directly to creating tar
	m.state = RunStateCreatingTar

	if m.conf.SimpleOutput() {
		return m, m.createTar
	}

	return m, tea.Sequence(
		tea.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓  Created run app: %s%s", m.appName, m.hardwareInfo))),
		m.createTar,
	)
}

func (m *RunView) handleTarCreated(msg tarCreatedMsg) (tea.Model, tea.Cmd) {
	m.tarPath = msg.tarPath
	m.tarSize = msg.tarSize
	m.state = RunStateUploadingRun

	if m.conf.SimpleOutput() {
		fmt.Printf("✓ Created archive (%s)\n", formatRunSize(msg.tarSize))
		return m, m.uploadRun
	}

	return m, tea.Sequence(
		tea.Println(ui.SuccessStyle.Render(fmt.Sprintf("✓  Created archive (%s)", formatRunSize(msg.tarSize)))),
		m.uploadRun,
	)
}

func (m *RunView) handleRunUploaded(msg runUploadedMsg) (tea.Model, tea.Cmd) {
	m.runID = msg.runID
	m.state = RunStateExecuting

	// Initialize log viewer with polling provider
	provider := logging.NewPollingAppLogProvider(logging.PollingAppLogProviderConfig{
		Client:       m.conf.Client,
		ProjectID:    m.conf.ProjectID,
		AppID:        m.appID,
		Follow:       true,
		RunID:        m.runID,
		PollInterval: ui.LOG_POLL_INTERVAL,
	})

	m.logViewer = logging.NewLogViewer(m.ctx, logging.LogViewerConfig{
		DisplayConfig: m.conf.DisplayConfig,
		Provider:      provider,
		TickInterval:  200 * time.Millisecond,
		ShowHelp:      false,
		AutoExpand:    true, // Stream logs via tea.Println
	})

	if m.conf.SimpleOutput() {
		fmt.Println("✓ Uploaded successfully")
		fmt.Println("Executing...")
		return m, tea.Batch(
			m.logViewer.Init(),
			m.checkRunStatus(),
			tea.Tick(time.Second, func(t time.Time) tea.Msg {
				return pollLogsTickMsg(t)
			}),
		)
	}

	return m, tea.Sequence(
		tea.Println(ui.SuccessStyle.Render("✓  Uploaded successfully")),
		tea.Batch(
			m.logViewer.Init(),
			m.checkRunStatus(),
			tea.Tick(time.Second, func(t time.Time) tea.Msg {
				return pollLogsTickMsg(t)
			}),
		),
	)
}

func (m *RunView) handleRunStatus(msg runStatusMsg) (tea.Model, tea.Cmd) {
	m.runStatus = msg.status

	if !m.runCompleted && (msg.status == "success" || msg.status == "failed" || msg.status == "fail") {
		m.runCompleted = true // Mark as completed to avoid reprocessing
		m.state = RunStateDrainingLogs

		if m.conf.SimpleOutput() {
			if msg.status == "success" {
				fmt.Println("\n✓ Run completed successfully.")
			} else {
				fmt.Println("\n✗ Run failed.")
			}
			fmt.Println("Waiting for logs... (Ctrl+C to exit)")
		}

		// Wait for remaining logs to arrive
		return m, tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
			return runLogDrainCompleteMsg(msg)
		})
	}

	return m, nil
}

func (m *RunView) handleLogDrainComplete(msg runLogDrainCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.status == "success" {
		m.state = RunStateSuccess

		if m.conf.SimpleOutput() {
			fmt.Println("\n✓ Run completed successfully")
			return m, tea.Quit
		}

		return m, tea.Sequence(
			tea.Println(""),
			tea.Println(ui.SuccessStyle.Render("✓  Run completed successfully")),
			tea.Quit,
		)
	}

	m.state = RunStateError
	err := ui.NewAPIError(fmt.Errorf("run failed"))
	err.SilentExit = true
	m.err = err

	if m.conf.SimpleOutput() {
		fmt.Println("\n✗ Run failed")
		return m, tea.Quit
	}

	return m, tea.Sequence(
		tea.Println(""),
		tea.Println(ui.ErrorStyle.Render("✗  Run failed")),
		tea.Quit,
	)
}

func (m *RunView) handlePollLogsTick() (tea.Model, tea.Cmd) {
	if m.runCompleted {
		return m, nil
	}

	// Check context for cancellation or timeout
	if err := m.ctx.Err(); err != nil {
		var errorMsg string
		if err == context.DeadlineExceeded {
			errorMsg = "Polling timeout reached"
		} else {
			errorMsg = "Run cancelled"
		}

		if m.conf.SimpleOutput() {
			fmt.Printf("\n⚠️ %s.\n", errorMsg)
		}
		m.state = RunStateError
		apiErr := ui.NewAPIError(fmt.Errorf("%s", strings.ToLower(errorMsg)))
		apiErr.SilentExit = true
		m.err = apiErr
		return m, tea.Quit
	}

	m.state = RunStatePollingLogs
	// LogViewerModel handles log polling via its own ticker
	// We only need to check run status periodically
	return m, tea.Batch(
		m.checkRunStatus(),
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return pollLogsTickMsg(t)
		}),
	)
}

func (m *RunView) handleError(msg *ui.UIError) (tea.Model, tea.Cmd) {
	msg.SilentExit = true
	m.err = msg
	m.state = RunStateError

	if m.tarPath != "" {
		//nolint:errcheck,gosec // Best effort cleanup of temp file, error not actionable
		os.Remove(m.tarPath)
	}

	if m.conf.SimpleOutput() {
		fmt.Printf("Error: %s\n", msg.Error())
		return m, tea.Quit
	}

	return m, tea.Sequence(
		tea.Println(ui.ErrorStyle.Render(fmt.Sprintf("✗  Error: %s", msg.Error()))),
		tea.Quit,
	)
}

func (m *RunView) handleDefault(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if !m.conf.SimpleOutput() {
		var spinnerCmd tea.Cmd
		spinnerModel, spinnerCmd := m.spinner.Update(msg)
		m.spinner = spinnerModel.(*ui.SpinnerModel) //nolint:errcheck // Type assertion guaranteed by SpinnerModel structure
		cmds = append(cmds, spinnerCmd)
	}

	// Forward to log viewer if active
	if m.logViewer != nil && (m.state == RunStateExecuting || m.state == RunStatePollingLogs || m.state == RunStateDrainingLogs) {
		updated, logCmd := m.logViewer.Update(msg)
		m.logViewer = updated.(*logging.LogViewerModel) //nolint:errcheck // Type assertion guaranteed by LogViewerModel structure
		cmds = append(cmds, logCmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the run view
// Only renders the ACTIVE state - completed states are printed to scrollback via tea.Println in Update()
func (m *RunView) View() string {
	if m.conf.SimpleOutput() {
		return ""
	}

	var output strings.Builder

	// Helper function to format state line (matching deploy.go pattern)
	formatStateLine := func(icon string, text string, styleFunc func(...string) string) string {
		return fmt.Sprintf("%s  %s", icon, styleFunc(text))
	}

	switch m.state {
	case RunStatePreparingFiles:
		output.WriteString(formatStateLine(m.spinner.View(), "Preparing files...", ui.ActiveStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Create run app", ui.PendingStyle.Render))
		output.WriteString("\n")
		if m.needsBaseImage {
			output.WriteString(formatStateLine("-", "Check dependencies", ui.PendingStyle.Render))
			output.WriteString("\n")
		}
		output.WriteString(formatStateLine("-", "Create archive", ui.PendingStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Upload to Cerebrium", ui.PendingStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Execute", ui.PendingStyle.Render))
		output.WriteString("\n")

	case RunStateCreatingApp:
		output.WriteString(formatStateLine(m.spinner.View(), "Creating run app...", ui.ActiveStyle.Render))
		output.WriteString("\n")
		if m.needsBaseImage {
			output.WriteString(formatStateLine("-", "Check dependencies", ui.PendingStyle.Render))
			output.WriteString("\n")
		}
		output.WriteString(formatStateLine("-", "Create archive", ui.PendingStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Upload to Cerebrium", ui.PendingStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Execute", ui.PendingStyle.Render))
		output.WriteString("\n")

	case RunStateCheckingDependencies, RunStateCreatingBaseImage:
		output.WriteString(formatStateLine(m.spinner.View(), "Checking dependencies...", ui.ActiveStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Create archive", ui.PendingStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Upload to Cerebrium", ui.PendingStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Execute", ui.PendingStyle.Render))
		output.WriteString("\n")

	case RunStateCreatingTar:
		output.WriteString(formatStateLine(m.spinner.View(), "Creating archive...", ui.ActiveStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Upload to Cerebrium", ui.PendingStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Execute", ui.PendingStyle.Render))
		output.WriteString("\n")

	case RunStateUploadingRun:
		output.WriteString(formatStateLine(m.spinner.View(), "Uploading to Cerebrium...", ui.ActiveStyle.Render))
		output.WriteString("\n")
		output.WriteString(formatStateLine("-", "Execute", ui.PendingStyle.Render))
		output.WriteString("\n")

	case RunStateExecuting, RunStatePollingLogs:
		// LogViewer handles log printing via tea.Println
		// Just show the running spinner below the logs
		output.WriteString(formatStateLine(m.spinner.View(), "Running...", ui.ActiveStyle.Render))
		output.WriteString("\n")
		output.WriteString(ui.HelpStyle.Render("(Ctrl+C to exit)"))
		output.WriteString("\n")

	case RunStateDrainingLogs:
		output.WriteString(formatStateLine(m.spinner.View(), "Finishing up...", ui.ActiveStyle.Render))
		output.WriteString("\n")

	case RunStateSuccess, RunStateError:
		// Final states print via tea.Println and quit, so View() returns empty
		return ""
	}

	return output.String()
}

// GetError returns any error that occurred
func (m *RunView) GetError() *ui.UIError {
	return m.err
}

// Message types

type runPreparedMsg struct {
	fileList       []string
	appName        string
	appID          string
	region         string
	hardwareInfo   string
	jsonData       map[string]any
	needsInjection bool
}

type baseImageCreatedMsg struct {
	imageDigest string
}

type runAppCreatedMsg struct{}

type tarCreatedMsg struct {
	tarPath string
	tarSize int64
}

type runUploadedMsg struct {
	runID string
}

type runStatusMsg struct {
	status string
}

type pollLogsTickMsg time.Time

type runLogDrainCompleteMsg struct {
	status string
}

func (m *RunView) prepareRun() tea.Msg {
	jsonData := m.conf.DataMap

	// Determine region
	region := m.conf.Region
	if region == "" && m.conf.Config != nil && m.conf.Config.Hardware.Region != nil && *m.conf.Config.Hardware.Region != "" {
		region = *m.conf.Config.Hardware.Region
	}
	if region == "" {
		cfg, err := config.Load()
		if err == nil && cfg.DefaultRegion != "" {
			region = cfg.DefaultRegion
		} else {
			region = "us-east-1"
		}
	}

	appName := ""
	if m.conf.Config != nil && m.conf.Config.Deployment.Name != "" {
		appName = m.conf.Config.Deployment.Name
	}
	if appName == "" || appName == "." {
		cwd, _ := os.Getwd()
		appName = filepath.Base(cwd)
		if appName == "." || appName == "" {
			appName = "app"
		}
	}

	hardwareInfo := ""
	if m.conf.Config != nil {
		var info []string
		if m.conf.Config.Hardware.Compute != nil && *m.conf.Config.Hardware.Compute != "" {
			info = append(info, fmt.Sprintf("Compute: %s", *m.conf.Config.Hardware.Compute))
		}
		if m.conf.Config.Hardware.GPUCount != nil && *m.conf.Config.Hardware.GPUCount > 0 {
			info = append(info, fmt.Sprintf("GPU: %d", *m.conf.Config.Hardware.GPUCount))
		}
		if m.conf.Config.Hardware.CPU != nil && *m.conf.Config.Hardware.CPU > 0 {
			info = append(info, fmt.Sprintf("CPU: %.1f", *m.conf.Config.Hardware.CPU))
		}
		if m.conf.Config.Hardware.Memory != nil && *m.conf.Config.Hardware.Memory > 0 {
			info = append(info, fmt.Sprintf("Memory: %.1f", *m.conf.Config.Hardware.Memory))
		}
		if len(info) > 0 {
			hardwareInfo = fmt.Sprintf(" (%s)", strings.Join(info, ", "))
		}
	}

	// Check if we need to inject main guard
	// Inject if the file doesn't have "if __name__ == '__main__':"
	needsInjection := false
	content, err := os.ReadFile(m.conf.Filename)
	if err == nil {
		if !strings.Contains(string(content), `if __name__ == "__main__":`) {
			needsInjection = true
		}
	}

	fileList, err := files.DetermineIncludes(
		m.conf.Config.Deployment.Include,
		m.conf.Config.Deployment.Exclude,
	)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to determine files: %w", err))
	}

	if len(fileList) == 0 {
		return ui.NewValidationError(fmt.Errorf("no files to upload"))
	}

	appID := fmt.Sprintf("%s-%s", m.conf.ProjectID, appName)

	return runPreparedMsg{
		fileList:       fileList,
		appName:        appName,
		appID:          appID,
		region:         region,
		hardwareInfo:   hardwareInfo,
		jsonData:       jsonData,
		needsInjection: needsInjection,
	}
}

func (m *RunView) createBaseImage() tea.Msg {
	depsJSON := map[string]any{
		"pip":   m.conf.Config.Dependencies.Pip,
		"conda": m.conf.Config.Dependencies.Conda,
		"apt":   m.conf.Config.Dependencies.Apt,
	}

	depsBytes, err := json.Marshal(depsJSON)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to marshal dependencies: %w", err))
	}

	if len(depsBytes) > maxDepsSize {
		return ui.NewValidationError(fmt.Errorf("dependency list size (%dKB) exceeds 380KB limit for cerebrium run", len(depsBytes)/1024))
	}

	payload := api.BaseImagePayload{
		Dependencies:     depsJSON,
		PreBuildCommands: encodeBase64Strings(m.conf.Config.Deployment.PreBuildCommands),
		ShellCommands:    encodeBase64Strings(m.conf.Config.Deployment.ShellCommands),
		BaseImageURI:     m.conf.Config.Deployment.DockerBaseImageURL,
	}

	imageDigest, err := m.conf.Client.CreateBaseImage(
		m.ctx,
		m.conf.ProjectID,
		m.appID,
		m.region,
		payload,
	)
	if err != nil {
		return ui.NewAPIError(fmt.Errorf("failed to create base image: %w", err))
	}

	return baseImageCreatedMsg{imageDigest: imageDigest}
}

// encodeBase64Strings encodes each string in the slice to base64
func encodeBase64Strings(commands []string) []string {
	encoded := make([]string, len(commands))
	for i, cmd := range commands {
		encoded[i] = base64.StdEncoding.EncodeToString([]byte(cmd))
	}
	return encoded
}

func (m *RunView) createApp() tea.Msg {
	err := m.conf.Client.CreateRunApp(
		m.ctx,
		m.conf.ProjectID,
		m.appID,
		m.region,
	)
	if err != nil {
		return ui.NewAPIError(fmt.Errorf("failed to create app: %w", err))
	}

	return runAppCreatedMsg{}
}

func (m *RunView) createTar() tea.Msg {
	tmpFile, err := os.CreateTemp("", "cerebrium-run-*.tar")
	if err != nil {
		return ui.NewFileSystemError(fmt.Errorf("failed to create temp file: %w", err))
	}
	tarPath := tmpFile.Name()

	err = func() error {
		//nolint:errcheck // Deferred close, error not actionable
		defer tmpFile.Close()

		tw := tar.NewWriter(tmpFile)
		//nolint:errcheck // Deferred close, error not actionable
		defer tw.Close()

		for _, filePath := range m.fileList {
			if m.needsInjection && filePath == m.conf.Filename {
				content, err := os.ReadFile(filePath) //nolint:gosec // File path from user's project
				if err != nil {
					return fmt.Errorf("failed to read file %s: %w", filePath, err)
				}

				injectContent := getInjectMainContent()
				modifiedContent := append(content, []byte("\n\n"+injectContent)...)

				hdr := &tar.Header{
					Name:    filePath,
					Mode:    0644,
					Size:    int64(len(modifiedContent)),
					ModTime: time.Now(),
				}
				if err := tw.WriteHeader(hdr); err != nil {
					return fmt.Errorf("failed to write tar header: %w", err)
				}
				if _, err := tw.Write(modifiedContent); err != nil {
					return fmt.Errorf("failed to write tar content: %w", err)
				}
			} else {
				err := func() error {
					file, err := os.Open(filePath) //nolint:gosec // File path from user's project
					if err != nil {
						return fmt.Errorf("failed to open file %s: %w", filePath, err)
					}
					//nolint:errcheck // Deferred close, error not actionable
					defer file.Close()

					info, err := file.Stat()
					if err != nil {
						return fmt.Errorf("failed to stat file %s: %w", filePath, err)
					}

					if info.IsDir() {
						return nil
					}

					hdr, err := tar.FileInfoHeader(info, "")
					if err != nil {
						return fmt.Errorf("failed to create tar header: %w", err)
					}
					hdr.Name = filePath

					if err := tw.WriteHeader(hdr); err != nil {
						return fmt.Errorf("failed to write tar header: %w", err)
					}

					if _, err := io.Copy(tw, file); err != nil {
						return fmt.Errorf("failed to copy file to tar: %w", err)
					}

					return nil
				}()

				if err != nil {
					return err
				}
			}
		}

		if err := tw.Close(); err != nil {
			return fmt.Errorf("failed to close tar writer: %w", err)
		}

		return nil
	}()

	if err != nil {
		os.Remove(tarPath) //nolint:errcheck,gosec // Best effort cleanup of temp file, error not actionable
		return ui.NewFileSystemError(err)
	}

	// Get tar size
	info, err := os.Stat(tarPath)
	if err != nil {
		return ui.NewFileSystemError(fmt.Errorf("failed to stat tar file: %w", err))
	}

	// Check tar size limit
	if info.Size() > maxTarSize {
		os.Remove(tarPath) //nolint:errcheck,gosec // Best effort cleanup of temp file, error not actionable
		return ui.NewValidationError(fmt.Errorf("tar file size (%dMB) exceeds 4MB limit for cerebrium run", info.Size()/(1024*1024)))
	}

	return tarCreatedMsg{
		tarPath: tarPath,
		tarSize: info.Size(),
	}
}

func (m *RunView) uploadRun() tea.Msg {
	// Build hardware config
	hardwareConfig := make(map[string]any)
	if m.conf.Config != nil {
		if m.conf.Config.Hardware.Compute != nil && *m.conf.Config.Hardware.Compute != "" {
			hardwareConfig["computeType"] = *m.conf.Config.Hardware.Compute
		}
		if m.conf.Config.Hardware.GPUCount != nil && *m.conf.Config.Hardware.GPUCount > 0 {
			hardwareConfig["gpuCount"] = *m.conf.Config.Hardware.GPUCount
		}
		if m.conf.Config.Hardware.CPU != nil && *m.conf.Config.Hardware.CPU > 0 {
			hardwareConfig["cpu"] = *m.conf.Config.Hardware.CPU
		}
		if m.conf.Config.Hardware.Memory != nil && *m.conf.Config.Hardware.Memory > 0 {
			hardwareConfig["memoryGb"] = *m.conf.Config.Hardware.Memory
		}
	}

	// Check data size limit (2MB)
	dataBytes, err := json.Marshal(m.jsonData)
	if err != nil {
		return ui.NewValidationError(fmt.Errorf("failed to marshal data: %w", err))
	}
	maxDataSize := 2 * 1024 * 1024
	if len(dataBytes) > maxDataSize {
		return ui.NewValidationError(fmt.Errorf("JSON data size (%dMB) exceeds 2MB limit", len(dataBytes)/(1024*1024)))
	}

	// Upload and run
	resp, err := m.conf.Client.RunApp(
		m.ctx,
		m.conf.ProjectID,
		m.appID,
		m.region,
		m.conf.Filename,
		m.conf.FunctionName,
		m.imageDigest,
		hardwareConfig,
		m.tarPath,
		m.jsonData,
	)
	if err != nil {
		return ui.NewAPIError(fmt.Errorf("failed to run app: %w", err))
	}

	// Clean up tar file after successful upload
	os.Remove(m.tarPath) //nolint:errcheck,gosec // Best effort cleanup of temp file, error not actionable
	m.tarPath = ""

	return runUploadedMsg{runID: resp.RunID}
}

func (m *RunView) checkRunStatus() tea.Cmd {
	return func() tea.Msg {
		status, err := m.conf.Client.GetRunStatus(
			m.ctx,
			m.conf.ProjectID,
			m.appName,
			m.runID,
		)
		if err != nil {
			// Status check errors are not fatal
			return nil
		}

		return runStatusMsg{status: strings.ToLower(status.Item.Status)}
	}
}

// Helper functions

func formatRunSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

//go:embed inject_main.py
var injectedPyContent string

// getInjectMainContent returns the Python code to inject for main guard
func getInjectMainContent() string {
	return injectedPyContent
}
