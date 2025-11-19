package auth

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDockerAuth(t *testing.T) {
	t.Run("returns empty when no config exists", func(t *testing.T) {
		// Set HOME and USERPROFILE to a temp directory with no Docker config
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome) // For Windows

		auth, err := GetDockerAuth()
		assert.NoError(t, err)
		assert.Empty(t, auth)
	})

	t.Run("returns auth JSON when config has auth entries", func(t *testing.T) {
		// Create a temp HOME with Docker config
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome) // For Windows

		dockerDir := filepath.Join(tmpHome, ".docker")
		err := os.MkdirAll(dockerDir, 0700)
		require.NoError(t, err)

		configPath := filepath.Join(dockerDir, "config.json")
		configData := `{
			"auths": {
				"docker.io": {
					"auth": "dXNlcjpwYXNz"
				},
				"gcr.io": {
					"auth": "anNvbl9rZXk="
				}
			}
		}`

		err = os.WriteFile(configPath, []byte(configData), 0600)
		require.NoError(t, err)

		auth, err := GetDockerAuth()
		assert.NoError(t, err)
		assert.NotEmpty(t, auth)

		// Verify it contains the expected registries
		assert.Contains(t, auth, "docker.io")
		assert.Contains(t, auth, "gcr.io")
		assert.Contains(t, auth, "dXNlcjpwYXNz")
		assert.Contains(t, auth, "anNvbl9rZXk=")
	})

	t.Run("returns empty when no auth entries", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome) // For Windows

		dockerDir := filepath.Join(tmpHome, ".docker")
		err := os.MkdirAll(dockerDir, 0700)
		require.NoError(t, err)

		configPath := filepath.Join(dockerDir, "config.json")
		// Config with empty auths
		configData := `{
			"auths": {}
		}`
		
		err = os.WriteFile(configPath, []byte(configData), 0600)
		require.NoError(t, err)

		auth, err := GetDockerAuth()
		assert.NoError(t, err)
		assert.Empty(t, auth, "Should return empty when no auth entries")
	})

	t.Run("returns empty on config read error", func(t *testing.T) {
		tmpHome := t.TempDir()
		t.Setenv("HOME", tmpHome)
		t.Setenv("USERPROFILE", tmpHome) // For Windows

		dockerDir := filepath.Join(tmpHome, ".docker")
		err := os.MkdirAll(dockerDir, 0700)
		require.NoError(t, err)

		configPath := filepath.Join(dockerDir, "config.json")
		// Write invalid JSON
		err = os.WriteFile(configPath, []byte("invalid json"), 0600)
		require.NoError(t, err)

		auth, err := GetDockerAuth()
		assert.NoError(t, err) // We intentionally return nil error for optional Docker auth
		assert.Empty(t, auth)
	})

}
