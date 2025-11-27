package ui

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// DisplayConfigContextKey is the key used to store DisplayConfig in context
type DisplayConfigContextKey struct{}

// GetDisplayConfigContextKey returns the key used to store DisplayConfig in context
func GetDisplayConfigContextKey() DisplayConfigContextKey {
	return DisplayConfigContextKey{}
}

// DisplayConfig contains display-related configuration
type DisplayConfig struct {
	DisableAnimation bool
	IsInteractive    bool
}

func (d DisplayConfig) SimpleOutput() bool {
	return !d.IsInteractive || d.DisableAnimation
}

// NewDisplayConfig extracts display options from persistent flags and TTY detection
func NewDisplayConfig(cmd *cobra.Command, verbose bool) (DisplayConfig, error) {
	// Get persistent flags (these are inherited from root command)
	noColor, _ := cmd.Flags().GetBool("no-color")
	noAnsi, _ := cmd.Flags().GetBool("no-ansi")
	disableAnimationFlag, _ := cmd.Flags().GetBool("disable-animation")

	// Detect if stdout is a TTY
	// We only check stdout because that's where the TUI output goes
	stdoutIsTTY := isatty.IsTerminal(os.Stdout.Fd())

	// Check if stdout and stderr point to the same file
	// This matters for verbose mode: if they're separate, verbose logs won't interfere with TUI
	stderrRedirectedToStdout := false
	if stat1, err1 := os.Stdout.Stat(); err1 == nil {
		if stat2, err2 := os.Stderr.Stat(); err2 == nil {
			stderrRedirectedToStdout = os.SameFile(stat1, stat2)
		}
	}

	// Determine if animations should be disabled
	disableAnimation := noColor || noAnsi || disableAnimationFlag

	// Verbose mode only forces simple output if stderr and stdout are the same
	// If they're redirected to different places (e.g., --verbose 2>stderr.log),
	// we can still use fancy mode because logs won't interfere with TUI
	verboseForcesSimpleOutput := verbose && stderrRedirectedToStdout

	// Interactive mode requires:
	// 1. Stdout is a TTY (not piped/redirected)
	// 2. Animations are not disabled (--no-color, --no-ansi)
	// 3. Verbose mode is not forcing simple output (only when stderr == stdout)
	isInteractive := stdoutIsTTY && !disableAnimation && !verboseForcesSimpleOutput

	opts := DisplayConfig{
		DisableAnimation: disableAnimation,
		IsInteractive:    isInteractive,
	}

	// Debug logging to help diagnose display options
	slog.Debug("Display options determined",
		"command", cmd.Name(),
		"no-color-flag", noColor,
		"no-ansi-flag", noAnsi,
		"disable-animation-flag", disableAnimationFlag,
		"verbose-flag", verbose,
		"stdout-is-tty", stdoutIsTTY,
		"stderr-is-tty", isatty.IsTerminal(os.Stderr.Fd()),
		"stderr-same-as-stdout", stderrRedirectedToStdout,
		"verbose-forces-simple", verboseForcesSimpleOutput,
		"disable-animation", disableAnimation,
		"is-interactive", isInteractive,
		"simple-output", opts.SimpleOutput(),
	)

	return opts, nil
}

// GetDisplayConfigFromContext retrieves DisplayConfig from the command context
func GetDisplayConfigFromContext(cmd *cobra.Command) (DisplayConfig, error) {
	ctx := cmd.Context()
	if ctx == nil {
		return DisplayConfig{}, fmt.Errorf("command context is nil")
	}

	opts, ok := ctx.Value(GetDisplayConfigContextKey()).(DisplayConfig)
	if !ok {
		return DisplayConfig{}, fmt.Errorf("display options not found in context")
	}

	return opts, nil
}
