package logrium

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetupForTesting(t *testing.T) {
	var buf bytes.Buffer

	// Setup logger to write to buffer
	SetupForTesting(t, &buf, slog.LevelDebug)

	// Log some messages
	slog.Debug("debug message", "key1", "value1")
	slog.Info("info message", "key2", "value2")
	slog.Warn("warn message", "key3", "value3")
	slog.Error("error message", "key4", "value4")

	// Verify all messages were logged
	output := buf.String()
	assert.Contains(t, output, "debug message")
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "warn message")
	assert.Contains(t, output, "error message")

	// Verify structured fields are present
	assert.Contains(t, output, "key1=value1")
	assert.Contains(t, output, "key2=value2")
	assert.Contains(t, output, "key3=value3")
	assert.Contains(t, output, "key4=value4")

	// Verify log levels are present
	assert.Contains(t, output, "level=DEBUG")
	assert.Contains(t, output, "level=INFO")
	assert.Contains(t, output, "level=WARN")
	assert.Contains(t, output, "level=ERROR")
}

func TestSetupForTesting_LogLevel(t *testing.T) {
	var buf bytes.Buffer

	// Setup logger with INFO level (DEBUG should be filtered)
	SetupForTesting(t, &buf, slog.LevelInfo)

	// Log at different levels
	slog.Debug("debug message")
	slog.Info("info message")
	slog.Warn("warn message")

	output := buf.String()

	// Debug should be filtered out
	assert.NotContains(t, output, "debug message")

	// Info and above should be present
	assert.Contains(t, output, "info message")
	assert.Contains(t, output, "warn message")
}

func TestSetupForTesting_Cleanup(t *testing.T) {
	// Get the original default logger
	originalLogger := slog.Default()

	// Run a subtest with custom logger
	t.Run("with_custom_logger", func(t *testing.T) {
		var buf bytes.Buffer
		SetupForTesting(t, &buf, slog.LevelDebug)

		// Custom logger should be active
		assert.NotEqual(t, originalLogger, slog.Default())

		slog.Info("test message")
		assert.Contains(t, buf.String(), "test message")
	})

	// After subtest completes, original logger should be restored
	assert.Equal(t, originalLogger, slog.Default())
}

func TestSetupForTesting_MultipleTests(t *testing.T) {
	// Each test should have isolated logging
	t.Run("test1", func(t *testing.T) {
		var buf1 bytes.Buffer
		SetupForTesting(t, &buf1, slog.LevelInfo)

		slog.Info("message from test1")

		assert.Contains(t, buf1.String(), "message from test1")
		assert.NotContains(t, buf1.String(), "message from test2")
	})

	t.Run("test2", func(t *testing.T) {
		var buf2 bytes.Buffer
		SetupForTesting(t, &buf2, slog.LevelInfo)

		slog.Info("message from test2")

		assert.Contains(t, buf2.String(), "message from test2")
		assert.NotContains(t, buf2.String(), "message from test1")
	})
}

func TestSetupForTesting_RealWorldExample(t *testing.T) {
	var buf bytes.Buffer
	SetupForTesting(t, &buf, slog.LevelDebug)

	// Simulate some application code that logs
	simulateAPIRequest := func() {
		slog.Debug("API request", "method", "GET", "path", "/projects")
		slog.Info("API request successful", "statusCode", 200, "duration", "1.5s")
	}

	simulateAPIRequest()

	// Verify the logs
	output := buf.String()
	assert.Contains(t, output, "API request")
	assert.Contains(t, output, "method=GET")
	assert.Contains(t, output, "path=/projects")
	assert.Contains(t, output, "statusCode=200")

	// Count log entries
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 2, len(lines), "Should have 2 log entries")
}
