package projectconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMainPy(t *testing.T) {
	baseConfig := func() *ProjectConfig {
		return &ProjectConfig{
			Deployment: DeploymentConfig{Name: "test-app"},
		}
	}

	t.Run("partner service does not require main.py", func(t *testing.T) {
		t.Chdir(t.TempDir())

		config := baseConfig()
		config.PartnerService = &PartnerServiceConfig{Name: "deepgram"}

		assert.NoError(t, Validate(config))
	})

	t.Run("partner service alongside custom runtime does not require main.py", func(t *testing.T) {
		t.Chdir(t.TempDir())

		config := baseConfig()
		config.PartnerService = &PartnerServiceConfig{Name: "rime"}
		config.CustomRuntime = &CustomRuntimeConfig{Entrypoint: []string{"uvicorn"}}

		assert.NoError(t, Validate(config))
	})

	t.Run("cortex runtime still requires main.py", func(t *testing.T) {
		t.Chdir(t.TempDir())

		err := Validate(baseConfig())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "main.py not found")
	})

	t.Run("cortex runtime passes when main.py exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.py"), nil, 0644))
		t.Chdir(tmpDir)

		assert.NoError(t, Validate(baseConfig()))
	})
}
