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

// mockBuildLogClient implements api.Client for testing build logs
type mockBuildLogClient struct {
	fetchBuildLogsFunc func(ctx context.Context, projectID, appName, buildID string) (*api.BuildLogsResponse, error)
}

func (m *mockBuildLogClient) FetchBuildLogs(ctx context.Context, projectID, appName, buildID string) (*api.BuildLogsResponse, error) {
	if m.fetchBuildLogsFunc != nil {
		return m.fetchBuildLogsFunc(ctx, projectID, appName, buildID)
	}
	return &api.BuildLogsResponse{}, nil
}

// Implement other api.Client methods as no-ops
func (m *mockBuildLogClient) GetApps(ctx context.Context, projectID string) ([]api.App, error) {
	return nil, nil
}
func (m *mockBuildLogClient) GetApp(ctx context.Context, projectID, appID string) (*api.AppDetails, error) {
	return nil, nil
}
func (m *mockBuildLogClient) DeleteApp(ctx context.Context, projectID, appID string) error {
	return nil
}
func (m *mockBuildLogClient) UpdateApp(ctx context.Context, projectID, appID string, updates map[string]any) error {
	return nil
}
func (m *mockBuildLogClient) GetProjects(ctx context.Context) ([]api.Project, error) {
	return nil, nil
}
func (m *mockBuildLogClient) GetRuns(ctx context.Context, projectID, appID string, asyncOnly bool) ([]api.Run, error) {
	return nil, nil
}
func (m *mockBuildLogClient) CreateApp(ctx context.Context, projectID string, payload map[string]any) (*api.CreateAppResponse, error) {
	return nil, nil
}
func (m *mockBuildLogClient) UploadZip(ctx context.Context, uploadURL string, zipPath string) error {
	return nil
}
func (m *mockBuildLogClient) FetchAppLogs(ctx context.Context, projectID, appID string, opts api.AppLogOptions) (*api.AppLogsResponse, error) {
	return nil, nil
}
func (m *mockBuildLogClient) FetchNotifications(ctx context.Context) ([]api.Notification, error) {
	return nil, nil
}
func (m *mockBuildLogClient) CancelBuild(ctx context.Context, projectID, appName, buildID string) error {
	return nil
}
func (m *mockBuildLogClient) CreateRunApp(ctx context.Context, projectID, appID, region string) error {
	return nil
}
func (m *mockBuildLogClient) RunApp(ctx context.Context, projectID, appID, region, filename string, functionName *string, imageDigest *string, hardwareConfig map[string]any, tarPath string, data map[string]any) (*api.RunResponse, error) {
	return nil, nil
}
func (m *mockBuildLogClient) GetRunStatus(ctx context.Context, projectID, appName, runID string) (*api.RunStatus, error) {
	return nil, nil
}
func (m *mockBuildLogClient) FetchRunLogs(ctx context.Context, projectID, appName, runID, nextToken string) (*api.RunLogsResponse, error) {
	return nil, nil
}
func (m *mockBuildLogClient) CreateBaseImage(ctx context.Context, projectID, region string, dependencies map[string]any) (string, error) {
	return "", nil
}
func (m *mockBuildLogClient) ListFiles(ctx context.Context, projectID, path, region string) ([]api.FileInfo, error) {
	return nil, nil
}
func (m *mockBuildLogClient) InitiateUpload(ctx context.Context, projectID, filePath, region string, partCount int) (*api.InitiateUploadResponse, error) {
	return nil, nil
}
func (m *mockBuildLogClient) UploadPart(ctx context.Context, url string, data []byte) (string, error) {
	return "", nil
}
func (m *mockBuildLogClient) CompleteUpload(ctx context.Context, projectID, filePath, uploadID, region string, parts []api.PartInfo) error {
	return nil
}
func (m *mockBuildLogClient) GetDownloadURL(ctx context.Context, projectID, filePath, region string) (string, error) {
	return "", nil
}
func (m *mockBuildLogClient) DeleteFile(ctx context.Context, projectID, filePath, region string) error {
	return nil
}

func TestPollingBuildLogProvider_Collect(t *testing.T) {
	ctx := context.Background()

	tcs := []struct {
		name           string
		mockResponses  []*api.BuildLogsResponse
		mockErrors     []error
		expectedLogs   int
		expectedError  bool
		validateLogs   func(t *testing.T, logs []Log)
		cancelAfter    time.Duration
		expectCanceled bool
	}{
		{
			name: "successful build with logs",
			mockResponses: []*api.BuildLogsResponse{
				{
					Logs: []api.BuildLog{
						{CreatedAt: "2024-01-01T10:00:00Z", Log: "Building..."},
						{CreatedAt: "2024-01-01T10:00:01Z", Log: "Installing dependencies..."},
					},
					Status: "building",
				},
				{
					Logs: []api.BuildLog{
						{CreatedAt: "2024-01-01T10:00:00Z", Log: "Building..."},
						{CreatedAt: "2024-01-01T10:00:01Z", Log: "Installing dependencies..."},
						{CreatedAt: "2024-01-01T10:00:02Z", Log: "Build complete!"},
					},
					Status: "success",
				},
			},
			expectedLogs: 3,
			validateLogs: func(t *testing.T, logs []Log) {
				assert.Len(t, logs, 3)
				assert.Equal(t, "Building...", logs[0].Content)
				assert.Equal(t, "Installing dependencies...", logs[1].Content)
				assert.Equal(t, "Build complete!", logs[2].Content)

				// Check metadata
				assert.Equal(t, "success", logs[2].Metadata["buildStatus"])
			},
		},
		{
			name: "build with no new logs on second poll",
			mockResponses: []*api.BuildLogsResponse{
				{
					Logs: []api.BuildLog{
						{CreatedAt: "2024-01-01T10:00:00Z", Log: "Starting build..."},
					},
					Status: "building",
				},
				{
					Logs: []api.BuildLog{
						{CreatedAt: "2024-01-01T10:00:00Z", Log: "Starting build..."},
					},
					Status: "success",
				},
			},
			expectedLogs: 1, // Deduplication should prevent duplicate
			validateLogs: func(t *testing.T, logs []Log) {
				assert.Len(t, logs, 1)
				assert.Equal(t, "Starting build...", logs[0].Content)
			},
		},
		{
			name: "build failure status",
			mockResponses: []*api.BuildLogsResponse{
				{
					Logs: []api.BuildLog{
						{CreatedAt: "2024-01-01T10:00:00Z", Log: "Building..."},
					},
					Status: "building",
				},
				{
					Logs: []api.BuildLog{
						{CreatedAt: "2024-01-01T10:00:00Z", Log: "Building..."},
						{CreatedAt: "2024-01-01T10:00:01Z", Log: "Error: build failed"},
					},
					Status: "build_failure",
				},
			},
			expectedLogs: 2,
			validateLogs: func(t *testing.T, logs []Log) {
				assert.Len(t, logs, 2)
				assert.Equal(t, "Error: build failed", logs[1].Content)
				assert.Equal(t, "build_failure", logs[1].Metadata["buildStatus"])
			},
		},
		{
			name:          "API error on first fetch",
			mockErrors:    []error{errors.New("API connection failed")},
			expectedError: true,
		},
		{
			name: "API error on second fetch",
			mockResponses: []*api.BuildLogsResponse{
				{
					Logs:   []api.BuildLog{{CreatedAt: "2024-01-01T10:00:00Z", Log: "Starting..."}},
					Status: "building",
				},
			},
			mockErrors:    []error{nil, errors.New("connection lost")},
			expectedError: true,
			expectedLogs:  1, // First batch should succeed
		},
		{
			name: "context cancellation during polling",
			mockResponses: []*api.BuildLogsResponse{
				{
					Logs:   []api.BuildLog{{CreatedAt: "2024-01-01T10:00:00Z", Log: "Building..."}},
					Status: "building",
				},
				{
					Logs:   []api.BuildLog{{CreatedAt: "2024-01-01T10:00:01Z", Log: "Still building..."}},
					Status: "building",
				},
				{
					Logs:   []api.BuildLog{{CreatedAt: "2024-01-01T10:00:02Z", Log: "More building..."}},
					Status: "building",
				},
			},
			cancelAfter:    25 * time.Millisecond, // Cancel during polling loop
			expectCanceled: true,
			// Note: Due to immediate fetch + ticker, we may get 1 or 2 logs depending on timing
			// This is acceptable behavior - cancellation is eventually respected
		},
		{
			name: "empty logs but completes successfully",
			mockResponses: []*api.BuildLogsResponse{
				{
					Logs:   []api.BuildLog{},
					Status: "success",
				},
			},
			expectedLogs: 0,
		},
		{
			name: "multiple terminal statuses",
			mockResponses: []*api.BuildLogsResponse{
				{
					Logs:   []api.BuildLog{{CreatedAt: "2024-01-01T10:00:00Z", Log: "Ready"}},
					Status: "ready",
				},
			},
			expectedLogs: 1,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			callCount := 0
			collectedLogs := []Log{}

			mockClient := &mockBuildLogClient{
				fetchBuildLogsFunc: func(ctx context.Context, projectID, appName, buildID string) (*api.BuildLogsResponse, error) {
					defer func() { callCount++ }()

					if callCount < len(tc.mockErrors) && tc.mockErrors[callCount] != nil {
						return nil, tc.mockErrors[callCount]
					}

					if callCount < len(tc.mockResponses) {
						return tc.mockResponses[callCount], nil
					}

					// Shouldn't reach here in normal operation
					return &api.BuildLogsResponse{Status: "success"}, nil
				},
			}

			provider := NewPollingBuildLogProvider(PollingBuildLogProviderConfig{
				Client:       mockClient,
				ProjectID:    "test-project",
				AppName:      "test-app",
				BuildID:      "build-123",
				PollInterval: 10 * time.Millisecond, // Fast polling for tests
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

			if tc.expectedLogs > 0 {
				assert.Len(t, collectedLogs, tc.expectedLogs)
			}

			if tc.validateLogs != nil {
				tc.validateLogs(t, collectedLogs)
			}
		})
	}
}

func TestPollingBuildLogProvider_Deduplication(t *testing.T) {
	ctx := context.Background()

	callCount := 0
	mockClient := &mockBuildLogClient{
		fetchBuildLogsFunc: func(ctx context.Context, projectID, appName, buildID string) (*api.BuildLogsResponse, error) {
			defer func() { callCount++ }()

			// Always return same logs to test deduplication
			return &api.BuildLogsResponse{
				Logs: []api.BuildLog{
					{CreatedAt: "2024-01-01T10:00:00Z", Log: "Log line 1"},
					{CreatedAt: "2024-01-01T10:00:01Z", Log: "Log line 2"},
				},
				Status: func() string {
					if callCount >= 2 {
						return "success"
					}
					return "building"
				}(),
			}, nil
		},
	}

	provider := NewPollingBuildLogProvider(PollingBuildLogProviderConfig{
		Client:       mockClient,
		ProjectID:    "test-project",
		AppName:      "test-app",
		BuildID:      "build-123",
		PollInterval: 10 * time.Millisecond,
	})

	collectedLogs := []Log{}
	err := provider.Collect(ctx, func(logs []Log) error {
		collectedLogs = append(collectedLogs, logs...)
		return nil
	})

	require.NoError(t, err)

	// Should only collect 2 unique logs despite multiple polls
	assert.Len(t, collectedLogs, 2)
	assert.Equal(t, "Log line 1", collectedLogs[0].Content)
	assert.Equal(t, "Log line 2", collectedLogs[1].Content)
}

func TestPollingBuildLogProvider_CallbackError(t *testing.T) {
	ctx := context.Background()

	mockClient := &mockBuildLogClient{
		fetchBuildLogsFunc: func(ctx context.Context, projectID, appName, buildID string) (*api.BuildLogsResponse, error) {
			return &api.BuildLogsResponse{
				Logs: []api.BuildLog{
					{CreatedAt: "2024-01-01T10:00:00Z", Log: "Test log"},
				},
				Status: "building",
			}, nil
		},
	}

	provider := NewPollingBuildLogProvider(PollingBuildLogProviderConfig{
		Client:       mockClient,
		ProjectID:    "test-project",
		AppName:      "test-app",
		BuildID:      "build-123",
		PollInterval: 10 * time.Millisecond,
	})

	callbackErr := errors.New("callback processing failed")
	err := provider.Collect(ctx, func(logs []Log) error {
		return callbackErr
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "callback error")
}

func Test_isTerminalStatus(t *testing.T) {
	tcs := []struct {
		status     string
		isTerminal bool
	}{
		{status: "success", isTerminal: true},
		{status: "build_failure", isTerminal: true},
		{status: "init_failure", isTerminal: true},
		{status: "ready", isTerminal: true},
		{status: "failure", isTerminal: true},
		{status: "cancelled", isTerminal: true},
		{status: "init_timeout", isTerminal: true},
		{status: "building", isTerminal: false},
		{status: "pending", isTerminal: false},
		{status: "unknown", isTerminal: false},
		{status: "", isTerminal: false},
	}

	for _, tc := range tcs {
		t.Run(tc.status, func(t *testing.T) {
			result := isTerminalStatus(tc.status)
			assert.Equal(t, tc.isTerminal, result)
		})
	}
}
