package timeutil

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSinceTime(t *testing.T) {
	// Set a known local timezone for consistent testing
	// Note: In real usage, time.Local is determined by the OS
	localTZ := time.FixedZone("TestLocal", -8*3600) // UTC-8 (PST)

	tcs := []struct {
		name           string
		input          string
		expectedError  bool
		validateOutput func(t *testing.T, output string)
	}{
		{
			name:  "RFC3339 with Z",
			input: "2024-01-15T10:30:00Z",
			validateOutput: func(t *testing.T, output string) {
				assert.Equal(t, "2024-01-15T10:30:00.000Z", output)
			},
		},
		{
			name:  "RFC3339 with timezone offset",
			input: "2024-01-15T10:30:00-05:00",
			validateOutput: func(t *testing.T, output string) {
				// Should convert to UTC: 10:30 EST = 15:30 UTC
				assert.Equal(t, "2024-01-15T15:30:00.000Z", output)
			},
		},
		{
			name:  "RFC3339Nano with nanoseconds",
			input: "2024-01-15T10:30:00.123456789Z",
			validateOutput: func(t *testing.T, output string) {
				// Should truncate to milliseconds
				assert.Equal(t, "2024-01-15T10:30:00.123Z", output)
			},
		},
		{
			name:  "DateTime format (assumes local timezone)",
			input: "2024-01-15 10:30:00",
			validateOutput: func(t *testing.T, output string) {
				// Parse the input in local timezone, convert to UTC
				localTime, _ := time.ParseInLocation(time.DateTime, "2024-01-15 10:30:00", localTZ)
				expected := localTime.UTC().Format("2006-01-02T15:04:05.000Z")
				assert.Equal(t, expected, output)
			},
		},
		{
			name:  "DateTime with milliseconds (assumes local timezone)",
			input: "2024-01-15 10:30:00.456",
			validateOutput: func(t *testing.T, output string) {
				localTime, _ := time.ParseInLocation("2006-01-02 15:04:05.999", "2024-01-15 10:30:00.456", localTZ)
				expected := localTime.UTC().Format("2006-01-02T15:04:05.000Z")
				assert.Equal(t, expected, output)
			},
		},
		{
			name:  "DateOnly (assumes local timezone, midnight)",
			input: "2024-01-15",
			validateOutput: func(t *testing.T, output string) {
				localTime, _ := time.ParseInLocation(time.DateOnly, "2024-01-15", localTZ)
				expected := localTime.UTC().Format("2006-01-02T15:04:05.000Z")
				assert.Equal(t, expected, output)
			},
		},
		{
			name:  "relative time - 1 hour",
			input: "1h",
			validateOutput: func(t *testing.T, output string) {
				// Should be ~1 hour ago from now
				parsed, err := time.Parse(time.RFC3339, output)
				require.NoError(t, err)
				diff := time.Since(parsed)
				// Allow some tolerance for test execution time
				assert.InDelta(t, time.Hour.Seconds(), diff.Seconds(), 5.0)
			},
		},
		{
			name:  "relative time - 30 minutes",
			input: "30m",
			validateOutput: func(t *testing.T, output string) {
				parsed, err := time.Parse(time.RFC3339, output)
				require.NoError(t, err)
				diff := time.Since(parsed)
				assert.InDelta(t, (30 * time.Minute).Seconds(), diff.Seconds(), 5.0)
			},
		},
		{
			name:  "relative time - 2 days",
			input: "2d",
			validateOutput: func(t *testing.T, output string) {
				parsed, err := time.Parse(time.RFC3339, output)
				require.NoError(t, err)
				diff := time.Since(parsed)
				assert.InDelta(t, (48 * time.Hour).Seconds(), diff.Seconds(), 5.0)
			},
		},
		{
			name:  "relative time - 45 seconds",
			input: "45s",
			validateOutput: func(t *testing.T, output string) {
				parsed, err := time.Parse(time.RFC3339, output)
				require.NoError(t, err)
				diff := time.Since(parsed)
				assert.InDelta(t, (45 * time.Second).Seconds(), diff.Seconds(), 5.0)
			},
		},
		{
			name:          "invalid format",
			input:         "not-a-date",
			expectedError: true,
		},
		{
			name:          "invalid relative time unit",
			input:         "5x",
			expectedError: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// For tests that use local timezone, temporarily override time.Local
			if tc.input == "2024-01-15 10:30:00" || tc.input == "2024-01-15 10:30:00.456" || tc.input == "2024-01-15" {
				originalLocal := time.Local
				time.Local = localTZ
				defer func() { time.Local = originalLocal }()
			}

			result, err := ParseSinceTime(tc.input)

			if tc.expectedError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				if tc.validateOutput != nil {
					tc.validateOutput(t, result)
				}
			}
		})
	}
}

func TestParseSinceTime_OutputFormat(t *testing.T) {
	// Verify output is always in the correct format
	result, err := ParseSinceTime("2024-01-15T10:30:00Z")
	require.NoError(t, err)

	// Should be able to parse the output as RFC3339
	_, err = time.Parse(time.RFC3339, result)
	assert.NoError(t, err, "Output should be valid RFC3339")

	// Should end with Z (UTC)
	assert.Contains(t, result, "Z", "Output should be in UTC (contain Z)")

	// Should have milliseconds
	assert.Regexp(t, `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z`, result, "Output should include milliseconds")
}
