# Cerebrium CLI

Python wrapper for the Cerebrium CLI (Go-based).

## Installation

```bash
pip install cerebrium
```

## Usage

After installation, use the `cerebrium` command:

```bash
cerebrium login
cerebrium deploy
cerebrium apps list
cerebrium apps get <app-id>
```

## What's Inside

This package downloads and manages the Go-based Cerebrium CLI binary. When you run `pip install cerebrium`, it:

1. Downloads the appropriate binary for your platform (macOS, Linux, or Windows)
2. Installs it to `~/.cerebrium/bin/`
3. Provides a `cerebrium` command that executes the binary

## Manual Installation

If you prefer to install the Go binary directly:

```bash
# macOS (Homebrew)
brew install cerebriumai/tap/cerebrium

# Linux/macOS (direct download)
curl -fsSL https://github.com/CerebriumAI/cerebrium/releases/latest/download/install.sh | sh

# Or download from releases
# https://github.com/CerebriumAI/cerebrium/releases
```

## Documentation

Visit [docs.cerebrium.ai](https://docs.cerebrium.ai) for full documentation.

## License

AGPL-3.0
