package files

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
