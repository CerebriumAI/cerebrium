package wsapi

import (
	"context"
	"time"
)

// Client defines the interface for websocket API operations.
type Client interface {
	// StreamBuildLogs connects to the build logs websocket and streams log messages.
	// The client handles connection management, reconnection on failures, and keep-alive.
	//
	// The callback is invoked for each log message received. If the callback returns
	// an error, streaming stops and that error is returned.
	//
	// Returns nil when the server closes the connection normally (build complete).
	// Returns context.Canceled or context.DeadlineExceeded when the context is done.
	// Returns an error if max reconnection attempts are exceeded or callback fails.
	StreamBuildLogs(ctx context.Context, projectID, buildID string, from time.Time, callback func(BuildLogMessage) error) error

	// StreamAppLogs connects to the app logs websocket and streams runtime log messages.
	// The client handles connection management, reconnection on failures, and keep-alive.
	//
	// Options allow filtering by containerID and runID.
	//
	// The callback is invoked for each log message received. If the callback returns
	// an error, streaming stops and that error is returned.
	//
	// Returns context.Canceled or context.DeadlineExceeded when the context is done.
	// Returns an error if max reconnection attempts are exceeded or callback fails.
	StreamAppLogs(ctx context.Context, projectID, appID string, opts AppLogStreamOptions, callback func(AppLogMessage) error) error
}

// AppLogStreamOptions contains options for streaming app logs.
type AppLogStreamOptions struct {
	// From is the timestamp to start streaming from (optional)
	From time.Time

	// ContainerID filters logs to a specific container (optional)
	ContainerID string

	// RunID filters logs to a specific run (optional)
	RunID string
}
