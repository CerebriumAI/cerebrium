package logging

import (
	"errors"
	"testing"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	apimock "github.com/cerebriumai/cerebrium/internal/api/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_pollingAppLogProvider_fetchOnce(t *testing.T) {
	tcs := []struct {
		name          string
		mockResponse  *api.AppLogsResponse
		mockError     error
		existingState *pollingAppLogProvider
		expectedLogs  int
		expectedError bool
		validateLogs  func(t *testing.T, logs []Log)
		validateState func(t *testing.T, provider *pollingAppLogProvider)
	}{
		{
			name: "successful fetch with logs",
			mockResponse: &api.AppLogsResponse{
				Logs: []api.AppLogEntry{
					{
						LogID:     "log-1",
						Timestamp: "2024-01-01T10:00:00Z",
						LogLine:   "Application started",
						Stream:    "stdout",
						RunID:     "run-123",
					},
					{
						LogID:     "log-2",
						Timestamp: "2024-01-01T10:00:01Z",
						LogLine:   "Processing request",
						Stream:    "stdout",
						RunID:     "run-123",
					},
				},
			},
			expectedLogs: 2,
			validateLogs: func(t *testing.T, logs []Log) {
				assert.Len(t, logs, 2)
				assert.Equal(t, "Application started", logs[0].Content)
				assert.Equal(t, "log-1", logs[0].ID)
				assert.Equal(t, "stdout", logs[0].Stream)
				assert.Equal(t, "run-123", logs[0].Metadata["runID"])
				assert.Equal(t, "Processing request", logs[1].Content)
				assert.Equal(t, "log-2", logs[1].ID)
			},
		},
		{
			name: "empty logs",
			mockResponse: &api.AppLogsResponse{
				Logs: []api.AppLogEntry{},
			},
			expectedLogs: 0,
		},
		{
			name:          "API error",
			mockError:     errors.New("API connection failed"),
			expectedError: true,
		},
		{
			name: "deduplication - seen logs filtered out",
			mockResponse: &api.AppLogsResponse{
				Logs: []api.AppLogEntry{
					{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "First", Stream: "stdout"},
					{LogID: "log-2", Timestamp: "2024-01-01T10:00:01Z", LogLine: "Second", Stream: "stdout"},
				},
			},
			existingState: &pollingAppLogProvider{
				seenIDs: map[string]bool{
					"log-1": true,
				},
			},
			expectedLogs: 1,
			validateLogs: func(t *testing.T, logs []Log) {
				assert.Len(t, logs, 1)
				assert.Equal(t, "log-2", logs[0].ID)
				assert.Equal(t, "Second", logs[0].Content)
			},
		},
		{
			name: "timestamp tracking - updates lastTimestamp",
			mockResponse: &api.AppLogsResponse{
				Logs: []api.AppLogEntry{
					{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Log 1", Stream: "stdout"},
					{LogID: "log-2", Timestamp: "2024-01-01T10:00:02Z", LogLine: "Log 2", Stream: "stdout"},
					{LogID: "log-3", Timestamp: "2024-01-01T10:00:01Z", LogLine: "Log 3", Stream: "stdout"},
				},
			},
			expectedLogs: 3,
			validateState: func(t *testing.T, provider *pollingAppLogProvider) {
				expected, _ := time.Parse(time.RFC3339, "2024-01-01T10:00:02Z")
				assert.Equal(t, expected, provider.lastTimestamp)
			},
		},
		{
			name: "uses lastTimestamp for afterDate",
			mockResponse: &api.AppLogsResponse{
				Logs: []api.AppLogEntry{
					{LogID: "log-3", Timestamp: "2024-01-01T10:00:02Z", LogLine: "New log", Stream: "stdout"},
				},
			},
			existingState: &pollingAppLogProvider{
				lastTimestamp: mustParseTime("2024-01-01T10:00:01Z"),
			},
			expectedLogs: 1,
		},
		{
			name: "metadata extraction",
			mockResponse: &api.AppLogsResponse{
				Logs: []api.AppLogEntry{
					{
						LogID:         "log-1",
						Timestamp:     "2024-01-01T10:00:00Z",
						LogLine:       "Test log",
						Stream:        "stderr",
						RunID:         "run-123",
						ContainerID:   "container-456",
						ContainerName: "web-1",
						LineNumber:    42,
					},
				},
			},
			expectedLogs: 1,
			validateLogs: func(t *testing.T, logs []Log) {
				assert.Len(t, logs, 1)
				assert.Equal(t, "Test log", logs[0].Content)
				assert.Equal(t, "log-1", logs[0].ID)
				assert.Equal(t, "stderr", logs[0].Stream)
				assert.Equal(t, "run-123", logs[0].Metadata["runID"])
				assert.Equal(t, "container-456", logs[0].Metadata["containerID"])
				assert.Equal(t, "web-1", logs[0].Metadata["containerName"])
				assert.Equal(t, 42, logs[0].Metadata["lineNumber"])
			},
		},
		{
			name: "callback error propagates",
			mockResponse: &api.AppLogsResponse{
				Logs: []api.AppLogEntry{
					{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Test", Stream: "stdout"},
				},
			},
			expectedError: true,
			expectedLogs:  1,
		},
		{
			name: "sinceTime used as initial afterDate",
			mockResponse: &api.AppLogsResponse{
				Logs: []api.AppLogEntry{
					{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Test", Stream: "stdout"},
				},
			},
			existingState: &pollingAppLogProvider{
				sinceTime:     "2024-01-01T09:00:00Z",
				lastTimestamp: mustParseTime("2024-01-01T09:00:00Z"),
			},
			expectedLogs: 1,
		},
		{
			name: "runID filter passed to API",
			mockResponse: &api.AppLogsResponse{
				Logs: []api.AppLogEntry{
					{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Run log", Stream: "stdout", RunID: "run-123"},
				},
			},
			existingState: &pollingAppLogProvider{
				runID: "run-123",
			},
			expectedLogs: 1,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			var collectedLogs []Log

			mockClient := apimock.NewMockClient(t)

			// Build expected options
			var expectedOpts api.AppLogOptions
			if tc.existingState != nil {
				if !tc.existingState.lastTimestamp.IsZero() {
					expectedOpts.AfterDate = tc.existingState.lastTimestamp.Format(time.RFC3339)
				}
				if tc.existingState.runID != "" {
					expectedOpts.RunID = tc.existingState.runID
				}
			}

			mockClient.On("FetchAppLogs", ctx, "test-project", "test-app-id", expectedOpts).
				Return(tc.mockResponse, tc.mockError).Once()

			// Create provider with existing state if provided
			provider := &pollingAppLogProvider{
				client:       mockClient,
				projectID:    "test-project",
				appID:        "test-app-id",
				pollInterval: 10 * time.Millisecond,
				seenIDs:      make(map[string]bool),
			}

			if tc.existingState != nil {
				if tc.existingState.seenIDs != nil {
					provider.seenIDs = tc.existingState.seenIDs
				}
				provider.lastTimestamp = tc.existingState.lastTimestamp
				provider.sinceTime = tc.existingState.sinceTime
				provider.runID = tc.existingState.runID
			}

			// Callback that collects logs and potentially returns error
			callback := func(logs []Log) error {
				collectedLogs = append(collectedLogs, logs...)
				if tc.name == "callback error propagates" {
					return errors.New("callback processing failed")
				}
				return nil
			}

			err := provider.fetchOnce(ctx, callback)

			if tc.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Len(t, collectedLogs, tc.expectedLogs)

			if tc.validateLogs != nil {
				tc.validateLogs(t, collectedLogs)
			}

			if tc.validateState != nil {
				tc.validateState(t, provider)
			}
		})
	}
}

// Helper function to parse time or panic
func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
