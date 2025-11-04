# Incremental Upload Implementation Plan

## Overview
Breaking the incremental upload feature into 4 separate PRs for easier review.

## PR Breakdown

### PR #1: Manifest Functionality (Branch: `feature/manifest`)
**Files to create/modify:**
- `internal/files/manifest.go` - Core manifest generation logic
- `internal/files/manifest_test.go` - Unit tests for manifest
- `internal/files/hash.go` - File hashing utilities  
- `internal/files/hash_test.go` - Tests for hashing

**What it does:**
- Generates file manifests with MD5 hashes
- Handles .cerebriumignore patterns
- Provides utilities for comparing manifests

**Estimated LOC:** ~400 lines

---

### PR #2: API Client Changes (Branch: `feature/api-incremental`)
**Files to modify:**
- `internal/api/interface.go` - Add generic Request method
- `internal/api/client.go` - Implement Request method
- `internal/api/types.go` - Add incremental response types
- `internal/api/mock/client_gen.go` - Update mocks
- `internal/api/client_test.go` - Add tests for new methods

**What it does:**
- Adds generic Request method for custom API calls
- Adds types for incremental upload responses
- Updates mocks for testing

**Estimated LOC:** ~200 lines

---

### PR #3: Incremental Upload Module (Branch: `feature/incremental-upload`)
**Files to create:**
- `internal/ui/commands/incremental_upload.go` - Core incremental logic
- `internal/ui/commands/incremental_upload_test.go` - Tests

**What it does:**
- Handles file upload to presigned URLs
- Manages completion endpoint calls
- Provides progress tracking for uploads
- Implements retry logic and hash verification

**Estimated LOC:** ~600 lines

---

### PR #4: Deploy Integration (Branch: `feature/deploy-incremental`)
**Files to modify:**
- `internal/ui/commands/deploy.go` - Integrate incremental flow

**What it does:**
- Adds incremental upload path to deploy command
- Checks eligibility (zip > 100MB, files < 500)
- Falls back to traditional zip upload when needed
- Updates progress messages

**Estimated LOC:** ~200 lines

---

## Implementation Order

1. **Start with PR #1 (Manifest)**
   - Most independent piece
   - Can be tested in isolation
   - Foundation for other PRs

2. **Then PR #2 (API Client)**
   - Small, focused changes
   - Easy to review
   - Needed by PR #3

3. **Then PR #3 (Incremental Upload)**
   - Core logic, depends on #1 and #2
   - Can be tested independently

4. **Finally PR #4 (Deploy Integration)**
   - Ties everything together
   - Smallest change
   - Easy to test end-to-end

## Testing Strategy

Each PR includes its own tests:
- PR #1: Unit tests for manifest generation
- PR #2: Unit tests for API client
- PR #3: Unit tests for upload logic (with mocks)
- PR #4: Integration tests with feature flag

## Migration from cerebrium-cli-go

We have working code in `/Users/harris/Projects/cerebrium-cli-go` that we'll adapt:
- `internal/files/manifest.go` → PR #1
- `internal/api/client.go` changes → PR #2  
- `internal/ui/commands/incremental_upload.go` → PR #3
- `internal/ui/commands/deploy.go` changes → PR #4

## Notes for Review

- Each PR is self-contained and can be reviewed independently
- Total diff: ~1400 lines spread across 4 PRs (350 lines average)
- Each PR has clear purpose and tests
- Can be merged independently without breaking existing functionality
