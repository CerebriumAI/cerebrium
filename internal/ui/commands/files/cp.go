package files

import (
	"context"
	"fmt"
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

type CpState int

const (
	StatePreparingFiles CpState = iota
	StateUploadingFiles
	StateSuccess
	StateError
)

type CpConfig struct {
	ui.DisplayConfig

	Client     api.Client
	Config     *config.Config
	Region     string
	LocalPath  string
	RemotePath string
}

// FileUploadView handles file and directory uploads to persistent storage
type FileUploadView struct {
	ctx context.Context

	state         CpState
	fileList      []fileToUpload
	totalSize     int64
	uploadedBytes int64
	currentFile   string
	filesUploaded int
	startTime     time.Time
	uploadSpeed   float64 // Cached upload speed in bytes/sec
	spinner       *ui.SpinnerModel
	progressBar   progress.Model
	err           error

	atomicBytesUploaded *atomic.Int64
	lastPrintedPercent  int // Track last printed percentage for SimpleOutput

	conf CpConfig
}

type fileToUpload struct {
	localPath  string
	remotePath string
	size       int64
}

func NewFileUploadView(ctx context.Context, conf CpConfig) *FileUploadView {
	prog := progress.New(
		progress.WithSolidFill("#EB3A6F"),
		progress.WithWidth(50),
		progress.WithoutPercentage(),
	)

	return &FileUploadView{
		ctx:                 ctx,
		state:               StatePreparingFiles,
		spinner:             ui.NewSpinner(),
		progressBar:         prog,
		atomicBytesUploaded: &atomic.Int64{},
		conf:                conf,
	}
}

// Error returns the error if any occurred during execution
func (m *FileUploadView) Error() error {
	return m.err
}

func (m *FileUploadView) Init() tea.Cmd {
	return tea.Batch(m.spinner.Init(), m.prepareFiles)
}

func (m *FileUploadView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch v := msg.(type) {
	case ui.SignalCancelMsg:
		return m.onCancel()

	case filesPreparedMsg:
		return m.onFilesPrepared(v)

	case fileUploadedMsg:
		return m.onFileUploaded(v)

	case allFilesUploadedMsg:
		return m.onAllFilesUploaded()

	case progressTickMsg:
		if m.state == StateUploadingFiles && m.atomicBytesUploaded != nil {
			m.uploadedBytes = m.atomicBytesUploaded.Load()

			// Calculate upload speed (only if we have uploaded data)
			if !m.startTime.IsZero() && m.uploadedBytes > 0 {
				elapsed := time.Since(m.startTime).Seconds()
				if elapsed > 0 {
					m.uploadSpeed = float64(m.uploadedBytes) / elapsed
				}
			}

			// In SimpleOutput mode, print progress every 10%
			if m.conf.SimpleOutput() && m.totalSize > 0 {
				currentPercent := int((float64(m.uploadedBytes) / float64(m.totalSize)) * 100)
				// Round down to nearest 10%
				percentDecile := (currentPercent / 10) * 10

				// Print if we've crossed a 10% threshold
				if percentDecile > m.lastPrintedPercent && percentDecile <= 100 {
					m.lastPrintedPercent = percentDecile
					m.printSimpleProgress(percentDecile)
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

func (m *FileUploadView) onCancel() (tea.Model, tea.Cmd) {
	if m.conf.SimpleOutput() {
		fmt.Fprintf(os.Stderr, "\nUpload cancelled by user\n")
	}
	m.state = StateError
	m.err = ui.NewUserCancelledError()
	return m, tea.Quit
}

func (m *FileUploadView) onFilesPrepared(msg filesPreparedMsg) (tea.Model, tea.Cmd) {
	m.fileList = msg.files
	m.totalSize = msg.totalSize

	// Check for empty file list
	if len(m.fileList) == 0 {
		errMsg := fmt.Errorf("no files found to upload")
		if m.conf.SimpleOutput() {
			fmt.Printf("Error: %s\n", errMsg.Error())
		}
		m.state = StateError
		m.err = ui.NewFileSystemError(errMsg)
		return m, tea.Quit
	}

	m.state = StateUploadingFiles
	m.startTime = time.Now()
	m.lastPrintedPercent = 0 // Reset progress tracking

	if m.conf.SimpleOutput() {
		fmt.Printf("Uploading %d files (%s)...\n", len(m.fileList), FormatBytes(m.totalSize))
		if m.totalSize > 50*1024*1024 { // If > 50MB, show a note about initialization
			fmt.Printf("Initializing upload (this may take a moment for large files)...\n")
		}
	}

	m.currentFile = filepath.Base(m.fileList[0].localPath)

	return m, tea.Batch(
		m.uploadNextFile(0),
		m.tickProgress(),
	)
}

func (m *FileUploadView) onFileUploaded(msg fileUploadedMsg) (tea.Model, tea.Cmd) {
	m.filesUploaded++

	if m.conf.SimpleOutput() {
		fmt.Printf("✓ Uploaded %s (%d/%d)\n", msg.fileName, m.filesUploaded, len(m.fileList))
	}

	if m.filesUploaded < len(m.fileList) {
		m.currentFile = filepath.Base(m.fileList[m.filesUploaded].localPath)
		return m, m.uploadNextFile(m.filesUploaded)
	}

	return m, func() tea.Msg { return allFilesUploadedMsg{} }
}

func (m *FileUploadView) onAllFilesUploaded() (tea.Model, tea.Cmd) {
	m.state = StateSuccess

	if m.conf.SimpleOutput() {
		// Ensure we show 100% if we haven't already
		if m.lastPrintedPercent < 100 {
			m.uploadedBytes = m.totalSize
			m.printSimpleProgress(100)
		}
		fmt.Printf("✓ Upload completed successfully! Total: %s\n", FormatBytes(m.totalSize))
	}

	return m, tea.Quit
}

func (m *FileUploadView) onError(err *ui.UIError) (tea.Model, tea.Cmd) {
	err.SilentExit = true
	m.err = err
	m.state = StateError

	if m.conf.SimpleOutput() {
		fmt.Printf("Error: %s\n", err.Error())
	}

	return m, tea.Quit
}

func (m *FileUploadView) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.conf.SimpleOutput() {
		return m, nil
	}

	switch msg.String() {
	case "q", "esc", tea.KeyCtrlC.String():
		return m.onCancel()
	}

	return m, nil
}

func (m *FileUploadView) onDefault(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.conf.SimpleOutput() {
		return m, nil
	}

	var cmd tea.Cmd
	var spinnerModel tea.Model
	spinnerModel, cmd = m.spinner.Update(msg)
	m.spinner = spinnerModel.(*ui.SpinnerModel) //nolint:errcheck // Type assertion guaranteed by SpinnerModel structure
	return m, cmd
}

type filesPreparedMsg struct {
	files     []fileToUpload
	totalSize int64
}

type fileUploadedMsg struct {
	fileName string
	fileSize int64
}

type allFilesUploadedMsg struct{}

type progressTickMsg time.Time

func (m *FileUploadView) prepareFiles() tea.Msg {
	info, err := os.Stat(m.conf.LocalPath)
	if err != nil {
		if os.IsNotExist(err) {
			return ui.NewFileSystemError(fmt.Errorf("source path does not exist: %s", m.conf.LocalPath))
		}
		return ui.NewFileSystemError(fmt.Errorf("failed to access source path: %w", err))
	}

	var files []fileToUpload
	var totalSize int64

	if info.IsDir() {
		err := filepath.Walk(m.conf.LocalPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				relPath, _ := filepath.Rel(m.conf.LocalPath, path)
				remotePath := filepath.Join(m.conf.RemotePath, relPath)
				remotePath = filepath.ToSlash(remotePath)

				files = append(files, fileToUpload{
					localPath:  path,
					remotePath: remotePath,
					size:       info.Size(),
				})
				totalSize += info.Size()
			}
			return nil
		})
		if err != nil {
			return ui.NewFileSystemError(fmt.Errorf("failed to walk directory: %w", err))
		}
	} else {
		files = append(files, fileToUpload{
			localPath:  m.conf.LocalPath,
			remotePath: m.conf.RemotePath,
			size:       info.Size(),
		})
		totalSize = info.Size()
	}

	if len(files) == 0 {
		return ui.NewFileSystemError(fmt.Errorf("no files to upload"))
	}

	return filesPreparedMsg{files: files, totalSize: totalSize}
}

func (m *FileUploadView) uploadNextFile(index int) tea.Cmd {
	if index >= len(m.fileList) {
		return func() tea.Msg { return allFilesUploadedMsg{} }
	}

	file := m.fileList[index]
	fileName := filepath.Base(file.localPath)

	return func() tea.Msg {
		fileSize, err := m.uploadSingleFile(file, m.atomicBytesUploaded)
		if err != nil {
			return ui.NewAPIError(fmt.Errorf("failed to upload %s: %w", file.localPath, err))
		}

		return fileUploadedMsg{
			fileName: fileName,
			fileSize: fileSize,
		}
	}
}

func (m *FileUploadView) View() string {
	if m.conf.SimpleOutput() {
		return ""
	}

	var output strings.Builder

	switch m.state {
	case StatePreparingFiles:
		output.WriteString(m.spinner.View() + " Preparing files for upload...\n")

	case StateUploadingFiles:
		output.WriteString("✓ " + ui.SuccessStyle.Render("Prepared files") + "\n\n")

		// Show different message based on upload progress
		if m.uploadedBytes == 0 && m.totalSize > 0 {
			// Initial state - no bytes uploaded yet
			preparingText := lipgloss.NewStyle().
				Foreground(lipgloss.Color("11")).
				Render(fmt.Sprintf("⚡ Initializing upload of %s...", FormatBytes(m.totalSize)))
			output.WriteString(fmt.Sprintf("%s %s\n\n", m.spinner.View(), preparingText))
		} else {
			// Actively uploading
			uploadingText := lipgloss.NewStyle().
				Foreground(lipgloss.Color("12")).
				Render(fmt.Sprintf("↑ Uploading: %s", m.currentFile))
			output.WriteString(fmt.Sprintf("%s %s\n\n", m.spinner.View(), uploadingText))
		}

		progressPercent := float64(0)
		if m.totalSize > 0 {
			progressPercent = float64(m.uploadedBytes) / float64(m.totalSize)
		}

		progressView := m.progressBar.ViewAs(progressPercent)
		percentage := int(progressPercent * 100)

		percentStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)
		output.WriteString(fmt.Sprintf("  %s %s\n\n", progressView, percentStyle.Render(fmt.Sprintf("%3d%%", percentage))))

		statsStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		var stats []string

		uploaded := FormatBytes(m.uploadedBytes)
		total := FormatBytes(m.totalSize)
		stats = append(stats, fmt.Sprintf("%s / %s", uploaded, total))

		// Use cached upload speed
		if m.uploadSpeed > 0 {
			stats = append(stats, fmt.Sprintf("%s/s", FormatBytes(int64(m.uploadSpeed))))

			if m.uploadedBytes < m.totalSize {
				remaining := float64(m.totalSize-m.uploadedBytes) / m.uploadSpeed
				eta := time.Duration(remaining) * time.Second
				stats = append(stats, fmt.Sprintf("ETA %s", eta))
			}
		}

		fileCounter := fmt.Sprintf("%d/%d files", m.filesUploaded, len(m.fileList))
		stats = append(stats, fileCounter)

		output.WriteString("  " + statsStyle.Render(strings.Join(stats, " • ")) + "\n")

	case StateSuccess:
		output.WriteString("✓ " + ui.SuccessStyle.Render("Prepared files") + "\n")
		output.WriteString("✓ " + ui.SuccessStyle.Render("Upload completed") + "\n\n")
		successMsg := fmt.Sprintf("Successfully uploaded %d files (%s)", len(m.fileList), FormatBytes(m.totalSize))
		output.WriteString(ui.SuccessStyle.Render(successMsg))

	case StateError:
		if m.err != nil {
			output.WriteString("\n" + ui.FormatError(m.err))
		}
	}

	return output.String()
}

func (m *FileUploadView) tickProgress() tea.Cmd {
	return tea.Tick(fastProgressUpdate, func(t time.Time) tea.Msg {
		return progressTickMsg(t)
	})
}

// printSimpleProgress prints a simple progress bar for non-TTY mode
func (m *FileUploadView) printSimpleProgress(percent int) {
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
	uploaded := FormatBytes(m.uploadedBytes)
	total := FormatBytes(m.totalSize)
	stats := fmt.Sprintf("%d%% (%s / %s)", percent, uploaded, total)

	// Add speed if available (use cached speed)
	if m.uploadSpeed > 0 {
		stats += fmt.Sprintf(" • %s/s", FormatBytes(int64(m.uploadSpeed)))
	}

	// Add file counter
	stats += fmt.Sprintf(" • %d/%d files", m.filesUploaded, len(m.fileList))

	fmt.Printf("%s %s\n", bar.String(), stats)
}
