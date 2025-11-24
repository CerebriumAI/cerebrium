# Release Process

This document describes how to create and publish releases for the Cerebrium CLI.

## Versioning Strategy

We use **Semantic Versioning** (SemVer): `MAJOR.MINOR.PATCH`

- **MAJOR**: Breaking changes (e.g., 1.x.x → 2.0.0)
- **MINOR**: New features, backward compatible (e.g., 1.1.0 → 1.2.0)
- **PATCH**: Bug fixes, backward compatible (e.g., 1.1.0 → 1.1.1)

### Current Version
For the Go CLI migration:
- Start at `v2.0.0` (major version bump for the rewrite)

## How to Create a Release

### Step 1: Tag the Release

```bash
# Create and push a tag
git tag -a v2.1.0 -m "Release v2.1.0"
git push origin v2.1.0
```

**Tag format:** Always use `v` prefix followed by semantic version (e.g., `v2.0.0`)

### Step 2: Automated Workflows

Once you push the tag, the following happens automatically:

1. **release.yml**: GoReleaser builds and publishes:
   - Binaries for all platforms (macOS, Linux, Windows)
   - Archives (tar.gz, zip)
   - Checksums
   - Updates Homebrew tap
   - Creates Debian/RPM packages
   - Creates GitHub release with changelog

2. **pypi-publish.yml**: Python wrapper publishing:
   - Builds the Python package
   - Publishes to PyPI (for `pip install cerebrium`)
   - Handles beta/RC versions appropriately

## What Gets Released

### GoReleaser Produces:
- **Binaries**: macOS (amd64/arm64), Linux (amd64/arm64), Windows (amd64/arm64)
- **Archives**: `.tar.gz` for Unix, `.zip` for Windows
- **Checksums**: `checksums.txt` for verification
- **Homebrew Formula**: Auto-updated in `cerebriumai/homebrew-tap`
- **Debian Package**: `.deb` for Ubuntu/Debian
- **RPM Package**: `.rpm` for RedHat/Fedora/CentOS
- **GitHub Release**: With auto-generated changelog from commit messages

### Python Wrapper Provides:
- **PyPI Package**: `pip install cerebrium` downloads the Go binary
- **Backward Compatibility**: Existing Python CLI users can continue using pip

## Version Synchronization

The Go CLI version is determined by the git tag at build time:
- GoReleaser injects the version via ldflags during build
- Python wrapper reads version from the downloaded binary

## Verify the Release

After creating a release, test installation on different platforms:

```bash
# Homebrew (macOS/Linux)
brew update
brew upgrade cerebrium
cerebrium version

# Pip (all platforms)
pip install --upgrade cerebrium
cerebrium version

# Direct download (Linux/macOS)
curl -fsSL https://github.com/CerebriumAI/cerebrium/releases/latest/download/install.sh | sh
cerebrium version
```

## Pre-release Versions

For beta or release candidate versions:

```bash
# Beta release
git tag -a v2.1.0-beta.1 -m "Release v2.1.0-beta.1"
git push origin v2.1.0-beta.1

# Release candidate
git tag -a v2.1.0-rc.1 -m "Release v2.1.0-rc.1"
git push origin v2.1.0-rc.1
```

These will:
- Create a GitHub pre-release
- Not update the Homebrew formula (stable releases only)
- Be available on PyPI with appropriate version specifier

## Local Testing

Before creating a release, you can test locally:

```bash
# Test GoReleaser configuration
make release-dry

# Build locally with specific version
make build VERSION=2.1.0
./bin/cerebrium version
```

## Required Secrets

The following secrets must be configured in GitHub repository settings:

- **GH_PAT**: GitHub Personal Access Token with repo scope (for releases and Homebrew tap updates)
- **PYPI_API_TOKEN**: PyPI API token for publishing Python packages

## 🔔 Update Notifications

The CLI automatically checks for updates once per day (cached in `~/.cerebrium/version_cache.json`).

**How it works:**
1. On every command (except `version`), the CLI checks GitHub API for latest release
2. Compares current version with latest release
3. Shows update notification if newer version exists
4. Caches result for 24 hours to avoid API rate limits

**Example notification:**
```
⚠️  A new version of Cerebrium CLI is available: v2.0.1 (you have v2.0.0)
Update with:
  • Homebrew: brew upgrade cerebrium
  • Pip: pip install --upgrade cerebrium
  • Download: https://github.com/CerebriumAI/cerebrium/releases/latest
```

## Quick Reference

```bash
# Check current version
cerebrium version

# Create a patch release (bug fixes)
git tag -a v2.0.1 -m "Release v2.0.1: Fix authentication bug"
git push origin v2.0.1

# Create a minor release (new features)
git tag -a v2.1.0 -m "Release v2.1.0: Add support for custom regions"
git push origin v2.1.0

# Create a major release (breaking changes)
git tag -a v3.0.0 -m "Release v3.0.0: New configuration format"
git push origin v3.0.0

# Delete a tag if needed
git tag -d v2.0.1
git push origin --delete v2.0.1
```

## Troubleshooting

### Release workflow fails
- Check GitHub Actions logs for specific error
- Ensure all secrets are configured correctly
- Verify `.goreleaser.yaml` configuration is valid

### PyPI publish fails
- Ensure version doesn't already exist on PyPI
- Check PyPI API token is valid
- Verify `python-wrapper/setup.py` is correctly formatted

### Homebrew formula not updating
- Only stable releases update Homebrew (not pre-releases)
- Check GH_PAT has write access to tap repository
- Verify tap repository exists at `cerebriumai/homebrew-tap`