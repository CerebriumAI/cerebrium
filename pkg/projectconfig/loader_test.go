package projectconfig

import (
	"os"
	"path/filepath"
	"strings"
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

	t.Run("returns error when no valid config found", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[other]
name = "test-app"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		_, err = Load(configPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no valid configuration found")
	})

	t.Run("parses cortex runtime section", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.runtime.cortex]
python_version = "3.12"
docker_base_image_url = "python:3.12-slim"
shell_commands = ["pip install numpy"]
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, config.Runtime)
		assert.Equal(t, "cortex", config.Runtime.Type)
		assert.Equal(t, "3.12", config.Runtime.Params["python_version"])
		assert.Equal(t, "python:3.12-slim", config.Runtime.Params["docker_base_image_url"])
		assert.Equal(t, "cortex", config.GetRuntimeType())
	})

	t.Run("parses python runtime section", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.runtime.python]
python_version = "3.11"
entrypoint = ["uvicorn", "myapp:app"]
port = 9000
healthcheck_endpoint = "/health"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, config.Runtime)
		assert.Equal(t, "python", config.Runtime.Type)
		assert.Equal(t, "3.11", config.Runtime.Params["python_version"])
		assert.Equal(t, int64(9000), config.Runtime.Params["port"])
		assert.Equal(t, "/health", config.Runtime.Params["healthcheck_endpoint"])
		assert.Equal(t, "python", config.GetRuntimeType())
	})

	t.Run("parses docker runtime section", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		// Create a dummy Dockerfile
		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		err := os.WriteFile(dockerfilePath, []byte("FROM python:3.11"), 0644)
		require.NoError(t, err)

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.runtime.docker]
dockerfile_path = "` + dockerfilePath + `"
port = 8080
entrypoint = ["python", "main.py"]
healthcheck_endpoint = "/healthz"
`
		err = os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, config.Runtime)
		assert.Equal(t, "docker", config.Runtime.Type)
		assert.Equal(t, dockerfilePath, config.Runtime.GetDockerfilePath())
		assert.Equal(t, int64(8080), config.Runtime.Params["port"])
		assert.Equal(t, "/healthz", config.Runtime.Params["healthcheck_endpoint"])
		assert.Equal(t, "docker", config.GetRuntimeType())
	})

	t.Run("adds deprecation warning for custom runtime", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.runtime.custom]
entrypoint = ["python", "app.py"]
port = 8000
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, config.Runtime)
		assert.Equal(t, "custom", config.Runtime.Type)
		// Should have both prefix deprecation warning and custom runtime warning
		assert.GreaterOrEqual(t, len(config.DeprecationWarnings), 1)
		foundCustomWarning := false
		for _, warning := range config.DeprecationWarnings {
			if strings.Contains(warning, "runtime.custom] is deprecated") {
				foundCustomWarning = true
				break
			}
		}
		assert.True(t, foundCustomWarning, "expected deprecation warning for custom runtime")
	})

	t.Run("adds deprecation warnings for deployment fields", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"
python_version = "3.11"
docker_base_image_url = "debian:bookworm-slim"
shell_commands = ["echo hello"]
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(config.DeprecationWarnings), 3)
	})

	t.Run("defaults to cortex runtime when no runtime specified", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		assert.Equal(t, "cortex", config.GetRuntimeType())
		assert.Nil(t, config.Runtime) // No explicit runtime section
	})

	t.Run("deprecated deployment fields are used in payload with warnings", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		// Use old-style config with deprecated fields in deployment section
		content := `[cerebrium.deployment]
name = "test-app"
python_version = "3.10"
docker_base_image_url = "python:3.10-slim"
shell_commands = ["pip install requests"]
pre_build_commands = ["echo pre-build"]
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)

		// Verify deprecation warnings are present
		assert.GreaterOrEqual(t, len(config.DeprecationWarnings), 4)

		// Verify payload uses the deprecated values as fallback
		payload := config.ToPayload()
		assert.Equal(t, "3.10", payload["pythonVersion"])
		assert.Equal(t, "python:3.10-slim", payload["baseImage"])
		assert.Equal(t, []string{"pip install requests"}, payload["shellCommands"])
		assert.Equal(t, []string{"echo pre-build"}, payload["preBuildCommands"])
		assert.Equal(t, "cortex", payload["runtime"])
	})

	t.Run("cortex runtime fields take precedence over deprecated deployment fields", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		// Config with both deprecated deployment fields and new cortex runtime fields
		content := `[cerebrium.deployment]
name = "test-app"
python_version = "3.10"
docker_base_image_url = "python:3.10-slim"

[cerebrium.runtime.cortex]
python_version = "3.12"
docker_base_image_url = "debian:bookworm-slim"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)

		// Verify deprecation warnings are present for deployment fields
		assert.GreaterOrEqual(t, len(config.DeprecationWarnings), 2)

		// Verify payload uses cortex runtime values (not deprecated deployment values)
		payload := config.ToPayload()
		assert.Equal(t, "3.12", payload["pythonVersion"])
		assert.Equal(t, "debian:bookworm-slim", payload["baseImage"])
		assert.Equal(t, "cortex", payload["runtime"])
	})

	t.Run("parses arbitrary runtime types for backend validation", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		// Test with a partner service like rime
		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.runtime.rime]
model_name = "arcana"
language = "en"
port = 8080
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, config.Runtime)
		assert.Equal(t, "rime", config.Runtime.Type)
		assert.Equal(t, "arcana", config.Runtime.Params["model_name"])
		assert.Equal(t, "en", config.Runtime.Params["language"])
		assert.Equal(t, int64(8080), config.Runtime.Params["port"])
		assert.Equal(t, "rime", config.GetRuntimeType())
	})

	t.Run("rejects multiple runtime types", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.runtime.cortex]
python_version = "3.11"

[cerebrium.runtime.python]
entrypoint = ["uvicorn", "app:app"]
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		_, err = Load(configPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "only one runtime type can be specified")
	})

	t.Run("parses runtime-specific dependencies", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.runtime.cortex]
python_version = "3.12"

[cerebrium.runtime.cortex.deps.pip]
torch = "2.0.0"
numpy = "latest"

[cerebrium.runtime.cortex.deps.apt]
ffmpeg = ""
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, config.Runtime)

		// Check runtime dependencies were parsed
		runtimeDeps := config.Runtime.GetDependencies()
		require.NotNil(t, runtimeDeps)
		assert.Equal(t, "2.0.0", runtimeDeps.Pip["torch"])
		assert.Equal(t, "latest", runtimeDeps.Pip["numpy"])
		assert.Equal(t, "", runtimeDeps.Apt["ffmpeg"])
	})

	t.Run("adds deprecation warning for top-level dependencies", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.dependencies.pip]
torch = "2.0.0"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)

		// Check for deprecation warning
		foundDeprecationWarning := false
		for _, warning := range config.DeprecationWarnings {
			if strings.Contains(warning, "[cerebrium.dependencies] is deprecated") {
				foundDeprecationWarning = true
				break
			}
		}
		assert.True(t, foundDeprecationWarning, "expected deprecation warning for top-level dependencies")
	})

	t.Run("no deprecation warning for runtime-only dependencies", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.runtime.cortex]
python_version = "3.12"

[cerebrium.runtime.cortex.deps.pip]
torch = "2.0.0"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)

		// Check there's no deprecation warning for dependencies
		for _, warning := range config.DeprecationWarnings {
			assert.NotContains(t, warning, "[cerebrium.dependencies] is deprecated")
		}
	})

	t.Run("merges top-level and runtime dependencies with runtime winning", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[cerebrium.dependencies.pip]
requests = "2.28.0"
numpy = "1.23.0"

[cerebrium.runtime.cortex]
python_version = "3.12"

[cerebrium.runtime.cortex.deps.pip]
torch = "2.0.0"
numpy = "1.24.0"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)

		effectiveDeps := config.GetEffectiveDependencies()

		// Runtime wins for numpy
		assert.Equal(t, "1.24.0", effectiveDeps.Pip["numpy"])
		// Top-level preserved for requests
		assert.Equal(t, "2.28.0", effectiveDeps.Pip["requests"])
		// Runtime-only package
		assert.Equal(t, "2.0.0", effectiveDeps.Pip["torch"])
	})

	// Tests for new format (without cerebrium. prefix)
	t.Run("parses new format without cerebrium prefix", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[deployment]
name = "test-app"

[runtime.cortex]
python_version = "3.12"

[hardware]
cpu = 2
memory = 4.0

[scaling]
min_replicas = 1
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)

		assert.Equal(t, "test-app", config.Deployment.Name)
		assert.Equal(t, "cortex", config.Runtime.Type)
		assert.Equal(t, "3.12", config.Runtime.Params["python_version"])
		assert.Equal(t, float64(2), *config.Hardware.CPU)
		assert.Equal(t, float64(4.0), *config.Hardware.Memory)
		assert.Equal(t, 1, *config.Scaling.MinReplicas)

		// No deprecation warning for cerebrium prefix in new format
		for _, warning := range config.DeprecationWarnings {
			assert.NotContains(t, warning, "[cerebrium.*] prefix is deprecated")
		}
	})

	t.Run("adds deprecation warning for cerebrium prefix", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)

		// Should have deprecation warning for cerebrium. prefix
		foundPrefixWarning := false
		for _, warning := range config.DeprecationWarnings {
			if strings.Contains(warning, "[cerebrium.*] prefix is deprecated") {
				foundPrefixWarning = true
				break
			}
		}
		assert.True(t, foundPrefixWarning, "expected deprecation warning for cerebrium. prefix")
	})

	t.Run("rejects mixed format", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[cerebrium.deployment]
name = "test-app"

[hardware]
cpu = 2
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		_, err = Load(configPath)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot mix")
	})

	t.Run("new format with runtime dependencies", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[deployment]
name = "test-app"

[runtime.cortex]
python_version = "3.12"

[runtime.cortex.deps.pip]
torch = "2.0.0"
numpy = "latest"

[runtime.cortex.deps.apt]
ffmpeg = ""
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)

		effectiveDeps := config.GetEffectiveDependencies()
		assert.Equal(t, "2.0.0", effectiveDeps.Pip["torch"])
		assert.Equal(t, "latest", effectiveDeps.Pip["numpy"])
		assert.Equal(t, "", effectiveDeps.Apt["ffmpeg"])
	})

	t.Run("new format python runtime", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		content := `[deployment]
name = "fastapi-app"

[runtime.python]
python_version = "3.11"
entrypoint = ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8000"]
port = 8000
healthcheck_endpoint = "/health"
`
		err := os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)

		assert.Equal(t, "python", config.Runtime.Type)
		assert.Equal(t, "3.11", config.Runtime.Params["python_version"])
		assert.Equal(t, int64(8000), config.Runtime.Params["port"])
		assert.Equal(t, "/health", config.Runtime.Params["healthcheck_endpoint"])
	})

	t.Run("new format docker runtime", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "cerebrium.toml")

		// Create a dummy Dockerfile
		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")
		err := os.WriteFile(dockerfilePath, []byte("FROM python:3.11"), 0644)
		require.NoError(t, err)

		content := `[deployment]
name = "docker-app"

[runtime.docker]
dockerfile_path = "` + dockerfilePath + `"
port = 8080
`
		err = os.WriteFile(configPath, []byte(content), 0644)
		require.NoError(t, err)

		config, err := Load(configPath)
		require.NoError(t, err)

		assert.Equal(t, "docker", config.Runtime.Type)
		assert.Equal(t, dockerfilePath, config.Runtime.GetDockerfilePath())
		assert.Equal(t, int64(8080), config.Runtime.Params["port"])
	})
}
