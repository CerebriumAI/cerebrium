package logging

// isTerminalStatus checks if a build status is terminal
func isTerminalStatus(status string) bool {
	switch status {
	case "success", "build_failure", "init_failure", "ready", "failure", "cancelled", "init_timeout":
		return true
	default:
		return false
	}
}
