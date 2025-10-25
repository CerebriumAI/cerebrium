package ui

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// SetupSignalHandling sets up signal handling for graceful cancellation
// NOTE: this should be called before p.Run(), since it alters the program config
func SetupSignalHandling(p *tea.Program, shutdownTimeout time.Duration) chan<- struct{} {
	if shutdownTimeout == 0 {
		// Just give bubbletea some small amount of time to finish up
		// In the case that bubbletea finishes before this timer elapses, the program will exit
		// so it's okay to set it to something longer
		shutdownTimeout = 100 * time.Millisecond
	}
	// Remove the bubbletea signal handler when we initialise our own
	tea.WithoutSignalHandler()(p)

	sigChan := make(chan os.Signal, 1)
	doneCh := make(chan struct{})
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		defer signal.Stop(sigChan)

		sig := <-sigChan
		// Send cancellation message to the Bubbletea program
		// The Update function will handle cancellation synchronously and then quit
		p.Send(SignalCancelMsg{Signal: sig})

		// If user sends another signal (impatient) or we hit a shutdownTimeout, force exit
		timer := time.NewTimer(shutdownTimeout)
		defer timer.Stop()

		select {
		case <-sigChan:
			fmt.Fprintf(os.Stderr, "\nForce quitting...\n")
			os.Exit(130)
		case <-timer.C:
			fmt.Fprintf(os.Stderr, "\nTimeout trying to clean up, force quitting...\n")
			os.Exit(130)
		case <-doneCh:
			// Normal exit - graceful shutdown completed
			return
		}
	}()
	return doneCh
}
