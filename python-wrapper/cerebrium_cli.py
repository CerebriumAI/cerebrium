"""
Python wrapper that executes the Go-based Cerebrium CLI binary.
"""

import os
import subprocess
import sys
from pathlib import Path

# Must match VERSION in setup.py
EXPECTED_VERSION = "2.1.4"


def get_binary_dir():
    """Get the directory where the cerebrium binary is stored."""
    return Path.home() / ".cerebrium" / "bin"


def get_binary_path():
    """Get the path to the cerebrium binary."""
    bin_dir = get_binary_dir()
    binary_name = "cerebrium.exe" if os.name == "nt" else "cerebrium"
    return bin_dir / binary_name


def get_installed_version():
    """Get the version of the currently installed binary."""
    version_file = get_binary_dir() / ".version"
    if version_file.exists():
        return version_file.read_text().strip()
    
    # Fall back to running the binary
    binary_path = get_binary_path()
    if binary_path.exists():
        try:
            result = subprocess.run(
                [str(binary_path), "version"],
                capture_output=True,
                text=True,
                timeout=5,
            )
            # Parse version from output like "cerebrium 2.1.4 (commit: ...)"
            output = result.stdout.strip()
            if output.startswith("cerebrium "):
                parts = output.split()
                if len(parts) >= 2:
                    return parts[1]
        except Exception:
            pass
    
    return None


def download_binary():
    """Download the binary using the setup.py download logic."""
    print(f"Downloading Cerebrium CLI v{EXPECTED_VERSION}...")
    
    # Import the download function from setup.py logic
    import hashlib
    import io
    import platform
    import stat
    import tarfile
    import zipfile
    from urllib.request import urlopen

    RELEASE_URL_TEMPLATE = (
        "https://github.com/CerebriumAI/cerebrium/releases/download/"
        "v{version}/cerebrium_{version}_{os}_{arch}.{ext}"
    )
    CHECKSUMS_URL_TEMPLATE = (
        "https://github.com/CerebriumAI/cerebrium/releases/download/"
        "v{version}/checksums.txt"
    )

    # Determine platform
    system = platform.system().lower()
    machine = platform.machine().lower()

    os_map = {"darwin": "darwin", "linux": "linux", "windows": "windows"}
    arch_map = {"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}

    os_name = os_map.get(system)
    arch_name = arch_map.get(machine)

    if not os_name or not arch_name:
        print(
            f"Error: Unsupported platform: {system} {machine}\n"
            f"Please install manually from https://github.com/CerebriumAI/cerebrium/releases",
            file=sys.stderr,
        )
        sys.exit(1)

    ext = "zip" if system == "windows" else "tar.gz"
    archive_name = f"cerebrium_{EXPECTED_VERSION}_{os_name}_{arch_name}.{ext}"
    url = RELEASE_URL_TEMPLATE.format(
        version=EXPECTED_VERSION, os=os_name, arch=arch_name, ext=ext
    )

    # Download checksums
    checksums_url = CHECKSUMS_URL_TEMPLATE.format(version=EXPECTED_VERSION)
    try:
        with urlopen(checksums_url) as response:
            checksums_data = response.read().decode("utf-8")
    except Exception as e:
        print(f"Error: Failed to download checksums: {e}", file=sys.stderr)
        sys.exit(1)

    # Download binary
    try:
        with urlopen(url) as response:
            data = response.read()
    except Exception as e:
        print(f"Error: Failed to download binary: {e}", file=sys.stderr)
        sys.exit(1)

    # Verify checksum
    sha256_hash = hashlib.sha256(data).hexdigest()
    expected_checksum = None
    for line in checksums_data.split("\n"):
        if archive_name in line:
            parts = line.split()
            if len(parts) >= 2:
                expected_checksum = parts[0]
                break

    if not expected_checksum or sha256_hash != expected_checksum:
        print("Error: Checksum verification failed!", file=sys.stderr)
        sys.exit(1)

    # Extract binary
    bin_dir = get_binary_dir()
    bin_dir.mkdir(parents=True, exist_ok=True)

    binary_name = "cerebrium.exe" if os_name == "windows" else "cerebrium"
    binary_path = bin_dir / binary_name

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
                        with open(binary_path, "wb") as dst:
                            dst.write(src.read())
                    break

    # Make executable
    if os_name != "windows":
        binary_path.chmod(binary_path.stat().st_mode | stat.S_IEXEC)

    # Write version file
    version_file = bin_dir / ".version"
    version_file.write_text(EXPECTED_VERSION)

    print(f"✓ Cerebrium CLI v{EXPECTED_VERSION} installed")


def ensure_binary():
    """Ensure the correct version of the binary is installed."""
    binary_path = get_binary_path()
    installed_version = get_installed_version()

    # Check if binary exists and version matches
    if binary_path.exists() and installed_version == EXPECTED_VERSION:
        return binary_path

    # Need to download/update
    if not binary_path.exists():
        print(f"Cerebrium CLI binary not found. Installing v{EXPECTED_VERSION}...")
    elif installed_version:
        print(f"Updating Cerebrium CLI from v{installed_version} to v{EXPECTED_VERSION}...")
    else:
        print(f"Installing Cerebrium CLI v{EXPECTED_VERSION}...")

    download_binary()
    return binary_path


def main():
    """Execute the Go binary with the provided arguments."""
    binary = ensure_binary()

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
