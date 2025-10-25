package ui

import (
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestDisplayConfig_SimpleOutput(t *testing.T) {
	tests := []struct {
		name           string
		opts           DisplayConfig
		expectedSimple bool
		expectedReason string
	}{
		{
			name: "interactive TTY with animations enabled",
			opts: DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: false,
			},
			expectedSimple: false,
			expectedReason: "Should use full UI when interactive and animations enabled",
		},
		{
			name: "interactive TTY with animations disabled (--no-color)",
			opts: DisplayConfig{
				IsInteractive:    true,
				DisableAnimation: true,
			},
			expectedSimple: true,
			expectedReason: "Should use simple output when animations disabled even if interactive",
		},
		{
			name: "non-interactive (piped/redirected) with animations enabled",
			opts: DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: false,
			},
			expectedSimple: true,
			expectedReason: "Should use simple output when not interactive",
		},
		{
			name: "non-interactive with animations disabled",
			opts: DisplayConfig{
				IsInteractive:    false,
				DisableAnimation: true,
			},
			expectedSimple: true,
			expectedReason: "Should use simple output when both not interactive and animations disabled",
		},
		{
			name: "stderr redirected to stdout (2>&1)",
			opts: DisplayConfig{
				IsInteractive:    false, // This would be false due to stderr redirect check
				DisableAnimation: false,
			},
			expectedSimple: true,
			expectedReason: "Should use simple output when stderr is redirected to stdout to prevent corruption",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.opts.SimpleOutput()
			assert.Equal(t, tt.expectedSimple, result, tt.expectedReason)
		})
	}
}

// TestDisplayConfig_Scenarios tests the logic that determines IsInteractive and DisableAnimation
// This test documents the expected behavior for different scenarios
func TestDisplayConfig_Scenarios(t *testing.T) {
	// Note: This is a documentation test showing the expected logic.
	// Actually testing NewDisplayConfig would require mocking TTY detection.

	tests := []struct {
		name                     string
		stdoutIsTTY              bool
		noColorFlag              bool
		noAnsiFlag               bool
		verboseFlag              bool
		stderrSameAsStdout       bool
		expectedIsInteractive    bool
		expectedDisableAnimation bool
		expectedSimpleOutput     bool
	}{
		{
			name:                     "normal TTY usage",
			stdoutIsTTY:              true,
			noColorFlag:              false,
			noAnsiFlag:               false,
			verboseFlag:              false,
			stderrSameAsStdout:       true, // Normal terminal, same device
			expectedIsInteractive:    true,
			expectedDisableAnimation: false,
			expectedSimpleOutput:     false, // Full UI with colors
		},
		{
			name:                     "TTY with --no-color flag",
			stdoutIsTTY:              true,
			noColorFlag:              true,
			noAnsiFlag:               false,
			verboseFlag:              false,
			stderrSameAsStdout:       true,
			expectedIsInteractive:    false,
			expectedDisableAnimation: true,
			expectedSimpleOutput:     true, // Simple output, no colors
		},
		{
			name:                     "TTY with --no-ansi flag",
			stdoutIsTTY:              true,
			noColorFlag:              false,
			noAnsiFlag:               true,
			verboseFlag:              false,
			stderrSameAsStdout:       true,
			expectedIsInteractive:    false,
			expectedDisableAnimation: true,
			expectedSimpleOutput:     true, // Simple output, no animations
		},
		{
			name:                     "TTY with --verbose (stderr same as stdout)",
			stdoutIsTTY:              true,
			noColorFlag:              false,
			noAnsiFlag:               false,
			verboseFlag:              true,
			stderrSameAsStdout:       true,  // Normal terminal
			expectedIsInteractive:    false, // Verbose forces simple when stderr == stdout
			expectedDisableAnimation: false,
			expectedSimpleOutput:     true, // Simple output so logs appear inline
		},
		{
			name:                     "TTY with --verbose (stderr to different file: 2>stderr.log)",
			stdoutIsTTY:              true,
			noColorFlag:              false,
			noAnsiFlag:               false,
			verboseFlag:              true,
			stderrSameAsStdout:       false, // Redirected to different file
			expectedIsInteractive:    true,  // Can use fancy mode, logs won't interfere
			expectedDisableAnimation: false,
			expectedSimpleOutput:     false, // Fancy mode!
		},
		{
			name:                     "piped output (not TTY)",
			stdoutIsTTY:              false,
			noColorFlag:              false,
			noAnsiFlag:               false,
			verboseFlag:              false,
			stderrSameAsStdout:       false,
			expectedIsInteractive:    false,
			expectedDisableAnimation: false,
			expectedSimpleOutput:     true, // Simple output for pipes
		},
		{
			name:                     "piped output with --verbose and 2>&1",
			stdoutIsTTY:              false,
			noColorFlag:              false,
			noAnsiFlag:               false,
			verboseFlag:              true,
			stderrSameAsStdout:       true, // Redirected together
			expectedIsInteractive:    false,
			expectedDisableAnimation: false,
			expectedSimpleOutput:     true, // Simple output
		},
		{
			name:                     "non-TTY with --no-color",
			stdoutIsTTY:              false,
			noColorFlag:              true,
			noAnsiFlag:               false,
			verboseFlag:              false,
			stderrSameAsStdout:       false,
			expectedIsInteractive:    false,
			expectedDisableAnimation: true,
			expectedSimpleOutput:     true, // Simple output
		},
		{
			name:                     "TTY with all flags (--verbose --no-color, stderr same)",
			stdoutIsTTY:              true,
			noColorFlag:              true,
			noAnsiFlag:               false,
			verboseFlag:              true,
			stderrSameAsStdout:       true,
			expectedIsInteractive:    false,
			expectedDisableAnimation: true,
			expectedSimpleOutput:     true, // Simple output
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This simulates the logic in NewDisplayConfig
			disableAnimation := tt.noColorFlag || tt.noAnsiFlag
			verboseForcesSimpleOutput := tt.verboseFlag && tt.stderrSameAsStdout
			isInteractive := tt.stdoutIsTTY && !disableAnimation && !verboseForcesSimpleOutput

			opts := DisplayConfig{
				IsInteractive:    isInteractive,
				DisableAnimation: disableAnimation,
			}

			assert.Equal(t, tt.expectedIsInteractive, opts.IsInteractive,
				"IsInteractive should be %v", tt.expectedIsInteractive)
			assert.Equal(t, tt.expectedDisableAnimation, opts.DisableAnimation,
				"DisableAnimation should be %v", tt.expectedDisableAnimation)
			assert.Equal(t, tt.expectedSimpleOutput, opts.SimpleOutput(),
				"SimpleOutput() should return %v", tt.expectedSimpleOutput)

			// Document the logic
			t.Logf("Scenario: StdoutIsTTY=%v, NoColor=%v, NoAnsi=%v, Verbose=%v, StderrSame=%v",
				tt.stdoutIsTTY, tt.noColorFlag, tt.noAnsiFlag, tt.verboseFlag, tt.stderrSameAsStdout)
			t.Logf("Result: IsInteractive=%v, DisableAnimation=%v, SimpleOutput=%v",
				opts.IsInteractive, opts.DisableAnimation, opts.SimpleOutput())
		})
	}
}

// TestSimpleOutputLogic documents the exact boolean logic
func TestSimpleOutputLogic(t *testing.T) {
	// SimpleOutput = !IsInteractive || DisableAnimation
	// This means SimpleOutput is TRUE when:
	// - IsInteractive is FALSE (non-TTY, or forced non-interactive)
	// - OR DisableAnimation is TRUE (--no-color or --no-ansi)

	// Truth table
	truthTable := []struct {
		isInteractive    bool
		disableAnimation bool
		expectedSimple   bool
	}{
		{true, false, false}, // Interactive + animations = Full UI
		{true, true, true},   // Interactive + no animations = Simple
		{false, false, true}, // Non-interactive + animations = Simple
		{false, true, true},  // Non-interactive + no animations = Simple
	}

	for _, row := range truthTable {
		opts := DisplayConfig{
			IsInteractive:    row.isInteractive,
			DisableAnimation: row.disableAnimation,
		}
		result := opts.SimpleOutput()
		assert.Equal(t, row.expectedSimple, result,
			"For IsInteractive=%v, DisableAnimation=%v, expected SimpleOutput=%v",
			row.isInteractive, row.disableAnimation, row.expectedSimple)
	}
}

// TestDisplayConfig_FlagHandling tests that cobra flags are properly parsed
func TestDisplayConfig_FlagHandling(t *testing.T) {
	tests := []struct {
		name                     string
		args                     []string
		expectedDisableAnimation bool
		description              string
	}{
		{
			name:                     "no flags set",
			args:                     []string{"child"},
			expectedDisableAnimation: false,
			description:              "Animation should be enabled when no flags are set",
		},
		{
			name:                     "only --no-color flag",
			args:                     []string{"child", "--no-color"},
			expectedDisableAnimation: true,
			description:              "Animation should be disabled when --no-color is set",
		},
		{
			name:                     "only --no-ansi flag",
			args:                     []string{"child", "--no-ansi"},
			expectedDisableAnimation: true,
			description:              "Animation should be disabled when --no-ansi is set",
		},
		{
			name:                     "both --no-color and --no-ansi flags",
			args:                     []string{"child", "--no-color", "--no-ansi"},
			expectedDisableAnimation: true,
			description:              "Animation should be disabled when either flag is set",
		},
		{
			name:                     "global --no-color flag",
			args:                     []string{"--no-color", "child"},
			expectedDisableAnimation: true,
			description:              "Animation should be disabled with global --no-color",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a root command with the flags
			rootCmd := &cobra.Command{
				Use: "test",
			}
			rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output")
			rootCmd.PersistentFlags().Bool("no-ansi", false, "Disable ANSI output")
			rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Verbose logging")

			// Create a child command that will inherit the flags
			childCmd := &cobra.Command{
				Use: "child",
				Run: func(cmd *cobra.Command, args []string) {},
			}
			rootCmd.AddCommand(childCmd)

			// Set the args and find the command that would be executed
			rootCmd.SetArgs(tt.args)
			cmd, args, err := rootCmd.Find(tt.args)
			assert.NoError(t, err)

			// Parse flags on the found command
			err = cmd.ParseFlags(args)
			assert.NoError(t, err)

			// Call NewDisplayConfig
			verbose, err := cmd.Flags().GetBool("verbose")
			require.NoError(t, err)
			opts, err := NewDisplayConfig(cmd, verbose)
			assert.NoError(t, err)

			// Check DisableAnimation flag
			assert.Equal(t, tt.expectedDisableAnimation, opts.DisableAnimation, tt.description)

			t.Logf("Args: %v => DisableAnimation=%v", tt.args, opts.DisableAnimation)
			t.Logf("IsInteractive=%v (depends on actual TTY status)", opts.IsInteractive)
			t.Logf("SimpleOutput=%v", opts.SimpleOutput())
		})
	}
}

// TestDisplayConfig_FlagPrecedence tests various flag combinations
func TestDisplayConfig_FlagPrecedence(t *testing.T) {
	createCommand := func(noColor, noAnsi bool) *cobra.Command {
		cmd := &cobra.Command{
			Use: "test",
		}
		cmd.Flags().Bool("no-color", noColor, "")
		cmd.Flags().Bool("no-ansi", noAnsi, "")
		cmd.Flags().BoolP("verbose", "v", false, "")
		return cmd
	}

	// Test that either flag triggers DisableAnimation
	testCases := []struct {
		name        string
		noColor     bool
		noAnsi      bool
		expectation string
	}{
		{
			name:        "neither flag",
			noColor:     false,
			noAnsi:      false,
			expectation: "animations enabled",
		},
		{
			name:        "no-color only",
			noColor:     true,
			noAnsi:      false,
			expectation: "animations disabled due to no-color",
		},
		{
			name:        "no-ansi only",
			noColor:     false,
			noAnsi:      true,
			expectation: "animations disabled due to no-ansi",
		},
		{
			name:        "both flags",
			noColor:     true,
			noAnsi:      true,
			expectation: "animations disabled due to both flags",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := createCommand(tc.noColor, tc.noAnsi)

			verbose, err := cmd.Flags().GetBool("verbose")
			require.NoError(t, err)
			opts, err := NewDisplayConfig(cmd, verbose)
			assert.NoError(t, err)

			expectedDisabled := tc.noColor || tc.noAnsi
			assert.Equal(t, expectedDisabled, opts.DisableAnimation, tc.expectation)

			// In a non-TTY environment or with animations disabled, SimpleOutput should be true
			if !opts.IsInteractive || opts.DisableAnimation {
				assert.True(t, opts.SimpleOutput(),
					"SimpleOutput should be true when not interactive or animations disabled")
			}
		})
	}
}
