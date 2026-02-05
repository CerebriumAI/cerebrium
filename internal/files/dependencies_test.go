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

func TestParseDepLine(t *testing.T) {
	tcs := []struct {
		line    string
		wantPkg string
		wantVer string
	}{
		// bare package
		{"requests", "requests", ""},
		// pinned
		{"numpy==1.24.0", "numpy", "==1.24.0"},
		// range
		{"flask>=2.0", "flask", ">=2.0"},
		{"flask<=2.0", "flask", "<=2.0"},
		{"flask~=2.0", "flask", "~=2.0"},
		{"flask!=2.0", "flask", "!=2.0"},
		{"flask>2.0", "flask", ">2.0"},
		{"flask<3.0", "flask", "<3.0"},
		// extras
		{"uvicorn[standard]", "uvicorn[standard]", ""},
		{"uvicorn[standard]>=0.20", "uvicorn[standard]", ">=0.20"},
		{"package[extra1]==1.0.0", "package[extra1]", "==1.0.0"},
		// whitespace
		{"  requests  ", "requests", ""},
		{"numpy == 1.24.0", "numpy", "==1.24.0"},
		// git URL (no version specifier)
		{"git+https://github.com/org/repo.git", "git+https://github.com/org/repo.git", ""},
	}

	for _, tc := range tcs {
		t.Run(tc.line, func(t *testing.T) {
			pkg, ver := parseDepLine(tc.line)
			assert.Equal(t, tc.wantPkg, pkg)
			assert.Equal(t, tc.wantVer, ver)
		})
	}
}

func TestParseRequirementsContent(t *testing.T) {
	tcs := []struct {
		name    string
		content string
		want    map[string]string
	}{
		{
			name:    "mixed formats",
			content: "requests\nnumpy==1.24.0\nflask>=2.0\nuvicorn[standard]\n",
			want: map[string]string{
				"requests":          "latest",
				"numpy":             "==1.24.0",
				"flask":             ">=2.0",
				"uvicorn[standard]": "latest",
			},
		},
		{
			name:    "skips comments and blanks",
			content: "# this is a comment\nrequests\n\n# another comment\nflask\n",
			want: map[string]string{
				"requests": "latest",
				"flask":    "latest",
			},
		},
		{
			name:    "skips pip flags",
			content: "-r other.txt\n--index-url https://pypi.org\nrequests\n-e .\n",
			want: map[string]string{
				"requests": "latest",
			},
		},
		{
			name:    "empty content",
			content: "",
			want:    map[string]string{},
		},
		{
			name:    "deduplicates packages keeping last",
			content: "requests\nrequests==2.28.0\n",
			want: map[string]string{
				"requests": "==2.28.0",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseRequirementsContent(tc.content)
			assert.Equal(t, tc.want, got)
		})
	}
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
