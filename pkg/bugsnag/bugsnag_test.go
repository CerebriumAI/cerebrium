package bugsnag

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsDoNotTrackEnabled(t *testing.T) {
	tcs := []struct {
		name     string
		value    string
		set      bool
		expected bool
	}{
		{name: "unset - tracking allowed", set: false, expected: false},
		{name: "1 - opted out", value: "1", set: true, expected: true},
		{name: "false - tracking allowed", value: "false", set: true, expected: false},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set {
				t.Setenv("DO_NOT_TRACK", tc.value)
			}

			assert.Equal(t, tc.expected, IsDoNotTrackEnabled())
		})
	}
}

func TestInitializeHonorsDoNotTrack(t *testing.T) {
	t.Setenv("DO_NOT_TRACK", "1")
	t.Setenv("CEREBRIUM_CONFIG_PATH", filepath.Join(t.TempDir(), "config.yaml"))

	originalInitialized := initialized
	originalEnabled := enabled
	originalAPIKey := BugsnagAPIKey
	t.Cleanup(func() {
		initialized = originalInitialized
		enabled = originalEnabled
		BugsnagAPIKey = originalAPIKey
	})

	initialized = false
	enabled = false
	BugsnagAPIKey = "0123456789abcdef0123456789abcdef"

	require.NoError(t, Initialize())

	assert.True(t, initialized)
	assert.False(t, IsEnabled(), "Bugsnag must stay disabled when DO_NOT_TRACK is set")
}
