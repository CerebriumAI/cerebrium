package commands

import (
	"bytes"
	"os"
	"testing"

	"github.com/cerebriumai/cerebrium/internal/ui"
	"github.com/cerebriumai/cerebrium/pkg/config"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRootCommand_FlagConfiguration tests that flags are properly configured on root command
func TestRootCommand_FlagConfiguration(t *testing.T) {
	rootCmd := NewRootCmd()

	// Check that persistent flags are defined
	noColorFlag := rootCmd.PersistentFlags().Lookup("no-color")
	assert.NotNil(t, noColorFlag, "no-color flag should be defined")
	assert.Equal(t, "false", noColorFlag.DefValue, "no-color should default to false")

	noAnsiFlag := rootCmd.PersistentFlags().Lookup("no-ansi")
	assert.NotNil(t, noAnsiFlag, "no-ansi flag should be defined")
	assert.Equal(t, "false", noAnsiFlag.DefValue, "no-ansi should default to false")

	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	assert.NotNil(t, verboseFlag, "verbose flag should be defined")
	assert.Equal(t, "false", verboseFlag.DefValue, "verbose should default to false")
}

// TestRootCommand_FlagInheritance tests that child commands inherit persistent flags
func TestRootCommand_FlagInheritance(t *testing.T) {
	rootCmd := NewRootCmd()

	// Get a child command (deploy)
	deployCmd, _, err := rootCmd.Find([]string{"deploy"})
	require.NoError(t, err, "deploy command should exist")

	// Parse flags to ensure inheritance is set up
	rootCmd.SetArgs([]string{"deploy", "--help"})
	rootCmd.Execute()

	// Check that flags are inherited via InheritedFlags
	inheritedFlags := deployCmd.InheritedFlags()

	noColorFlag := inheritedFlags.Lookup("no-color")
	assert.NotNil(t, noColorFlag, "deploy command should inherit no-color flag")

	noAnsiFlag := inheritedFlags.Lookup("no-ansi")
	assert.NotNil(t, noAnsiFlag, "deploy command should inherit no-ansi flag")

	verboseFlag := inheritedFlags.Lookup("verbose")
	assert.NotNil(t, verboseFlag, "deploy command should inherit verbose flag")
}

// TestRootCommand_DisplayConfig tests that flags correctly affect DisplayConfig
func TestRootCommand_ConfigOptions(t *testing.T) {
	tests := []struct {
		name                     string
		args                     []string
		expectedDisableAnimation bool
		description              string
	}{
		{
			name:                     "no flags",
			args:                     nil,
			expectedDisableAnimation: false,
			description:              "Animation should be enabled by default",
		},
		{
			name:                     "with --no-color",
			args:                     []string{"--no-color"},
			expectedDisableAnimation: true,
			description:              "Animation should be disabled with --no-color",
		},
		{
			name:                     "with --no-ansi",
			args:                     []string{"--no-ansi"},
			expectedDisableAnimation: true,
			description:              "Animation should be disabled with --no-ansi",
		},
		{
			name:                     "with both flags",
			args:                     []string{"--no-color", "--no-ansi"},
			expectedDisableAnimation: true,
			description:              "Animation should be disabled with both flags",
		},
		{
			name:                     "with verbose only",
			args:                     []string{"--verbose"},
			expectedDisableAnimation: false,
			description:              "Verbose flag should not affect animation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh root command for each test
			rootCmd := NewRootCmd()

			// Parse the flags
			err := rootCmd.ParseFlags(tt.args)
			require.NoError(t, err, "Flag parsing should succeed")

			// Get display options (this simulates what happens in PersistentPreRun)
			verbose, _ := rootCmd.Flags().GetBool("verbose")
			displayOpts, err := ui.NewDisplayConfig(rootCmd, verbose)
			require.NoError(t, err, "NewDisplayConfig should succeed")

			// Check DisableAnimation
			assert.Equal(t, tt.expectedDisableAnimation, displayOpts.DisableAnimation, tt.description)

			// Log the results for debugging
			t.Logf("Args: %v", tt.args)
			t.Logf("DisableAnimation: %v", displayOpts.DisableAnimation)
			t.Logf("IsInteractive: %v (depends on TTY)", displayOpts.IsInteractive)
			t.Logf("SimpleOutput: %v", displayOpts.SimpleOutput())
		})
	}
}

// TestRootCommand_ChildCommandDisplayConfig tests that child commands get correct DisplayConfig
func TestRootCommand_ChildCommandDisplayConfig(t *testing.T) {
	tests := []struct {
		name                     string
		command                  string
		args                     []string
		expectedDisableAnimation bool
	}{
		{
			name:                     "deploy without flags",
			command:                  "deploy",
			args:                     []string{"deploy"},
			expectedDisableAnimation: false,
		},
		{
			name:                     "deploy with --no-color",
			command:                  "deploy",
			args:                     []string{"deploy", "--no-color"},
			expectedDisableAnimation: true,
		},
		{
			name:                     "deploy with global --no-color",
			command:                  "deploy",
			args:                     []string{"--no-color", "deploy"},
			expectedDisableAnimation: true,
		},
		{
			name:                     "logs with --no-ansi",
			command:                  "logs",
			args:                     []string{"logs", "test-app", "--no-ansi"},
			expectedDisableAnimation: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh root command
			rootCmd := NewRootCmd()

			// Set the args
			rootCmd.SetArgs(tt.args)

			// Find and execute flags parsing
			// This is what actually happens when a command is run
			cmd, args, err := rootCmd.Find(tt.args)
			require.NoError(t, err, "Should find command: %s", tt.command)

			// Parse the remaining args as flags for the found command
			// This is crucial - we need to parse the flags on the actual command
			err = cmd.ParseFlags(args)
			require.NoError(t, err, "Should parse flags")

			// Get display options for the command
			verbose, _ := cmd.Flags().GetBool("verbose")
			displayOpts, err := ui.NewDisplayConfig(cmd, verbose)
			require.NoError(t, err, "NewDisplayConfig should succeed")

			// Verify
			assert.Equal(t, tt.expectedDisableAnimation, displayOpts.DisableAnimation,
				"DisableAnimation should be %v for command: %s with args: %v",
				tt.expectedDisableAnimation, tt.command, tt.args)

			// Log for debugging
			t.Logf("Command: %s, Args: %v", tt.command, tt.args)
			t.Logf("DisableAnimation: %v, IsInteractive: %v, SimpleOutput: %v",
				displayOpts.DisableAnimation, displayOpts.IsInteractive, displayOpts.SimpleOutput())
		})
	}
}

// TestRootCommand_PersistentPreRun tests that config is loaded and stored in context
func TestRootCommand_PersistentPreRun(t *testing.T) {
	// Skip this test in CI or if config doesn't exist
	// This test requires a valid config file
	if os.Getenv("CI") != "" {
		t.Skip("Skipping config test in CI environment")
	}

	// Try to load config to see if it exists
	if _, err := config.Load(); err != nil {
		t.Skip("Skipping test - no valid config file found")
	}

	rootCmd := NewRootCmd()

	// Capture output
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)

	// Create a test command that checks for config in context
	var configFound bool
	testCmd := &cobra.Command{
		Use: "test-config",
		Run: func(cmd *cobra.Command, args []string) {
			cfg, err := config.GetConfigFromContext(cmd)
			configFound = (cfg != nil && err == nil)
		},
	}
	rootCmd.AddCommand(testCmd)

	// Run the test command
	rootCmd.SetArgs([]string{"test-config"})
	err := rootCmd.Execute()

	// Check results
	assert.NoError(t, err, "Command should execute successfully")
	assert.True(t, configFound, "Config should be available in context")
}

// TestRootCommand_SimpleOutputLogic documents the exact logic for SimpleOutput
func TestRootCommand_SimpleOutputLogic(t *testing.T) {
	// This test documents the relationship between flags and SimpleOutput
	// SimpleOutput = !IsInteractive || DisableAnimation

	testCases := []struct {
		name             string
		flags            []string
		isTTY            bool // Would need to mock this in real test
		expectedBehavior string
	}{
		{
			name:             "TTY without flags",
			flags:            nil,
			isTTY:            true,
			expectedBehavior: "Full UI with colors and animations",
		},
		{
			name:             "TTY with --no-color",
			flags:            []string{"--no-color"},
			isTTY:            true,
			expectedBehavior: "Simple output (no colors/animations)",
		},
		{
			name:             "TTY with --no-ansi",
			flags:            []string{"--no-ansi"},
			isTTY:            true,
			expectedBehavior: "Simple output (no ANSI sequences)",
		},
		{
			name:             "Non-TTY (piped)",
			flags:            nil,
			isTTY:            false,
			expectedBehavior: "Simple output (not interactive)",
		},
		{
			name:             "Non-TTY with --no-color",
			flags:            []string{"--no-color"},
			isTTY:            false,
			expectedBehavior: "Simple output (both conditions)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rootCmd := NewRootCmd()

			// Parse flags
			err := rootCmd.ParseFlags(tc.flags)
			require.NoError(t, err)

			// Get display options
			verbose, _ := rootCmd.Flags().GetBool("verbose")
			opts, err := ui.NewDisplayConfig(rootCmd, verbose)
			require.NoError(t, err)

			// Document the expected behavior
			t.Logf("Scenario: %s", tc.name)
			t.Logf("Flags: %v", tc.flags)
			t.Logf("Expected TTY: %v", tc.isTTY)
			t.Logf("Expected behavior: %s", tc.expectedBehavior)
			t.Logf("Actual DisableAnimation: %v", opts.DisableAnimation)
			t.Logf("Actual IsInteractive: %v", opts.IsInteractive)
			t.Logf("Actual SimpleOutput: %v", opts.SimpleOutput())
		})
	}
}

// TestRootCommand_Execute tests that the root command can be executed
func TestRootCommand_Execute(t *testing.T) {
	// This test verifies the command executes without errors when given --help
	rootCmd := NewRootCmd()

	// Capture output
	var stdout bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetArgs([]string{"--help"})

	err := rootCmd.Execute()
	assert.NoError(t, err, "Root command should execute successfully with --help")
	assert.Contains(t, stdout.String(), "Command line interface for the Cerebrium platform",
		"Help output should contain description")
}
