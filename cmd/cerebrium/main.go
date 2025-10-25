package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/cerebriumai/cerebrium/internal/commands"
)

func main() {
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
