package logging

import (
	"context"
	"time"
)

// LogProvider abstracts the mechanism for fetching logs (polling, websockets, etc.)
// It's framework-agnostic and can be used from Cobra commands, Bubbletea models, or anywhere else.
type LogProvider interface {
	// Collect fetches logs and invokes callback with batches of log entries.
	// Returns when context is cancelled, an error occurs, or stream completes naturally.
	// If callback returns an error, collection stops immediately.
	// The callback may be called with an empty slice if no new logs are available in a polling cycle.
	Collect(ctx context.Context, callback func([]Log) error) error
}

// Log represents a single log entry from any source
type Log struct {
	// Timestamp is when the log was created
	Timestamp time.Time

	// Content is the log message content
	Content string

	// ID is a unique identifier for deduplication
	ID string

	// Stream is the stream type ("stdout", "stderr", or "")
	Stream string

	// Metadata contains additional provider-specific data (e.g., buildStatus, runID, containerID)
	Metadata map[string]any
}
