package logging

import (
	"context"
	"fmt"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
)

// pollingAppLogProvider fetches app runtime logs via HTTP polling
type pollingAppLogProvider struct {
	client       api.Client
	projectID    string
	appID        string
	follow       bool
	sinceTime    string
	runID        string
	pollInterval time.Duration

	// State
	lastTimestamp time.Time // Track as time.Time for proper comparison
	hasFetched    bool
	seenIDs       map[string]bool // Deduplication using log IDs
}

// PollingAppLogProviderConfig configures a pollingAppLogProvider
type PollingAppLogProviderConfig struct {
	Client       api.Client
	ProjectID    string
	AppID        string
	Follow       bool          // If false, fetch once and complete (default: true)
	SinceTime    string        // ISO timestamp to start from (optional)
	RunID        string        // Filter by specific run (optional)
	PollInterval time.Duration // Default: 5 seconds
}

// NewPollingAppLogProvider creates a new polling provider for app runtime logs
func NewPollingAppLogProvider(cfg PollingAppLogProviderConfig) LogProvider {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 5 * time.Second
	}

	// Parse sinceTime to time.Time if provided
	var lastTimestamp time.Time
	if cfg.SinceTime != "" {
		parsed, err := time.Parse(time.RFC3339, cfg.SinceTime)
		if err == nil {
			lastTimestamp = parsed
		}
		// If parsing fails, leave as zero value (will fetch all logs)
	}

	return &pollingAppLogProvider{
		client:        cfg.Client,
		projectID:     cfg.ProjectID,
		appID:         cfg.AppID,
		follow:        cfg.Follow,
		sinceTime:     cfg.SinceTime,
		runID:         cfg.RunID,
		pollInterval:  cfg.PollInterval,
		lastTimestamp: lastTimestamp,
		seenIDs:       make(map[string]bool),
	}
}

// Collect fetches app logs by polling the API
func (p *pollingAppLogProvider) Collect(ctx context.Context, callback func([]Log) error) error {
	// If not following, fetch once and return
	if !p.follow {
		return p.fetchOnce(ctx, callback)
	}

	// Follow mode: poll continuously
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	// Fetch immediately first
	if err := p.fetchOnce(ctx, callback); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-ticker.C:
			if err := p.fetchOnce(ctx, callback); err != nil {
				return err
			}
		}
	}
}

// fetchOnce fetches logs once from the API
func (p *pollingAppLogProvider) fetchOnce(ctx context.Context, callback func([]Log) error) error {
	// Convert lastTimestamp to string for API (empty string if zero value)
	var afterDate string
	if !p.lastTimestamp.IsZero() {
		afterDate = p.lastTimestamp.Format(time.RFC3339)
	}

	resp, err := p.client.FetchAppLogs(ctx, p.projectID, p.appID, api.AppLogOptions{
		AfterDate: afterDate,
		RunID:     p.runID,
	})
	if err != nil {
		return fmt.Errorf("failed to fetch app logs: %w", err)
	}

	p.hasFetched = true

	// Convert API logs to Log structs with deduplication
	newLogs := make([]Log, 0, len(resp.Logs))
	logIDsToAdd := make([]string, 0, len(resp.Logs)) // Track IDs to add for cleanup

	for _, apiLog := range resp.Logs {
		// Skip duplicates using log ID
		if p.seenIDs[apiLog.LogID] {
			continue
		}

		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, apiLog.Timestamp)
		if err != nil {
			// Fallback to current time if parsing fails
			timestamp = time.Now()
		}

		log := Log{
			Timestamp: timestamp,
			Content:   apiLog.LogLine,
			ID:        apiLog.LogID,
			Stream:    apiLog.Stream,
			Metadata: map[string]any{
				"runID":         apiLog.RunID,
				"containerID":   apiLog.ContainerID,
				"containerName": apiLog.ContainerName,
				"lineNumber":    apiLog.LineNumber,
			},
		}

		newLogs = append(newLogs, log)
		logIDsToAdd = append(logIDsToAdd, apiLog.LogID)

		// Track last timestamp for next fetch (compare as time.Time)
		if timestamp.After(p.lastTimestamp) {
			p.lastTimestamp = timestamp
		}
	}

	// Add new IDs to seenIDs and enforce memory limit
	// We can't remove specific old IDs since we don't track which logs were evicted
	// from the viewer, but we can limit the map size by periodically clearing old entries
	for _, id := range logIDsToAdd {
		p.seenIDs[id] = true
	}

	// If seenIDs map grows too large (2x the log limit), clear it
	// This is safe because we use timestamp-based filtering (afterDate) for the API
	// The seenIDs map is just for additional deduplication within recent logs
	if len(p.seenIDs) > MaxLogsInMemory*2 {
		// Keep only the most recently added IDs
		newSeenIDs := make(map[string]bool, len(logIDsToAdd))
		for _, id := range logIDsToAdd {
			newSeenIDs[id] = true
		}
		p.seenIDs = newSeenIDs
	}

	// Invoke callback with batch (even if empty to maintain consistency)
	if len(newLogs) > 0 {
		if err := callback(newLogs); err != nil {
			return fmt.Errorf("callback error: %w", err)
		}
	}

	return nil
}
