package logging

import (
	"cmp"
	"context"
	"fmt"
	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/ui"
	"time"
)

type pollingBuildLogProvider struct {
	client       api.Client
	projectID    string
	appName      string
	buildID      string
	pollInterval time.Duration

	// State
	seenIDs map[string]bool // Deduplication using log IDs
}

type PollingBuildLogProviderConfig struct {
	Client       api.Client
	ProjectID    string
	AppName      string
	BuildID      string
	PollInterval time.Duration
}

func NewPollingBuildLogProvider(cfg PollingBuildLogProviderConfig) LogProvider {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = ui.LOG_POLL_INTERVAL
	}

	return &pollingBuildLogProvider{
		client:       cfg.Client,
		projectID:    cfg.ProjectID,
		appName:      cfg.AppName,
		buildID:      cfg.BuildID,
		pollInterval: cfg.PollInterval,
		seenIDs:      make(map[string]bool),
	}
}

func (p *pollingBuildLogProvider) Collect(ctx context.Context, callback func([]Log) error) error {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	err := p.fetchOnce(ctx, callback)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			err := p.fetchOnce(ctx, callback)
			if err != nil {
				return err
			}
		}
	}
}

// fetchOnce fetches logs once from the API and returns whether the build is complete
func (p *pollingBuildLogProvider) fetchOnce(ctx context.Context, callback func([]Log) error) error {
	resp, err := p.client.FetchBuildLogs(ctx, p.projectID, p.appName, p.buildID)
	if err != nil {
		return fmt.Errorf("failed to fetch build logs: %w", err)
	}

	// Process new logs with deduplication
	newLogs := make([]Log, 0, len(resp.Logs))

	for _, apiLog := range resp.Logs {
		// Generate log ID
		logID := cmp.Or(
			apiLog.ID,
			apiLog.CreatedAt+apiLog.Log, // Simple ID combining timestamp + content
		)
		if p.seenIDs[logID] {
			continue
		}
		p.seenIDs[logID] = true

		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, apiLog.CreatedAt)
		if err != nil {
			// Fallback to current time if parsing fails
			timestamp = time.Now()
		}

		log := Log{
			ID:        logID,
			Timestamp: timestamp,
			Content:   apiLog.Log,
			Stream:    "",
			Metadata: map[string]any{
				"buildStatus": resp.Status,
			},
		}

		newLogs = append(newLogs, log)
	}

	// Invoke callback with batch
	if len(newLogs) > 0 {
		if err := callback(newLogs); err != nil {
			return fmt.Errorf("callback error: %w", err)
		}
	}

	return nil
}
