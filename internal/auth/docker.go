package auth

import (
	"github.com/cerebriumai/cerebrium/pkg/dockerconfig"
)

// GetDockerAuth reads the Docker auth configuration, filters to only usable credentials,
// and returns it as a JSON string. Returns empty string if no usable auth is available.
func GetDockerAuth() (string, error) {
	config, err := dockerconfig.Load()
	if err != nil || config == nil {
		return "", nil
	}

	// Build a clean config with only usable auth entries.
	// This filters out: empty auth (credential store), OAuth tokens, empty registry keys.
	usable := &dockerconfig.Config{
		Auths: make(map[string]dockerconfig.Auth),
	}
	for registry, auth := range config.Auths {
		if registry == "" || auth.Auth == "" {
			continue
		}
		if dockerconfig.IsOAuthTokenRegistry(registry) {
			continue
		}
		usable.Auths[registry] = auth
	}

	if len(usable.Auths) == 0 {
		return "", nil
	}

	return usable.ToJSON()
}

// GetDockerAuthWarnings checks the Docker config for common issues that would cause
// private image pulls to fail, and returns user-facing warning messages.
// The privateImage parameter is the docker_base_image_url from the user's config.
func GetDockerAuthWarnings(privateImage string) []string {
	config, err := dockerconfig.Load()
	if err != nil {
		if privateImage != "" {
			return []string{"Failed to read Docker config: " + err.Error()}
		}
		return nil
	}

	return config.Warnings(privateImage)
}
