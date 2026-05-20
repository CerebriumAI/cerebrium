package projectconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("DisableAuth is nil when not specified (backend applies default)", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		assert.Nil(t, config.Deployment.DisableAuth)
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

	t.Run("compute_tier is nil when not specified", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		assert.Nil(t, config.Scaling.ComputeTier)
	})

	t.Run("preserves explicit compute_tier protected", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.scaling]
compute_tier = "protected"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, config.Scaling.ComputeTier)
		assert.Equal(t, "protected", *config.Scaling.ComputeTier)
	})

	t.Run("returns error for invalid compute_tier", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.scaling]
compute_tier = "bogus"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		_, err = Load(configPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid compute_tier")
	})

	t.Run("container_runtime is nil when not specified", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		assert.Nil(t, config.ContainerRuntime)
	})

	t.Run("preserves explicit container_runtime v2", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.runtime]
container_runtime = "v2"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, config.ContainerRuntime)
		assert.Equal(t, "v2", *config.ContainerRuntime)
	})

	t.Run("container_runtime coexists with custom runtime sub-table", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.runtime]
container_runtime = "v2"

[cerebrium.runtime.custom]
port = 9000
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, config.ContainerRuntime)
		assert.Equal(t, "v2", *config.ContainerRuntime)
		require.NotNil(t, config.CustomRuntime)
		assert.Equal(t, 9000, config.CustomRuntime.Port)
	})

	t.Run("returns error for invalid container_runtime", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.runtime]
container_runtime = "v3"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		_, err = Load(configPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid container_runtime")
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
