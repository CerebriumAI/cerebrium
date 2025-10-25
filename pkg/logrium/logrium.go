package logrium

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mattn/go-isatty"
)

// Setup configures the global slog logger based on display options and log level.
// It automatically handles TTY detection and respects shell redirection (2>).
//
// Logging behavior:
//   - isInteractive=true + stderr is terminal: Logs to timestamped file in temp dir
//   - isInteractive=true + stderr redirected: Logs to stderr (respects user's 2> redirect)
//   - isInteractive=false: Logs to stderr
//
// Returns the fully qualified log file path (empty string if logging to stderr).
//
// Example usage:
//
//	logFile, err := logrium.Setup(true, slog.LevelInfo)
//	if err != nil {
//	    return err
//	}
//	if logFile != "" {
//	    fmt.Printf("Debug logs: %s\n", logFile)
//	}
//	slog.Info("Logger configured")
func Setup(isInteractive bool, level slog.Level) (string, error) {
	var output io.Writer
	var logFilePath string

	// Only use debug file if BOTH:
	// 1. Interactive mode (Bubbletea UI is running)
	// 2. stderr is still pointing to a terminal (not redirected)
	//
	// This avoids corrupting the TUI while respecting user's shell redirection.
	if isInteractive && isatty.IsTerminal(os.Stderr.Fd()) {
		// User is in interactive mode and hasn't redirected stderr
		// Create timestamped log file in temp directory
		timestamp := time.Now().Format("2006-01-02T15-04-05")
		logFileName := fmt.Sprintf("cerebrium-debug-%s.log", timestamp)
		logFilePath = filepath.Join(os.TempDir(), logFileName)

		logFile, err := os.OpenFile(logFilePath, //nolint:gosec // Log file in temp directory
			os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			return "", err
		}

		output = logFile
	} else {
		// All other cases: write to stderr
		// This respects user's shell redirection with 2>
		output = os.Stderr
		logFilePath = "" // Empty string indicates logging to stderr
	}

	// Configure handler
	handler := slog.NewTextHandler(output, &slog.HandlerOptions{
		Level: level,
	})

	// Set as global logger
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logFilePath, nil
}

// Disable configures slog to discard all log output.
// This is used when --verbose is not set to completely disable logging.
func Disable() {
	// Create a handler that discards all output
	handler := slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
		Level: slog.LevelError + 1, // Set level higher than any log level to discard everything
	})

	// Set as global logger
	logger := slog.New(handler)
	slog.SetDefault(logger)
}

// SetupForTesting configures slog to write to a custom writer for testing.
// The original logger is automatically restored when the test completes.
//
// Example usage:
//
//	func TestMyFunction(t *testing.T) {
//	    var buf bytes.Buffer
//	    logrium.SetupForTesting(t, &buf, slog.LevelDebug)
//
//	    // Run code that logs
//	    myFunction()
//
//	    // Assert on log output
//	    assert.Contains(t, buf.String(), "expected log message")
//	}
func SetupForTesting(t *testing.T, w io.Writer, level slog.Level) {
	// Save the current default logger
	originalLogger := slog.Default()

	// Create a new handler that writes to the provided writer
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: level,
	})

	// Set as the default logger
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Restore the original logger when the test completes
	t.Cleanup(func() {
		slog.SetDefault(originalLogger)
	})
}
