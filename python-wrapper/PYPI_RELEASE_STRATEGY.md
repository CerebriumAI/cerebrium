# PyPI Release Strategy

## How It Works

1. Run the `Release` workflow (`.github/workflows/release.yml`) with a version (e.g. `v2.1.0` or `v2.1.0-beta.1`). See [RELEASING.md](../RELEASING.md).
2. After the binaries are built and verified on the GitHub release, the orchestrator calls `.github/workflows/pypi-publish.yml`, which:
   - Updates `VERSION` in `cerebrium_cli.py` (GitHub format: `2.1.0-beta.1`)
   - Updates `version` in `pyproject.toml` (PEP 440 format: `2.1.0b1`)
   - Builds and publishes to PyPI
   - Tests installation

The PyPI publish runs **only after** the binaries exist on the release, so the wrapper
never ships pointing at missing binaries. The pip package is a thin wrapper that
downloads the Go binary on first run.

## Version Formats

| GitHub Tag | PyPI Version | User Install |
|------------|--------------|--------------|
| `v2.1.0` | `2.1.0` | `pip install cerebrium` |
| `v2.1.0-beta.1` | `2.1.0b1` | `pip install cerebrium==2.1.0b1` |
| `v2.1.0-alpha.1` | `2.1.0a1` | `pip install cerebrium==2.1.0a1` |

**Pre-releases are safe**: `pip install cerebrium` won't install beta/alpha versions unless explicitly requested.

## Local Testing

```bash
cd python-wrapper
source .venv/bin/activate
python -m build

# Test in clean environment
python -m venv /tmp/test-cerebrium
source /tmp/test-cerebrium/bin/activate
pip install dist/cerebrium-*.whl
cerebrium version

# Cleanup
deactivate
rm -rf /tmp/test-cerebrium ~/.cerebrium/bin
```

## Emergency Rollback

```bash
# Yank a broken version (prevents new installs)
twine yank cerebrium==2.1.0

# Or publish a patch
# Update version, rebuild, and upload
```
