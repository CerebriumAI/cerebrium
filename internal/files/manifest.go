package files

import (
	"crypto/md5" //nolint:gosec // MD5 is required for S3 ETag compatibility, not used for security
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileManifest represents a collection of files with their hashes
type FileManifest struct {
	Version string      `json:"version"`
	Files   []FileEntry `json:"files"`
}

// FileEntry represents a single file in the manifest
type FileEntry struct {
	Path string `json:"path"`
	Hash string `json:"hash"` // MD5 hash to match S3 ETags
	Size int64  `json:"size"`
}

// BuildManifest creates a manifest of all files in the directory
//
// Pattern matching behavior:
//   - Patterns ending with "/" match directories only from root (e.g., "build/" matches "build/file" but not "src/build/file")
//   - Wildcard patterns (e.g., "*.pyc") match anywhere in the tree
//   - This is more restrictive than .gitignore (which matches anywhere without leading slash)
//   - To match a directory anywhere, use a wildcard pattern like "*/build/"
func BuildManifest(rootDir string, ignorePatterns []string) (*FileManifest, error) {
	manifest := &FileManifest{
		Version: "1.0",
		Files:   []FileEntry{},
	}

	// Create ignore matcher
	matcher := newIgnoreMatcher(ignorePatterns)

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			shouldIgnore, err := matcher.shouldIgnore(relPath + "/")
			if err != nil {
				return fmt.Errorf("failed to check ignore pattern: %w", err)
			}
			if shouldIgnore {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip files that should be ignored
		shouldIgnore, err := matcher.shouldIgnore(relPath)
		if err != nil {
			return fmt.Errorf("failed to check ignore pattern: %w", err)
		}
		if shouldIgnore {
			return nil
		}

		// Compute MD5 hash
		hash, err := computeFileMD5(path)
		if err != nil {
			return fmt.Errorf("failed to hash %s: %w", relPath, err)
		}

		// Add to manifest
		manifest.Files = append(manifest.Files, FileEntry{
			Path: relPath,
			Hash: hash,
			Size: info.Size(),
		})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to build manifest: %w", err)
	}

	return manifest, nil
}

// computeFileMD5 computes the MD5 hash of a file and returns it as a hex-encoded string
// This format matches S3 ETag format for single-part uploads
func computeFileMD5(filepath string) (string, error) {
	file, err := os.Open(filepath) //nolint:gosec // Path is provided by trusted source (BuildManifest)
	if err != nil {
		return "", err
	}
	defer file.Close() //nolint:errcheck // Deferred close error is non-critical for read operation

	hash := md5.New() //nolint:gosec // MD5 required for S3 ETag compatibility
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CompareManifests compares two manifests and returns what changed
func CompareManifests(current, previous FileManifest) (added, modified, deleted []string) {
	// Create maps for efficient comparison
	currentFiles := make(map[string]FileEntry)
	for _, file := range current.Files {
		currentFiles[file.Path] = file
	}

	previousFiles := make(map[string]FileEntry)
	for _, file := range previous.Files {
		previousFiles[file.Path] = file
	}

	// Find added and modified files
	for path, currentFile := range currentFiles {
		previousFile, exists := previousFiles[path]
		if !exists {
			added = append(added, path)
		} else if currentFile.Hash != previousFile.Hash {
			modified = append(modified, path)
		}
	}

	// Find deleted files
	for path := range previousFiles {
		if _, exists := currentFiles[path]; !exists {
			deleted = append(deleted, path)
		}
	}

	return added, modified, deleted
}

// ignoreMatcher handles file ignore patterns
type ignoreMatcher struct {
	patterns []string
}

// newIgnoreMatcher creates a new ignore matcher
func newIgnoreMatcher(patterns []string) ignoreMatcher {
	return ignoreMatcher{
		patterns: patterns,
	}
}

// shouldIgnore checks if a path should be ignored
func (m ignoreMatcher) shouldIgnore(path string) (bool, error) {
	// Always ignore .git directory
	if strings.HasPrefix(path, ".git/") || path == ".git" {
		return true, nil
	}

	// Always ignore .cerebrium directory
	if strings.HasPrefix(path, ".cerebrium/") || path == ".cerebrium" {
		return true, nil
	}

	// Check against patterns
	for _, pattern := range m.patterns {
		// Simple pattern matching (can be enhanced with glob patterns)
		matched, err := filepath.Match(pattern, path)
		if err != nil {
			return false, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		if matched {
			return true, nil
		}

		// Check if pattern represents a directory that contains the path
		// Handle directory patterns (ending with /)
		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(path, pattern) {
				return true, nil
			}
		} else {
			// For non-directory patterns, only match exact path or as parent directory
			if path == pattern {
				return true, nil
			}
			if strings.HasPrefix(path, pattern+"/") {
				return true, nil
			}
		}

		// Check if any part of the path matches
		parts := strings.Split(path, string(filepath.Separator))
		for _, part := range parts {
			matched, err := filepath.Match(pattern, part)
			if err != nil {
				return false, fmt.Errorf("invalid pattern %q: %w", pattern, err)
			}
			if matched {
				return true, nil
			}
		}
	}

	return false, nil
}
