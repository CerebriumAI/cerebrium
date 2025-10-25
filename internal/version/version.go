package version

// Version information - updated during releases
// This will be overridden at build time using -ldflags
var (
	// Version is the current version of the CLI
	Version = "dev"

	// Commit is the git commit hash
	Commit = "unknown"

	// BuildDate is when the binary was built
	BuildDate = "unknown"
)

// GetFullVersion returns detailed version information
func GetFullVersion() string {
	return "cerebrium " + Version + " (commit: " + Commit + ", built: " + BuildDate + ")"
}
