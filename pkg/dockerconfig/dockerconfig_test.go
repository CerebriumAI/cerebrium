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

	t.Run("loads config with empty auths", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")

		configData := `{
			"auths": {}
		}`

		err := os.WriteFile(configPath, []byte(configData), 0600)
		require.NoError(t, err)

		config, err := LoadFromPath(configPath)
		assert.NoError(t, err)
		assert.NotNil(t, config)
		assert.Empty(t, config.Auths)
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

func TestConfig_HasUsableAuth(t *testing.T) {
	t.Run("returns false for nil config", func(t *testing.T) {
		var config *Config
		assert.False(t, config.HasUsableAuth())
	})

	t.Run("returns false when auth entries are empty (credential store)", func(t *testing.T) {
		config := &Config{
			Auths: map[string]Auth{
				"docker.io": {Auth: ""},
				"gcr.io":    {Auth: ""},
			},
		}
		assert.False(t, config.HasUsableAuth())
	})

	t.Run("returns true when auth entries have credentials", func(t *testing.T) {
		config := &Config{
			Auths: map[string]Auth{
				"docker.io": {Auth: "dXNlcjpwYXNz"},
			},
		}
		assert.True(t, config.HasUsableAuth())
	})

	t.Run("returns true when mix of empty and valid entries", func(t *testing.T) {
		config := &Config{
			Auths: map[string]Auth{
				"docker.io": {Auth: ""},
				"gcr.io":    {Auth: "dXNlcjpwYXNz"},
			},
		}
		assert.True(t, config.HasUsableAuth())
	})
}

func TestConfig_HasCredsStore(t *testing.T) {
	t.Run("returns false when no credsStore", func(t *testing.T) {
		config := &Config{}
		assert.False(t, config.HasCredsStore())
	})

	t.Run("returns true for Docker Desktop", func(t *testing.T) {
		config := &Config{CredsStore: "desktop"}
		assert.True(t, config.HasCredsStore())
	})

	t.Run("returns true for osxkeychain", func(t *testing.T) {
		config := &Config{CredsStore: "osxkeychain"}
		assert.True(t, config.HasCredsStore())
	})
}

func TestConfig_Warnings(t *testing.T) {
	t.Run("no warnings for valid config", func(t *testing.T) {
		config := &Config{
			Auths: map[string]Auth{
				"docker.io": {Auth: "dXNlcjpwYXNz"},
			},
		}
		warnings := config.Warnings("mycompany/private-image:latest")
		assert.Empty(t, warnings)
	})

	t.Run("warns about credsStore with no usable auth", func(t *testing.T) {
		config := &Config{
			CredsStore: "desktop",
			Auths: map[string]Auth{
				"docker.io": {Auth: ""},
			},
		}
		warnings := config.Warnings("mycompany/private-image:latest")
		assert.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "credential store")
		assert.Contains(t, warnings[0], "desktop")
		assert.Contains(t, warnings[0], "docker login -u")
	})

	t.Run("no warning for credsStore when usable auth exists", func(t *testing.T) {
		config := &Config{
			CredsStore: "desktop",
			Auths: map[string]Auth{
				"docker.io": {Auth: "dXNlcjpwYXNz"},
			},
		}
		warnings := config.Warnings("mycompany/private-image:latest")
		assert.Empty(t, warnings)
	})

	t.Run("warns about credHelpers with no usable auth", func(t *testing.T) {
		config := &Config{
			CredHelpers: map[string]string{
				"gcr.io": "gcloud",
			},
			Auths: map[string]Auth{},
		}
		warnings := config.Warnings("gcr.io/my-project/image:latest")
		assert.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "credential helpers")
		assert.Contains(t, warnings[0], "docker login -u")
	})

	t.Run("warns about empty auth entries", func(t *testing.T) {
		config := &Config{
			Auths: map[string]Auth{
				"docker.io": {Auth: ""},
			},
		}
		warnings := config.Warnings("mycompany/private-image:latest")
		assert.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "empty")
	})

	t.Run("warns when nil config and private image", func(t *testing.T) {
		var config *Config
		warnings := config.Warnings("mycompany/private-image:latest")
		assert.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "No Docker config found")
	})

	t.Run("no warnings when nil config and no private image", func(t *testing.T) {
		var config *Config
		warnings := config.Warnings("")
		assert.Empty(t, warnings)
	})

	t.Run("warns about registry mismatch", func(t *testing.T) {
		config := &Config{
			Auths: map[string]Auth{
				"https://index.docker.io/v1/": {Auth: "dXNlcjpwYXNz"},
			},
		}
		warnings := config.Warnings("123456.dkr.ecr.us-east-1.amazonaws.com/my-image:latest")
		assert.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "credentials found for")
		assert.Contains(t, warnings[0], "123456.dkr.ecr.us-east-1.amazonaws.com")
	})

	t.Run("no mismatch warning when Docker Hub auth matches Docker Hub image", func(t *testing.T) {
		config := &Config{
			Auths: map[string]Auth{
				"https://index.docker.io/v1/": {Auth: "dXNlcjpwYXNz"},
			},
		}
		warnings := config.Warnings("mycompany/private-image:latest")
		assert.Empty(t, warnings)
	})

	t.Run("no mismatch warning when ECR auth matches ECR image", func(t *testing.T) {
		config := &Config{
			Auths: map[string]Auth{
				"123456.dkr.ecr.us-east-1.amazonaws.com": {Auth: "dXNlcjpwYXNz"},
			},
		}
		warnings := config.Warnings("123456.dkr.ecr.us-east-1.amazonaws.com/my-image:latest")
		assert.Empty(t, warnings)
	})

	t.Run("loads credsStore from real Docker Desktop config", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.json")

		// Real Docker Desktop config.json
		configData := `{
			"auths": {
				"https://index.docker.io/v1/": {}
			},
			"credsStore": "desktop"
		}`

		err := os.WriteFile(configPath, []byte(configData), 0600)
		require.NoError(t, err)

		config, err := LoadFromPath(configPath)
		require.NoError(t, err)
		assert.True(t, config.HasCredsStore())
		assert.Equal(t, "desktop", config.CredsStore)
		assert.False(t, config.HasUsableAuth())

		warnings := config.Warnings("mycompany/private-image:latest")
		assert.NotEmpty(t, warnings)
	})
}
