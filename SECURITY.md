# Security Features

This document describes the security measures implemented in the Cerebrium CLI.

## Python Wrapper Security

### Checksum Verification

The Python wrapper (`pip install cerebrium`) implements **mandatory checksum verification** to prevent supply chain attacks.

**How it works:**

1. **Download checksums.txt** from GitHub Release
   ```
   https://github.com/CerebriumAI/cerebrium/releases/download/v2.0.0/checksums.txt
   ```

2. **Download binary archive**
   ```
   https://github.com/.../cerebrium_2.0.0_darwin_amd64.tar.gz
   ```

3. **Calculate SHA256 hash** of downloaded archive

4. **Verify against checksums.txt**
   - Parse checksums.txt to find expected hash
   - Compare with calculated hash
   - **Abort installation if mismatch**

5. **Only then extract** the binary

**Protection against:**
- ✅ Man-in-the-Middle (MITM) attacks
- ✅ Corrupted downloads
- ✅ Tampered releases
- ✅ CDN cache poisoning

**Error handling:**

```python
# If checksums.txt is missing
raise RuntimeError("Failed to download checksums - cannot verify integrity")

# If archive not found in checksums.txt
raise RuntimeError("Checksum not found - may indicate compromised release")

# If checksum mismatch
raise RuntimeError(
    f"Checksum verification failed!\n"
    f"Expected: abc123...\n"
    f"Got:      def456...\n"
    f"This may indicate a corrupted download or security issue."
)
```

**Code location:** `python-wrapper/setup.py:73-105`

### Binary Extraction Safety

**Secure extraction:**
- Searches for binary within archive (handles subdirectories)
- Verifies binary exists after extraction
- Sets proper permissions (Unix: executable)
- Fails safe if extraction incomplete

**Code location:** `python-wrapper/setup.py:149-177`

## Release Security

### GoReleaser Checksums

GoReleaser automatically generates `checksums.txt` for all release artifacts:

```
# checksums.txt format
abc123...  cerebrium_2.0.0_darwin_amd64.tar.gz
def456...  cerebrium_2.0.0_linux_amd64.tar.gz
...
```

**Included in every release:**
- SHA256 checksums for all binaries
- Uploaded to GitHub Releases
- Used by Python wrapper for verification
- Available for manual verification

**Manual verification:**

```bash
# Download binary and checksums
curl -LO https://github.com/.../cerebrium_2.0.0_darwin_amd64.tar.gz
curl -LO https://github.com/.../checksums.txt

# Verify checksum (macOS/Linux)
shasum -a 256 -c checksums.txt --ignore-missing

# Or manually
shasum -a 256 cerebrium_2.0.0_darwin_amd64.tar.gz
# Compare output with checksums.txt
```

## Supply Chain Security

### Dependency Management

- All Go dependencies pinned in `go.mod` with checksums in `go.sum`
- Python wrapper has minimal dependencies (setuptools only)
- No runtime dependencies for the Go binary

### Build Reproducibility

Version information injected at build time:
```go
// internal/version/version.go
Version = "2.0.0"      // From git tag
Commit = "abc123..."   // Git commit hash
BuildDate = "2025-..."  // Build timestamp
```

Users can verify build authenticity:
```bash
cerebrium version
# Output: cerebrium 2.0.0 (commit: abc123, built: 2025-10-10)
```

## Token Security

### OAuth Token Storage

- Tokens stored in `~/.cerebrium/config.yaml`
- File permissions: `0644` (user read/write only)
- Tokens not logged or printed to stdout/stderr
- In `config list`, tokens are truncated: `eyJraWQi...` (first 20 chars only)

### Service Account Tokens

- Supported via `CEREBRIUM_SERVICE_ACCOUNT` environment variable
- Checked before OAuth tokens (auth/service_account.go)
- Not persisted to disk

## Future Security Enhancements

1. **Code signing** for binaries (macOS: codesign, Windows: signtool)
2. **SLSA provenance** attestations
3. **Dependabot** for automated dependency updates
4. **Security scanning** in CI/CD (gosec, trivy)
5. **Token encryption** in config file (currently plaintext)

## Reporting Security Issues

Please report security vulnerabilities to: security@cerebrium.ai

Do NOT open public GitHub issues for security vulnerabilities.
