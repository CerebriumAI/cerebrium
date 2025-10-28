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

func Test_pollingBuildLogProvider_fetchOnce(t *testing.T) {
	tcs := []struct {
		name          string
		mockResponse  *api.BuildLogsResponse
		mockError     error
		existingLogs  map[string]bool
		expectedLogs  int
		expectedError bool
		validateLogs  func(t *testing.T, logs []Log)
	}{
		{
			name: "successful fetch with logs",
			mockResponse: &api.BuildLogsResponse{
				Logs: []api.BuildLog{
					{CreatedAt: "2024-01-01T10:00:00Z", Log: "Building..."},
					{CreatedAt: "2024-01-01T10:00:01Z", Log: "Installing dependencies..."},
				},
				Status: "building",
			},
			expectedLogs: 2,
			validateLogs: func(t *testing.T, logs []Log) {
				assert.Len(t, logs, 2)
				assert.Equal(t, "Building...", logs[0].Content)
				assert.Equal(t, "Installing dependencies...", logs[1].Content)
				assert.Equal(t, "building", logs[0].Metadata["buildStatus"])
			},
		},
		{
			name: "deduplication - seen logs filtered out",
			mockResponse: &api.BuildLogsResponse{
				Logs: []api.BuildLog{
					{CreatedAt: "2024-01-01T10:00:00Z", Log: "Building..."},
					{CreatedAt: "2024-01-01T10:00:01Z", Log: "Installing dependencies..."},
				},
				Status: "building",
			},
			existingLogs: map[string]bool{
				"2024-01-01T10:00:00ZBuilding...": true,
			},
			expectedLogs: 1,
			validateLogs: func(t *testing.T, logs []Log) {
				assert.Len(t, logs, 1)
				assert.Equal(t, "Installing dependencies...", logs[0].Content)
			},
		},
		{
			name: "build failure status in metadata",
			mockResponse: &api.BuildLogsResponse{
				Logs: []api.BuildLog{
					{CreatedAt: "2024-01-01T10:00:00Z", Log: "Error: build failed"},
				},
				Status: "build_failure",
			},
			expectedLogs: 1,
			validateLogs: func(t *testing.T, logs []Log) {
				assert.Len(t, logs, 1)
				assert.Equal(t, "Error: build failed", logs[0].Content)
				assert.Equal(t, "build_failure", logs[0].Metadata["buildStatus"])
			},
		},
		{
			name:          "API error",
			mockError:     errors.New("API connection failed"),
			expectedError: true,
		},
		{
			name: "empty logs",
			mockResponse: &api.BuildLogsResponse{
				Logs:   []api.BuildLog{},
				Status: "success",
			},
			expectedLogs: 0,
		},
		{
			name: "logs with ID field",
			mockResponse: &api.BuildLogsResponse{
				Logs: []api.BuildLog{
					{ID: "log-1", CreatedAt: "2024-01-01T10:00:00Z", Log: "Log with ID"},
				},
				Status: "building",
			},
			expectedLogs: 1,
			validateLogs: func(t *testing.T, logs []Log) {
				assert.Len(t, logs, 1)
				assert.Equal(t, "log-1", logs[0].ID)
				assert.Equal(t, "Log with ID", logs[0].Content)
			},
		},
		{
			name: "deduplication with ID field",
			mockResponse: &api.BuildLogsResponse{
				Logs: []api.BuildLog{
					{ID: "log-1", CreatedAt: "2024-01-01T10:00:00Z", Log: "Log with ID"},
					{ID: "log-2", CreatedAt: "2024-01-01T10:00:01Z", Log: "Another log"},
				},
				Status: "building",
			},
			existingLogs: map[string]bool{
				"log-1": true,
			},
			expectedLogs: 1,
			validateLogs: func(t *testing.T, logs []Log) {
				assert.Len(t, logs, 1)
				assert.Equal(t, "log-2", logs[0].ID)
			},
		},
		{
			name: "callback error propagates",
			mockResponse: &api.BuildLogsResponse{
				Logs: []api.BuildLog{
					{CreatedAt: "2024-01-01T10:00:00Z", Log: "Test log"},
				},
				Status: "building",
			},
			expectedError: true,
			expectedLogs:  1,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			var collectedLogs []Log

			mockClient := apimock.NewMockClient(t)
			mockClient.On("FetchBuildLogs", ctx, "test-project", "test-app", "build-123").
				Return(tc.mockResponse, tc.mockError).Once()

			provider := &pollingBuildLogProvider{
				client:       mockClient,
				projectID:    "test-project",
				appName:      "test-app",
				buildID:      "build-123",
				pollInterval: 10 * time.Millisecond,
				seenIDs:      tc.existingLogs,
			}

			if provider.seenIDs == nil {
				provider.seenIDs = make(map[string]bool)
			}

			// Callback that collects logs and potentially returns error for callback error test
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
		})
	}
}
