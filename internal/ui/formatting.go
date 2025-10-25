package ui

import (
	"fmt"
	"strings"
	"time"
)

// ColorizeStatus applies color styling to app status
// Handles statuses like "ready", "success", "notready", "pending", "deploying", "error", etc.
func ColorizeStatus(status string) string {
	// Normalize: split on underscore, capitalize each word
	parts := strings.Split(status, "_")
	var capitalized []string
	for _, part := range parts {
		if len(part) > 0 {
			capitalized = append(capitalized, strings.ToUpper(part[:1])+strings.ToLower(part[1:]))
		}
	}
	displayStatus := strings.Join(capitalized, " ")

	// Normalize status for matching (remove underscores and lowercase)
	statusNormalized := strings.ToLower(strings.ReplaceAll(status, "_", ""))

	// Determine color based on status
	switch statusNormalized {
	case "ready", "active", "success":
		return GreenStyle.Render(displayStatus)
	case "cold":
		return CyanStyle.Render(displayStatus)
	case "pending":
		return PendingStyle.Render(displayStatus)
	case "deploying":
		return MagentaStyle.Render(displayStatus)
	case "notready":
		return PendingStyle.Render("Not Ready")
	default:
		if strings.Contains(statusNormalized, "error") {
			return RedStyle.Render(displayStatus)
		}
		// Default: bold but no specific color
		return BoldStyle.Render(displayStatus)
	}
}

// FormatTimestamp formats a time.Time to a human-readable string
func FormatTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// FormatError formats an error message with styling
// NOTE: Adds a new line manually. Use strings.TrimSpace if you want to strip it.
func FormatError(err error) string {
	if err == nil {
		return ""
	}
	// Append a new line because there is some slightly odd behaviour with bubbletea in terminals - the last
	// line when the program exits appears to be overwritten. Seems like this is a problem with Bubbletea itself.
	// Issue here: https://github.com/charmbracelet/bubbletea/issues/304
	return ErrorStyle.Render(fmt.Sprintf("✗ Error: %s", err.Error())) + "\n"
}
