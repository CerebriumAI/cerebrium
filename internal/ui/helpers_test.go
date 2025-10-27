package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTerminalStatus(t *testing.T) {
	tcs := []struct {
		status     string
		isTerminal bool
	}{
		{status: "success", isTerminal: true},
		{status: "build_failure", isTerminal: true},
		{status: "init_failure", isTerminal: true},
		{status: "ready", isTerminal: true},
		{status: "failure", isTerminal: true},
		{status: "cancelled", isTerminal: true},
		{status: "init_timeout", isTerminal: true},
		{status: "building", isTerminal: false},
		{status: "pending", isTerminal: false},
		{status: "unknown", isTerminal: false},
		{status: "", isTerminal: false},
	}

	for _, tc := range tcs {
		t.Run(tc.status, func(t *testing.T) {
			result := IsTerminalStatus(tc.status)
			assert.Equal(t, tc.isTerminal, result)
		})
	}
}