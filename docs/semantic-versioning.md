# Semantic Versioning

This project uses [semantic-release](https://github.com/semantic-release/semantic-release) to automatically version and release new versions based on commit messages.

## How it Works

When commits are pushed to `main` or `master`, the semantic-release workflow analyzes the commit messages and:

1. Determines if a new release is needed
2. Calculates the version bump (major, minor, or patch)
3. Creates a git tag
4. Updates CHANGELOG.md
5. Creates a GitHub release
6. Triggers GoReleaser to build and publish binaries

## Commit Message Format

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types and Version Bumps

| Type | Description | Version Bump |
|------|-------------|--------------|
| `feat` | New feature | Minor (0.x.0) |
| `fix` | Bug fix | Patch (0.0.x) |
| `perf` | Performance improvement | Patch (0.0.x) |
| `revert` | Revert a previous commit | Patch (0.0.x) |
| `docs` | Documentation only | No release |
| `style` | Code style changes | No release |
| `refactor` | Code refactoring | No release |
| `test` | Adding tests | No release |
| `chore` | Maintenance tasks | No release |
| `build` | Build system changes | No release |
| `ci` | CI/CD changes | No release |

### Breaking Changes

To trigger a major version bump, include `BREAKING CHANGE:` in the commit body or footer:

```
feat: remove deprecated API endpoints

BREAKING CHANGE: Removed v1 API endpoints. Please migrate to v2.
```

Or use `!` after the type:

```
feat!: redesign configuration format
```

## Examples

### Minor Version Bump (New Feature)
```
feat: add support for custom regions
```

### Patch Version Bump (Bug Fix)
```
fix: resolve authentication error with refresh tokens
```

### No Version Bump (Documentation)
```
docs: update installation instructions
```

### Major Version Bump (Breaking Change)
```
feat!: change deployment API response format

BREAKING CHANGE: The deployment API now returns a different JSON structure.
Old format: {status: "success", id: "123"}
New format: {deployment: {id: "123", status: "success"}}
```

## Manual Release

To manually trigger a release:

1. Go to Actions → Semantic Release workflow
2. Click "Run workflow"
3. Select the branch
4. Click "Run workflow"

## Configuration

The semantic-release configuration is defined in:
- `.releaserc.yml` - Main configuration file
- `package.json` - Dependencies
- `.github/workflows/semantic-release.yml` - GitHub Actions workflow

## Local Testing

To test semantic-release locally without making actual releases:

```bash
npm ci
npx semantic-release --dry-run
```

This will show what version would be released based on recent commits.