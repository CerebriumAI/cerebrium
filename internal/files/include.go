package files

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// DetermineIncludes walks the directory and returns files matching include/exclude patterns
func DetermineIncludes(include, exclude []string) ([]string, error) {
	var fileList []string

	// Normalize patterns
	includePatterns := normalizePatterns(include)
	excludePatterns := normalizePatterns(exclude)

	// Walk the current directory
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip if it's a directory
		if info.IsDir() {
			return nil
		}

		// Normalize path for matching
		normalizedPath := filepath.ToSlash(path)

		// Check if file matches include patterns
		included := false
		for _, pattern := range includePatterns {
			matched, err := filepath.Match(pattern, normalizedPath)
			if err != nil {
				// Try glob matching as fallback
				matched, _ = filepath.Match(pattern, filepath.Base(path))
			}
			if matched || matchesGlob(normalizedPath, pattern) {
				included = true
				break
			}
		}

		// Check if file matches exclude patterns
		excluded := false
		if included {
			for _, pattern := range excludePatterns {
				matched, err := filepath.Match(pattern, normalizedPath)
				if err != nil {
					matched, _ = filepath.Match(pattern, filepath.Base(path))
				}
				if matched || matchesGlob(normalizedPath, pattern) {
					excluded = true
					break
				}
			}
		}

		// Add file if included and not excluded
		if included && !excluded {
			fileList = append(fileList, normalizedPath)
		}

		return nil
	})

	return fileList, err
}

// normalizePatterns cleans up the patterns for consistent matching
func normalizePatterns(patterns []string) []string {
	var normalized []string
	for _, pattern := range patterns {
		p := strings.TrimSpace(pattern)
		if p == "" {
			continue
		}

		// Convert to forward slashes for consistency
		p = filepath.ToSlash(p)

		// Handle "./*" and "*" as "include everything recursively"
		if p == "./*" || p == "*" {
			p = "**/*"
		} else {
			// Remove leading "./"
			p = strings.TrimPrefix(p, "./")

			// If pattern ends with /, append ** for recursive directory matching
			// e.g., "assets/" becomes "assets/**" to include all files in subdirectories
			if strings.HasSuffix(p, "/") {
				p = p + "**"
			}
		}

		normalized = append(normalized, p)
	}
	return normalized
}

// matchesGlob checks if path matches a glob-style pattern (e.g., **/*.py)
func matchesGlob(path, pattern string) bool {
	// Use doublestar for proper glob matching with ** support
	// This handles all edge cases including:
	// - **/*.py (matches any .py file in any subdirectory)
	// - foo/**/bar (matches foo/bar, foo/x/bar, foo/x/y/bar, etc.)
	// - *.{py,txt} (brace expansion)
	// - [abc].txt (character classes)
	matched, err := doublestar.Match(pattern, path)
	if err != nil {
		// If pattern is invalid, don't match
		return false
	}
	return matched
}

// DetectDevFolders checks if any development folders are included in the file list
func DetectDevFolders(fileList []string) []string {
	devFolders := []string{"venv", "virtualenv", ".venv", ".git", "node_modules", "__pycache__"}
	detected := make(map[string]bool)

	for _, file := range fileList {
		parts := strings.Split(filepath.ToSlash(file), "/")
		if len(parts) > 0 {
			rootFolder := parts[0]
			for _, devFolder := range devFolders {
				if rootFolder == devFolder {
					detected[devFolder] = true
				}
			}
		}
	}

	var result []string
	for folder := range detected {
		result = append(result, folder)
	}
	return result
}
