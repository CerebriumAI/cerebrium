package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_determineAppID(t *testing.T) {
	tcs := []struct {
		name             string
		appNameInput     string
		currentProjectID string
		expectedAppID    string
	}{
		{
			name:             "short app name - prepends project",
			appNameInput:     "my-app",
			currentProjectID: "p-abc12345",
			expectedAppID:    "p-abc12345-my-app",
		},
		{
			name:             "short app name - prepends dev project",
			appNameInput:     "my-app",
			currentProjectID: "dev-p-xyz78901",
			expectedAppID:    "dev-p-xyz78901-my-app",
		},
		{
			name:             "fully qualified prod app ID - uses as-is",
			appNameInput:     "p-abc12345-my-app",
			currentProjectID: "p-different",
			expectedAppID:    "p-abc12345-my-app",
		},
		{
			name:             "fully qualified dev app ID - uses as-is",
			appNameInput:     "dev-p-xyz78901-my-app",
			currentProjectID: "dev-p-different",
			expectedAppID:    "dev-p-xyz78901-my-app",
		},
		{
			name:             "fully qualified local app ID - uses as-is",
			appNameInput:     "local-p-test1234-my-app",
			currentProjectID: "dev-p-current99",
			expectedAppID:    "local-p-test1234-my-app",
		},
		{
			name:             "app name with dashes - prepends project",
			appNameInput:     "my-complex-app-name",
			currentProjectID: "p-abc12345",
			expectedAppID:    "p-abc12345-my-complex-app-name",
		},
		{
			name:             "fully qualified with multiple dashes - uses as-is",
			appNameInput:     "p-abc12345-my-complex-app-name",
			currentProjectID: "p-different",
			expectedAppID:    "p-abc12345-my-complex-app-name",
		},
		{
			name:             "edge case - starts with 'p-' but not a project ID pattern",
			appNameInput:     "p-my-app",
			currentProjectID: "p-abc12345",
			expectedAppID:    "p-abc12345-p-my-app", // Not a valid project format (too short), so prepend
		},
		{
			name:             "real world example - dev project with 8 char ID",
			appNameInput:     "3-cpu-only",
			currentProjectID: "dev-p-0780791d",
			expectedAppID:    "dev-p-0780791d-3-cpu-only",
		},
		{
			name:             "real world example - fully qualified dev app",
			appNameInput:     "dev-p-0780791d-3-cpu-only",
			currentProjectID: "dev-p-0780791d",
			expectedAppID:    "dev-p-0780791d-3-cpu-only",
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			result := determineAppID(tc.appNameInput, tc.currentProjectID)
			assert.Equal(t, tc.expectedAppID, result)
		})
	}
}
