package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// DockerConfig represents the Docker config.json structure
type DockerConfig struct {
	Auths map[string]DockerAuth `json:"auths"`
}

// DockerAuth represents auth for a single registry
type DockerAuth struct {
	Auth string `json:"auth,omitempty"`
}

// GetDockerAuth reads the Docker config from the user's machine
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

	// Read the config file
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		// If we can't read it, return empty string (not an error)
		return "", nil
	}

	// Parse to check for credential helpers
	var configMap map[string]interface{}
	if err := json.Unmarshal(configBytes, &configMap); err != nil {
		// Invalid JSON, return empty string
		return "", nil
	}

	// If using credStore or credHelpers, credentials are stored externally
	if _, hasCredStore := configMap["credStore"]; hasCredStore {
		return "", nil
	}
	if _, hasCredHelpers := configMap["credHelpers"]; hasCredHelpers {
		return "", nil
	}

	// Return the raw JSON string only if not using credential helpers
	return string(configBytes), nil
}

// GetDockerAuthForRegistry extracts auth for a specific registry from Docker config
func GetDockerAuthForRegistry(registryURL string) (string, error) {
	configStr, err := GetDockerAuth()
	if err != nil {
		return "", err
	}

	if configStr == "" {
		return "", nil
	}

	var config DockerConfig
	if err := json.Unmarshal([]byte(configStr), &config); err != nil {
		return "", fmt.Errorf("failed to parse Docker config: %w", err)
	}

	// Check if auth exists for the specific registry
	if auth, exists := config.Auths[registryURL]; exists {
		authJSON, err := json.Marshal(map[string]map[string]DockerAuth{
			"auths": {
				registryURL: auth,
			},
		})
		if err != nil {
			return "", fmt.Errorf("failed to marshal auth: %w", err)
		}
		return string(authJSON), nil
	}

	return "", nil // No auth found for this registry
}

// HasDockerAuth checks if Docker authentication is available
func HasDockerAuth() bool {
	auth, _ := GetDockerAuth()
	return auth != ""
}
