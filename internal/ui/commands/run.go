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
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/cerebriumai/cerebrium/pkg/projectconfig"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	RunStateSuccess
	RunStateError
)

const (
	// maxTarSize is the maximum allowed tar file size for cerebrium run (4MB)
	maxTarSize = 4 * 1024 * 1024
	// maxDepsSize is the maximum allowed dependencies size (380KB)
	maxDepsSize = 380 * 1024
	// finalLogPollAttempts is the number of times to poll for final logs
	finalLogPollAttempts = 20
	// maxLogsToDisplay is the maximum number of logs to display in the UI
	maxLogsToDisplay = 20
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
	message string

	fileList        []string
	tarPath         string
	tarSize         int64
	imageDigest     *string
	runID           string
	appName         string
	appID           string
	logs            []string
	seenLogIDs      map[string]bool
	printedLogCount int // Track how many logs we've printed in non-TTY mode
	runStatus       string
	runCompleted    bool // Track if we've already handled completion
	needsInjection  bool
	region          string
	hardwareInfo    string
	jsonData        map[string]any
	nextLogToken    string // Track pagination token for logs

	conf RunConfig
}

// NewRunView creates a new run view
func NewRunView(ctx context.Context, conf RunConfig) *RunView {
	return &RunView{
		ctx:        ctx,
		state:      RunStatePreparingFiles,
		spinner:    ui.NewSpinner(),
		seenLogIDs: make(map[string]bool),
		conf:       conf,
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

	case logsReceivedMsg:
		return m.handleLogsReceived(msg)

	case runStatusMsg:
		return m.handleRunStatus(msg)

	case finalLogsMsg:
		return m.handleFinalLogs()

	case pollFinalLogsMsg:
		return m.handlePollFinalLogs()

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

	if m.conf.SimpleOutput() {
		fmt.Printf("✓ Prepared %d files\n", len(msg.fileList))
	}

	if m.conf.Config != nil && (len(m.conf.Config.Dependencies.Pip) > 0 ||
		len(m.conf.Config.Dependencies.Conda) > 0 ||
		len(m.conf.Config.Dependencies.Apt) > 0) {
		m.state = RunStateCheckingDependencies

		if m.conf.SimpleOutput() {
			fmt.Println("Checking dependencies...")
		}

		return m, m.createBaseImage
	}

	m.state = RunStateCreatingApp
	return m, m.createApp
}

func (m *RunView) handleBaseImageCreated(msg baseImageCreatedMsg) (tea.Model, tea.Cmd) {
	m.imageDigest = &msg.imageDigest
	m.state = RunStateCreatingApp

	if m.conf.SimpleOutput() {
		fmt.Printf("✓ Base image ready: %s\n", msg.imageDigest)
	}

	return m, m.createApp
}

func (m *RunView) handleAppCreated() (tea.Model, tea.Cmd) {
	m.state = RunStateCreatingTar

	if m.conf.SimpleOutput() {
		fmt.Printf("✓ Created run app: %s%s\n", m.appName, m.hardwareInfo)
	}

	return m, m.createTar
}

func (m *RunView) handleTarCreated(msg tarCreatedMsg) (tea.Model, tea.Cmd) {
	m.tarPath = msg.tarPath
	m.tarSize = msg.tarSize
	m.state = RunStateUploadingRun

	if m.conf.SimpleOutput() {
		fmt.Printf("✓ Created tar file (%s)\n", formatRunSize(msg.tarSize))
	}

	return m, m.uploadRun
}

func (m *RunView) handleRunUploaded(msg runUploadedMsg) (tea.Model, tea.Cmd) {
	m.runID = msg.runID
	m.state = RunStateExecuting

	if m.conf.SimpleOutput() {
		fmt.Println("✓ App uploaded successfully!")
		fmt.Println("Executing...")
	}

	return m, tea.Batch(
		m.pollLogs(),
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return pollLogsTickMsg(t)
		}),
	)
}

func (m *RunView) handleLogsReceived(msg logsReceivedMsg) (tea.Model, tea.Cmd) {
	for _, log := range msg.logs {
		if !m.seenLogIDs[log.LogID] {
			m.seenLogIDs[log.LogID] = true
			formattedLog := m.formatLog(log)
			m.logs = append(m.logs, formattedLog)

			// Don't print logs immediately in non-TTY mode
			// We'll print them all at once after "Waiting for logs..." message
		}
	}

	m.nextLogToken = msg.nextToken

	return m, nil
}

func (m *RunView) handleRunStatus(msg runStatusMsg) (tea.Model, tea.Cmd) {
	m.runStatus = msg.status

	if !m.runCompleted && (msg.status == "success" || msg.status == "failed" || msg.status == "fail") {
		m.runCompleted = true // Mark as completed to avoid reprocessing

		// Store the completion message but keep state as polling for logs
		if msg.status == "success" {
			m.message = "✓ Run completed successfully."

			if m.conf.SimpleOutput() {
				fmt.Println("\n✓ Run completed successfully.")
				fmt.Println("Waiting for logs... (Ctrl+C to exit)")
			}
		} else {
			m.message = "✗ Run failed."

			if m.conf.SimpleOutput() {
				fmt.Println("\n✗ Run failed.")
				fmt.Println("Waiting for logs... (Ctrl+C to exit)")
			}
		}

		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
			return finalLogsMsg{}
		})
	}

	return m, nil
}

func (m *RunView) handleFinalLogs() (tea.Model, tea.Cmd) {
	// In non-TTY mode, print all buffered logs that haven't been printed yet
	if m.conf.SimpleOutput() && m.printedLogCount < len(m.logs) {
		for i := m.printedLogCount; i < len(m.logs); i++ {
			fmt.Println(m.logs[i])
		}
		m.printedLogCount = len(m.logs)
	}

	// Fetch logs multiple times with delays to ensure we get all remaining logs
	var cmds []tea.Cmd
	// Poll multiple times like Python CLI
	for range finalLogPollAttempts {
		cmds = append(cmds, m.pollLogs())
		cmds = append(cmds, tea.Tick(ui.LOG_POLL_INTERVAL, func(t time.Time) tea.Msg {
			return pollFinalLogsMsg{}
		}))
	}

	// Set the final state only right before quitting
	cmds = append(cmds, func() tea.Msg {
		if m.runStatus == "success" {
			m.state = RunStateSuccess
		} else {
			m.state = RunStateError
			err := ui.NewAPIError(fmt.Errorf("run failed"))
			err.SilentExit = true
			m.err = err
		}
		return nil
	})

	// Give a moment to render the final state with all logs
	cmds = append(cmds, tea.Tick(200*time.Millisecond, func(t time.Time) tea.Msg {
		return tea.Quit()
	}))

	return m, tea.Sequence(cmds...)
}

func (m *RunView) handlePollFinalLogs() (tea.Model, tea.Cmd) {
	// In non-TTY mode, print any new logs that have arrived
	if m.conf.SimpleOutput() && m.printedLogCount < len(m.logs) {
		for i := m.printedLogCount; i < len(m.logs); i++ {
			fmt.Println(m.logs[i])
		}
		m.printedLogCount = len(m.logs)
	}

	return m, m.pollLogs()
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
	return m, tea.Batch(
		m.pollLogs(),
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

	if m.conf.SimpleOutput() {
		fmt.Printf("Error: %s\n", msg.Error())
	}

	if m.tarPath != "" {
		//nolint:errcheck,gosec // Best effort cleanup of temp file, error not actionable
		os.Remove(m.tarPath)
	}

	return m, tea.Quit
}

func (m *RunView) handleDefault(msg tea.Msg) (tea.Model, tea.Cmd) {
	if !m.conf.SimpleOutput() {
		var cmd tea.Cmd
		spinnerModel, cmd := m.spinner.Update(msg)
		m.spinner = spinnerModel.(*ui.SpinnerModel) //nolint:errcheck // Type assertion guaranteed by SpinnerModel structure
		return m, cmd
	}
	return m, nil
}

// View renders the run view
func (m *RunView) View() string {
	if m.conf.SimpleOutput() {
		return ""
	}

	var output strings.Builder

	// Helper function to format state line (matching deploy.go pattern)
	formatStateLine := func(icon string, text string, styleFunc func(...string) string) string {
		return fmt.Sprintf("%s  %s", icon, styleFunc(text))
	}

	// State 1: Preparing files
	switch {
	case m.state == RunStatePreparingFiles:
		output.WriteString(formatStateLine(m.spinner.View(), "Preparing files...", ui.ActiveStyle.Render))
	case m.state > RunStatePreparingFiles:
		output.WriteString(formatStateLine("✓", fmt.Sprintf("Prepared %d files", len(m.fileList)), ui.SuccessStyle.Render))
	default:
		output.WriteString(formatStateLine("-", "Preparing files", ui.PendingStyle.Render))
	}
	output.WriteString("\n")

	// State 2: Dependencies (if needed)
	if m.state >= RunStateCheckingDependencies && m.imageDigest != nil {
		switch {
		case m.state == RunStateCheckingDependencies:
			output.WriteString(formatStateLine(m.spinner.View(), "Checking dependencies...", ui.ActiveStyle.Render))
		case m.state == RunStateCreatingBaseImage:
			output.WriteString(formatStateLine(m.spinner.View(), "Creating base image...", ui.ActiveStyle.Render))
		case m.state > RunStateCreatingBaseImage:
			output.WriteString(formatStateLine("✓", fmt.Sprintf("Base image ready: %s", *m.imageDigest), ui.SuccessStyle.Render))
		default:
			output.WriteString(formatStateLine("-", "Checking dependencies", ui.PendingStyle.Render))
		}
		output.WriteString("\n")
	}

	// State 3: Creating app
	if m.state >= RunStateCreatingApp {
		switch {
		case m.state == RunStateCreatingApp:
			output.WriteString(formatStateLine(m.spinner.View(), "Creating run app...", ui.ActiveStyle.Render))
		case m.state > RunStateCreatingApp:
			output.WriteString(formatStateLine("✓", fmt.Sprintf("Created run app: %s%s", m.appName, m.hardwareInfo), ui.SuccessStyle.Render))
		default:
			output.WriteString(formatStateLine("-", "Creating run app", ui.PendingStyle.Render))
		}
		output.WriteString("\n")
	}

	// State 4: Creating archive
	if m.state >= RunStateCreatingTar {
		switch {
		case m.state == RunStateCreatingTar:
			output.WriteString(formatStateLine(m.spinner.View(), "Creating archive...", ui.ActiveStyle.Render))
		case m.state > RunStateCreatingTar && m.tarSize > 0:
			output.WriteString(formatStateLine("✓", fmt.Sprintf("Created archive (%s)", formatRunSize(m.tarSize)), ui.SuccessStyle.Render))
		default:
			output.WriteString(formatStateLine("-", "Creating archive", ui.PendingStyle.Render))
		}
		output.WriteString("\n")
	}

	// State 5: Uploading
	if m.state >= RunStateUploadingRun {
		switch {
		case m.state == RunStateUploadingRun:
			output.WriteString(formatStateLine(m.spinner.View(), "Uploading to Cerebrium...", ui.ActiveStyle.Render))
		case m.state > RunStateUploadingRun:
			output.WriteString(formatStateLine("✓", "Uploaded successfully", ui.SuccessStyle.Render))
		default:
			output.WriteString(formatStateLine("-", "Uploading to Cerebrium", ui.PendingStyle.Render))
		}
		output.WriteString("\n")
	}

	// State 6: Executing and polling logs
	if m.state >= RunStateExecuting {
		switch m.state {
		case RunStateExecuting:
			output.WriteString(formatStateLine(m.spinner.View(), "Executing...", ui.ActiveStyle.Render))
		case RunStatePollingLogs:
			if m.runCompleted {
				if m.runStatus == "success" {
					output.WriteString(formatStateLine("✓", "Run completed successfully", ui.SuccessStyle.Render))
					output.WriteString("\n")
					output.WriteString(ui.HelpStyle.Render("Waiting for logs... (Ctrl+C to exit)"))
				} else {
					output.WriteString(formatStateLine("✗", "Run failed", ui.ErrorStyle.Render))
					output.WriteString("\n")
					output.WriteString(ui.HelpStyle.Render("Waiting for logs... (Ctrl+C to exit)"))
				}
			} else {
				output.WriteString(formatStateLine(m.spinner.View(), "Running...", ui.ActiveStyle.Render))
				output.WriteString("\n")
				output.WriteString(ui.HelpStyle.Render("(Ctrl+C to exit)"))
			}
		case RunStateSuccess:
			output.WriteString(formatStateLine("✓", "Run completed successfully", ui.SuccessStyle.Render))
		case RunStateError:
			output.WriteString(formatStateLine("✗", "Run failed", ui.ErrorStyle.Render))
		default:
			output.WriteString(formatStateLine("-", "Executing", ui.PendingStyle.Render))
		}
		output.WriteString("\n")
	}

	// Show logs only after run is completed in TTY mode
	if len(m.logs) > 0 && m.runCompleted {
		output.WriteString("\n")

		// Show last logs up to maxLogsToDisplay
		startIdx := 0
		if len(m.logs) > maxLogsToDisplay {
			startIdx = len(m.logs) - maxLogsToDisplay
		}

		logBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("14")).
			Width(100).
			Padding(0, 1)

		var logContent strings.Builder
		for i := startIdx; i < len(m.logs); i++ {
			logContent.WriteString(m.logs[i])
			if i < len(m.logs)-1 {
				logContent.WriteString("\n")
			}
		}

		output.WriteString(logBox.Render(logContent.String()))
		output.WriteString("\n")
	}

	// Error message
	if m.state == RunStateError && m.err != nil {
		output.WriteString("\n")
		output.WriteString(ui.FormatError(m.err))
	}

	// Success message
	if m.state == RunStateSuccess && m.message != "" {
		output.WriteString("\n")
		output.WriteString(ui.SuccessStyle.Render(m.message))
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

type logsReceivedMsg struct {
	logs      []api.RunLog
	nextToken string
}

type runStatusMsg struct {
	status string
}

type pollLogsTickMsg time.Time

type finalLogsMsg struct{}

type pollFinalLogsMsg struct{}

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

	needsInjection := false
	if m.conf.FunctionName == nil {
		content, err := os.ReadFile(m.conf.Filename)
		if err == nil {
			if !strings.Contains(string(content), `if __name__ == "__main__":`) {
				needsInjection = true
			}
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

func (m *RunView) pollLogs() tea.Cmd {
	return func() tea.Msg {
		logs, err := m.conf.Client.FetchRunLogs(
			m.ctx,
			m.conf.ProjectID,
			m.appName,
			m.runID,
			m.nextLogToken, // Use the stored pagination token
		)
		if err != nil {
			// Log polling errors are not fatal
			return nil
		}

		return logsReceivedMsg{
			logs:      logs.Logs,
			nextToken: logs.NextPageToken, // Return the next token for pagination
		}
	}
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

func (m *RunView) formatLog(log api.RunLog) string {
	// Parse timestamp
	t, err := time.Parse(time.RFC3339, log.Timestamp)
	if err != nil {
		// Try alternate formats
		t, err = time.Parse("2006-01-02T15:04:05Z", log.Timestamp)
		if err != nil {
			// Use timestamp as-is
			return fmt.Sprintf("[cyan]%s %s[/cyan]", log.Timestamp, log.LogLine)
		}
	}

	formatted := t.Local().Format("15:04:05")
	return fmt.Sprintf("%s %s", ui.CyanStyle.Render(formatted), log.LogLine)
}

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
