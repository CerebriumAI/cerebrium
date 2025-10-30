package files

import (
	"crypto/md5"
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
func BuildManifest(rootDir string, ignorePatterns []string) (*FileManifest, error) {
	manifest := &FileManifest{
		Version: "1.0",
		Files:   []FileEntry{},
	}

	// Create ignore matcher
	ignoreMatcher := NewIgnoreMatcher(ignorePatterns)

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			// Check if directory should be ignored
			relPath, err := filepath.Rel(rootDir, path)
			if err != nil {
				return err
			}
			if ignoreMatcher.ShouldIgnore(relPath + "/") {
				return filepath.SkipDir
			}
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}

		// Check if file should be ignored
		if ignoreMatcher.ShouldIgnore(relPath) {
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

// computeFileMD5 computes the MD5 hash of a file
func computeFileMD5(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CompareManifests compares two manifests and returns what changed
func CompareManifests(current, previous *FileManifest) (added, modified, deleted []string) {
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

// IgnoreMatcher handles file ignore patterns
type IgnoreMatcher struct {
	patterns []string
}

// NewIgnoreMatcher creates a new ignore matcher
func NewIgnoreMatcher(patterns []string) *IgnoreMatcher {
	return &IgnoreMatcher{
		patterns: patterns,
	}
}

// ShouldIgnore checks if a path should be ignored
func (m *IgnoreMatcher) ShouldIgnore(path string) bool {
	// Always ignore .git directory
	if strings.HasPrefix(path, ".git/") || path == ".git" {
		return true
	}

	// Always ignore .cerebrium directory
	if strings.HasPrefix(path, ".cerebrium/") || path == ".cerebrium" {
		return true
	}

	// Check against patterns
	for _, pattern := range m.patterns {
		// Simple pattern matching (can be enhanced with glob patterns)
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}

		// Check if path starts with pattern (for directories)
		if strings.HasPrefix(path, pattern) {
			return true
		}

		// Check if any part of the path matches
		parts := strings.Split(path, string(filepath.Separator))
		for _, part := range parts {
			if matched, _ := filepath.Match(pattern, part); matched {
				return true
			}
		}
	}

	return false
}
