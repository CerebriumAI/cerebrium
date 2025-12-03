package files

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_matchesGlob(t *testing.T) {
	tcs := []struct {
		name     string
		path     string
		pattern  string
		expected bool
	}{
		{
			name:     "simple wildcard match",
			path:     "file.py",
			pattern:  "*.py",
			expected: true,
		},
		{
			name:     "simple wildcard no match",
			path:     "file.txt",
			pattern:  "*.py",
			expected: false,
		},
		{
			name:     "doublestar any subdirectory",
			path:     "src/utils/helper.py",
			pattern:  "**/*.py",
			expected: true,
		},
		{
			name:     "doublestar at start",
			path:     "deep/nested/file.py",
			pattern:  "**/*.py",
			expected: true,
		},
		{
			name:     "doublestar in middle",
			path:     "src/nested/utils/file.py",
			pattern:  "src/**/file.py",
			expected: true,
		},
		{
			name:     "doublestar in middle no match",
			path:     "other/nested/utils/file.py",
			pattern:  "src/**/file.py",
			expected: false,
		},
		{
			name:     "exact file match",
			path:     "requirements.txt",
			pattern:  "requirements.txt",
			expected: true,
		},
		{
			name:     "directory prefix",
			path:     "src/main.py",
			pattern:  "src/*.py",
			expected: true,
		},
		{
			name:     "nested directory no match",
			path:     "src/utils/main.py",
			pattern:  "src/*.py",
			expected: false,
		},
		{
			name:     "character class",
			path:     "test1.py",
			pattern:  "test[0-9].py",
			expected: true,
		},
		{
			name:     "question mark wildcard",
			path:     "test1.py",
			pattern:  "test?.py",
			expected: true,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			result := matchesGlob(tc.path, tc.pattern)
			assert.Equal(t, tc.expected, result,
				"matchesGlob(%q, %q) = %v, want %v", tc.path, tc.pattern, result, tc.expected)
		})
	}
}

func Test_normalizePatterns(t *testing.T) {
	tcs := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "directory with trailing slash becomes recursive",
			input:    []string{"assets/"},
			expected: []string{"assets/**"},
		},
		{
			name:     "multiple directories",
			input:    []string{"src/", "models/", "assets/"},
			expected: []string{"src/**", "models/**", "assets/**"},
		},
		{
			name:     "file patterns unchanged",
			input:    []string{"main.py", "cerebrium.toml"},
			expected: []string{"main.py", "cerebrium.toml"},
		},
		{
			name:     "mixed files and directories",
			input:    []string{"src/", "main.py", "assets/"},
			expected: []string{"src/**", "main.py", "assets/**"},
		},
		{
			name:     "removes leading ./",
			input:    []string{"./src/", "./main.py"},
			expected: []string{"src/**", "main.py"},
		},
		{
			name:     "glob patterns unchanged",
			input:    []string{"**/*.py", "*.txt"},
			expected: []string{"**/*.py", "*.txt"},
		},
		{
			name:     "empty patterns filtered",
			input:    []string{"src/", "", "  ", "main.py"},
			expected: []string{"src/**", "main.py"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizePatterns(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestDetermineIncludes_DirectoryWithSubdirectories(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	
	// Change to temp directory for the test
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()

	// Create directory structure like a real project
	dirs := []string{
		"assets/shaders/nested",
		"src/utils",
		"models/v1",
	}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
	}

	// Create files
	files := []string{
		"main.py",
		"cerebrium.toml",
		"assets/image.jpg",
		"assets/shaders/vertex.glsl",
		"assets/shaders/fragment.glsl",
		"assets/shaders/nested/compute.glsl",
		"src/main.py",
		"src/utils/helper.py",
		"models/model.pt",
		"models/v1/weights.bin",
	}
	for _, file := range files {
		err := os.WriteFile(file, []byte("test"), 0644)
		require.NoError(t, err)
	}

	// Test that directory patterns include subdirectories
	include := []string{"assets/", "src/", "models/", "main.py", "cerebrium.toml"}
	exclude := []string{}

	result, err := DetermineIncludes(include, exclude)
	require.NoError(t, err)

	// All files should be included
	expected := []string{
		"assets/image.jpg",
		"assets/shaders/fragment.glsl",
		"assets/shaders/nested/compute.glsl",
		"assets/shaders/vertex.glsl",
		"cerebrium.toml",
		"main.py",
		"models/model.pt",
		"models/v1/weights.bin",
		"src/main.py",
		"src/utils/helper.py",
	}

	assert.ElementsMatch(t, expected, result, 
		"Directory patterns should include all files in subdirectories")
}

func TestDetermineIncludes_ExcludeSubdirectory(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	
	// Change to temp directory for the test
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer func() { _ = os.Chdir(originalDir) }()

	// Create directory structure
	err = os.MkdirAll("assets/shaders", 0755)
	require.NoError(t, err)

	// Create files
	files := []string{
		"assets/image.jpg",
		"assets/shaders/vertex.glsl",
	}
	for _, file := range files {
		err := os.WriteFile(file, []byte("test"), 0644)
		require.NoError(t, err)
	}

	// Include assets but exclude shaders subdirectory
	include := []string{"assets/"}
	exclude := []string{"assets/shaders/"}

	result, err := DetermineIncludes(include, exclude)
	require.NoError(t, err)

	// Only the image should be included, not the shader
	expected := []string{"assets/image.jpg"}
	assert.ElementsMatch(t, expected, result,
		"Exclude patterns should work on subdirectories")
}
