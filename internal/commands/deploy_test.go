package commands

import (
	"errors"
	"testing"

	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_validateConfirmationPrompt(t *testing.T) {
	tcs := []struct {
		name                string
		stdinIsTTY          bool
		disableConfirmation bool
		expectError         bool
	}{
		{
			name:                "TTY with confirmation required - prompt allowed",
			stdinIsTTY:          true,
			disableConfirmation: false,
			expectError:         false,
		},
		{
			name:                "TTY with confirmation disabled - no prompt needed",
			stdinIsTTY:          true,
			disableConfirmation: true,
			expectError:         false,
		},
		{
			name:                "non-TTY with confirmation disabled - no prompt needed",
			stdinIsTTY:          false,
			disableConfirmation: true,
			expectError:         false,
		},
		{
			name:                "non-TTY with confirmation required - fail fast",
			stdinIsTTY:          false,
			disableConfirmation: false,
			expectError:         true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfirmationPrompt(tc.stdinIsTTY, tc.disableConfirmation)

			if !tc.expectError {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)

			var uiErr *ui.UIError
			require.True(t, errors.As(err, &uiErr), "guard should return a structured UIError")
			assert.Equal(t, ui.ErrorTypeValidation, uiErr.Type)
			assert.Contains(t, err.Error(), "stdin is not a TTY")
			assert.Contains(t, err.Error(), "-y/--yes", "error should name the flag that skips confirmation")
		})
	}
}

func Test_confirmationDisabled(t *testing.T) {
	tcs := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "no flags",
			args:     []string{},
			expected: false,
		},
		{
			name:     "--disable-confirmation",
			args:     []string{"--disable-confirmation"},
			expected: true,
		},
		{
			name:     "-y shorthand",
			args:     []string{"-y"},
			expected: true,
		},
		{
			name:     "--yes alias",
			args:     []string{"--yes"},
			expected: true,
		},
		{
			name:     "both flags",
			args:     []string{"--disable-confirmation", "--yes"},
			expected: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			cmd := NewDeployCmd()
			require.NoError(t, cmd.ParseFlags(tc.args))

			assert.Equal(t, tc.expected, confirmationDisabled(cmd.Flags()))
		})
	}
}
