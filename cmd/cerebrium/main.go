package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/commands"
	cerebrium_bugsnag "github.com/cerebriumai/cerebrium/pkg/bugsnag"
)

func main() {
	// Initialize Bugsnag error tracking
	if err := cerebrium_bugsnag.Initialize(); err != nil {
		// Don't fail if Bugsnag initialization fails, just log it
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize error tracking: %v\n", err)
	}

	// Recover from panics and report them to Bugsnag
	defer cerebrium_bugsnag.NotifyOnPanic(context.Background())

	rootCmd := commands.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		// Commands handle their own error presentation logic
		// If we get an error here, check if it's an unknown command error
		// and show usage if so
		errMsg := err.Error()
		if strings.HasPrefix(errMsg, "unknown command") {
			// Unknown command - we've suppressed usage for commands, so we need to manually do this
			_ = rootCmd.Usage()
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, err)
		} else if strings.HasPrefix(errMsg, "unknown flag") {
			// Unknown flag - Cobra already showed usage, don't duplicate
			fmt.Fprintln(os.Stderr, err)
		} else {
			// Other errors - just print the error
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
