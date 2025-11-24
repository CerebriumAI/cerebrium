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

## How Releases Work

### Automated Semantic Versioning (Default)

The project uses [semantic-release](https://github.com/semantic-release/semantic-release) to automatically version and release based on commit messages.

**How it works:**
1. Push commits to `main` or `master` with conventional commit messages
2. The `semantic-release.yml` workflow analyzes commits and determines version bump:
   - `feat:` commits → Minor version (2.0.0 → 2.1.0)
   - `fix:` commits → Patch version (2.0.0 → 2.0.1)
   - `feat!:` or `BREAKING CHANGE:` → Major version (2.0.0 → 3.0.0)
3. Automatically creates git tag, GitHub release, and triggers publishing

**Example commits:**
```bash
git commit -m "feat: add support for custom regions"  # → 2.1.0
git commit -m "fix: resolve authentication error"      # → 2.0.1
git commit -m "feat!: redesign configuration format"   # → 3.0.0
```

### Manual Release (Alternative)

If you need to create a release manually:

```bash
# Create and push a tag
git tag -a v2.1.0 -m "Release v2.1.0"
git push origin v2.1.0
```

This will trigger the automated release pipeline.

## What Happens During a Release

When a new tag is created (either by semantic-release or manually), the following workflows run automatically:

1. **release.yml**: Runs GoReleaser which:
   - Builds binaries for all platforms (macOS, Linux, Windows)
   - Creates archives (tar.gz, zip)
   - Generates checksums
   - Updates Homebrew tap
   - Creates Debian/RPM packages

2. **pypi-publish.yml**: Publishes to PyPI:
   - Builds the Python wrapper package
   - Publishes to PyPI (pip install cerebrium)
   - Handles beta versions for prereleases

### 5. Verify the Release

Test installation on different platforms:

```bash
# Homebrew (macOS/Linux)
brew install cerebriumai/tap/cerebrium
cerebrium version

# Pip (all platforms)
pip install --upgrade cerebrium
cerebrium version

# Direct download (Linux/macOS)
curl -fsSL https://github.com/CerebriumAI/cerebrium/releases/latest/download/install.sh | sh
```

## What Gets Released

### GoReleaser Produces:
- **Binaries**: macOS (amd64/arm64), Linux (amd64/arm64), Windows (amd64/arm64)
- **Archives**: `.tar.gz` for Unix, `.zip` for Windows
- **Checksums**: `checksums.txt` for verification
- **Homebrew Formula**: Auto-updated in `cerebriumai/homebrew-tap`
- **Debian Package**: `.deb` for Ubuntu/Debian
- **RPM Package**: `.rpm` for RedHat/Fedora/CentOS
- **GitHub Release**: With auto-generated changelog

### Python Wrapper Provides:
- **PyPI Package**: `pip install cerebrium` downloads the Go binary
- **Backward Compatibility**: Existing users can continue using `pip install cerebrium`

## Version Synchronization

The Go CLI and Python wrapper versions should always match:
- Go CLI: `internal/version/version.go` (injected at build time via git tag)
- Python wrapper: `python-wrapper/setup.py` VERSION constant

## First-Time Setup

### GoReleaser
```bash
# Install GoReleaser
brew install goreleaser

# Or on Linux
go install github.com/goreleaser/goreleaser@latest
```

### Homebrew Tap
Create a GitHub repository: `github.com/CerebriumAI/homebrew-tap`

### GitHub Token
Create a GitHub Personal Access Token with `repo` scope:
```bash
export GITHUB_TOKEN="ghp_xxxxxxxxxxxxx"
```

### PyPI Credentials
Create a PyPI account and API token:
```bash
# Store in ~/.pypirc
[pypi]
username = __token__
password = pypi-xxxxxxxxxxxxx
```

## Automated Releases (Future)

When moving to the new repository, set up GitHub Actions to automate this:

```yaml
# .github/workflows/release.yml
on:
  push:
    tags:
      - 'v*'

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v4
      - uses: goreleaser/goreleaser-action@v5
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

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
# Check version
./bin/cerebrium version

# Create release
git tag -a v1.49.0 -m "Release v1.49.0"
git push origin v1.49.0
goreleaser release --clean

# Test release locally
make release-dry

# Build with custom version
make build VERSION=1.49.0
./bin/cerebrium version
```
