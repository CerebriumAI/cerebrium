# PR #1: File Manifest Functionality

## Branch: `feature/manifest`

## Overview
This PR adds the foundational manifest functionality needed for incremental uploads. It provides the ability to generate file manifests with MD5 hashes, compare manifests to detect changes, and handle ignore patterns.

## Files Added
- `internal/files/manifest.go` (184 lines)
- `internal/files/manifest_test.go` (154 lines)
- `internal/files/hash.go` (49 lines)
- `internal/files/hash_test.go` (145 lines)

**Total: 532 lines**

## Key Features

### 1. Manifest Generation
- `BuildManifest()` - Creates a manifest of all files in a directory
- Computes MD5 hashes (matches S3 ETag format)
- Records file paths and sizes
- Respects ignore patterns

### 2. Ignore Patterns
- `IgnoreMatcher` - Handles .cerebriumignore patterns
- Always ignores `.git/` and `.cerebrium/` directories
- Supports glob patterns and directory prefixes

### 3. Manifest Comparison
- `CompareManifests()` - Detects added, modified, and deleted files
- Efficient map-based comparison

### 4. Hash Utilities
- `HashFile()` - Computes MD5 hash of a file
- `HashBytes()` - Hashes byte data
- `HashString()` - Hashes strings
- `VerifyFileHash()` - Verifies file integrity

## Testing
All functions have comprehensive test coverage:
- ✅ Manifest generation with various file structures
- ✅ Ignore pattern matching
- ✅ Hash computation and verification
- ✅ Manifest comparison logic

## Usage Example
```go
// Generate manifest
manifest, err := BuildManifest("./project", []string{"node_modules", "*.pyc"})

// Compare with previous manifest
added, modified, deleted := CompareManifests(currentManifest, previousManifest)

// Verify file integrity
err := VerifyFileHash("file.txt", expectedHash)
```

## Why This PR?
This is the first step in implementing incremental uploads. The manifest functionality allows us to:
1. Track which files exist in a project
2. Detect what has changed between deployments
3. Upload only the changed files (in future PRs)

## Next Steps
- PR #2: API client changes to support incremental responses
- PR #3: Incremental upload logic using these manifests
- PR #4: Integration into the deploy command

## How to Test
```bash
# Run tests
go test ./internal/files -v

# All tests should pass
```

## Review Notes
- This PR is self-contained and doesn't break any existing functionality
- The MD5 hash format matches S3 ETags for compatibility
- Ignore patterns follow standard gitignore-like syntax
- All edge cases are tested (empty directories, missing files, etc.)
