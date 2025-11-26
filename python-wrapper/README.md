# Cerebrium CLI

Official Python package for the Cerebrium CLI - deploy and manage AI applications with ease.

## Installation

```bash
pip install cerebrium
```

This will download and install the appropriate Cerebrium CLI binary for your platform.

## Quick Start

```bash
# Login to Cerebrium
cerebrium login

# Initialize a new project
cerebrium init my-app

# Deploy your application
cerebrium deploy
```

## About

This Python package is a wrapper that downloads and manages the Cerebrium Go CLI binary. On first run, it will:

1. Detect your operating system and architecture
2. Download the appropriate pre-compiled binary from GitHub releases
3. Verify checksums for security
4. Install it to `~/.cerebrium/bin/`

Subsequent runs use the cached binary.

## Supported Platforms

- **macOS**: Intel (x86_64) and Apple Silicon (arm64)
- **Linux**: x86_64 and arm64
- **Windows**: x86_64

## Documentation

For full documentation, visit [docs.cerebrium.ai](https://docs.cerebrium.ai)

## Support

- GitHub Issues: [github.com/CerebriumAI/cerebrium/issues](https://github.com/CerebriumAI/cerebrium/issues)
- Email: support@cerebrium.ai

## License

MIT License - see [LICENSE](https://github.com/CerebriumAI/cerebrium/blob/main/LICENSE) for details.