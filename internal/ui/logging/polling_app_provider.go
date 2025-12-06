package logging

import (
	"context"
	"fmt"
	"log/slog"
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
	direction    string
	containerID  string
	stream       string
	searchTerm   string
	pageSize     int32
	pollInterval time.Duration

	// State
	nextToken     string    // Token for pagination (preferred method)
	lastTimestamp time.Time // Track as time.Time for fallback comparison
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
	Direction    string        // "forward" or "backward" (optional, default: "forward" for follow mode)
	ContainerID  string        // Filter by container (optional)
	Stream       string        // "stdout" or "stderr" (optional)
	SearchTerm   string        // Filter logs by search term (optional)
	PageSize     int32         // Number of logs per page (optional)
	PollInterval time.Duration // Default: 5 seconds
}

// NewPollingAppLogProvider creates a new polling provider for app runtime logs
func NewPollingAppLogProvider(cfg PollingAppLogProviderConfig) LogProvider {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 5 * time.Second
	}

	// Default to "forward" direction for follow mode (matches Python CLI behavior)
	direction := cfg.Direction
	if direction == "" && cfg.Follow {
		direction = "forward"
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
		direction:     direction,
		containerID:   cfg.ContainerID,
		stream:        cfg.Stream,
		searchTerm:    cfg.SearchTerm,
		pageSize:      cfg.PageSize,
		pollInterval:  cfg.PollInterval,
		lastTimestamp: lastTimestamp,
		seenIDs:       make(map[string]bool),
	}
}

// Collect fetches app logs by polling the API
func (p *pollingAppLogProvider) Collect(ctx context.Context, callback func([]Log) error) error {
	var afterDate string
	if p.nextToken == "" && !p.lastTimestamp.IsZero() {
		afterDate = p.lastTimestamp.Format(time.RFC3339)
	}

	slog.Info("polling app logs",
		"projectID", p.projectID,
		"appID", p.appID,
		"runID", p.runID,
		"direction", p.direction,
		"nextToken", p.nextToken,
		"afterDate", afterDate,
		"lastTimestamp", p.lastTimestamp,
		"seenIDsCount", len(p.seenIDs),
	)

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
	// Build API options
	// Use nextToken if available (preferred), otherwise fall back to afterDate
	var afterDate string
	if p.nextToken == "" && !p.lastTimestamp.IsZero() {
		afterDate = p.lastTimestamp.Format(time.RFC3339)
	}

	resp, err := p.client.FetchAppLogs(ctx, p.projectID, p.appID, api.AppLogOptions{
		ContainerID: p.containerID,
		AfterDate:   afterDate,
		PageSize:    p.pageSize,
		NextToken:   p.nextToken,
		Direction:   p.direction,
		SearchTerm:  p.searchTerm,
		Stream:      p.stream,
		RunID:       p.runID,
	})
	if err != nil {
		slog.Error("failed to fetch app logs", "error", err)
		return fmt.Errorf("failed to fetch app logs: %w", err)
	}

	// Update nextToken for next fetch (token-based pagination)
	if resp.NextPageToken != nil {
		p.nextToken = *resp.NextPageToken
	}

	if len(resp.Logs) > 0 {
		slog.Info("received app logs from API",
			"totalLogsFromAPI", len(resp.Logs),
			"runID", p.runID,
			"nextPageToken", resp.NextPageToken,
			"hasMore", resp.HasMore,
		)
	}

	p.hasFetched = true

	// Convert API logs to Log structs with deduplication
	newLogs := make([]Log, 0, len(resp.Logs))
	logIDsToAdd := make([]string, 0, len(resp.Logs)) // Track IDs to add for cleanup
	duplicateCount := 0

	for _, apiLog := range resp.Logs {
		// Skip duplicates using log ID
		if p.seenIDs[apiLog.LogID] {
			slog.Info("filtering out duplicate log",
				"logID", apiLog.LogID,
				"runID", apiLog.RunID,
				"timestamp", apiLog.Timestamp,
			)
			duplicateCount++
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

		slog.Info("accepting new log",
			"logID", apiLog.LogID,
			"runID", apiLog.RunID,
			"timestamp", apiLog.Timestamp,
			"stream", apiLog.Stream,
			"contentPreview", truncateString(apiLog.LogLine, 50),
		)

		newLogs = append(newLogs, log)
		logIDsToAdd = append(logIDsToAdd, apiLog.LogID)

		// Track last timestamp for next fetch (compare as time.Time)
		if timestamp.After(p.lastTimestamp) {
			p.lastTimestamp = timestamp
		}
	}

	//slog.Info("fetchOnce complete",
	//	"newLogsAccepted", len(newLogs),
	//	"duplicatesFiltered", duplicateCount,
	//	"updatedLastTimestamp", p.lastTimestamp,
	//)

	// Add new IDs to seenIDs and enforce memory limit
	// We can't remove specific old IDs since we don't track which logs were evicted
	// from the viewer, but we can limit the map size by periodically clearing old entries
	for _, id := range logIDsToAdd {
		p.seenIDs[id] = true
	}

	// If seenIDs map grows too large (2x the log limit), clear it
	// This is safe because we use timestamp-based filtering (afterDate) for the API
	// The seenIDs map is just for additional deduplication within recent logs
	if len(p.seenIDs) > maxLogsInMemory*2 {
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

// truncateString truncates a string to maxLen characters, adding "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
