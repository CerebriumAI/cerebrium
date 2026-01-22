package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunInit(t *testing.T) {
	tcs := []struct {
		name          string
		projectName   string
		dir           string
		setupFunc     func(t *testing.T, dir string, projectName string)
		expectedError string
		validate      func(t *testing.T, dir string, projectName string)
	}{
		{
			name:        "successful initialization in current directory",
			projectName: "test-app",
			dir:         "./",
			setupFunc: func(t *testing.T, dir string, projectName string) {
				// No setup needed - directory shouldn't exist
			},
			expectedError: "",
			validate: func(t *testing.T, dir string, projectName string) {
				projectPath := filepath.Join(dir, projectName)

				// Check directory was created
				info, err := os.Stat(projectPath)
				require.NoError(t, err)
				assert.True(t, info.IsDir())

				// Check main.py exists and has correct content
				mainPath := filepath.Join(projectPath, "main.py")
				mainContent, err := os.ReadFile(mainPath)
				require.NoError(t, err)
				assert.Contains(t, string(mainContent), "def run(prompt: str):")
				assert.Contains(t, string(mainContent), "Running on Cerebrium")
				assert.Contains(t, string(mainContent), "cerebrium deploy")

				// Check requirements.txt does NOT exist (dependencies are in cerebrium.toml)
				requirementsPath := filepath.Join(projectPath, "requirements.txt")
				_, err = os.Stat(requirementsPath)
				assert.True(t, os.IsNotExist(err), "requirements.txt should not be created")

				// Check cerebrium.toml exists and has correct structure
				tomlPath := filepath.Join(projectPath, "cerebrium.toml")
				tomlContent, err := os.ReadFile(tomlPath)
				require.NoError(t, err)

				var config map[string]any
				err = toml.Unmarshal(tomlContent, &config)
				require.NoError(t, err)

				// Validate TOML structure
				cerebrium, ok := config["cerebrium"].(map[string]any)
				require.True(t, ok, "cerebrium key should exist")

				// Check deployment config
				deployment, ok := cerebrium["deployment"].(map[string]any)
				require.True(t, ok, "deployment section should exist")
				assert.Equal(t, projectName, deployment["name"])
				assert.Equal(t, true, deployment["disable_auth"])
				// python_version and docker_base_image_url are now in runtime.cortex
				assert.Nil(t, deployment["python_version"], "python_version should not be in deployment")
				assert.Nil(t, deployment["docker_base_image_url"], "docker_base_image_url should not be in deployment")

				// Check runtime.cortex config
				runtime, ok := cerebrium["runtime"].(map[string]any)
				require.True(t, ok, "runtime section should exist")
				cortex, ok := runtime["cortex"].(map[string]any)
				require.True(t, ok, "runtime.cortex section should exist")
				assert.Equal(t, "3.11", cortex["python_version"])
				assert.Equal(t, "debian:bookworm-slim", cortex["docker_base_image_url"])

				// Check include/exclude arrays
				include, ok := deployment["include"].([]any)
				require.True(t, ok, "include should be an array")
				assert.Len(t, include, 3)
				exclude, ok := deployment["exclude"].([]any)
				require.True(t, ok, "exclude should be an array")
				assert.Len(t, exclude, 1)

				// Check hardware config
				hardware, ok := cerebrium["hardware"].(map[string]any)
				require.True(t, ok, "hardware section should exist")
				assert.Equal(t, 2.0, hardware["cpu"])
				assert.Equal(t, 2.0, hardware["memory"])
				assert.Equal(t, "CPU", hardware["compute"])
				assert.Equal(t, "us-east-1", hardware["region"])
				// provider should NOT be in the template - it defaults to "aws" when loaded
				assert.NotContains(t, hardware, "provider")
				// gpu_count should NOT be present when compute is CPU
				assert.NotContains(t, hardware, "gpu_count")

				// Check scaling config
				scaling, ok := cerebrium["scaling"].(map[string]any)
				require.True(t, ok, "scaling section should exist")
				assert.Equal(t, int64(0), scaling["min_replicas"])
				assert.Equal(t, int64(2), scaling["max_replicas"])
				assert.Equal(t, int64(30), scaling["cooldown"])
				assert.Equal(t, int64(1), scaling["replica_concurrency"])
				assert.Equal(t, "concurrency_utilization", scaling["scaling_metric"])

				// Check dependencies config exists with pip section
				dependencies, ok := cerebrium["dependencies"].(map[string]any)
				require.True(t, ok, "dependencies section should exist")
				pip, ok := dependencies["pip"].(map[string]any)
				require.True(t, ok, "dependencies.pip section should exist")
				assert.Equal(t, "latest", pip["numpy"], "numpy should be set to latest")
			},
		},
		{
			name:        "successful initialization in custom directory",
			projectName: "my-custom-app",
			dir:         t.TempDir(),
			setupFunc: func(t *testing.T, dir string, projectName string) {
				// Directory should exist but project subdirectory shouldn't
			},
			expectedError: "",
			validate: func(t *testing.T, dir string, projectName string) {
				projectPath := filepath.Join(dir, projectName)

				// Check directory was created
				info, err := os.Stat(projectPath)
				require.NoError(t, err)
				assert.True(t, info.IsDir())

				// Check files exist
				mainPath := filepath.Join(projectPath, "main.py")
				_, err = os.Stat(mainPath)
				assert.NoError(t, err)

				tomlPath := filepath.Join(projectPath, "cerebrium.toml")
				_, err = os.Stat(tomlPath)
				assert.NoError(t, err)
			},
		},
		{
			name:        "error when directory already exists",
			projectName: "existing-app",
			dir:         "./",
			setupFunc: func(t *testing.T, dir string, projectName string) {
				// Create the directory that we're trying to init
				projectPath := filepath.Join(dir, projectName)
				err := os.MkdirAll(projectPath, 0755)
				require.NoError(t, err)
			},
			expectedError: "directory already exists",
			validate: func(t *testing.T, dir string, projectName string) {
				// Should not have created files
				projectPath := filepath.Join(dir, projectName)
				mainPath := filepath.Join(projectPath, "main.py")
				_, err := os.Stat(mainPath)
				assert.True(t, os.IsNotExist(err), "main.py should not exist")

				tomlPath := filepath.Join(projectPath, "cerebrium.toml")
				_, err = os.Stat(tomlPath)
				assert.True(t, os.IsNotExist(err), "cerebrium.toml should not exist")
			},
		},
		{
			name:        "project name with special characters",
			projectName: "my-app_v2",
			dir:         "./",
			setupFunc: func(t *testing.T, dir string, projectName string) {
				// No setup needed
			},
			expectedError: "",
			validate: func(t *testing.T, dir string, projectName string) {
				projectPath := filepath.Join(dir, projectName)

				// Check directory was created with correct name
				info, err := os.Stat(projectPath)
				require.NoError(t, err)
				assert.True(t, info.IsDir())
				assert.Equal(t, projectName, info.Name())

				// Check TOML has correct name
				tomlPath := filepath.Join(projectPath, "cerebrium.toml")
				tomlContent, err := os.ReadFile(tomlPath)
				require.NoError(t, err)

				var config map[string]any
				err = toml.Unmarshal(tomlContent, &config)
				require.NoError(t, err)

				cerebrium := config["cerebrium"].(map[string]any)
				deployment := cerebrium["deployment"].(map[string]any)
				assert.Equal(t, projectName, deployment["name"])
			},
		},
		{
			name:          "error with path traversal attempt",
			projectName:   "../evil-app",
			dir:           "./",
			setupFunc:     func(t *testing.T, dir string, projectName string) {},
			expectedError: "project name cannot contain path separators",
			validate:      func(t *testing.T, dir string, projectName string) {},
		},
		{
			name:        "error with absolute path",
			projectName: filepath.Join(os.TempDir(), "evil-app"),
			dir:         "./",
			setupFunc:   func(t *testing.T, dir string, projectName string) {},
			// Absolute paths are detected on all platforms using filepath.Join with os.TempDir()
			expectedError: "project name cannot be an absolute path",
			validate:      func(t *testing.T, dir string, projectName string) {},
		},
		{
			name:          "error with dot name",
			projectName:   "..",
			dir:           "./",
			setupFunc:     func(t *testing.T, dir string, projectName string) {},
			expectedError: "project name cannot be '.' or '..'",
			validate:      func(t *testing.T, dir string, projectName string) {},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// Use test-specific temp directory
			ctx := t.Context()
			_ = ctx

			// Create temp directory for this test if using current directory
			var testDir string
			if tc.dir == "./" {
				testDir = t.TempDir()
			} else {
				testDir = tc.dir
			}

			// Setup
			if tc.setupFunc != nil {
				tc.setupFunc(t, testDir, tc.projectName)
			}

			// Cleanup after test
			defer func() {
				projectPath := filepath.Join(testDir, tc.projectName)
				os.RemoveAll(projectPath)
			}()

			// Create a mock cobra command for testing
			cmd := NewInitCmd()
			cmd.SetArgs([]string{tc.projectName, "--dir", testDir})

			// Execute command
			err := cmd.Execute()

			// Assert error expectation
			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
			}

			// Run validation
			if tc.validate != nil {
				tc.validate(t, testDir, tc.projectName)
			}
		})
	}
}

func TestCreateDefaultConfig(t *testing.T) {
	tcs := []struct {
		name          string
		projectName   string
		expectedError string
		validate      func(t *testing.T, path string, projectName string)
	}{
		{
			name:          "creates valid TOML file",
			projectName:   "test-project",
			expectedError: "",
			validate: func(t *testing.T, path string, projectName string) {
				// Read and parse the TOML file
				content, err := os.ReadFile(path)
				require.NoError(t, err)

				var config map[string]any
				err = toml.Unmarshal(content, &config)
				require.NoError(t, err)

				// Validate structure
				cerebrium, ok := config["cerebrium"].(map[string]any)
				require.True(t, ok)

				deployment, ok := cerebrium["deployment"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, projectName, deployment["name"])
				assert.NotNil(t, deployment["include"])
				assert.NotNil(t, deployment["exclude"])
				// python_version and docker_base_image_url are now in runtime.cortex
				assert.Nil(t, deployment["python_version"], "python_version should not be in deployment")
				assert.Nil(t, deployment["docker_base_image_url"], "docker_base_image_url should not be in deployment")

				// Check runtime.cortex config
				runtime, ok := cerebrium["runtime"].(map[string]any)
				require.True(t, ok, "runtime section should exist")
				cortex, ok := runtime["cortex"].(map[string]any)
				require.True(t, ok, "runtime.cortex section should exist")
				assert.NotNil(t, cortex["python_version"])
				assert.NotNil(t, cortex["docker_base_image_url"])

				hardware, ok := cerebrium["hardware"].(map[string]any)
				require.True(t, ok)
				assert.NotNil(t, hardware["cpu"])
				assert.NotNil(t, hardware["memory"])
				assert.NotNil(t, hardware["compute"])
				// provider should NOT be in the template - it defaults to "aws" when loaded
				assert.Nil(t, hardware["provider"])
				assert.NotNil(t, hardware["region"])

				scaling, ok := cerebrium["scaling"].(map[string]any)
				require.True(t, ok)
				assert.NotNil(t, scaling["min_replicas"])
				assert.NotNil(t, scaling["max_replicas"])
				assert.NotNil(t, scaling["cooldown"])
				assert.NotNil(t, scaling["replica_concurrency"])
				assert.NotNil(t, scaling["scaling_metric"])

				// Dependencies section should exist with pip
				dependencies, ok := cerebrium["dependencies"].(map[string]any)
				require.True(t, ok, "dependencies section should exist")
				pip, ok := dependencies["pip"].(map[string]any)
				require.True(t, ok, "dependencies.pip section should exist")
				assert.Equal(t, "latest", pip["numpy"], "numpy should be set to latest")
			},
		},
		{
			name:          "handles special characters in name",
			projectName:   "test-app_123",
			expectedError: "",
			validate: func(t *testing.T, path string, projectName string) {
				content, err := os.ReadFile(path)
				require.NoError(t, err)

				var config map[string]any
				err = toml.Unmarshal(content, &config)
				require.NoError(t, err)

				cerebrium := config["cerebrium"].(map[string]any)
				deployment := cerebrium["deployment"].(map[string]any)
				assert.Equal(t, projectName, deployment["name"])
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			// Create temp directory and file path
			tempDir := t.TempDir()
			configPath := filepath.Join(tempDir, "cerebrium.toml")

			// Run function
			err := createDefaultConfig(configPath, tc.projectName)

			// Assert error
			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
			} else {
				require.NoError(t, err)
			}

			// Run validation
			if tc.validate != nil {
				tc.validate(t, configPath, tc.projectName)
			}
		})
	}
}

func Test_exampleMain(t *testing.T) {
	// Validate the example main.py content
	assert.Contains(t, exampleMain, "def run(prompt: str):")
	assert.Contains(t, exampleMain, "Running on Cerebrium")
	assert.Contains(t, exampleMain, "cerebrium run main.py::run")
	assert.Contains(t, exampleMain, "cerebrium deploy")
	assert.Contains(t, exampleMain, `return {"my_result": prompt}`)
}
