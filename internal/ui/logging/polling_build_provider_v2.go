package logging

import (
	"cmp"
	"context"
	"fmt"
	"github.com/cerebriumai/cerebrium/internal/api"
	"time"
)

type pollingBuildLogProviderV2 struct {
	client       api.Client
	projectID    string
	appName      string
	buildID      string
	pollInterval time.Duration
}

type PollingBuildLogProviderConfigV2 struct {
	Client       api.Client
	ProjectID    string
	AppName      string
	BuildID      string
	PollInterval time.Duration // Default: 2 seconds
}

func NewPollingBuildLogProviderV2(cfg PollingBuildLogProviderConfigV2) LogProvider {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 2 * time.Second
	}

	return &pollingBuildLogProviderV2{
		client:       cfg.Client,
		projectID:    cfg.ProjectID,
		appName:      cfg.AppName,
		buildID:      cfg.BuildID,
		pollInterval: cfg.PollInterval,
	}
}

func (p *pollingBuildLogProviderV2) Collect(ctx context.Context, callback func([]Log) error) error {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	// Wrap FetchBuildLogs in the fetchLogsFn type
	fetcher := func(ctx context.Context) (*api.BuildLogsResponse, error) {
		return p.client.FetchBuildLogs(ctx, p.projectID, p.appName, p.buildID)
	}

	// Fetch immediately
	err := fetchOnce(ctx, fetcher, callback)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			err := fetchOnce(ctx, fetcher, callback)
			if err != nil {
				return err
			}
		}
	}
}

// fetchLogsFn defines the requirements for `fetchOnce` to get a set of logs. Aliased for testing.
type fetchLogsFn func(ctx context.Context) (*api.BuildLogsResponse, error)

func fetchOnce(ctx context.Context,
	fn fetchLogsFn,
	callback func([]Log) error) error {

	resp, err := fn(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch build logs: %w", err)
	}

	// Process new logs
	newLogs := make([]Log, 0, len(resp.Logs))
	logIDsToAdd := make([]string, 0, len(resp.Logs))

	for _, apiLog := range resp.Logs {
		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, apiLog.CreatedAt)
		if err != nil {
			// Fallback to current time if parsing fails
			timestamp = time.Now()
		}

		log := Log{
			ID: cmp.Or(
				apiLog.ID,
				apiLog.CreatedAt+apiLog.Log, // Simple ID combining timestamp + content
			),
			Timestamp: timestamp,
			Content:   apiLog.Log,
			Stream:    "",
			Metadata: map[string]any{
				"buildStatus": resp.Status,
			},
		}

		newLogs = append(newLogs, log)
		logIDsToAdd = append(logIDsToAdd, log.ID)
	}

	// Invoke callback with batch (even if empty to maintain consistency)
	if len(newLogs) > 0 {
		if err := callback(newLogs); err != nil {
			return fmt.Errorf("callback error: %w", err)
		}
	}

	// Check if build is complete
	return nil
}
