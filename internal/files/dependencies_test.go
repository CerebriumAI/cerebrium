package files

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cerebriumai/cerebrium/pkg/projectconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateDependencyFiles(t *testing.T) {
	tcs := []struct {
		name          string
		config        *projectconfig.ProjectConfig
		expectedFiles map[string]string
		expectedError string
	}{
		{
			name: "generates pip requirements from inline dependencies",
			config: &projectconfig.ProjectConfig{
				Dependencies: projectconfig.DependenciesConfig{
					Pip: map[string]string{
						"numpy":    "1.24.0",
						"requests": ">=2.28.0",
						"flask":    "latest",
					},
				},
			},
			expectedFiles: map[string]string{
				"requirements.txt": "flask\nnumpy==1.24.0\nrequests>=2.28.0\n",
			},
		},
		{
			name: "generates apt requirements from inline dependencies",
			config: &projectconfig.ProjectConfig{
				Dependencies: projectconfig.DependenciesConfig{
					Apt: map[string]string{
						"git":    "",
						"curl":   "latest",
						"wget":   "*",
						"vim":    "",
						"ffmpeg": "latest",
					},
				},
			},
			expectedFiles: map[string]string{
				"pkglist.txt": "curl\nffmpeg\ngit\nvim\nwget\n",
			},
		},
		{
			name: "generates conda requirements from inline dependencies",
			config: &projectconfig.ProjectConfig{
				Dependencies: projectconfig.DependenciesConfig{
					Conda: map[string]string{
						"pandas":     "2.0.0",
						"numpy":      "1.24.0",
						"matplotlib": ">=3.7.0",
						"scipy":      "latest",
						"jupyterlab": "*",
					},
				},
			},
			expectedFiles: map[string]string{
				"conda_pkglist.txt": "jupyterlab\nmatplotlib>=3.7.0\nnumpy==1.24.0\npandas==2.0.0\nscipy\n",
			},
		},
		{
			name: "generates pip requirements with git URLs",
			config: &projectconfig.ProjectConfig{
				Dependencies: projectconfig.DependenciesConfig{
					Pip: map[string]string{
						"numpy": "1.24.0",
						"git+https://github.com/huggingface/accelerate.git": "latest",
						"flask": "2.0.0",
						"git+https://github.com/openai/whisper.git":                   "",
						"git+https://github.com/facebookresearch/detectron2.git@v0.6": "*",
						"requests": ">=2.28.0",
					},
				},
			},
			expectedFiles: map[string]string{
				"requirements.txt": "flask==2.0.0\ngit+https://github.com/facebookresearch/detectron2.git@v0.6\ngit+https://github.com/huggingface/accelerate.git\ngit+https://github.com/openai/whisper.git\nnumpy==1.24.0\nrequests>=2.28.0\n",
			},
		},
		{
			name: "no dependencies generates no files",
			config: &projectconfig.ProjectConfig{
				Dependencies: projectconfig.DependenciesConfig{},
			},
			expectedFiles: make(map[string]string),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			files, err := GenerateDependencyFiles(tc.config)

			if tc.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, len(tc.expectedFiles), len(files))

			for expectedFile, expectedContent := range tc.expectedFiles {
				actualContent, ok := files[expectedFile]
				assert.True(t, ok, "expected file %s to be generated", expectedFile)

				// With sorted output, we can now check for exact matches
				assert.Equal(t, expectedContent, actualContent, "content mismatch for %s", expectedFile)
			}
		})
	}
}

func TestGenerateDependencyFiles_WithFilePaths(t *testing.T) {
	// Create a temp directory with a requirements.txt file
	tmpDir := t.TempDir()
	requirementsPath := filepath.Join(tmpDir, "requirements.txt")
	requirementsContent := "django==4.2.0\ncelery>=5.0.0\n"
	err := os.WriteFile(requirementsPath, []byte(requirementsContent), 0644)
	require.NoError(t, err)

	config := &projectconfig.ProjectConfig{
		Dependencies: projectconfig.DependenciesConfig{
			Paths: projectconfig.DependencyPathsConfig{
				Pip: requirementsPath,
			},
		},
	}

	files, err := GenerateDependencyFiles(config)
	require.NoError(t, err)

	assert.Len(t, files, 1)
	assert.Equal(t, requirementsContent, files["requirements.txt"])
}

func TestGenerateDependencyFiles_FileNotFound(t *testing.T) {
	config := &projectconfig.ProjectConfig{
		Dependencies: projectconfig.DependenciesConfig{
			Paths: projectconfig.DependencyPathsConfig{
				Pip: "/nonexistent/requirements.txt",
			},
		},
	}

	_, err := GenerateDependencyFiles(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "was not found")
}

func TestGenerateDependencyFiles_BothInlineAndFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	requirementsPath := filepath.Join(tmpDir, "requirements.txt")
	requirementsContent := "django==4.2.0\n"
	err := os.WriteFile(requirementsPath, []byte(requirementsContent), 0644)
	require.NoError(t, err)

	config := &projectconfig.ProjectConfig{
		Dependencies: projectconfig.DependenciesConfig{
			Pip: map[string]string{
				"numpy": "1.24.0",
			},
			Paths: projectconfig.DependencyPathsConfig{
				Pip: requirementsPath,
			},
		},
	}

	_, err = GenerateDependencyFiles(config)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "both")
	assert.Contains(t, err.Error(), "please specify only one")
}

func TestGenerateDependencyFiles_WithShellCommands(t *testing.T) {
	config := &projectconfig.ProjectConfig{
		Dependencies: projectconfig.DependenciesConfig{},
		Deployment: projectconfig.DeploymentConfig{
			ShellCommands: []string{
				"echo 'Hello'",
				"pip install custom-package",
			},
			PreBuildCommands: []string{
				"apt-get update",
			},
		},
	}

	files, err := GenerateDependencyFiles(config)
	require.NoError(t, err)

	assert.Len(t, files, 2)
	assert.Contains(t, files, "shell_commands.sh")
	assert.Contains(t, files, "pre_build_commands.sh")

	shellContent := files["shell_commands.sh"]
	assert.Contains(t, shellContent, "#!/bin/bash")
	assert.Contains(t, shellContent, "set -e")
	assert.Contains(t, shellContent, "echo 'Hello'")
	assert.Contains(t, shellContent, "pip install custom-package")

	preBuildContent := files["pre_build_commands.sh"]
	assert.Contains(t, preBuildContent, "#!/bin/bash")
	assert.Contains(t, preBuildContent, "set -e")
	assert.Contains(t, preBuildContent, "apt-get update")
}

func TestGenerateDependencyFiles_WithRuntimeDependencies(t *testing.T) {
	config := &projectconfig.ProjectConfig{
		Dependencies: projectconfig.DependenciesConfig{},
		Runtime: &projectconfig.RuntimeConfig{
			Type: "cortex",
			Params: map[string]any{
				"python_version": "3.12",
				"deps": map[string]any{
					"pip": map[string]any{
						"torch": "2.0.0",
						"numpy": "latest",
					},
					"apt": map[string]any{
						"ffmpeg": "",
					},
				},
			},
		},
	}

	files, err := GenerateDependencyFiles(config)
	require.NoError(t, err)

	assert.Len(t, files, 2)
	assert.Equal(t, "numpy\ntorch==2.0.0\n", files["requirements.txt"])
	assert.Equal(t, "ffmpeg\n", files["pkglist.txt"])
}

func TestGenerateDependencyFiles_MergeTopLevelAndRuntimeDependencies(t *testing.T) {
	config := &projectconfig.ProjectConfig{
		Dependencies: projectconfig.DependenciesConfig{
			Pip: map[string]string{
				"requests": "2.28.0",
				"numpy":    "1.23.0", // Will be overridden by runtime
			},
			Apt: map[string]string{
				"git": "",
			},
		},
		Runtime: &projectconfig.RuntimeConfig{
			Type: "cortex",
			Params: map[string]any{
				"python_version": "3.12",
				"deps": map[string]any{
					"pip": map[string]any{
						"torch": "2.0.0",
						"numpy": "1.24.0", // Overrides top-level
					},
					"apt": map[string]any{
						"ffmpeg": "",
					},
				},
			},
		},
	}

	files, err := GenerateDependencyFiles(config)
	require.NoError(t, err)

	// Pip should have merged deps with runtime winning
	assert.Contains(t, files["requirements.txt"], "numpy==1.24.0") // Runtime wins
	assert.Contains(t, files["requirements.txt"], "torch==2.0.0")
	assert.Contains(t, files["requirements.txt"], "requests==2.28.0")

	// Apt should have merged deps
	assert.Contains(t, files["pkglist.txt"], "ffmpeg")
	assert.Contains(t, files["pkglist.txt"], "git")
}

func TestGenerateDependencyFiles_RuntimeShellCommands(t *testing.T) {
	config := &projectconfig.ProjectConfig{
		Dependencies: projectconfig.DependenciesConfig{},
		Runtime: &projectconfig.RuntimeConfig{
			Type: "cortex",
			Params: map[string]any{
				"shell_commands":     []any{"echo 'from runtime'"},
				"pre_build_commands": []any{"apt-get update"},
			},
		},
	}

	files, err := GenerateDependencyFiles(config)
	require.NoError(t, err)

	assert.Len(t, files, 2)
	assert.Contains(t, files["shell_commands.sh"], "echo 'from runtime'")
	assert.Contains(t, files["pre_build_commands.sh"], "apt-get update")
}

func TestGenerateDependencyFiles_RuntimeShellCommandsOverrideDeployment(t *testing.T) {
	config := &projectconfig.ProjectConfig{
		Dependencies: projectconfig.DependenciesConfig{},
		Deployment: projectconfig.DeploymentConfig{
			ShellCommands: []string{"echo 'from deployment'"},
		},
		Runtime: &projectconfig.RuntimeConfig{
			Type: "cortex",
			Params: map[string]any{
				"shell_commands": []any{"echo 'from runtime'"},
			},
		},
	}

	files, err := GenerateDependencyFiles(config)
	require.NoError(t, err)

	// Runtime shell commands should override deprecated deployment shell commands
	assert.Contains(t, files["shell_commands.sh"], "echo 'from runtime'")
	assert.NotContains(t, files["shell_commands.sh"], "echo 'from deployment'")
}
