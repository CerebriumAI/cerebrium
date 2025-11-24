"""
Python wrapper that executes the Go-based Cerebrium CLI binary.
"""

import hashlib
import io
import json
import os
import platform
import stat
import subprocess
import sys
import tarfile
import zipfile
from pathlib import Path
from urllib.request import urlopen


def get_github_version():
    """Get the version to download - matches what was packaged."""
    # Try to get from setup.py if available (development mode)
    try:
        import setup

        return setup.GITHUB_VERSION
    except ImportError:
        # Fallback to hardcoded version for installed package
        # This should match the version in setup.py when package was built
        return "2.1.0-beta.2"


def get_platform_info():
    """Determine OS and architecture for binary download."""
    system = platform.system().lower()
    machine = platform.machine().lower()

    os_map = {
        "darwin": "darwin",
        "linux": "linux",
        "windows": "windows",
    }

    arch_map = {
        "x86_64": "amd64",
        "amd64": "amd64",
        "aarch64": "arm64",
        "arm64": "arm64",
    }

    os_name = os_map.get(system)
    arch_name = arch_map.get(machine)

    if not os_name or not arch_name:
        raise RuntimeError(f"Unsupported platform: {system} {machine}")

    ext = "zip" if system == "windows" else "tar.gz"
    return os_name, arch_name, ext


def download_binary():
    """Download and install the Cerebrium CLI binary."""
    print("Cerebrium CLI not found. Downloading...", file=sys.stderr)

    os_name, arch_name, ext = get_platform_info()

    # Build download URL
    archive_name = f"cerebrium_cli_{os_name}_{arch_name}.{ext}"
    url = (
        f"https://github.com/CerebriumAI/cerebrium/releases/download/"
        f"v{GITHUB_VERSION}/{archive_name}"
    )

    print(f"Downloading from: {url}", file=sys.stderr)

    try:
        # Download the binary
        with urlopen(url) as response:
            data = response.read()

        # Create bin directory
        bin_dir = Path.home() / ".cerebrium" / "bin"
        bin_dir.mkdir(parents=True, exist_ok=True)

        binary_name = "cerebrium.exe" if os_name == "windows" else "cerebrium"
        binary_path = bin_dir / binary_name

        # Extract the binary
        if ext == "zip":
            with zipfile.ZipFile(io.BytesIO(data)) as zf:
                for member in zf.namelist():
                    if member.endswith(binary_name) or member == binary_name:
                        with zf.open(member) as src, open(binary_path, "wb") as dst:
                            dst.write(src.read())
                        break
        else:
            with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as tf:
                for member in tf.getmembers():
                    if member.name.endswith(binary_name) or member.name == binary_name:
                        src = tf.extractfile(member)
                        if src:
                            with src, open(binary_path, "wb") as dst:
                                dst.write(src.read())
                            break

        # Make executable (Unix only)
        if os_name != "windows":
            binary_path.chmod(binary_path.stat().st_mode | stat.S_IEXEC)

        print(f"✓ Downloaded Cerebrium CLI v{GITHUB_VERSION}", file=sys.stderr)
        return binary_path

    except Exception as e:
        print(
            f"Failed to download Cerebrium CLI: {e}\n"
            f"Please install manually from: "
            f"https://github.com/CerebriumAI/cerebrium/releases",
            file=sys.stderr,
        )
        sys.exit(1)


def get_binary_path():
    """Get the path to the cerebrium binary, downloading if necessary."""
    bin_dir = Path.home() / ".cerebrium" / "bin"
    binary_name = "cerebrium.exe" if os.name == "nt" else "cerebrium"
    binary_path = bin_dir / binary_name

    if not binary_path.exists():
        # Auto-download the binary
        binary_path = download_binary()

    return binary_path


def main():
    """Execute the Go binary with the provided arguments."""
    binary = get_binary_path()

    # Pass through all arguments to the Go binary
    try:
        result = subprocess.run([str(binary)] + sys.argv[1:])
        sys.exit(result.returncode)
    except KeyboardInterrupt:
        sys.exit(130)
    except Exception as e:
        print(f"Error executing Cerebrium CLI: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
