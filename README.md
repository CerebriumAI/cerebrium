# Cerebrium CLI

[![Go Version](https://img.shields.io/badge/go-1.25.2-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Command-line interface for [Cerebrium](https://cerebrium.ai) - the serverless infrastructure platform for deploying AI and compute workloads with instant scaling.

## Installation

### macOS

```bash
# Using Homebrew (recommended)
brew tap cerebriumai/tap
brew install cerebrium

# Or download the latest release directly
curl -L https://github.com/CerebriumAI/cerebrium/releases/latest/download/cerebrium_cli_darwin_arm64.tar.gz | tar xz
sudo mv cerebrium /usr/local/bin/
# For Intel Macs, use: cerebrium_cli_darwin_amd64.tar.gz
```

**Note:** Code signing and notarization are coming soon. In the meantime, if macOS blocks the binary, remove the quarantine flag:

```bash
xattr -d com.apple.quarantine /usr/local/bin/cerebrium
```

Or right-click the binary in Finder → Open → confirm the security prompt.

### Linux

```bash
# Download the latest release
curl -L https://github.com/CerebriumAI/cerebrium/releases/latest/download/cerebrium_cli_linux_amd64.tar.gz | tar xz
sudo mv cerebrium /usr/local/bin/
# For ARM64, use: cerebrium_cli_linux_arm64.tar.gz

# Or install via package manager (Ubuntu/Debian)
wget https://github.com/CerebriumAI/cerebrium/releases/latest/download/cerebrium_linux_amd64.deb
sudo dpkg -i cerebrium_linux_amd64.deb
```

### Windows

**PowerShell (Run as Administrator):**

```powershell
# Download the latest release for AMD64
Invoke-WebRequest -Uri "https://github.com/CerebriumAI/cerebrium/releases/latest/download/cerebrium_cli_windows_amd64.zip" -OutFile "cerebrium.zip"

# Extract the archive
Expand-Archive -Path "cerebrium.zip" -DestinationPath "."

# Move to a directory in PATH
New-Item -ItemType Directory -Force -Path "C:\Program Files\cerebrium"
Move-Item -Force cerebrium.exe "C:\Program Files\cerebrium\cerebrium.exe"

# Add to PATH (permanent)
$env:Path += ";C:\Program Files\cerebrium"
[Environment]::SetEnvironmentVariable("Path", $env:Path, [EnvironmentVariableTarget]::Machine)

# Clean up
Remove-Item cerebrium.zip
```

**Or download manually:**
1. Visit https://github.com/CerebriumAI/cerebrium/releases/latest
2. Download `cerebrium_cli_windows_amd64.zip` (or `arm64` for ARM)
3. Extract and add `cerebrium.exe` to your PATH

**Note:** Package manager support (Chocolatey/Scoop) coming soon!

### Verify Installation

```bash
cerebrium version
```

## Quick Start

```bash
# Authenticate with Cerebrium
cerebrium login

# Initialize a new project
cerebrium init my-app
cd my-app

# Deploy your application
cerebrium deploy
```

## Usage

The CLI provides commands for managing your Cerebrium apps and infrastructure:

```bash
cerebrium [command]

Available Commands:
  deploy      Deploy your application to Cerebrium
  login       Authenticate with Cerebrium
  logs        View application logs
  app         Manage applications (list, delete, describe)
  project     Manage projects
  run         Execute code in your project context
  status      Check service status

Run "cerebrium [command] --help" for detailed usage information.
```

## Configuration

The CLI uses `cerebrium.toml` for project configuration and `~/.cerebrium/config.yaml` for global settings.

Example `cerebrium.toml`:

```toml
[cerebrium.deployment]
name = "my-app"
python_version = "3.11"

[cerebrium.hardware]
cpu = 2.0
memory = 4.0
compute = "CPU"
region = "us-east-1"

[cerebrium.scaling]
min_replicas = 0
max_replicas = 3
```

For complete configuration options:
- Run `cerebrium init` to generate a template
- See the [documentation](https://docs.cerebrium.ai)

## Non-Interactive Mode

The CLI supports non-interactive usage for CI/CD pipelines:

```bash
# Disable animations and colors
cerebrium deploy --no-color

# Skip confirmation prompts
cerebrium deploy --disable-confirmation

# Combine for CI environments
CEREBRIUM_TOKEN=xxx cerebrium deploy --no-color -y
```

## Documentation

- **Full Documentation**: [docs.cerebrium.ai](https://docs.cerebrium.ai)
- **API Reference**: [api.cerebrium.ai](https://api.cerebrium.ai)
- **Examples**: [github.com/CerebriumAI/examples](https://github.com/CerebriumAI/examples)

## Development

This CLI is written in Go and uses:
- [Cobra](https://github.com/spf13/cobra) for command structure
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) for interactive UI
- [Viper](https://github.com/spf13/viper) for configuration management

To build from source:

```bash
git clone https://github.com/CerebriumAI/cerebrium.git
cd cerebrium
make build
./bin/cerebrium version
```

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Support

- **Issues**: [GitHub Issues](https://github.com/CerebriumAI/cerebrium/issues)
- **Discord**: [Join our community](https://discord.gg/cerebrium)
- **Email**: support@cerebrium.ai

## License

MIT - See [LICENSE](LICENSE) for details.
