package auth

import (
	"github.com/cerebriumai/cerebrium/pkg/dockerconfig"
)

// GetDockerAuth reads the Docker auth configuration and returns it as a JSON string
// Returns empty string if no auth is available or if using credential helpers
func GetDockerAuth() (string, error) {
	// Load Docker config from default location
	config, err := dockerconfig.Load()
	if err != nil {
		// We treat this as non-fatal since Docker auth is optional
		//nolint:nilerr // Intentionally returning nil - Docker auth is optional
		return "", nil
	}

	// No config found
	if config == nil {
		return "", nil
	}

	// If using credential helpers, credentials are stored externally
	// and we can't access them directly
	if config.HasCredentialHelpers() {
		return "", nil
	}

	// Convert to JSON for the backend
	// We need the raw JSON string for compatibility
	jsonStr, err := config.ToJSON()
	if err != nil {
		// We treat this as non-fatal since Docker auth is optional
		//nolint:nilerr // Intentionally returning nil - Docker auth is optional
		return "", nil
	}

	return jsonStr, nil
}
