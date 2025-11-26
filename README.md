# Cerebrium CLI

[![Go Version](https://img.shields.io/badge/go-1.25.2-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Command-line interface for [Cerebrium](https://cerebrium.ai) - the serverless infrastructure platform for deploying AI and compute workloads with instant scaling.

## Installation

### Brew (macOS)

```bash
# Using Homebrew (recommended)
brew tap cerebriumai/tap
brew install cerebrium

# Or download the latest release directly
curl -L https://github.com/CerebriumAI/cerebrium/releases/latest/download/cerebrium_cli_darwin_arm64.tar.gz | tar xz
sudo mv cerebrium /usr/local/bin/
# For Intel Macs, use: cerebrium_cli_darwin_amd64.tar.gz
```

### Pip

```bash
pip install cerebrium
```

NOTE: Our pip installer is a thin wrapper around the packaged go binary. On first use, it downloads and installs the go binary, and then passes calls down to the go binary.

### Direct Download

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

## Usage

The CLI provides commands for managing your Cerebrium apps and infrastructure:

```bash
Command line interface for the Cerebrium platform

Usage:
  cerebrium [command]

Available Commands:
  apps        Manage Cerebrium apps (alias: `app`)
  config      Manage CLI configuration
  cp          Copy files to persistent storage
  deploy      Deploy a Cerebrium app
  download    Download files from persistent storage
  help        Help about any command
  init        Initialize an empty Cerebrium Cortex project
  login       Authenticate with Cerebrium
  logs        View logs for an app
  ls          List contents of persistent storage
  projects    Manage Cerebrium projects (alias: `project`)
  region      Manage default region
  rm          Remove files from persistent storage
  run         Run a file in the current project context
  runs        Manage app runs
  status      Check Cerebrium service status
  version     Print the version number
```

## Documentation

In order to start building with Cerebrium, you can check out the following resources:

- **Getting Started**: [Introduction Guide](https://docs.cerebrium.ai/cerebrium/getting-started/introduction)
- **Configuration**: [TOML Reference](https://docs.cerebrium.ai/toml-reference/toml-reference)
- **Full Documentation**: [docs.cerebrium.ai](https://docs.cerebrium.ai)
- **API Reference**: [api.cerebrium.ai](https://api.cerebrium.ai)
- **Examples**: [github.com/CerebriumAI/examples](https://github.com/CerebriumAI/examples)

### Autocompletion

The CLI supports shell autocompletion for bash, zsh, fish, and PowerShell. Run `cerebrium completion --help` for details.

**Quick setup:**

```bash
# Bash (macOS)
cerebrium completion bash > $(brew --prefix)/etc/bash_completion.d/cerebrium

# Bash (Linux)
cerebrium completion bash > /etc/bash_completion.d/cerebrium

# Zsh (macOS)
cerebrium completion zsh > $(brew --prefix)/share/zsh/site-functions/_cerebrium

# Zsh (Linux)
cerebrium completion zsh > "${fpath[1]}/_cerebrium"

# Fish
cerebrium completion fish > ~/.config/fish/completions/cerebrium.fish

# PowerShell (add to your profile for persistence)
cerebrium completion powershell | Out-String | Invoke-Expression
```

Restart your shell after setup.

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
