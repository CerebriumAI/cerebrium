# PyPI Release Strategy for Cerebrium 2.0

## ⚠️ IMPORTANT: Do NOT release to PyPI until ready for public announcement

### Why We Need to Be Careful

1. **Auto-updates**: Users who run `pip install --upgrade cerebrium` will automatically get v2.0.0
2. **Breaking changes**: v2.0.0 may have different commands, configs, or behaviors
3. **No rollback**: Once published to PyPI, we can't remove or hide versions (only yank in emergencies)
4. **User surprise**: Users expect stability from pip packages

## Release Strategy Options

### Option 1: Beta Channel (Recommended)
1. **Pre-release versions**: Publish as `2.0.0b1`, `2.0.0b2`, etc.
   - These won't auto-install with `pip install cerebrium`
   - Users must explicitly request: `pip install cerebrium==2.0.0b1`
2. **Testing period**: Let early adopters test
3. **Final release**: When ready, publish `2.0.0`

### Option 2: Separate Package Name
1. Create `cerebrium-v2` or `cerebrium-beta` package
2. Early adopters use: `pip install cerebrium-v2`
3. When ready, update main `cerebrium` package

### Option 3: Hold Until Ready (Simplest)
1. **Don't publish to PyPI** until v2.0.0 is fully ready
2. Early adopters use GitHub releases or direct downloads
3. Publish all at once when announcing

## Pre-Release Checklist

Before publishing v2.0.0 to PyPI:

- [ ] All v2.0 features are stable
- [ ] Migration guide from v1.x to v2.0 is ready
- [ ] Documentation is updated at docs.cerebrium.ai
- [ ] Announcement blog post/email is prepared
- [ ] Support team is briefed on changes
- [ ] Rollback plan exists (if critical issues found)

## Testing Before Release

### 1. Test on Test PyPI First
```bash
# Upload to Test PyPI
twine upload --repository testpypi dist/*

# Install from Test PyPI
pip install --index-url https://test.pypi.org/simple/ cerebrium
```

### 2. Beta Testing with Pre-release
```bash
# In setup.py, set version to "2.0.0b1"
VERSION = "2.0.0b1"

# Upload to real PyPI (but as beta)
twine upload dist/*

# Beta testers install with:
pip install cerebrium==2.0.0b1
```

### 3. Version Pinning Recommendation
Advise current users to pin their version before v2.0 release:
```bash
# In requirements.txt or pip install
cerebrium<2.0.0
```

## GitHub Actions Setup (When Ready)

The workflow is already created but needs tokens:

1. **Test PyPI Token** (for testing):
   - Go to https://test.pypi.org/manage/account/token/
   - Create token with scope "Entire account"
   - Add as GitHub secret: `TEST_PYPI_API_TOKEN`

2. **Production PyPI Token** (DO NOT ADD YET):
   - Go to https://pypi.org/manage/account/token/
   - Create token with scope "Project: cerebrium"
   - Add as GitHub secret: `PYPI_API_TOKEN`
   - ⚠️ **Only add this when ready to release!**

## Emergency Rollback

If something goes wrong after PyPI release:

1. **Yank the version** (marks as broken):
```bash
# This prevents NEW installations but doesn't affect existing
twine yank cerebrium==2.0.0
```

2. **Publish a patch**:
```bash
# Quickly release 2.0.1 with fixes
VERSION="2.0.1"
python -m build
twine upload dist/*
```

## Recommended Timeline

1. **Now - 2 weeks**: Internal testing, GitHub releases only
2. **Week 3**: Publish beta to PyPI (`2.0.0b1`)
3. **Week 4-5**: Gather feedback, fix issues
4. **Week 6**: Final release with announcement

## Commands When Ready

```bash
# Final release process (ONLY when ready):
cd python-wrapper

# Update version in setup.py and pyproject.toml to "2.0.0"

# Build
python -m build

# Final check
twine check dist/*

# Upload to PyPI (THIS IS THE POINT OF NO RETURN)
twine upload dist/*
```

## Remember

- **APT/Homebrew users**: Already controlled, they choose when to update
- **Direct download users**: Also controlled via GitHub releases  
- **pip users**: Will auto-update - this is why we wait!

The PyPI release should be the LAST step in the v2.0 rollout, done only when you're ready for all pip users to migrate.