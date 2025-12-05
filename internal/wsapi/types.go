package wsapi

import "time"

// BuildLogMessage represents a build log entry received from the websocket stream.
type BuildLogMessage struct {
	// BuildID is the unique identifier for the build
	BuildID string

	// AppID is the application identifier
	AppID string

	// Timestamp is when the log was created (parsed to local timezone)
	Timestamp time.Time

	// Stream indicates the output stream ("stdout", "stderr", or "")
	Stream string

	// Log is the actual log message content
	Log string

	// LineNumber is the sequential line number within the build
	LineNumber int

	// Stage indicates the build stage (e.g., "build", "push")
	Stage string
}

// rawBuildLogMessage is the JSON structure received from the websocket server.
type rawBuildLogMessage struct {
	BuildID    string `json:"build_id"`
	AppID      string `json:"app_id"`
	Timestamp  string `json:"timestamp"`
	Stream     string `json:"stream"`
	Log        string `json:"log"`
	LineNumber int    `json:"line_number"`
	Stage      string `json:"stage"`
}

// parseTimestamp parses a timestamp string and converts it to local timezone.
func parseTimestamp(ts string) time.Time {
	// Try RFC3339Nano first (most precise)
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t.Local()
	}

	// Try RFC3339 as fallback
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.Local()
	}

	// Return current local time if parsing fails
	return time.Now()
}
