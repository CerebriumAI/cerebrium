# Contributing to Cerebrium CLI

Thank you for your interest in contributing to the Cerebrium CLI! This document provides guidelines and instructions for contributing to this project.

## Ways to Contribute

### Reporting Bugs

Before opening a bug report:
- Check existing [issues](https://github.com/CerebriumAI/cerebrium/issues) to avoid duplicates
- Verify you're using the latest version: `cerebrium version`

When reporting a bug, include:
- Steps to reproduce
- Expected vs. actual behavior
- Your environment (OS, Go version, CLI version)
- Relevant logs (use `--verbose` flag)

### Suggesting Features

Feature requests are welcome! Please:
- Check existing issues first
- Clearly describe the use case
- Explain how it would benefit Cerebrium users

### Submitting Pull Requests

**Please do:**
- Add tests for new functionality
- Update documentation as needed
- Follow the architecture patterns described below
- Leave the repo in a better state than you found it

**Please don't:**
- Submit PRs for issues without clear acceptance criteria
- Expand scope beyond the issue description
- Submit a PR without an associated Issue
- Break existing tests or golden files, unless contextually appropriate

## Development Setup

### Prerequisites

- **Go 1.25.2+** (check `go.mod` for exact version)
- **Make** (for build commands)
- **Git**
- Optional: **golangci-lint** for local linting

## Architecture Guidelines

This project follows specific architectural patterns documented in [CLAUDE.md](./CLAUDE.md).

## Code Style

### Go Standards

Follow the patterns in [CLAUDE.md](./CLAUDE.md), and defer to [Effective Go](https://go.dev/doc/effective_go) where no obvious convention exists, especially for naming.

### Formatting

```bash
# Format code
go fmt ./...

# Or use the Makefile
make fmt
```

### Linting

All code must pass golangci-lint checks. Configuration is in `.golangci.yml`.

## Tests

Add tests for any new features or bug fixes. Ideally, each PR increases the test coverage.

- [Table tests](https://go.dev/wiki/TableDrivenTests) preferred where possible
- [testing/harness.go](https://github.com/CerebriumAI/cerebrium/blob/main/internal/ui/testing/harness.go) provides a Harness for testing Bubbletea models. Please follow the associated README

## Submitting Changes

### 1. Create a Branch

```bash
git checkout -b feature/my-feature
# or
git checkout -b fix/my-bugfix
```

### 2. Make Your Changes

- Write clear, focused commits
- Add tests for new functionality
- Ensure all tests pass: `go test ./...`
- Run linter: `golangci-lint run`

### 3. Commit Messages

Use clear, [conventional commit messages](https://www.conventionalcommits.org/):

```
feat: add support for custom regions
fix: handle nil pointer in deploy command
docs: update installation instructions
test: add golden tests for login flow
```

### 4. Push and Create PR

```bash
git push origin feature/my-feature
gh pr create --web
```

Or use GitHub's web interface.

### 5. PR Guidelines

Your PR should:
- Reference the issue it addresses (`Fixes #123`)
- Include a clear description of changes
- Pass all CI checks (tests, lint)
- Update documentation if needed
- Include tests for new functionality

## Development Tips

### Debugging Bubbletea Models

In interactive mode, logs go to `stderr` to avoid corrupting the TUI:

```bash
# Run your command, and reroute stderr to a file of your choosing.
./bin/cerebrium deploy --verbose 2>logfile.log
# In another terminal, tail the logs
tail -f logfile.log

# If stdout and stderr write to the same stream (default) with --verbose mode on, we default to simple mode
./bin/cerebrium deploy --verbose
```

`slog` is configured to write to `stderr`, so any `slog.Debug|Info|Warn|Error` is written there.

### Testing Non-TTY Mode

```bash
# Simulate non-TTY environment
./bin/cerebrium deploy --no-color
```

## Code of Conduct

Please be respectful and constructive in all interactions.

## License

By contributing, you agree that your contributions will be licensed under the same license as the project (see [LICENSE](./LICENSE)).

---

Thank you for contributing to Cerebrium CLI!
