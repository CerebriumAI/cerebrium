package ui

import "os"

// SignalCancelMsg is sent when a termination signal is received (SIGINT, SIGTERM)
type SignalCancelMsg struct {
	Signal os.Signal
}
