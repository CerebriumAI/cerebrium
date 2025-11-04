package files

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildManifest(t *testing.T) {
	// Create a temporary directory with test files
	tmpDir, err := os.MkdirTemp("", "manifest-test-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	// Create test file structure
	testFiles := map[string]string{
		"main.py":                   "print('hello')",
		"requirements.txt":          "numpy==1.21.0",
		"data/model.pkl":            "model data",
		".git/config":               "git config",
		".cerebrium/cache":          "cache data",
		"node_modules/package.json": "package",
		"__pycache__/cache.pyc":     "cache",
	}

	for path, content := range testFiles {
		fullPath := filepath.Join(tmpDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	t.Run("builds manifest with ignore patterns", func(t *testing.T) {
		ignorePatterns := []string{
			"node_modules/",
			"__pycache__/",
			"*.pyc",
		}

		manifest, err := BuildManifest(tmpDir, ignorePatterns)
		require.NoError(t, err)
		assert.NotNil(t, manifest)
		assert.Equal(t, "1.0", manifest.Version)

		// Should only include main.py, requirements.txt, and data/model.pkl
		assert.Len(t, manifest.Files, 3)

		// Check that the right files are included
		fileMap := make(map[string]FileEntry)
		for _, file := range manifest.Files {
			fileMap[file.Path] = file
		}

		assert.Contains(t, fileMap, "main.py")
		assert.Contains(t, fileMap, "requirements.txt")
		assert.Contains(t, fileMap, filepath.Join("data", "model.pkl"))

		// Check that ignored files are not included
		assert.NotContains(t, fileMap, filepath.Join(".git", "config"))
		assert.NotContains(t, fileMap, filepath.Join(".cerebrium", "cache"))
		assert.NotContains(t, fileMap, filepath.Join("node_modules", "package.json"))
		assert.NotContains(t, fileMap, filepath.Join("__pycache__", "cache.pyc"))
	})

	t.Run("computes correct MD5 hashes", func(t *testing.T) {
		manifest, err := BuildManifest(tmpDir, []string{"node_modules", "__pycache__"})
		require.NoError(t, err)

		// Find main.py in manifest
		var mainPy *FileEntry
		for _, file := range manifest.Files {
			if file.Path == "main.py" {
				mainPy = &file
				break
			}
		}

		require.NotNil(t, mainPy)
		// MD5 hash of "print('hello')"
		expectedHash := "e73b48e8e00d36304ea7204a0683c814"
		assert.Equal(t, expectedHash, mainPy.Hash)
		assert.Equal(t, int64(14), mainPy.Size) // len("print('hello')")
	})

	t.Run("handles empty directory", func(t *testing.T) {
		emptyDir, err := os.MkdirTemp("", "empty-manifest-test-*")
		require.NoError(t, err)
		t.Cleanup(func() {
			os.RemoveAll(emptyDir)
		})

		manifest, err := BuildManifest(emptyDir, nil)
		require.NoError(t, err)
		assert.NotNil(t, manifest)
		assert.Len(t, manifest.Files, 0)
	})
}

func TestCompareManifests(t *testing.T) {
	current := FileManifest{
		Version: "1.0",
		Files: []FileEntry{
			{Path: "main.py", Hash: "abc123", Size: 100},
			{Path: "new.py", Hash: "def456", Size: 200},
			{Path: "modified.py", Hash: "ghi789", Size: 300},
		},
	}

	previous := FileManifest{
		Version: "1.0",
		Files: []FileEntry{
			{Path: "main.py", Hash: "abc123", Size: 100},
			{Path: "modified.py", Hash: "old789", Size: 250},
			{Path: "deleted.py", Hash: "xyz999", Size: 150},
		},
	}

	added, modified, deleted := CompareManifests(current, previous)

	assert.Equal(t, []string{"new.py"}, added)
	assert.Equal(t, []string{"modified.py"}, modified)
	assert.Equal(t, []string{"deleted.py"}, deleted)
}

func TestIgnoreMatcher(t *testing.T) {
	t.Run("valid patterns", func(t *testing.T) {
		patterns := []string{
			"*.pyc",
			"__pycache__",
			"node_modules/",
			"*.log",
			"temp/",
		}

		matcher := newIgnoreMatcher(patterns)

		testCases := []struct {
			path     string
			expected bool
		}{
			// Should ignore
			{".git/config", true},
			{".git/", true},
			{".cerebrium/cache", true},
			{"file.pyc", true},
			{"__pycache__/cache.pyc", true},
			{"node_modules/package.json", true},
			{"debug.log", true},
			{"temp/file.txt", true},

			// Should not ignore
			{"main.py", false},
			{"requirements.txt", false},
			{"data/model.pkl", false},
			{"src/app.js", false},
		}

		for _, tc := range testCases {
			t.Run(tc.path, func(t *testing.T) {
				result, err := matcher.shouldIgnore(tc.path)
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result, "path: %s", tc.path)
			})
		}
	})

	t.Run("invalid pattern", func(t *testing.T) {
		patterns := []string{
			"[invalid", // Invalid pattern - unclosed bracket
		}
		matcher := newIgnoreMatcher(patterns)

		_, err := matcher.shouldIgnore("test.txt")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pattern")
	})

	t.Run("no false prefix matches", func(t *testing.T) {
		patterns := []string{
			"a/b/my_file",
			"src/test",
		}
		matcher := newIgnoreMatcher(patterns)

		testCases := []struct {
			path     string
			expected bool
		}{
			// Should match - exact matches
			{"a/b/my_file", true},
			{"src/test", true},

			// Should NOT match - partial prefix matches
			{"a/b/my_file_hello", false},
			{"a/b/my_file.txt", false},
			{"src/test_utils", false},
			{"src/testing", false},

			// Should match - directory patterns
			{"src/test/file.go", true},     // This should match because test is a directory
			{"a/b/my_file/data.txt", true}, // This should match because my_file is a directory
		}

		for _, tc := range testCases {
			t.Run(tc.path, func(t *testing.T) {
				result, err := matcher.shouldIgnore(tc.path)
				require.NoError(t, err)
				assert.Equal(t, tc.expected, result, "path: %s", tc.path)
			})
		}
	})
}
