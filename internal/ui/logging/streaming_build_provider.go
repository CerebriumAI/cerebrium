package logging

import (
	"context"
	"fmt"
	"github.com/cerebriumai/cerebrium/internal/wsapi"
	"log/slog"
	"time"
)

type streamingBuildProvider struct {
	client    wsapi.Client
	projectID string
	buildID   string

	// State for deduplication
	seenIDs map[string]bool
}

type StreamingBuildLogProviderConfig struct {
	Client    wsapi.Client
	ProjectID string
	BuildID   string
}

func NewStreamingBuildLogProvider(cfg StreamingBuildLogProviderConfig) LogProvider {
	return &streamingBuildProvider{
		client:    cfg.Client,
		projectID: cfg.ProjectID,
		buildID:   cfg.BuildID,
		seenIDs:   make(map[string]bool),
	}
}

func (p *streamingBuildProvider) Collect(ctx context.Context, callback func([]Log) error) error {
	return p.client.StreamBuildLogs(ctx, p.projectID, p.buildID, time.Now().Add(-10*time.Second), func(msg wsapi.BuildLogMessage) error {
		// Generate unique ID for deduplication
		logID := fmt.Sprintf("%s-%d-%s", msg.BuildID, msg.LineNumber, msg.Timestamp.Format("2006-01-02T15:04:05.999999999Z07:00"))
		if p.seenIDs[logID] {
			slog.Debug("Rejecting seen log", "ID:", logID)
			return nil // Already seen, skip
		}
		p.seenIDs[logID] = true

		log := Log{
			ID:        logID,
			Timestamp: msg.Timestamp,
			Content:   msg.Log,
			Stream:    msg.Stream,
			Metadata: map[string]any{
				"buildID":    msg.BuildID,
				"appID":      msg.AppID,
				"lineNumber": msg.LineNumber,
				"stage":      msg.Stage,
			},
		}

		slog.Info("Streamed log: ", "Log:", log.Content)

		return callback([]Log{log})
	})
}
