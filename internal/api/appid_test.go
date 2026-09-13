package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeAppID(t *testing.T) {
	tcs := []struct {
		name      string
		projectID string
		appName   string
		expected  string
	}{
		{
			name:      "app name without project prefix",
			projectID: "dev-p-0780791d",
			appName:   "5-dockerfile",
			expected:  "dev-p-0780791d-5-dockerfile",
		},
		{
			name:      "app name with project prefix",
			projectID: "dev-p-0780791d",
			appName:   "dev-p-0780791d-5-dockerfile",
			expected:  "dev-p-0780791d-5-dockerfile",
		},
		{
			name:      "app name with partial match",
			projectID: "dev-p-0780791d",
			appName:   "dev-p-123-myapp",
			expected:  "dev-p-0780791d-dev-p-123-myapp",
		},
		{
			name:      "simple app name",
			projectID: "project-123",
			appName:   "myapp",
			expected:  "project-123-myapp",
		},
		{
			name:      "app name already has full ID",
			projectID: "project-123",
			appName:   "project-123-myapp-v2",
			expected:  "project-123-myapp-v2",
		},
		{
			name:      "edge case - empty app name",
			projectID: "project-123",
			appName:   "",
			expected:  "project-123-",
		},
		{
			name:      "edge case - app name with only dash",
			projectID: "project-123",
			appName:   "-test",
			expected:  "project-123--test",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			result := NormalizeAppID(tc.projectID, tc.appName)
			assert.Equal(t, tc.expected, result)
		})
	}
}
