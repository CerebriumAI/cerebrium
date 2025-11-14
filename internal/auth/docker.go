package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// DockerConfig represents the Docker config.json structure
type DockerConfig struct {
	Auths       map[string]DockerAuth `json:"auths" mapstructure:"auths"`
	CredStore   string                `json:"credStore,omitempty" mapstructure:"credStore"`
	CredHelpers map[string]string     `json:"credHelpers,omitempty" mapstructure:"credHelpers"`
}

// DockerAuth represents auth for a single registry
type DockerAuth struct {
	Auth string `json:"auth,omitempty" mapstructure:"auth"`
}

// GetDockerAuth reads the Docker config from the user's machine using viper
func GetDockerAuth() (string, error) {
	// Find the Docker config file
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".docker", "config.json")

	// Check if the file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// No Docker config found, return empty string (not an error)
		return "", nil
	}

	// Set up viper to read Docker config
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("json")

	// Read the config file
	if err := v.ReadInConfig(); err != nil {
		// If we can't read it, return empty string (not an error)
		// We treat this as non-fatal since Docker auth is optional
		//nolint:nilerr // Intentionally returning nil - Docker auth is optional
		return "", nil
	}

	// Unmarshal into our struct to check for credential helpers
	var dockerConfig DockerConfig
	if err := v.Unmarshal(&dockerConfig); err != nil {
		// Invalid format, return empty string
		// We treat this as non-fatal since Docker auth is optional
		//nolint:nilerr // Intentionally returning nil - Docker auth is optional
		return "", nil
	}

	// If using credStore or credHelpers, credentials are stored externally
	if dockerConfig.CredStore != "" {
		return "", nil
	}
	if len(dockerConfig.CredHelpers) > 0 {
		return "", nil
	}

	// Marshal the config back to JSON string to return
	// We need the raw JSON for the backend
	configBytes, err := json.Marshal(dockerConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Docker config: %w", err)
	}

	// Return the raw JSON string only if not using credential helpers
	return string(configBytes), nil
}

// HasDockerAuth checks if Docker authentication is available
func HasDockerAuth() bool {
	auth, _ := GetDockerAuth()
	return auth != ""
}
