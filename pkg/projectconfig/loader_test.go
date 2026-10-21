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

func TestLoad_Compute(t *testing.T) {
	const header = `[cerebrium.deployment]
name = "test-app"

[cerebrium.hardware]
`

	write := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "cerebrium.toml")
		require.NoError(t, os.WriteFile(path, []byte(header+body), 0644))
		return path
	}

	t.Run("scalar form populates primary only", func(t *testing.T) {
		cfg, err := Load(write(t, `compute = "HOPPER_H100"`))
		require.NoError(t, err)
		assert.Equal(t, ComputeField{"HOPPER_H100"}, cfg.Hardware.Compute)
		assert.Equal(t, "HOPPER_H100", cfg.Hardware.Compute.Primary())
		assert.True(t, cfg.Hardware.Compute.IsSet())
	})

	t.Run("array form preserves order", func(t *testing.T) {
		cfg, err := Load(write(t, `compute = ["HOPPER_H100", "HOPPER_H200", "AMPERE_A100_80GB"]`))
		require.NoError(t, err)
		assert.Equal(t, ComputeField{"HOPPER_H100", "HOPPER_H200", "AMPERE_A100_80GB"}, cfg.Hardware.Compute)
		assert.Equal(t, "HOPPER_H100", cfg.Hardware.Compute.Primary())
	})

	t.Run("single-element array behaves like scalar", func(t *testing.T) {
		cfg, err := Load(write(t, `compute = ["HOPPER_H100"]`))
		require.NoError(t, err)
		assert.Equal(t, ComputeField{"HOPPER_H100"}, cfg.Hardware.Compute)
		assert.Equal(t, "HOPPER_H100", cfg.Hardware.Compute.Primary())
	})

	t.Run("missing compute leaves field empty", func(t *testing.T) {
		cfg, err := Load(write(t, ``))
		require.NoError(t, err)
		assert.False(t, cfg.Hardware.Compute.IsSet())
		assert.Equal(t, "", cfg.Hardware.Compute.Primary())
	})

	t.Run("empty array is rejected", func(t *testing.T) {
		_, err := Load(write(t, `compute = []`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "compute array must not be empty")
	})

	t.Run("empty string is rejected", func(t *testing.T) {
		_, err := Load(write(t, `compute = ""`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "compute must not be empty")
	})

	t.Run("non-string element is rejected", func(t *testing.T) {
		_, err := Load(write(t, `compute = ["HOPPER_H100", 42]`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "compute values must be non-empty strings")
	})

	t.Run("wrong type is rejected", func(t *testing.T) {
		_, err := Load(write(t, `compute = 42`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "compute must be a string or array of strings")
	})
}

func TestToPayload_Compute(t *testing.T) {
	t.Run("scalar form sends single-element array", func(t *testing.T) {
		pc := &ProjectConfig{
			Deployment: DeploymentConfig{Name: "x"},
			Hardware:   HardwareConfig{Compute: ComputeField{"HOPPER_H100"}},
		}
		assert.Equal(t, []string{"HOPPER_H100"}, pc.ToPayload()["compute"])
	})

	t.Run("multi-element form sends array", func(t *testing.T) {
		pc := &ProjectConfig{
			Deployment: DeploymentConfig{Name: "x"},
			Hardware:   HardwareConfig{Compute: ComputeField{"HOPPER_H100", "HOPPER_H200"}},
		}
		assert.Equal(t, []string{"HOPPER_H100", "HOPPER_H200"}, pc.ToPayload()["compute"])
	})

	t.Run("unset compute is omitted", func(t *testing.T) {
		pc := &ProjectConfig{Deployment: DeploymentConfig{Name: "x"}}
		_, present := pc.ToPayload()["compute"]
		assert.False(t, present)
	})
}

func TestPartnerImageVersion(t *testing.T) {
	loadPartner := func(t *testing.T, partnerBody string) *ProjectConfig {
		t.Helper()
		configPath := filepath.Join(t.TempDir(), "cerebrium.toml")
		content := `[cerebrium.deployment]
name = "test-deepgram"

[cerebrium.runtime.deepgram]
` + partnerBody
		require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))

		config, err := Load(configPath)
		require.NoError(t, err)
		require.NotNil(t, config.PartnerService)
		return config
	}

	t.Run("image_version is nil when not specified", func(t *testing.T) {
		config := loadPartner(t, "")
		assert.Nil(t, config.PartnerService.ImageVersion)
	})

	t.Run("parses image_version", func(t *testing.T) {
		config := loadPartner(t, "image_version = \"260416\"\n")
		require.NotNil(t, config.PartnerService.ImageVersion)
		assert.Equal(t, "260416", *config.PartnerService.ImageVersion)
	})

	t.Run("image_version coexists with other partner keys", func(t *testing.T) {
		config := loadPartner(t, "port = 8080\nimage_version = \"260416\"\n")
		require.NotNil(t, config.PartnerService.ImageVersion)
		assert.Equal(t, "260416", *config.PartnerService.ImageVersion)
		require.NotNil(t, config.PartnerService.Port)
		assert.Equal(t, 8080, *config.PartnerService.Port)
	})

	t.Run("payload carries imageVersion when set", func(t *testing.T) {
		config := loadPartner(t, "image_version = \"260416\"\n")
		payload := config.ToPayload()
		assert.Equal(t, "deepgram", payload["partnerService"])
		assert.Equal(t, "260416", payload["imageVersion"])
	})

	t.Run("payload omits imageVersion when unset", func(t *testing.T) {
		payload := loadPartner(t, "").ToPayload()
		_, present := payload["imageVersion"]
		assert.False(t, present)
	})

	t.Run("payload carries imageVersion alongside custom runtime", func(t *testing.T) {
		version := "260416"
		pc := &ProjectConfig{
			Deployment:     DeploymentConfig{Name: "test-deepgram"},
			CustomRuntime:  &CustomRuntimeConfig{Port: 8080},
			PartnerService: &PartnerServiceConfig{Name: "deepgram", ImageVersion: &version},
		}
		assert.Equal(t, "260416", pc.ToPayload()["imageVersion"])
	})
}
