package logging

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAppLogClient implements api.Client for testing app logs
type mockAppLogClient struct {
	fetchAppLogsFunc func(ctx context.Context, projectID, appID string, opts api.AppLogOptions) (*api.AppLogsResponse, error)
}

func (m *mockAppLogClient) FetchAppLogs(ctx context.Context, projectID, appID string, opts api.AppLogOptions) (*api.AppLogsResponse, error) {
	if m.fetchAppLogsFunc != nil {
		return m.fetchAppLogsFunc(ctx, projectID, appID, opts)
	}
	return &api.AppLogsResponse{}, nil
}

// Implement other api.Client methods as no-ops
func (m *mockAppLogClient) GetApps(ctx context.Context, projectID string) ([]api.App, error) {
	return nil, nil
}
func (m *mockAppLogClient) GetApp(ctx context.Context, projectID, appID string) (*api.AppDetails, error) {
	return nil, nil
}
func (m *mockAppLogClient) DeleteApp(ctx context.Context, projectID, appID string) error {
	return nil
}
func (m *mockAppLogClient) UpdateApp(ctx context.Context, projectID, appID string, updates map[string]any) error {
	return nil
}
func (m *mockAppLogClient) GetProjects(ctx context.Context) ([]api.Project, error) {
	return nil, nil
}
func (m *mockAppLogClient) GetRuns(ctx context.Context, projectID, appID string, asyncOnly bool) ([]api.Run, error) {
	return nil, nil
}
func (m *mockAppLogClient) CreateApp(ctx context.Context, projectID string, payload map[string]any) (*api.CreateAppResponse, error) {
	return nil, nil
}
func (m *mockAppLogClient) UploadZip(ctx context.Context, uploadURL string, zipPath string) error {
	return nil
}
func (m *mockAppLogClient) FetchBuildLogs(ctx context.Context, projectID, appName, buildID string) (*api.BuildLogsResponse, error) {
	return nil, nil
}
func (m *mockAppLogClient) FetchNotifications(ctx context.Context) ([]api.Notification, error) {
	return nil, nil
}
func (m *mockAppLogClient) CancelBuild(ctx context.Context, projectID, appName, buildID string) error {
	return nil
}
func (m *mockAppLogClient) CreateRunApp(ctx context.Context, projectID, appID, region string) error {
	return nil
}
func (m *mockAppLogClient) RunApp(ctx context.Context, projectID, appID, region, filename string, functionName *string, imageDigest *string, hardwareConfig map[string]any, tarPath string, data map[string]any) (*api.RunResponse, error) {
	return nil, nil
}
func (m *mockAppLogClient) GetRunStatus(ctx context.Context, projectID, appName, runID string) (*api.RunStatus, error) {
	return nil, nil
}
func (m *mockAppLogClient) FetchRunLogs(ctx context.Context, projectID, appName, runID, nextToken string) (*api.RunLogsResponse, error) {
	return nil, nil
}
func (m *mockAppLogClient) CreateBaseImage(ctx context.Context, projectID, region string, dependencies map[string]any) (string, error) {
	return "", nil
}
func (m *mockAppLogClient) ListFiles(ctx context.Context, projectID, path, region string) ([]api.FileInfo, error) {
	return nil, nil
}
func (m *mockAppLogClient) InitiateUpload(ctx context.Context, projectID, filePath, region string, partCount int) (*api.InitiateUploadResponse, error) {
	return nil, nil
}
func (m *mockAppLogClient) UploadPart(ctx context.Context, url string, data []byte) (string, error) {
	return "", nil
}
func (m *mockAppLogClient) CompleteUpload(ctx context.Context, projectID, filePath, uploadID, region string, parts []api.PartInfo) error {
	return nil
}
func (m *mockAppLogClient) GetDownloadURL(ctx context.Context, projectID, filePath, region string) (string, error) {
	return "", nil
}
func (m *mockAppLogClient) DeleteFile(ctx context.Context, projectID, filePath, region string) error {
	return nil
}

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
			mockClient := &mockAppLogClient{
				fetchAppLogsFunc: func(ctx context.Context, projectID, appID string, opts api.AppLogOptions) (*api.AppLogsResponse, error) {
					if tc.mockError != nil {
						return nil, tc.mockError
					}
					return tc.mockResponse, nil
				},
			}

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
			callCount := 0
			var collectedLogs []Log

			mockClient := &mockAppLogClient{
				fetchAppLogsFunc: func(ctx context.Context, projectID, appID string, opts api.AppLogOptions) (*api.AppLogsResponse, error) {
					defer func() { callCount++ }()

					if callCount < len(tc.mockErrors) && tc.mockErrors[callCount] != nil {
						return nil, tc.mockErrors[callCount]
					}

					if callCount < len(tc.mockResponses) {
						return tc.mockResponses[callCount], nil
					}

					// Return empty for subsequent calls
					return &api.AppLogsResponse{Logs: []api.AppLogEntry{}}, nil
				},
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

	var capturedOpts api.AppLogOptions
	mockClient := &mockAppLogClient{
		fetchAppLogsFunc: func(ctx context.Context, projectID, appID string, opts api.AppLogOptions) (*api.AppLogsResponse, error) {
			capturedOpts = opts
			return &api.AppLogsResponse{
				Logs: []api.AppLogEntry{
					{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Test log", Stream: "stdout"},
				},
			}, nil
		},
	}

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
	assert.Equal(t, "2024-01-01T09:00:00Z", capturedOpts.AfterDate)
}

func TestPollingAppLogProvider_RunIDFilter(t *testing.T) {
	ctx := context.Background()

	var capturedOpts api.AppLogOptions
	mockClient := &mockAppLogClient{
		fetchAppLogsFunc: func(ctx context.Context, projectID, appID string, opts api.AppLogOptions) (*api.AppLogsResponse, error) {
			capturedOpts = opts
			return &api.AppLogsResponse{
				Logs: []api.AppLogEntry{
					{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Run log", Stream: "stdout", RunID: "run-123"},
				},
			}, nil
		},
	}

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
	assert.Equal(t, "run-123", capturedOpts.RunID)
}

func TestPollingAppLogProvider_LastTimestampTracking(t *testing.T) {
	ctx := context.Background()

	callCount := 0
	var capturedOpts []api.AppLogOptions

	mockClient := &mockAppLogClient{
		fetchAppLogsFunc: func(ctx context.Context, projectID, appID string, opts api.AppLogOptions) (*api.AppLogsResponse, error) {
			capturedOpts = append(capturedOpts, opts)
			defer func() { callCount++ }()

			if callCount == 0 {
				return &api.AppLogsResponse{
					Logs: []api.AppLogEntry{
						{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Log 1", Stream: "stdout"},
						{LogID: "log-2", Timestamp: "2024-01-01T10:00:01Z", LogLine: "Log 2", Stream: "stdout"},
					},
				}, nil
			}

			return &api.AppLogsResponse{
				Logs: []api.AppLogEntry{
					{LogID: "log-3", Timestamp: "2024-01-01T10:00:02Z", LogLine: "Log 3", Stream: "stdout"},
				},
			}, nil
		},
	}

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

	require.GreaterOrEqual(t, len(capturedOpts), 2)

	// First request should have empty afterDate
	assert.Empty(t, capturedOpts[0].AfterDate)

	// Second request should have lastTimestamp from first batch
	assert.Equal(t, "2024-01-01T10:00:01Z", capturedOpts[1].AfterDate)
}

func TestPollingAppLogProvider_CallbackError(t *testing.T) {
	ctx := context.Background()

	mockClient := &mockAppLogClient{
		fetchAppLogsFunc: func(ctx context.Context, projectID, appID string, opts api.AppLogOptions) (*api.AppLogsResponse, error) {
			return &api.AppLogsResponse{
				Logs: []api.AppLogEntry{
					{LogID: "log-1", Timestamp: "2024-01-01T10:00:00Z", LogLine: "Test log", Stream: "stdout"},
				},
			}, nil
		},
	}

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

	mockClient := &mockAppLogClient{
		fetchAppLogsFunc: func(ctx context.Context, projectID, appID string, opts api.AppLogOptions) (*api.AppLogsResponse, error) {
			return &api.AppLogsResponse{
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
			}, nil
		},
	}

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
