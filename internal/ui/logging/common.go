package logging

import (
	"fmt"
	"github.com/cerebriumai/cerebrium/internal/ui"
)

func formatLogEntry(log Log) string {
	timestamp := log.Timestamp.Local().Format("15:04:05.000")
	styledTimestamp := ui.TimestampStyle.Render(timestamp)
	return fmt.Sprintf("%s %s", styledTimestamp, log.Content)
}
