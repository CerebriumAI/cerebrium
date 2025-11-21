# Release Workflow

## Overview

The release process is fully automated using semantic versioning and conventional commits.

## Flow Diagram

```
1. Developer pushes to main/master
   └─> feat: new feature
   └─> fix: bug fix
   └─> BREAKING CHANGE: major change

2. semantic-release.yml workflow triggers
   └─> Analyzes commits
   └─> Determines version (major.minor.patch)
   └─> Updates VERSION file
   └─> Updates CHANGELOG.md
   └─> Creates git tag (v2.1.0)
   └─> Creates GitHub Release

3. GitHub Release created
   ├─> Triggers release.yml (on tag push)
   │   └─> Runs GoReleaser
   │       ├─> Builds binaries for all platforms
   │       ├─> Signs macOS binaries (if certs available)
   │       ├─> Creates .tar.gz, .zip, .deb, .rpm packages
   │       └─> Updates Homebrew tap (CerebriumAI/homebrew-tap)
   │
   └─> Triggers pypi-publish.yml (on release published)
       └─> Builds Python wrapper
       └─> Publishes to PyPI
       └─> Tests installation

4. End result:
   ✅ New version tagged in git
   ✅ GitHub release with changelog
   ✅ Binaries attached to release
   ✅ Homebrew updated (brew upgrade cerebrium)
   ✅ PyPI updated (pip install --upgrade cerebrium)
```

## Current Configuration Status

### ✅ Working Components

1. **Semantic Release**
   - Configured in `.releaserc.yml`
   - Analyzes commits and creates releases
   - Updates VERSION and CHANGELOG.md
   - Creates GitHub releases with proper tags

2. **GoReleaser (Homebrew)**
   - Configured in `.goreleaser.yaml`
   - Builds and signs binaries
   - Updates `homebrew_casks` in CerebriumAI/homebrew-tap
   - Triggered by tag pushes (v*)

3. **PyPI Publishing**
   - Configured in `.github/workflows/pypi-publish.yml`
   - Triggered by GitHub release events
   - Handles beta/prerelease versions
   - Updates python-wrapper package

### 🔑 Required Secrets

All these secrets must be set in GitHub repository settings:

**For Semantic Release:**
- `GH_PAT` - GitHub Personal Access Token with repo permissions

**For GoReleaser/Homebrew:**
- `MACOS_CERTIFICATE_P12` - Apple Developer certificate (base64)
- `MACOS_CERTIFICATE_PASSWORD` - Certificate password
- `MACOS_NOTARIZATION_ISSUER_ID` - Apple notarization issuer
- `MACOS_NOTARIZATION_KEY_ID` - Apple notarization key ID
- `MACOS_NOTARIZATION_KEY` - Apple notarization key (base64)
- `BUGSNAG_API_KEY` - Bugsnag error tracking

**For PyPI:**
- `PYPI_API_TOKEN` - PyPI API token for publishing

## Version Bump Rules

| Commit Type | Example | Version Change |
|------------|---------|---------------|
| `feat:` | `feat: add region support` | Minor (1.0.0 → 1.1.0) |
| `fix:` | `fix: resolve auth error` | Patch (1.0.0 → 1.0.1) |
| `perf:` | `perf: optimize loading` | Patch (1.0.0 → 1.0.1) |
| `BREAKING CHANGE:` | `feat!: new API format` | Major (1.0.0 → 2.0.0) |
| `docs:`, `chore:`, `test:` | `docs: update README` | No release |

## Testing the Workflow

1. **Local Test (without publishing):**
   ```bash
   npm install
   npx semantic-release --dry-run --no-ci
   ```

2. **After Merging to Main:**
   - Watch Actions tab for semantic-release workflow
   - If commits warrant a release, it will:
     - Create a new version
     - Trigger release.yml (GoReleaser/Homebrew)
     - Trigger pypi-publish.yml (PyPI)

3. **Manual Release (if needed):**
   - Go to Actions → Semantic Release
   - Click "Run workflow"
   - Select main branch

## Troubleshooting

### Issue: No release created
- Check commit messages follow conventional format
- Ensure at least one `feat:` or `fix:` commit since last release

### Issue: Homebrew not updated
- Check GoReleaser logs in release.yml workflow
- Verify GITHUB_TOKEN has permissions to CerebriumAI/homebrew-tap
- Ensure macOS signing certificates are valid

### Issue: PyPI not updated
- Check pypi-publish.yml workflow logs
- Verify PYPI_API_TOKEN is valid
- Check python-wrapper/setup.py version format

### Issue: macOS binary not notarized
- Verify all MACOS_* secrets are set correctly
- Check certificate hasn't expired
- Ensure notarization credentials are valid

## Summary

✅ **Homebrew**: Will automatically update when semantic-release creates a tag
✅ **PyPI**: Will automatically publish when semantic-release creates a release
✅ **Signing**: macOS binaries will be signed if certificates are configured
✅ **Versioning**: Fully automated based on commit messages