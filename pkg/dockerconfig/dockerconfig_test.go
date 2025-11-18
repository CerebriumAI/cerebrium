package dockerconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromPath(t *testing.T) {
	t.Run("returns nil when file does not exist", func(t *testing.T) {
		config, err := LoadFromPath("/non/existent/path/config.json")
		assert.NoError(t, err)
		assert.Nil(t, config)
	})

	t.Run("loads valid config with auth", func(t *testing.T) {
		// Create a temporary config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		
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
		
		err := os.WriteFile(configPath, []byte(configData), 0600)
		require.NoError(t, err)
		
		config, err := LoadFromPath(configPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Len(t, config.Auths, 2)
		assert.Equal(t, "dXNlcjpwYXNz", config.Auths["docker.io"].Auth)
		assert.Equal(t, "anNvbl9rZXk=", config.Auths["gcr.io"].Auth)
	})

	t.Run("loads config with credential helpers", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		
		configData := `{
			"auths": {},
			"credStore": "osxkeychain",
			"credHelpers": {
				"gcr.io": "gcloud",
				"123456.dkr.ecr.us-east-1.amazonaws.com": "ecr-login"
			}
		}`
		
		err := os.WriteFile(configPath, []byte(configData), 0600)
		require.NoError(t, err)
		
		config, err := LoadFromPath(configPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Equal(t, "osxkeychain", config.CredStore)
		assert.Len(t, config.CredHelpers, 2)
		assert.Equal(t, "gcloud", config.CredHelpers["gcr.io"])
		assert.Equal(t, "ecr-login", config.CredHelpers["123456.dkr.ecr.us-east-1.amazonaws.com"])
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")
		
		err := os.WriteFile(configPath, []byte("invalid json"), 0600)
		require.NoError(t, err)
		
		config, err := LoadFromPath(configPath)
		assert.Error(t, err)
		assert.Nil(t, config)
		assert.Contains(t, err.Error(), "failed to parse Docker config")
	})
}

func TestConfig_ToJSON(t *testing.T) {
	t.Run("converts config to JSON string", func(t *testing.T) {
		config := &Config{
			Auths: map[string]Auth{
				"docker.io": {Auth: "dXNlcjpwYXNz"},
			},
		}
		
		jsonStr, err := config.ToJSON()
		assert.NoError(t, err)
		assert.NotEmpty(t, jsonStr)
		
		// Verify it's valid JSON
		var parsed map[string]interface{}
		err = json.Unmarshal([]byte(jsonStr), &parsed)
		assert.NoError(t, err)
		
		// Verify content
		auths := parsed["auths"].(map[string]interface{})
		dockerAuth := auths["docker.io"].(map[string]interface{})
		assert.Equal(t, "dXNlcjpwYXNz", dockerAuth["auth"])
	})

	t.Run("returns empty string for nil config", func(t *testing.T) {
		var config *Config
		jsonStr, err := config.ToJSON()
		assert.NoError(t, err)
		assert.Empty(t, jsonStr)
	})
}

func TestConfig_HasCredentialHelpers(t *testing.T) {
	t.Run("returns false for nil config", func(t *testing.T) {
		var config *Config
		assert.False(t, config.HasCredentialHelpers())
	})

	t.Run("returns false when no helpers", func(t *testing.T) {
		config := &Config{
			Auths: map[string]Auth{
				"docker.io": {Auth: "token"},
			},
		}
		assert.False(t, config.HasCredentialHelpers())
	})

	t.Run("returns true with credStore", func(t *testing.T) {
		config := &Config{
			CredStore: "osxkeychain",
		}
		assert.True(t, config.HasCredentialHelpers())
	})

	t.Run("returns true with credHelpers", func(t *testing.T) {
		config := &Config{
			CredHelpers: map[string]string{
				"gcr.io": "gcloud",
			},
		}
		assert.True(t, config.HasCredentialHelpers())
	})
}

func TestConfig_HasAuth(t *testing.T) {
	t.Run("returns false for nil config", func(t *testing.T) {
		var config *Config
		assert.False(t, config.HasAuth())
	})

	t.Run("returns false when no auth entries", func(t *testing.T) {
		config := &Config{
			Auths: map[string]Auth{},
		}
		assert.False(t, config.HasAuth())
	})

	t.Run("returns true when auth entries exist", func(t *testing.T) {
		config := &Config{
			Auths: map[string]Auth{
				"docker.io": {Auth: "token"},
			},
		}
		assert.True(t, config.HasAuth())
	})
}
