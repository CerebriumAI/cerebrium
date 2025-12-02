package ui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
)

// SimpleSpinner provides a non-Bubbletea spinner for simple CLI output
type SimpleSpinner struct {
	message string
	frames  []string
	stop    chan struct{}
	done    chan struct{}
	mu      sync.Mutex
}

// NewSimpleSpinner creates a new simple spinner with a message
func NewSimpleSpinner(message string) *SimpleSpinner {
	return &SimpleSpinner{
		message: message,
		frames:  []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation
func (s *SimpleSpinner) Start() {
	// Only show spinner if stdout is a TTY
	if !isatty.IsTerminal(os.Stdout.Fd()) {
		close(s.done)
		return
	}

	go func() {
		defer close(s.done)
		i := 0
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.stop:
				// Clear the spinner line
				fmt.Print("\r\033[K")
				return
			case <-ticker.C:
				s.mu.Lock()
				fmt.Printf("\r%s %s", s.frames[i%len(s.frames)], s.message)
				s.mu.Unlock()
				i++
			}
		}
	}()
}

// Stop stops the spinner animation
func (s *SimpleSpinner) Stop() {
	close(s.stop)
	<-s.done
}
