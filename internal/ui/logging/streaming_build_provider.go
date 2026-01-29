package logging

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/cerebriumai/cerebrium/internal/wsapi"
)

// processPrefixOnly matches process prefix lines with no content (e.g., "(EngineCore_DP0 pid=198)").
// These are artifacts from progress bar splitting where the content was overwritten.
var processPrefixOnly = regexp.MustCompile(`^(\x1b|\033)?\[[\d;]*m\([A-Za-z0-9_]+ pid=\d+\)(\x1b|\033)?\[[\d;]*m\s*$`)

// processPrefixPlain matches plain text process prefixes without ANSI codes.
var processPrefixPlain = regexp.MustCompile(`^\([A-Za-z0-9_]+ pid=\d+\)\s*$`)

// ansiCursorMovement matches ANSI cursor movement codes (e.g., \x1b[A for cursor up).
// These are stripped to prevent terminal overwriting when displaying progress bars sequentially.
var ansiCursorMovement = regexp.MustCompile(`\x1b\[\d*[ABCDEFGJKST]`)

// isJustProcessPrefix returns true if the line contains only a process prefix with no content.
func isJustProcessPrefix(line string) bool {
	trimmed := strings.TrimSpace(line)
	return processPrefixOnly.MatchString(trimmed) || processPrefixPlain.MatchString(trimmed)
}

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
		baseLogID := fmt.Sprintf("%s-%d-%s", msg.BuildID, msg.LineNumber, msg.Timestamp.Format(time.RFC3339Nano))
		if p.seenIDs[baseLogID] {
			return nil
		}
		p.seenIDs[baseLogID] = true

		// Strip ANSI cursor movement codes to prevent terminal overwriting,
		// then normalize \r to \n so each progress update becomes a separate line.
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
				Stream:    msg.Stream,
				Metadata: map[string]any{
					"buildID":    msg.BuildID,
					"appID":      msg.AppID,
					"lineNumber": msg.LineNumber,
					"stage":      msg.Stage,
				},
			})
		}

		if len(logs) == 0 {
			return nil
		}

		slog.Debug("Streamed logs", "count", len(logs))
		return callback(logs)
	})
}
