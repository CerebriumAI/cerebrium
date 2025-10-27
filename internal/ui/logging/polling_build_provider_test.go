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
			collectedLogs := []Log{}

			mockClient := apimock.NewMockClient(t)

			// Set up mock expectations for each call
			for i := 0; i < len(tc.mockResponses) || i < len(tc.mockErrors); i++ {
				var response *api.BuildLogsResponse
				var err error

				if i < len(tc.mockErrors) && tc.mockErrors[i] != nil {
					err = tc.mockErrors[i]
				} else if i < len(tc.mockResponses) {
					response = tc.mockResponses[i]
				} else {
					response = &api.BuildLogsResponse{Status: "success"}
				}

				// Use mock.Anything for context if this is a cancellation test
				if tc.cancelAfter > 0 {
					mockClient.On("FetchBuildLogs", mock.Anything, "test-project", "test-app", "build-123").
						Return(response, err).Once()
				} else {
					mockClient.On("FetchBuildLogs", ctx, "test-project", "test-app", "build-123").
						Return(response, err).Once()
				}
			}

			// Add terminal response if needed
			if tc.cancelAfter == 0 && len(tc.mockResponses) > 0 {
				mockClient.On("FetchBuildLogs", ctx, "test-project", "test-app", "build-123").
					Return(&api.BuildLogsResponse{Status: "success"}, nil).Maybe()
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

	mockClient := apimock.NewMockClient(t)

	// Set up multiple calls returning the same logs
	sameLogsResponse := &api.BuildLogsResponse{
		Logs: []api.BuildLog{
			{CreatedAt: "2024-01-01T10:00:00Z", Log: "Log line 1"},
			{CreatedAt: "2024-01-01T10:00:01Z", Log: "Log line 2"},
		},
		Status: "building",
	}

	// First two calls return building status
	mockClient.On("FetchBuildLogs", ctx, "test-project", "test-app", "build-123").
		Return(sameLogsResponse, nil).Twice()

	// Third call returns success status
	successResponse := &api.BuildLogsResponse{
		Logs: []api.BuildLog{
			{CreatedAt: "2024-01-01T10:00:00Z", Log: "Log line 1"},
			{CreatedAt: "2024-01-01T10:00:01Z", Log: "Log line 2"},
		},
		Status: "success",
	}
	mockClient.On("FetchBuildLogs", ctx, "test-project", "test-app", "build-123").
		Return(successResponse, nil).Once()

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

	mockClient := apimock.NewMockClient(t)

	mockClient.On("FetchBuildLogs", ctx, "test-project", "test-app", "build-123").
		Return(&api.BuildLogsResponse{
			Logs: []api.BuildLog{
				{CreatedAt: "2024-01-01T10:00:00Z", Log: "Test log"},
			},
			Status: "building",
		}, nil).Once()

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

