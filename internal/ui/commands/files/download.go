package files

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type DownloadState int

const (
	DownloadStateCheckingPath DownloadState = iota
	DownloadStateCollectingFiles
	DownloadStateDownloadingFiles
	DownloadStateSuccess
	DownloadStateError
)

type DownloadConfig struct {
	ui.DisplayConfig

	Client     api.Client
	Config     *config.Config
	Region     string
	RemotePath string
	LocalPath  string
}

// FileDownloadView handles file and directory downloads from persistent storage
type FileDownloadView struct {
	ctx context.Context

	// State
	state       DownloadState
	spinner     *ui.SpinnerModel
	progressBar progress.Model
	err         error

	// Directory download state
	isDirectory     bool
	fileList        []fileToDownload
	totalSize       int64
	downloadedBytes int64
	currentFile     string
	filesDownloaded int
	startTime       time.Time
	downloadSpeed   float64 // Cached download speed in bytes/sec

	atomicBytesDownloaded *atomic.Int64

	conf DownloadConfig
}

type fileToDownload struct {
	remotePath string
	localPath  string
	size       int64
}

// NewFileDownloadView creates a new file download view
func NewFileDownloadView(ctx context.Context, conf DownloadConfig) *FileDownloadView {
	prog := progress.New(
		progress.WithSolidFill("#EB3A6F"),
		progress.WithWidth(50),
		progress.WithoutPercentage(),
	)

	return &FileDownloadView{
		ctx:                   ctx,
		state:                 DownloadStateCheckingPath,
		spinner:               ui.NewSpinner(),
		progressBar:           prog,
		atomicBytesDownloaded: &atomic.Int64{},
		conf:                  conf,
	}
}

// Init starts the download process

// Error returns the error if any occurred during execution
func (m *FileDownloadView) Error() error {
	return m.err
}

func (m *FileDownloadView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.checkPathType)
}

// Update handles messages
func (m *FileDownloadView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case ui.SignalCancelMsg:
		return m.onCancel()

	case pathTypeMsg:
		return m.onPathType(v)

	case filesCollectedMsg:
		return m.onFilesCollected(v)

	case fileDownloadedMsg:
		return m.onFileDownloaded(v)

	case allFilesDownloadedMsg:
		return m.onAllFilesDownloaded()

	case downloadProgressTickMsg:
		if m.state == DownloadStateDownloadingFiles && m.atomicBytesDownloaded != nil {
			m.downloadedBytes = m.atomicBytesDownloaded.Load()

			// Calculate download speed (only if we have downloaded data)
			if !m.startTime.IsZero() && m.downloadedBytes > 0 {
				elapsed := time.Since(m.startTime).Seconds()
				if elapsed > 0 {
					m.downloadSpeed = float64(m.downloadedBytes) / elapsed
				}
			}

			return m, m.tickProgress()
		}
		return m, nil

	case *ui.UIError:
		return m.onError(v)

	case tea.KeyMsg:
		return m.onKey(v)

	default:
		return m.onDefault(msg)
	}
}

// Message handlers

func (m *FileDownloadView) onCancel() (tea.Model, tea.Cmd) {
	if m.conf.SimpleOutput() {
		fmt.Fprintf(os.Stderr, "\nDownload cancelled by user\n")
	}
	m.state = DownloadStateError
	m.err = ui.NewUserCancelledError()
	return m, tea.Quit
}

func (m *FileDownloadView) onPathType(msg pathTypeMsg) (tea.Model, tea.Cmd) {
	m.isDirectory = msg.isDirectory

	if m.isDirectory {
		m.state = DownloadStateCollectingFiles
		if m.conf.SimpleOutput() {
			fmt.Printf("Collecting files from directory %s...\n", m.conf.RemotePath)
		}
		return m, m.collectFiles
	} else {
		// Single file download
		m.fileList = []fileToDownload{{
			remotePath: m.conf.RemotePath,
			localPath:  m.conf.LocalPath,
			size:       msg.fileSize,
		}}
		m.totalSize = msg.fileSize
		m.currentFile = filepath.Base(m.conf.RemotePath) // Set current file for progress display
		m.state = DownloadStateDownloadingFiles
		m.startTime = time.Now()

		if m.conf.SimpleOutput() {
			fmt.Printf("Downloading %s (%s)...\n", m.conf.RemotePath, FormatBytes(m.totalSize))
		}

		return m, tea.Batch(
			m.downloadNextFile(0),
			m.tickProgress(),
		)
	}
}

func (m *FileDownloadView) onFilesCollected(msg filesCollectedMsg) (tea.Model, tea.Cmd) {
	m.fileList = msg.files
	m.totalSize = msg.totalSize

	// Check for empty file list (defensive programming)
	if len(m.fileList) == 0 {
		errMsg := fmt.Errorf("no files found to download from directory: %s", m.conf.RemotePath)
		if m.conf.SimpleOutput() {
			fmt.Printf("Error: %s\n", errMsg.Error())
		}
		m.state = DownloadStateError
		m.err = ui.NewFileSystemError(errMsg)
		return m, tea.Quit
	}

	m.state = DownloadStateDownloadingFiles
	m.startTime = time.Now()

	if m.conf.SimpleOutput() {
		fmt.Printf("Found %d files to download (%s)\n", len(m.fileList), FormatBytes(m.totalSize))
		fmt.Printf("Downloading files...\n")
	}

	m.currentFile = filepath.Base(m.fileList[0].remotePath)

	return m, tea.Batch(
		m.downloadNextFile(0),
		m.tickProgress(),
	)
}

func (m *FileDownloadView) onFileDownloaded(msg fileDownloadedMsg) (tea.Model, tea.Cmd) {
	m.filesDownloaded++
	m.currentFile = msg.fileName

	if m.conf.SimpleOutput() && m.isDirectory {
		fmt.Printf("✓ Downloaded %s (%d/%d)\n", msg.fileName, m.filesDownloaded, len(m.fileList))
	}

	if m.filesDownloaded < len(m.fileList) {
		return m, m.downloadNextFile(m.filesDownloaded)
	}

	return m, func() tea.Msg { return allFilesDownloadedMsg{} }
}

func (m *FileDownloadView) onAllFilesDownloaded() (tea.Model, tea.Cmd) {
	m.state = DownloadStateSuccess

	if m.conf.SimpleOutput() {
		if m.isDirectory {
			fmt.Printf("\n✓ Download completed successfully! Downloaded %d files (%s)\n",
				len(m.fileList), FormatBytes(m.totalSize))
		} else {
			fmt.Printf("\n✓ Download completed successfully!\n")
		}
	}

	return m, tea.Quit
}

func (m *FileDownloadView) onError(err *ui.UIError) (tea.Model, tea.Cmd) {
	err.SilentExit = true
	m.err = err
	m.state = DownloadStateError

	if m.conf.SimpleOutput() {
		fmt.Printf("Error: %s\n", err.Error())
	}

	return m, tea.Quit
}

func (m *FileDownloadView) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.conf.SimpleOutput() {
		return m, nil
	}

	switch msg.String() {
	case "ctrl+c", "q", "esc":
		return m.onCancel()
	}

	return m, nil
}

func (m *FileDownloadView) onDefault(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.conf.SimpleOutput() {
		return m, nil
	}

	var cmd tea.Cmd
	var spinnerModel tea.Model
	spinnerModel, cmd = m.spinner.Update(msg)
	m.spinner = spinnerModel.(*ui.SpinnerModel) //nolint:errcheck // Type assertion guaranteed by SpinnerModel structure
	return m, cmd
}

// View renders the output
func (m *FileDownloadView) View() string {
	if m.conf.SimpleOutput() {
		return ""
	}

	var output strings.Builder

	switch m.state {
	case DownloadStateCheckingPath:
		output.WriteString(m.spinner.View() + " Checking remote path...\n")

	case DownloadStateCollectingFiles:
		output.WriteString("✓ " + ui.SuccessStyle.Render("Path verified") + "\n")
		output.WriteString(m.spinner.View() + " Collecting files from directory...\n")

	case DownloadStateDownloadingFiles:
		if m.isDirectory {
			output.WriteString("✓ " + ui.SuccessStyle.Render("Path verified") + "\n")
			output.WriteString("✓ " + ui.SuccessStyle.Render(fmt.Sprintf("Found %d files", len(m.fileList))) + "\n\n")
		}

		if m.currentFile != "" {
			downloadingText := lipgloss.NewStyle().
				Foreground(lipgloss.Color("12")).
				Render(fmt.Sprintf("↓ Downloading: %s", m.currentFile))
			output.WriteString(fmt.Sprintf("%s %s\n\n", m.spinner.View(), downloadingText))
		} else {
			output.WriteString(fmt.Sprintf("%s Downloading files...\n\n", m.spinner.View()))
		}

		progressPercent := float64(0)
		if m.totalSize > 0 {
			progressPercent = float64(m.downloadedBytes) / float64(m.totalSize)
		}

		progressView := m.progressBar.ViewAs(progressPercent)
		percentage := int(progressPercent * 100)

		percentStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)
		output.WriteString(fmt.Sprintf("  %s %s\n\n", progressView, percentStyle.Render(fmt.Sprintf("%3d%%", percentage))))

		statsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		var stats []string

		downloaded := FormatBytes(m.downloadedBytes)
		total := FormatBytes(m.totalSize)
		stats = append(stats, fmt.Sprintf("%s / %s", downloaded, total))

		// Use cached download speed
		if m.downloadSpeed > 0 {
			stats = append(stats, fmt.Sprintf("%s/s", FormatBytes(int64(m.downloadSpeed))))

			if m.downloadedBytes < m.totalSize {
				remaining := float64(m.totalSize-m.downloadedBytes) / m.downloadSpeed
				eta := time.Duration(remaining) * time.Second
				stats = append(stats, fmt.Sprintf("ETA %s", eta))
			}
		}

		if m.isDirectory {
			fileCounter := fmt.Sprintf("%d/%d files", m.filesDownloaded, len(m.fileList))
			stats = append(stats, fileCounter)
		}

		output.WriteString("  " + statsStyle.Render(strings.Join(stats, " • ")) + "\n")

	case DownloadStateSuccess:
		if m.isDirectory {
			output.WriteString("✓ " + ui.SuccessStyle.Render("Path verified") + "\n")
			output.WriteString("✓ " + ui.SuccessStyle.Render(fmt.Sprintf("Found %d files", len(m.fileList))) + "\n")
		}
		output.WriteString("✓ " + ui.SuccessStyle.Render("Download completed") + "\n\n")

		var successMsg string
		if m.isDirectory {
			successMsg = fmt.Sprintf("Successfully downloaded %d files (%s) to %s",
				len(m.fileList), FormatBytes(m.totalSize), m.conf.LocalPath)
		} else {
			successMsg = fmt.Sprintf("Successfully downloaded to %s", m.conf.LocalPath)
		}
		output.WriteString(ui.SuccessStyle.Render(successMsg))

	case DownloadStateError:
		if m.err != nil {
			output.WriteString("\n" + ui.FormatError(m.err))
		}
	}

	return output.String()
}

// Messages

type pathTypeMsg struct {
	isDirectory bool
	fileSize    int64
}

type filesCollectedMsg struct {
	files     []fileToDownload
	totalSize int64
}

type fileDownloadedMsg struct {
	fileName string
	fileSize int64
}

type allFilesDownloadedMsg struct{}

type downloadProgressTickMsg time.Time

// Commands (async operations)

func (m *FileDownloadView) checkPathType() tea.Msg {
	// Try to find item in parent directory listing first (provides metadata)
	if fileInfo := m.findFileInParentDirectory(); fileInfo != nil {
		return pathTypeMsg{
			isDirectory: fileInfo.IsFolder,
			fileSize:    fileInfo.SizeBytes,
		}
	}

	// Not in parent listing - use trailing slash to determine type
	if strings.HasSuffix(m.conf.RemotePath, "/") {
		if m.tryListAsDirectory() {
			return pathTypeMsg{isDirectory: true, fileSize: 0}
		}
		return ui.NewAPIError(fmt.Errorf("directory not found: %s", m.conf.RemotePath))
	}

	// No trailing slash - verify file exists via download URL
	_, err := m.conf.Client.GetDownloadURL(m.ctx, m.conf.Config.ProjectID, m.conf.RemotePath, m.conf.Region)
	if err != nil {
		return ui.NewAPIError(fmt.Errorf("file not found: %s", m.conf.RemotePath))
	}

	return pathTypeMsg{isDirectory: false, fileSize: 0}
}

// findFileInParentDirectory looks for the target in its parent directory listing.
func (m *FileDownloadView) findFileInParentDirectory() *api.FileInfo {
	parentDir := filepath.Dir(m.conf.RemotePath)
	fileName := filepath.Base(m.conf.RemotePath)

	if parentDir == "." {
		parentDir = "/"
	}

	parentFiles, err := m.conf.Client.ListFiles(m.ctx, m.conf.Config.ProjectID, parentDir, m.conf.Region)
	if err != nil {
		return nil
	}

	for _, file := range parentFiles {
		if file.Name == fileName || file.Name == fileName+"/" {
			return &file
		}
	}

	return nil
}

// tryListAsDirectory checks if the path is a non-empty directory.
func (m *FileDownloadView) tryListAsDirectory() bool {
	files, err := m.conf.Client.ListFiles(m.ctx, m.conf.Config.ProjectID, m.conf.RemotePath, m.conf.Region)
	return err == nil && len(files) > 0
}

func (m *FileDownloadView) collectFiles() tea.Msg {
	var files []fileToDownload
	var totalSize int64

	// Recursively collect all files
	err := m.collectFilesRecursive(m.conf.RemotePath, m.conf.LocalPath, &files, &totalSize)
	if err != nil {
		return ui.NewAPIError(fmt.Errorf("failed to collect files: %w", err))
	}

	if len(files) == 0 {
		return ui.NewAPIError(fmt.Errorf("no files found in directory: %s", m.conf.RemotePath))
	}

	return filesCollectedMsg{files: files, totalSize: totalSize}
}

func (m *FileDownloadView) collectFilesRecursive(remotePath, localPath string, files *[]fileToDownload, totalSize *int64) error {
	fileList, err := m.conf.Client.ListFiles(m.ctx, m.conf.Config.ProjectID, remotePath, m.conf.Region)
	if err != nil {
		return err
	}

	for _, file := range fileList {
		remoteFilePath := remotePath
		if !strings.HasSuffix(remoteFilePath, "/") {
			remoteFilePath += "/"
		}
		remoteFilePath += file.Name

		localFilePath := filepath.Join(localPath, file.Name)

		if file.IsFolder {
			// Recursively collect files from subdirectory
			err := m.collectFilesRecursive(remoteFilePath, localFilePath, files, totalSize)
			if err != nil {
				return err
			}
		} else {
			*files = append(*files, fileToDownload{
				remotePath: remoteFilePath,
				localPath:  localFilePath,
				size:       file.SizeBytes,
			})
			*totalSize += file.SizeBytes
		}
	}

	return nil
}

func (m *FileDownloadView) downloadNextFile(index int) tea.Cmd {
	if index >= len(m.fileList) {
		return func() tea.Msg { return allFilesDownloadedMsg{} }
	}

	file := m.fileList[index]
	fileName := filepath.Base(file.remotePath)

	return func() tea.Msg {
		err := m.downloadSingleFileWithProgress(file, m.atomicBytesDownloaded)
		if err != nil {
			return ui.NewAPIError(fmt.Errorf("failed to download %s: %w", file.remotePath, err))
		}

		return fileDownloadedMsg{
			fileName: fileName,
			fileSize: file.size,
		}
	}
}

func (m *FileDownloadView) downloadSingleFileWithProgress(file fileToDownload, atomicCounter *atomic.Int64) error {
	// Get download URL
	downloadURL, err := m.conf.Client.GetDownloadURL(m.ctx, m.conf.Config.ProjectID, file.remotePath, m.conf.Region)
	if err != nil {
		return fmt.Errorf("failed to get download URL: %w", err)
	}

	// Download the file
	req, err := http.NewRequestWithContext(m.ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // Deferred close, error not actionable

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(file.localPath), 0755); err != nil { //nolint:gosec // Download directory needs standard permissions
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create local file
	out, err := os.Create(file.localPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close() //nolint:errcheck // Deferred close, error not actionable

	// Copy data with progress tracking
	buf := make([]byte, downloadBufferSize)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("failed to write file: %w", writeErr)
			}
			if atomicCounter != nil {
				atomicCounter.Add(int64(n))
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read response: %w", err)
		}
	}

	return nil
}

func (m *FileDownloadView) tickProgress() tea.Cmd {
	return tea.Tick(progressUpdateInterval, func(t time.Time) tea.Msg {
		return downloadProgressTickMsg(t)
	})
}
