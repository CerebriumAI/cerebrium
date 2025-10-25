package files

import (
	"os"
	"path/filepath"
	"strings"
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
				"requirements.txt": "numpy==1.24.0\nrequests>=2.28.0\nflask\n",
			},
		},
		{
			name: "generates apt requirements from inline dependencies",
			config: &projectconfig.ProjectConfig{
				Dependencies: projectconfig.DependenciesConfig{
					Apt: map[string]string{
						"git":  "",
						"curl": "latest",
					},
				},
			},
			expectedFiles: map[string]string{
				"pkglist.txt": "git\ncurl\n",
			},
		},
		{
			name: "generates conda requirements from inline dependencies",
			config: &projectconfig.ProjectConfig{
				Dependencies: projectconfig.DependenciesConfig{
					Conda: map[string]string{
						"pandas": "2.0.0",
					},
				},
			},
			expectedFiles: map[string]string{
				"conda_pkglist.txt": "pandas==2.0.0\n",
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

				// Since maps are unordered, check that all expected lines are present
				expectedLines := strings.Split(strings.TrimSpace(expectedContent), "\n")
				actualLines := strings.Split(strings.TrimSpace(actualContent), "\n")

				assert.Equal(t, len(expectedLines), len(actualLines), "number of lines should match for %s", expectedFile)

				// Check each expected line is in the actual content
				for _, expectedLine := range expectedLines {
					found := false
					for _, actualLine := range actualLines {
						if actualLine == expectedLine {
							found = true
							break
						}
					}
					assert.True(t, found, "expected line '%s' not found in %s", expectedLine, expectedFile)
				}
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
