package ui

import (
	"fmt"
	"github.com/charmbracelet/bubbles/table"
	"time"
)

const (
	// MAX_TABLE_HEIGHT defines the max number of rows viewable in one render.
	// Not including header. Tables longer than this should scroll.
	MAX_TABLE_HEIGHT = 15

	// MAX_LOGS_IN_VIEWER is the maximum number of logs to show in the log viewer in one render.
	MAX_LOGS_IN_VIEWER = 20

	// LOG_POLL_INTERVAL sets how fast log apis are polled.
	LOG_POLL_INTERVAL = 500 * time.Millisecond
)

func TableBiggerThanView(t table.Model) bool {
	return len(t.Rows()) > MAX_TABLE_HEIGHT
}

func FormatSize(bytes int64) string {
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

// IsTerminalStatus checks if a build status is terminal
func IsTerminalStatus(status string) bool {
	switch status {
	case "success", "build_failure", "init_failure", "ready", "failure", "cancelled", "init_timeout":
		return true
	default:
		return false
	}
}
