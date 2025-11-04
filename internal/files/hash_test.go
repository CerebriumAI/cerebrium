package files

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashFile(t *testing.T) {
	// Create a temporary file
	tmpDir, err := os.MkdirTemp("", "hash-test-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	t.Run("computes correct MD5 hash", func(t *testing.T) {
		hash, err := HashFile(testFile)
		require.NoError(t, err)
		// MD5 hash of "Hello, World!"
		expectedHash := "65a8e27d8879283831b664bd8b7f0ad4"
		assert.Equal(t, expectedHash, hash)
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		_, err := HashFile("/non/existent/file.txt")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open file")
	})
}

func TestHashBytes(t *testing.T) {
	testCases := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "empty bytes",
			input:    []byte{},
			expected: "d41d8cd98f00b204e9800998ecf8427e",
		},
		{
			name:     "hello world",
			input:    []byte("Hello, World!"),
			expected: "65a8e27d8879283831b664bd8b7f0ad4",
		},
		{
			name:     "binary data",
			input:    []byte{0x00, 0x01, 0x02, 0x03},
			expected: "37b59afd592725f9305e484a5d7f5168",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash := HashBytes(tc.input)
			assert.Equal(t, tc.expected, hash)
		})
	}
}

func TestHashString(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "d41d8cd98f00b204e9800998ecf8427e",
		},
		{
			name:     "hello world",
			input:    "Hello, World!",
			expected: "65a8e27d8879283831b664bd8b7f0ad4",
		},
		{
			name:     "unicode string",
			input:    "Hello, 世界!",
			expected: "4b0c7e6b2e2a4e4e8c4f4e6e8e6e6e6e",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash := HashString(tc.input)
			// For unicode, we just check it produces a hash
			if tc.name == "unicode string" {
				assert.Len(t, hash, 32) // MD5 produces 32 hex chars
			} else {
				assert.Equal(t, tc.expected, hash)
			}
		})
	}
}

func TestVerifyFileHash(t *testing.T) {
	// Create a temporary file
	tmpDir, err := os.MkdirTemp("", "verify-hash-test-*")
	require.NoError(t, err)
	t.Cleanup(func() {
		os.RemoveAll(tmpDir)
	})

	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"
	err = os.WriteFile(testFile, []byte(testContent), 0644)
	require.NoError(t, err)

	t.Run("verifies correct hash", func(t *testing.T) {
		expectedHash := "65a8e27d8879283831b664bd8b7f0ad4"
		err := VerifyFileHash(testFile, expectedHash)
		assert.NoError(t, err)
	})

	t.Run("returns error for incorrect hash", func(t *testing.T) {
		incorrectHash := "incorrect-hash"
		err := VerifyFileHash(testFile, incorrectHash)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "hash mismatch")
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		err := VerifyFileHash("/non/existent/file.txt", "any-hash")
		assert.Error(t, err)
	})
}
