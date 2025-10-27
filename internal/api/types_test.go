package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun_GetDisplayStatus(t *testing.T) {
	tcs := []struct {
		name       string
		statusCode *int
		status     string
		expected   string
	}{
		{
			name:       "status code -1 returns closed",
			statusCode: intPtr(-1),
			status:     "anything",
			expected:   "closed",
		},
		{
			name:       "status code 0 returns cancelled",
			statusCode: intPtr(0),
			status:     "anything",
			expected:   "cancelled",
		},
		{
			name:       "status code 200 returns success",
			statusCode: intPtr(200),
			status:     "anything",
			expected:   "success",
		},
		{
			name:       "status code 201 returns success",
			statusCode: intPtr(201),
			status:     "anything",
			expected:   "success",
		},
		{
			name:       "status code 299 returns success",
			statusCode: intPtr(299),
			status:     "anything",
			expected:   "success",
		},
		{
			name:       "status code 500 returns failure",
			statusCode: intPtr(500),
			status:     "anything",
			expected:   "failure",
		},
		{
			name:       "status code 503 returns failure",
			statusCode: intPtr(503),
			status:     "anything",
			expected:   "failure",
		},
		{
			name:       "status code 404 returns 404",
			statusCode: intPtr(404),
			status:     "anything",
			expected:   "404",
		},
		{
			name:       "status code 100 returns 100",
			statusCode: intPtr(100),
			status:     "anything",
			expected:   "100",
		},
		{
			name:       "containerQueued without status code returns queued",
			statusCode: nil,
			status:     "containerQueued",
			expected:   "queued",
		},
		{
			name:       "proxyQueued without status code returns queued",
			statusCode: nil,
			status:     "proxyQueued",
			expected:   "queued",
		},
		{
			name:       "pending without status code returns pending",
			statusCode: nil,
			status:     "pending",
			expected:   "pending",
		},
		{
			name:       "processing without status code returns processing",
			statusCode: nil,
			status:     "processing",
			expected:   "processing",
		},
		{
			name:       "other status returns lowercase",
			statusCode: nil,
			status:     "SomeStatus",
			expected:   "somestatus",
		},
		{
			name:       "empty status returns unknown",
			statusCode: nil,
			status:     "",
			expected:   "unknown",
		},
		{
			name:       "status code takes priority over containerQueued",
			statusCode: intPtr(200),
			status:     "containerQueued",
			expected:   "success",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			run := Run{
				StatusCode: tc.statusCode,
				Status:     tc.status,
			}

			result := run.GetDisplayStatus()
			assert.Equal(t, tc.expected, result)
		})
	}
}

// intPtr is a helper function to create int pointers for tests
func intPtr(i int) *int {
	return &i
}
