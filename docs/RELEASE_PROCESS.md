# Cerebrium CLI Release Process

## Overview

The Cerebrium CLI uses automated GitHub Actions for releases across multiple distribution channels:
- GitHub Releases (binaries)
- Homebrew (macOS)
- PyPI (Python pip)
- APT Repository (Debian/Ubuntu)

## Release Types

### 1. Beta/Pre-release (Recommended for testing)
Used for testing new features without affecting production users.

**How to create:**
```bash
# Create a beta tag
git tag v2.0.1-beta.1
git push origin v2.0.1-beta.1

# Or create via GitHub UI as a pre-release
```

**What happens:**
- ✅ GitHub Release created (marked as pre-release)
- ✅ Homebrew formula updated (users must explicitly upgrade)
- ✅ PyPI beta version published (e.g., `2.0.1b1`)
  - Users won't auto-upgrade
  - Must explicitly install: `pip install cerebrium==2.0.1b1`
- ✅ APT repository updated (manual update required)

### 2. Production Release
Full release for all users.

**How to create:**
```bash
# Create a release tag
git tag v2.0.1
git push origin v2.0.1

# Or create via GitHub UI (NOT marked as pre-release)
```

**What happens:**
- ✅ GitHub Release created
- ✅ Homebrew formula updated
- ✅ PyPI stable version published
  - ⚠️ Users running `pip install --upgrade` will auto-update
- ✅ APT repository updated

## Required GitHub Secrets

Add these at: https://github.com/CerebriumAI/cerebrium/settings/secrets/actions

### Already Configured:
- `GH_PAT` - GitHub Personal Access Token (for releases and Homebrew)
- `MACOS_CERTIFICATE_P12` - macOS signing certificate
- `MACOS_CERTIFICATE_PASSWORD` - Certificate password
- `MACOS_NOTARIZATION_ISSUER_ID` - Apple notarization issuer
- `MACOS_NOTARIZATION_KEY_ID` - Apple notarization key ID
- `MACOS_NOTARIZATION_KEY` - Apple notarization API key
- `BUGSNAG_API_KEY` - Error tracking

### Need to Add for PyPI:
1. **Test PyPI Token** (optional, for testing):
   - Go to https://test.pypi.org/manage/account/token/
   - Create token with scope "Entire account"
   - Add as `TEST_PYPI_API_TOKEN`

2. **Production PyPI Token** (required for pip releases):
   - Go to https://pypi.org/manage/account/token/
   - Create token with scope "Entire account" or "Project: cerebrium"
   - Add as `PYPI_API_TOKEN`
   - ⚠️ **Only add when ready for pip users to auto-update!**

## Step-by-Step Release Process

### For Beta Testing (v2.0.1-beta.1):

1. **Ensure main branch is up to date:**
   ```bash
   git checkout main
   git pull origin main
   ```

2. **Create and push beta tag:**
   ```bash
   git tag -a v2.0.1-beta.1 -m "Beta release for v2.0.1"
   git push origin v2.0.1-beta.1
   ```

3. **GitHub Actions will automatically:**
   - Build and sign binaries
   - Create GitHub pre-release
   - Update Homebrew formula
   - Publish to PyPI as `2.0.1b1` (if PYPI_API_TOKEN is set)

4. **Beta testers install with:**
   ```bash
   # Homebrew (macOS)
   brew upgrade cerebrium
   
   # Pip (Python) - must specify version
   pip install cerebrium==2.0.1b1
   
   # Direct download
   curl -L https://github.com/CerebriumAI/cerebrium/releases/download/v2.0.1-beta.1/cerebrium_darwin_arm64.tar.gz
   ```

### For Production Release (v2.0.1):

1. **After beta testing is complete:**
   ```bash
   git tag -a v2.0.1 -m "Release v2.0.1"
   git push origin v2.0.1
   ```

2. **Automatic distribution:**
   - All channels updated
   - Pip users will auto-upgrade on next `pip install --upgrade cerebrium`

## Testing Releases

### Local Testing Before Release:
```bash
# Test the release process locally
make release-dry-run

# Build without releasing
goreleaser build --clean --snapshot
```

### After Release:
```bash
# Test Homebrew
brew update && brew upgrade cerebrium
cerebrium version

# Test PyPI (beta)
pip install cerebrium==2.0.1b1
cerebrium version

# Test direct download
curl -L https://github.com/CerebriumAI/cerebrium/releases/latest/download/cerebrium_darwin_arm64.tar.gz | tar xz
./cerebrium version
```

## Rollback Process

If issues are found after release:

### For GitHub/Homebrew:
1. Delete the release tag:
   ```bash
   git push --delete origin v2.0.1
   git tag -d v2.0.1
   ```
2. Fix issues
3. Create new release with higher version

### For PyPI:
Cannot delete, but can yank (mark as broken):
```bash
# Install twine if needed
pip install twine

# Yank the broken version
twine yank cerebrium==2.0.1

# Release a fix as 2.0.2
```

## Version Numbering

- **Stable**: `v2.0.1`
- **Beta**: `v2.0.1-beta.1`, `v2.0.1-beta.2`
- **Release Candidate**: `v2.0.1-rc.1`
- **PyPI Beta**: Automatically converted to `2.0.1b1`, `2.0.1b2`

## Monitoring Releases

- **GitHub Actions**: https://github.com/CerebriumAI/cerebrium/actions
- **GitHub Releases**: https://github.com/CerebriumAI/cerebrium/releases
- **PyPI Project**: https://pypi.org/project/cerebrium/
- **Homebrew Formula**: https://github.com/CerebriumAI/homebrew-tap/blob/main/Formula/cerebrium.rb