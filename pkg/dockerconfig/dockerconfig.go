package dockerconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the Docker config.json structure
type Config struct {
	// Auths contains base64-encoded credentials for each registry
	// Example: {"docker.io": {"auth": "base64(username:password)"}}
	Auths map[string]Auth `json:"auths"`

	// CredStore specifies an external program to manage ALL registry credentials
	// Example: "osxkeychain" on macOS, "wincred" on Windows, "secretservice" on Linux
	// When set, credentials are stored in the OS credential store instead of this file
	CredStore string `json:"credStore,omitempty"`

	// CredHelpers specifies external programs for specific registries only
	// Example: {"gcr.io": "gcloud", "123456.dkr.ecr.us-east-1.amazonaws.com": "ecr-login"}
	// Takes precedence over CredStore for specified registries
	CredHelpers map[string]string `json:"credHelpers,omitempty"`
}

// Auth represents auth for a single registry
type Auth struct {
	Auth string `json:"auth,omitempty"`
}

// Load reads the Docker config from the default location (~/.docker/config.json)
func Load() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".docker", "config.json")
	return LoadFromPath(configPath)
}

// LoadFromPath reads the Docker config from a specific path
func LoadFromPath(configPath string) (*Config, error) {
	// Check if the file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// No Docker config found
		return nil, nil
	}

	// Read the config file directly
	configBytes, err := os.ReadFile(filepath.Clean(configPath)) // #nosec G304 - path is constructed from home directory
	if err != nil {
		return nil, fmt.Errorf("failed to read Docker config: %w", err)
	}

	// Parse the JSON
	var config Config
	if err := json.Unmarshal(configBytes, &config); err != nil {
		return nil, fmt.Errorf("failed to parse Docker config: %w", err)
	}

	return &config, nil
}

// ToJSON converts the config to a JSON string
func (c *Config) ToJSON() (string, error) {
	if c == nil {
		return "", nil
	}

	bytes, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("failed to marshal Docker config: %w", err)
	}

	return string(bytes), nil
}

// HasCredentialHelpers checks if the config uses external credential storage
func (c *Config) HasCredentialHelpers() bool {
	if c == nil {
		return false
	}
	return c.CredStore != "" || len(c.CredHelpers) > 0
}

// HasAuth checks if the config has any auth entries
func (c *Config) HasAuth() bool {
	if c == nil {
		return false
	}
	return len(c.Auths) > 0
}
