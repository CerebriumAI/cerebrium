package projectconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("applies default DisableAuth when not specified", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, config.Deployment.DisableAuth)
		assert.Equal(t, true, *config.Deployment.DisableAuth)
	})

	t.Run("preserves explicit DisableAuth false", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"
disable_auth = false
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, config.Deployment.DisableAuth)
		assert.Equal(t, false, *config.Deployment.DisableAuth)
	})

	t.Run("preserves explicit DisableAuth true", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"
disable_auth = true
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, config.Deployment.DisableAuth)
		assert.Equal(t, true, *config.Deployment.DisableAuth)
	})

	t.Run("returns error when file not found", func(t *testing.T) {
		_, err := Load("/nonexistent/cerebrium.toml")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config file not found")
	})

	t.Run("returns error when cerebrium key missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[other]
name = "test-app"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		_, err = Load(configPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "'cerebrium' key not found")
	})
}
