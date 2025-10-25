package logging

import (
	"context"
	"fmt"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
)

// pollingBuildLogProvider fetches build logs via HTTP polling
type pollingBuildLogProvider struct {
	client       api.Client
	projectID    string
	appName      string
	buildID      string
	pollInterval time.Duration
}

// PollingBuildLogProviderConfig configures a pollingBuildLogProvider
type PollingBuildLogProviderConfig struct {
	Client       api.Client
	ProjectID    string
	AppName      string
	BuildID      string
	PollInterval time.Duration // Default: 2 seconds
}

// NewPollingBuildLogProvider creates a new polling provider for build logs
func NewPollingBuildLogProvider(cfg PollingBuildLogProviderConfig) LogProvider {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 2 * time.Second
	}

	return &pollingBuildLogProvider{
		client:       cfg.Client,
		projectID:    cfg.ProjectID,
		appName:      cfg.AppName,
		buildID:      cfg.BuildID,
		pollInterval: cfg.PollInterval,
	}
}

// Collect fetches build logs by polling the API until the build reaches a terminal status
func (p *pollingBuildLogProvider) Collect(ctx context.Context, callback func([]Log) error) error {
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	seenIDs := make(map[string]bool) // Deduplication

	// Fetch immediately
	isComplete, err := p.fetchOnce(ctx, callback, seenIDs)
	if err != nil {
		return err
	} else if isComplete {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			isComplete, err := p.fetchOnce(ctx, callback, seenIDs)
			if err != nil {
				return err
			}
			if isComplete {
				return nil // Normal completion
			}
		}
	}
}

// fetchOnce fetches logs once from the API and returns whether the build is complete
func (p *pollingBuildLogProvider) fetchOnce(ctx context.Context, callback func([]Log) error, seenIDs map[string]bool) (isComplete bool, err error) {
	resp, err := p.client.FetchBuildLogs(ctx, p.projectID, p.appName, p.buildID)
	if err != nil {
		return false, fmt.Errorf("failed to fetch build logs: %w", err)
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
			Timestamp: timestamp,
			Content:   apiLog.Log,
			ID:        apiLog.CreatedAt + apiLog.Log, // Simple ID combining timestamp + content
			Stream:    "",
			Metadata: map[string]any{
				"buildStatus": resp.Status,
			},
		}

		// Skip duplicates
		if seenIDs[log.ID] {
			continue
		}

		newLogs = append(newLogs, log)
		logIDsToAdd = append(logIDsToAdd, log.ID)
	}

	// Add new IDs to seenIDs
	for _, id := range logIDsToAdd {
		seenIDs[id] = true
	}

	// Enforce memory limit on seenIDs map
	if len(seenIDs) > MaxLogsInMemory*2 {
		// Clear and keep only recent IDs
		newSeenIDs := make(map[string]bool, len(logIDsToAdd))
		for _, id := range logIDsToAdd {
			newSeenIDs[id] = true
		}
		// Replace the seenIDs map (caller's map is updated by reference)
		for k := range seenIDs {
			delete(seenIDs, k)
		}
		for k, v := range newSeenIDs {
			seenIDs[k] = v
		}
	}

	// Invoke callback with batch (even if empty to maintain consistency)
	if len(newLogs) > 0 {
		if err := callback(newLogs); err != nil {
			return false, fmt.Errorf("callback error: %w", err)
		}
	}

	// Check if build is complete
	return isTerminalStatus(resp.Status), nil
}
