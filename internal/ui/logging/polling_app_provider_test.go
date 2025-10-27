package logging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	apimock "github.com/cerebriumai/cerebrium/internal/api/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestPollingAppLogProvider_NoFollow(t *testing.T) {
	ctx := context.Background()

	tcs := []struct {
		name          string
		mockResponse  *api.AppLogsResponse
		mockError     error
		expectedLogs  int
		expectedError bool
		validateLogs  func(t *testing.T, logs []Log)
	}{
		{
			name: "fetch once with logs",
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
			name: "fetch once with empty logs",
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
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			mockClient := apimock.NewMockClient(t)

			// Set up the mock expectation
			mockClient.On("FetchAppLogs", ctx, "test-project", "test-app-id", api.AppLogOptions{}).
				Return(tc.mockResponse, tc.mockError).Once()

			provider := NewPollingAppLogProvider(PollingAppLogProviderConfig{
				Client:       mockClient,
				ProjectID:    "test-project",
				AppID:        "test-app-id",
				Follow:       false, // No follow mode
				PollInterval: 10 * time.Millisecond,
			})

			var collectedLogs []Log
			err := provider.Collect(ctx, func(logs []Log) error {
				collectedLogs = append(collectedLogs, logs...)
				return nil
			})

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Len(t, collectedLogs, tc.expectedLogs)

			if tc.validateLogs != nil {
				tc.validateLogs(t, collectedLogs)
			}
		})
	}
}

func TestPollingAppLogProvider_Follow(t *testing.T) {
	ctx := context.Background()

	tcs := []struct {
		name           string
		mockResponses  []*api.AppLogsResponse
		mockErrors     []error
		expectedLogs   int
		expectedError  bool
		cancelAfter    time.Duration
		expectCanceled bool
		validateLogs   func(t *testing.T, logs []Log)
	}{
		{
			name: "follow mode with multiple fetches",
			mockResponses: []*api.AppLogsResponse{
				{
					Logs: []api.AppLogEntry{
						{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "First log", Stream: "stdout"},
					},
				},
				{
					Logs: []api.AppLogEntry{
						{LogID: "log-2", Timestamp: "2024-01-01T10:00:01Z", LogLine: "Second log", Stream: "stdout"},
					},
				},
				{
					Logs: []api.AppLogEntry{
						{LogID: "log-3", Timestamp: "2024-01-01T10:00:02Z", LogLine: "Third log", Stream: "stderr"},
					},
				},
			},
			cancelAfter:    80 * time.Millisecond, // Cancel after 3 fetches
			expectCanceled: true,
			expectedLogs:   3,
			validateLogs: func(t *testing.T, logs []Log) {
				assert.Len(t, logs, 3)
				assert.Equal(t, "First log", logs[0].Content)
				assert.Equal(t, "Second log", logs[1].Content)
				assert.Equal(t, "Third log", logs[2].Content)
				assert.Equal(t, "stderr", logs[2].Stream)
			},
		},
		{
			name: "follow mode with empty fetches",
			mockResponses: []*api.AppLogsResponse{
				{Logs: []api.AppLogEntry{{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Log 1", Stream: "stdout"}}},
				{Logs: []api.AppLogEntry{}}, // Empty
				{Logs: []api.AppLogEntry{{LogID: "log-2", Timestamp: "2024-01-01T10:00:01Z", LogLine: "Log 2", Stream: "stdout"}}},
			},
			cancelAfter:    80 * time.Millisecond,
			expectCanceled: true,
			expectedLogs:   2, // Only non-empty batches
		},
		{
			name: "follow mode with API error",
			mockResponses: []*api.AppLogsResponse{
				{Logs: []api.AppLogEntry{{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Before error", Stream: "stdout"}}},
			},
			mockErrors:    []error{nil, errors.New("network error")},
			expectedError: true,
			expectedLogs:  1, // First fetch should succeed
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			var collectedLogs []Log

			mockClient := apimock.NewMockClient(t)

			// Set up mock expectations for each call
			// Use mock.Anything for context since follow mode uses testCtx which may be cancelled
			for i := 0; i < len(tc.mockResponses) || i < len(tc.mockErrors); i++ {
				var response *api.AppLogsResponse
				var err error

				if i < len(tc.mockErrors) && tc.mockErrors[i] != nil {
					err = tc.mockErrors[i]
				} else if i < len(tc.mockResponses) {
					response = tc.mockResponses[i]
				} else {
					response = &api.AppLogsResponse{Logs: []api.AppLogEntry{}}
				}

				mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app-id", mock.Anything).
					Return(response, err).Once()
			}

			// Add one more call for empty response if needed
			if len(tc.mockResponses) > 0 || len(tc.mockErrors) > 0 {
				mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app-id", mock.Anything).
					Return(&api.AppLogsResponse{Logs: []api.AppLogEntry{}}, nil).Maybe()
			}

			provider := NewPollingAppLogProvider(PollingAppLogProviderConfig{
				Client:       mockClient,
				ProjectID:    "test-project",
				AppID:        "test-app-id",
				Follow:       true, // Follow mode
				PollInterval: 20 * time.Millisecond,
			})

			testCtx := ctx
			if tc.cancelAfter > 0 {
				var cancel context.CancelFunc
				testCtx, cancel = context.WithCancel(ctx)
				time.AfterFunc(tc.cancelAfter, cancel)
			}

			err := provider.Collect(testCtx, func(logs []Log) error {
				collectedLogs = append(collectedLogs, logs...)
				return nil
			})

			if tc.expectCanceled {
				assert.ErrorIs(t, err, context.Canceled)
			} else if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Len(t, collectedLogs, tc.expectedLogs)

			if tc.validateLogs != nil {
				tc.validateLogs(t, collectedLogs)
			}
		})
	}
}

func TestPollingAppLogProvider_SinceTime(t *testing.T) {
	ctx := context.Background()

	mockClient := apimock.NewMockClient(t)

	expectedOpts := api.AppLogOptions{
		AfterDate: "2024-01-01T09:00:00Z",
	}

	mockClient.On("FetchAppLogs", ctx, "test-project", "test-app-id", expectedOpts).
		Return(&api.AppLogsResponse{
			Logs: []api.AppLogEntry{
				{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Test log", Stream: "stdout"},
			},
		}, nil).Once()

	provider := NewPollingAppLogProvider(PollingAppLogProviderConfig{
		Client:       mockClient,
		ProjectID:    "test-project",
		AppID:        "test-app-id",
		Follow:       false,
		SinceTime:    "2024-01-01T09:00:00Z",
		PollInterval: 10 * time.Millisecond,
	})

	err := provider.Collect(ctx, func(logs []Log) error {
		return nil
	})

	require.NoError(t, err)
}

func TestPollingAppLogProvider_RunIDFilter(t *testing.T) {
	ctx := context.Background()

	mockClient := apimock.NewMockClient(t)

	expectedOpts := api.AppLogOptions{
		RunID: "run-123",
	}

	mockClient.On("FetchAppLogs", ctx, "test-project", "test-app-id", expectedOpts).
		Return(&api.AppLogsResponse{
			Logs: []api.AppLogEntry{
				{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Run log", Stream: "stdout", RunID: "run-123"},
			},
		}, nil).Once()

	provider := NewPollingAppLogProvider(PollingAppLogProviderConfig{
		Client:       mockClient,
		ProjectID:    "test-project",
		AppID:        "test-app-id",
		Follow:       false,
		RunID:        "run-123",
		PollInterval: 10 * time.Millisecond,
	})

	err := provider.Collect(ctx, func(logs []Log) error {
		return nil
	})

	require.NoError(t, err)
}

func TestPollingAppLogProvider_LastTimestampTracking(t *testing.T) {
	ctx := context.Background()

	mockClient := apimock.NewMockClient(t)

	// First call - no afterDate (use mock.Anything for context since testCtx will be cancelled)
	firstOpts := api.AppLogOptions{}
	mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app-id", firstOpts).
		Return(&api.AppLogsResponse{
			Logs: []api.AppLogEntry{
				{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Log 1", Stream: "stdout"},
				{LogID: "log-2", Timestamp: "2024-01-01T10:00:01Z", LogLine: "Log 2", Stream: "stdout"},
			},
		}, nil).Once()

	// Second call - with afterDate from last timestamp
	secondOpts := api.AppLogOptions{AfterDate: "2024-01-01T10:00:01Z"}
	mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app-id", secondOpts).
		Return(&api.AppLogsResponse{
			Logs: []api.AppLogEntry{
				{LogID: "log-3", Timestamp: "2024-01-01T10:00:02Z", LogLine: "Log 3", Stream: "stdout"},
			},
		}, nil).Once()

	// Subsequent calls
	mockClient.On("FetchAppLogs", mock.Anything, "test-project", "test-app-id", mock.Anything).
		Return(&api.AppLogsResponse{Logs: []api.AppLogEntry{}}, nil).Maybe()

	provider := NewPollingAppLogProvider(PollingAppLogProviderConfig{
		Client:       mockClient,
		ProjectID:    "test-project",
		AppID:        "test-app-id",
		Follow:       true,
		PollInterval: 20 * time.Millisecond,
	})

	testCtx, cancel := context.WithCancel(ctx)
	time.AfterFunc(60*time.Millisecond, cancel)

	_ = provider.Collect(testCtx, func(logs []Log) error {
		return nil
	})
}

func TestPollingAppLogProvider_CallbackError(t *testing.T) {
	ctx := context.Background()

	mockClient := apimock.NewMockClient(t)

	mockClient.On("FetchAppLogs", ctx, "test-project", "test-app-id", api.AppLogOptions{}).
		Return(&api.AppLogsResponse{
			Logs: []api.AppLogEntry{
				{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Test log", Stream: "stdout"},
			},
		}, nil).Once()

	provider := NewPollingAppLogProvider(PollingAppLogProviderConfig{
		Client:       mockClient,
		ProjectID:    "test-project",
		AppID:        "test-app-id",
		Follow:       false,
		PollInterval: 10 * time.Millisecond,
	})

	callbackErr := errors.New("callback processing failed")
	err := provider.Collect(ctx, func(logs []Log) error {
		return callbackErr
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "callback error")
}

func TestPollingAppLogProvider_Metadata(t *testing.T) {
	ctx := context.Background()

	mockClient := apimock.NewMockClient(t)

	mockClient.On("FetchAppLogs", ctx, "test-project", "test-app-id", api.AppLogOptions{}).
		Return(&api.AppLogsResponse{
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
		}, nil).Once()

	provider := NewPollingAppLogProvider(PollingAppLogProviderConfig{
		Client:       mockClient,
		ProjectID:    "test-project",
		AppID:        "test-app-id",
		Follow:       false,
		PollInterval: 10 * time.Millisecond,
	})

	var capturedLog Log
	err := provider.Collect(ctx, func(logs []Log) error {
		if len(logs) > 0 {
			capturedLog = logs[0]
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, "Test log", capturedLog.Content)
	assert.Equal(t, "log-1", capturedLog.ID)
	assert.Equal(t, "stderr", capturedLog.Stream)
	assert.Equal(t, "run-123", capturedLog.Metadata["runID"])
	assert.Equal(t, "container-456", capturedLog.Metadata["containerID"])
	assert.Equal(t, "web-1", capturedLog.Metadata["containerName"])
	assert.Equal(t, 42, capturedLog.Metadata["lineNumber"])
}
