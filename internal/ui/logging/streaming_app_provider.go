package logging

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cerebriumai/cerebrium/internal/wsapi"
)

// streamingAppProvider implements LogProvider using WebSocket streaming for app runtime logs.
// It connects to the /ws-logs endpoint and delivers logs in real-time as they arrive,
// replacing the polling-based approach for better latency and reliability.
type streamingAppProvider struct {
	client    wsapi.Client
	projectID string
	appID     string
	runID     string

	seenIDs map[string]bool // Tracks seen log IDs to prevent duplicates on reconnection
}

// StreamingAppLogProviderConfig configures a streaming WebSocket provider for app runtime logs.
type StreamingAppLogProviderConfig struct {
	Client    wsapi.Client
	ProjectID string
	AppID     string
	RunID     string // Optional: filter logs to a specific run
}

// NewStreamingAppLogProvider creates a new WebSocket streaming provider for app runtime logs.
// This replaces the polling-based provider for real-time log delivery.
func NewStreamingAppLogProvider(cfg StreamingAppLogProviderConfig) LogProvider {
	return &streamingAppProvider{
		client:    cfg.Client,
		projectID: cfg.ProjectID,
		appID:     cfg.AppID,
		runID:     cfg.RunID,
		seenIDs:   make(map[string]bool),
	}
}

// Collect implements LogProvider.Collect by streaming logs via WebSocket.
// It connects to the backend WebSocket endpoint and invokes the callback for each
// batch of logs received. The connection is maintained until the context is cancelled.
func (p *streamingAppProvider) Collect(ctx context.Context, callback func([]Log) error) error {
	opts := wsapi.AppLogStreamOptions{
		From:  time.Now().Add(-10 * time.Second), // Start slightly in the past to catch recent logs
		RunID: p.runID,
	}

	return p.client.StreamAppLogs(ctx, p.projectID, p.appID, opts, func(msg wsapi.AppLogMessage) error {
		// Generate unique ID for deduplication
		baseLogID := fmt.Sprintf("%s-%s-%s", msg.AppID, msg.RunID, msg.Timestamp.Format(time.RFC3339Nano))
		if p.seenIDs[baseLogID] {
			return nil
		}
		p.seenIDs[baseLogID] = true

		// Strip ANSI cursor movement codes and normalize carriage returns
		// (same processing as build logs for progress bar support)
		cleanedLog := ansiCursorMovement.ReplaceAllString(msg.Log, "")
		normalizedLog := strings.ReplaceAll(cleanedLog, "\r", "\n")
		parts := strings.Split(normalizedLog, "\n")

		var logs []Log
		for i, part := range parts {
			if strings.TrimSpace(part) == "" || isJustProcessPrefix(part) {
				continue
			}

			logs = append(logs, Log{
				ID:        fmt.Sprintf("%s-%d", baseLogID, i),
				Timestamp: msg.Timestamp,
				Content:   part,
				Stream:    "stdout", // App logs don't have stream info from WebSocket
				Metadata: map[string]any{
					"appID":         msg.AppID,
					"runID":         msg.RunID,
					"containerName": msg.ContainerName,
				},
			})
		}

		if len(logs) == 0 {
			return nil
		}

		slog.Debug("Streamed app logs", "count", len(logs), "runID", msg.RunID)
		return callback(logs)
	})
}
