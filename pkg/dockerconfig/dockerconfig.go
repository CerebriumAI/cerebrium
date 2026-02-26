package dockerconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config represents the Docker config.json structure
type Config struct {
	// Auths contains base64-encoded credentials for each registry
	// Example: {"docker.io": {"auth": "base64(username:password)"}}
	Auths map[string]Auth `json:"auths"`

	// CredsStore is set when Docker Desktop manages credentials externally (e.g., "desktop", "osxkeychain")
	// When present, the Auths entries typically have empty auth fields
	CredsStore string `json:"credsStore,omitempty"`

	// CredHelpers maps specific registries to credential helpers (e.g., {"gcr.io": "gcloud"})
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

// HasAuth checks if the config has any auth entries (including empty ones from credential stores)
func (c *Config) HasAuth() bool {
	if c == nil {
		return false
	}
	return len(c.Auths) > 0
}

// HasUsableAuth checks if the config has any auth entries with non-empty credentials.
// Credential store entries have registry keys but empty auth values — those don't count.
func (c *Config) HasUsableAuth() bool {
	if c == nil {
		return false
	}
	for _, auth := range c.Auths {
		if auth.Auth != "" && !IsOAuthTokenRegistry(auth.Auth) {
			return true
		}
	}
	return false
}

// HasCredsStore returns true if Docker is configured to use an external credential store
// (e.g., Docker Desktop's "desktop" store, macOS "osxkeychain", Windows "wincred")
func (c *Config) HasCredsStore() bool {
	if c == nil {
		return false
	}
	return c.CredsStore != ""
}

// HasCredHelpers returns true if Docker is configured with registry-specific credential helpers
func (c *Config) HasCredHelpers() bool {
	if c == nil {
		return false
	}
	return len(c.CredHelpers) > 0
}

// UsableAuthRegistries returns the list of registries that have non-empty, usable auth credentials
func (c *Config) UsableAuthRegistries() []string {
	if c == nil {
		return nil
	}
	var registries []string
	for registry, auth := range c.Auths {
		if auth.Auth != "" && !IsOAuthTokenRegistry(registry) {
			registries = append(registries, registry)
		}
	}
	return registries
}

// Warnings returns a list of human-readable warnings about the Docker config state.
// These help users understand why their private Docker image pull might fail.
func (c *Config) Warnings(privateImage string) []string {
	if c == nil {
		if privateImage != "" {
			return []string{
				fmt.Sprintf("No Docker config found (~/.docker/config.json). "+
					"If '%s' is a private image, run: docker login -u <username>", privateImage),
			}
		}
		return nil
	}

	var warnings []string

	// Check for credential store with no usable inline auth
	if c.HasCredsStore() && !c.HasUsableAuth() {
		warnings = append(warnings, fmt.Sprintf(
			"Docker is using credential store '%s' which is not compatible with Cerebrium's build system. "+
				"Remove \"credsStore\" from ~/.docker/config.json and run: docker login -u <username>",
			c.CredsStore))
	}

	// Check for credential helpers
	if c.HasCredHelpers() && !c.HasUsableAuth() {
		helpers := make([]string, 0, len(c.CredHelpers))
		for registry, helper := range c.CredHelpers {
			helpers = append(helpers, fmt.Sprintf("%s→%s", registry, helper))
		}
		warnings = append(warnings, fmt.Sprintf(
			"Docker is using credential helpers (%s) which are not compatible with Cerebrium's build system. "+
				"Run: docker login -u <username> to store credentials directly",
			strings.Join(helpers, ", ")))
	}

	// Check for auth entries that are all empty (credential store side effect)
	if c.HasAuth() && !c.HasUsableAuth() && !c.HasCredsStore() && !c.HasCredHelpers() {
		warnings = append(warnings, "Docker credentials found but all entries are empty. "+
			"This usually happens with Docker Desktop's credential store. "+
			"Run: docker login -u <username> to store credentials directly")
	}

	// No auth at all for a private image
	if !c.HasAuth() && !c.HasCredHelpers() && privateImage != "" {
		warnings = append(warnings, fmt.Sprintf(
			"No Docker credentials found. If '%s' is a private image, run: docker login -u <username>",
			privateImage))
	}

	// Registry mismatch: have usable auth but none match the image's registry
	if c.HasUsableAuth() && privateImage != "" {
		imageRegistry := extractRegistry(privateImage)
		if imageRegistry != "" {
			usable := c.UsableAuthRegistries()
			matched := false
			for _, reg := range usable {
				if registriesMatch(reg, imageRegistry) {
					matched = true
					break
				}
			}
			if !matched {
				warnings = append(warnings, fmt.Sprintf(
					"Docker credentials found for [%s] but your image '%s' is from '%s'. "+
						"Run: docker login -u <username> %s",
					strings.Join(usable, ", "), privateImage, imageRegistry, imageRegistry))
			}
		}
	}

	return warnings
}

// extractRegistry extracts the registry hostname from a Docker image URL.
// Returns "" for Docker Hub official images (no namespace).
func extractRegistry(imageURL string) string {
	// Strip tag
	image := strings.Split(imageURL, ":")[0]

	parts := strings.Split(image, "/")
	if len(parts) == 1 {
		// Official image like "debian" — no registry
		return ""
	}

	// If first part has a dot or colon, it's a registry hostname
	// e.g., "gcr.io/project/image", "123456.dkr.ecr.us-east-1.amazonaws.com/image"
	if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") {
		return parts[0]
	}

	// namespace/image format → Docker Hub
	return "docker.io"
}

// registriesMatch checks if a credential registry entry matches a target registry.
// Handles Docker Hub aliases (docker.io, index.docker.io, registry-1.docker.io, etc.)
func registriesMatch(credRegistry, targetRegistry string) bool {
	credRegistry = normalizeRegistry(credRegistry)
	targetRegistry = normalizeRegistry(targetRegistry)
	return credRegistry == targetRegistry
}

func normalizeRegistry(reg string) string {
	reg = strings.TrimPrefix(reg, "https://")
	reg = strings.TrimPrefix(reg, "http://")
	reg = strings.TrimSuffix(reg, "/")
	// Remove common Docker Hub path suffixes
	reg = strings.TrimSuffix(reg, "/v1")
	reg = strings.TrimSuffix(reg, "/v2")

	// Normalize all Docker Hub variants to "docker.io"
	dockerHubAliases := []string{
		"index.docker.io",
		"registry-1.docker.io",
		"registry.docker.io",
		"registry.hub.docker.com",
	}
	for _, alias := range dockerHubAliases {
		if reg == alias {
			return "docker.io"
		}
	}

	return reg
}

// IsOAuthTokenRegistry checks if a registry URL is an OAuth-style token entry
// (e.g., /access-token, /refresh-token from Docker's web-based login flow)
func IsOAuthTokenRegistry(registry string) bool {
	return strings.HasSuffix(registry, "/access-token") ||
		strings.HasSuffix(registry, "/refresh-token")
}
