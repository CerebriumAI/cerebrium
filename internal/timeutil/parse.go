package timeutil

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// ParseSinceTime parses a --since parameter and returns an ISO timestamp string.
//
// Accepts multiple formats:
//   - Relative time: "1h", "30m", "2d", "45s"
//   - RFC3339: "2006-01-02T15:04:05Z" or "2006-01-02T15:04:05-07:00"
//   - RFC3339Nano: "2006-01-02T15:04:05.999999999Z"
//   - DateTime: "2006-01-02 15:04:05"
//   - DateTime with millis: "2006-01-02 15:04:05.999"
//   - DateOnly: "2006-01-02"
//
// For formats without timezone info, uses the local timezone.
//
// Returns:
//
//	ISO timestamp string in UTC (e.g., "2023-12-01T10:00:00.000Z")
//
// Errors:
//
//	Returns error if the format is invalid
func ParseSinceTime(sinceStr string) (string, error) {
	// Try multiple datetime formats
	formats := []struct {
		layout      string
		description string
		useLocal    bool // Whether to interpret as local time if no timezone
	}{
		{time.RFC3339Nano, "RFC3339Nano", false},
		{time.RFC3339, "RFC3339", false},
		{time.DateTime, "DateTime (2006-01-02 15:04:05)", true},
		{"2006-01-02 15:04:05.999", "DateTime with milliseconds", true},
		{"2006-01-02 15:04:05.999999", "DateTime with microseconds", true},
		{time.DateOnly, "DateOnly (2006-01-02)", true},
	}

	for _, fmt := range formats {
		t, err := time.Parse(fmt.layout, sinceStr)
		if err == nil {
			// If format uses local time, convert from local to UTC
			if fmt.useLocal {
				// time.Parse assumes UTC, so we need to interpret it as local time
				// Get local location
				loc := time.Local
				// Parse as if it were in local timezone
				t, err = time.ParseInLocation(fmt.layout, sinceStr, loc)
				if err != nil {
					continue
				}
			}
			// Convert to UTC and truncate to milliseconds
			utcTime := t.UTC().Truncate(time.Millisecond)
			return utcTime.Format("2006-01-02T15:04:05.000Z"), nil
		}
	}

	// Parse relative time format (e.g., "1h", "30m", "2d")
	relativeTimePattern := regexp.MustCompile(`^(\d+)([smhdw])$`)
	match := relativeTimePattern.FindStringSubmatch(sinceStr)

	if match == nil {
		return "", fmt.Errorf("invalid --since format: '%s'. Use relative time ('w|d|h|m|s') or absolute (e.g., '2006-01-02 15:04:05', '2006-01-02T15:04:05Z', '2006-01-02')", sinceStr)
	}

	amount, err := strconv.Atoi(match[1])
	if err != nil {
		return "", fmt.Errorf("invalid number in --since: %w", err)
	}

	unit := match[2]

	var duration time.Duration
	switch unit {
	case "s":
		duration = time.Duration(amount) * time.Second
	case "m":
		duration = time.Duration(amount) * time.Minute
	case "h":
		duration = time.Duration(amount) * time.Hour
	case "d":
		duration = time.Duration(amount) * 24 * time.Hour
	case "w":
		duration = time.Duration(amount) * 7 * 24 * time.Hour
	default:
		return "", fmt.Errorf("invalid time unit: '%s'. Use s, m, h, or d", unit)
	}

	// Use local time for relative times
	targetTime := time.Now().Add(-duration)
	// Convert to UTC and truncate to milliseconds
	utcTime := targetTime.UTC().Truncate(time.Millisecond)
	return utcTime.Format("2006-01-02T15:04:05.000Z"), nil
}
