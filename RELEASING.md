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

Releases are cut from a **single workflow** — `Release` (`.github/workflows/release.yml`).
Do **not** push a tag by hand: a manually pushed tag triggers nothing, and a tag
pushed by CI's `GITHUB_TOKEN` cannot trigger downstream workflows. The orchestrator
creates the tag itself as one ordered step in the pipeline.

### Run the Release workflow

From the GitHub UI: **Actions → Release → Run workflow**. Enter a version
(e.g. `v2.1.0` or `2.1.0` — the `v` is added if missing), or **leave it blank to
auto-bump the patch** of the latest stable release (e.g. `v2.5.2` → `v2.5.3`). Or via the CLI:

```bash
# Patch bump of the latest stable release (no version needed):
gh workflow run release.yml

# Or pin an explicit version:
gh workflow run release.yml -f version=v2.1.0

# optionally pin a commit (defaults to main HEAD) or skip tests on a re-run:
#   -f commit=<sha>   -f skip-tests=true
```

A minor or major bump must be given explicitly (`-f version=v2.6.0`); the blank-version
default only ever increments the patch.

### What the workflow does (one run, ordered by `needs:`)

```
validate → test → tag → goreleaser → verify-assets → wrapper-smoke → pypi → promote
                                                                              └ cleanup (on failure)
```

1. **validate** — normalize the version, reject it if the tag already exists, resolve the commit.
2. **test** — run the full test matrix against that commit (skippable with `skip-tests=true`).
3. **tag** — create and push the annotated tag (no event-chaining; the build is the next job).
4. **goreleaser** — build binaries for all platforms, checksums, Homebrew tap, deb/rpm, and the GitHub release. **The release is created as a *prerelease*.**
5. **verify-assets** — assert every archive + `checksums.txt` is actually on the release.
6. **wrapper-smoke** — build the Python wrapper and run it against the freshly published release (exercises the real binary download) *before* touching PyPI.
7. **pypi** — publish the wrapper to PyPI (`pip install cerebrium`).
8. **promote** — flip a stable release from prerelease to **"Latest"**. Runs only after everything above passes; actual prerelease tags (`-rc`/`-beta`) are left as prereleases.
9. **cleanup** — if the *build* fails (half-created tag/release), delete the tag + release so the run can be retried. Once goreleaser succeeds the prerelease is valid, so a later failure leaves it in place as an un-promoted prerelease. (PyPI cannot be un-published — only yanked — which is why PyPI runs late and `promote` is last.)

Because everything runs in one workflow, the Actions run page shows exactly where a
release stopped — there is no silent hand-off between workflows.

### Testing the pipeline (dry run)

```bash
gh workflow run release.yml -f version=v0.0.1-rc.1 -f dry-run=true
```

A dry run exercises the whole pipeline for real — it **pushes the tag** and creates a
real GitHub **prerelease** with binaries, and runs `verify-assets` + `wrapper-smoke` —
but it never **promotes** the release to "Latest", never **publishes to PyPI**, and skips
the **Homebrew** tap push and macOS notarization. The prerelease is left in place for you
to inspect; tear it down with `gh release delete <tag> --cleanup-tag`.

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

For beta or release candidate versions, run the same workflow with a prerelease version:

```bash
gh workflow run release.yml -f version=v2.1.0-beta.1
gh workflow run release.yml -f version=v2.1.0-rc.1
```

GoReleaser detects the prerelease from the tag (`prerelease: auto`) and:
- Creates a GitHub pre-release
- Does **not** update the Homebrew formula (stable releases only)
- Publishes to PyPI with the appropriate version specifier (e.g. `2.1.0b1`)

## Local Testing

Before creating a release, you can test locally:

```bash
# Test GoReleaser configuration
make release-dry

# Build locally with specific version
make build VERSION=2.1.0
./bin/cerebrium version
```

To exercise the **full pipeline** end-to-end without affecting stable users, run the
workflow against a throwaway prerelease tag (e.g. `gh workflow run release.yml -f version=v0.0.1-rc.1`)
and let `cleanup` (or a manual `gh release delete v0.0.1-rc.1 --cleanup-tag`) tear it down.

## Required Secrets

The following secrets must be configured in GitHub repository settings:

- **GH_PAT**: GitHub Personal Access Token with `repo` + `workflow` scope (used by GoReleaser for the release and Homebrew tap updates).
- PyPI publishing uses **OIDC trusted publishing** (`id-token: write`), so no PyPI API token is required — the project must be configured as a trusted publisher on PyPI for this repo's `pypi-publish.yml` workflow.
- macOS signing/notarization secrets (`MACOS_CERTIFICATE_P12`, `MACOS_CERTIFICATE_PASSWORD`, `MACOS_NOTARIZATION_ISSUER_ID`, `MACOS_NOTARIZATION_KEY_ID`, `MACOS_NOTARIZATION_KEY`) and `BUGSNAG_API_KEY`.

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

# Patch release (bug fixes)
gh workflow run release.yml -f version=v2.0.1

# Minor release (new features)
gh workflow run release.yml -f version=v2.1.0

# Major release (breaking changes)
gh workflow run release.yml -f version=v3.0.0

# Watch the run
gh run watch "$(gh run list --workflow=release.yml --limit 1 --json databaseId -q '.[0].databaseId')"

# Roll back a botched release (tag + GitHub release; PyPI can only be yanked)
gh release delete v2.0.1 --cleanup-tag
```

## Troubleshooting

### Release workflow fails
- Check GitHub Actions logs for specific error
- Ensure all secrets are configured correctly
- Verify `.goreleaser.yaml` configuration is valid

### PyPI publish fails
- Ensure the version doesn't already exist on PyPI (publishes use `skip-existing`, but a partial prior upload can still conflict)
- Confirm the repo is registered as a PyPI trusted publisher for `pypi-publish.yml` (OIDC)
- Verify `python-wrapper/pyproject.toml` is correctly formatted

### Homebrew formula not updating
- Only stable releases update Homebrew (not pre-releases)
- Check GH_PAT has write access to tap repository
- Verify tap repository exists at `cerebriumai/homebrew-tap`