"""
Python wrapper that executes the Go-based Cerebrium CLI binary.

Downloads the binary on first run if not present.
"""

import hashlib
import io
import os
import platform
import re
import stat
import subprocess
import sys
import tarfile
import zipfile
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import urlopen

# DO NOT EDIT: This version is automatically updated by the GitHub Action
# (.github/workflows/pypi-publish.yml) during release. It uses GitHub/semver
# format (e.g., "2.1.0-beta.1" for beta, "2.1.0" for stable).
VERSION = "2.1.0-beta.1"


def github_to_pypi_version(github_version: str) -> str:
    """Convert GitHub/semver version to PEP 440 format for PyPI.

    Examples:
        2.1.0-beta.1  -> 2.1.0b1
        2.1.0-alpha.1 -> 2.1.0a1
        2.1.0-rc.1    -> 2.1.0rc1
        2.1.0         -> 2.1.0
    """
    match = re.match(r"^(\d+\.\d+\.\d+)-(alpha|beta|rc|RC)\.(\d+)$", github_version)
    if match:
        base, pre_type, pre_num = match.groups()
        pre_map = {"alpha": "a", "beta": "b", "rc": "rc", "RC": "rc"}
        return f"{base}{pre_map[pre_type]}{pre_num}"

    return github_version  # Stable version, no change needed


# PEP 440 version for PyPI - computed from VERSION
# This is what setuptools reads for the package version
__version__ = github_to_pypi_version(VERSION)

# GitHub repository for releases
GITHUB_REPO = "CerebriumAI/cerebrium"

# GitHub release URL patterns
RELEASE_URL_TEMPLATE = (
    "https://github.com/{repo}/releases/download/"
    "v{version}/cerebrium_cli_{os}_{arch}.{ext}"
)
CHECKSUMS_URL_TEMPLATE = (
    "https://github.com/{repo}/releases/download/v{version}/checksums.txt"
)


def get_bin_dir() -> Path:
    """Get the directory where the binary should be installed."""
    return Path.home() / ".cerebrium" / "bin"


def get_binary_path() -> Path:
    """Get the path to the cerebrium binary."""
    binary_name = "cerebrium.exe" if os.name == "nt" else "cerebrium"
    return get_bin_dir() / binary_name


def get_version_file() -> Path:
    """Get the path to the version file that tracks installed version."""
    return get_bin_dir() / ".version"


def get_installed_version() -> str | None:
    """Get the currently installed version, if any."""
    version_file = get_version_file()
    if version_file.exists():
        return version_file.read_text().strip()
    return None


def save_installed_version(version: str) -> None:
    """Save the installed version to the version file."""
    version_file = get_version_file()
    version_file.parent.mkdir(parents=True, exist_ok=True)
    version_file.write_text(version)


def get_platform_info() -> tuple[str, str, str]:
    """Determine OS and architecture for binary download."""
    system = platform.system().lower()
    machine = platform.machine().lower()

    # Map Python platform names to GoReleaser naming
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
        print(
            f"Error: Unsupported platform: {system} {machine}\n"
            f"Please install the binary manually from:\n"
            f"https://github.com/{GITHUB_REPO}/releases",
            file=sys.stderr,
        )
        sys.exit(1)

    ext = "zip" if system == "windows" else "tar.gz"

    return os_name, arch_name, ext


def verify_checksum(data: bytes, expected_checksums: str, archive_name: str) -> None:
    """Verify the SHA256 checksum of downloaded data."""
    sha256_hash = hashlib.sha256(data).hexdigest()

    # Find the expected checksum for this archive
    expected_checksum = None
    for line in expected_checksums.split("\n"):
        if archive_name in line:
            parts = line.split()
            if len(parts) >= 2:
                expected_checksum = parts[0]
                break

    if not expected_checksum:
        raise RuntimeError(
            f"Checksum not found for {archive_name} in checksums.txt.\n"
            f"This may indicate a compromised release."
        )

    if sha256_hash != expected_checksum:
        raise RuntimeError(
            f"Checksum verification failed for {archive_name}!\n"
            f"Expected: {expected_checksum}\n"
            f"Got:      {sha256_hash}\n"
            f"This may indicate a corrupted download or security issue."
        )

    print(f"  Checksum verified: {sha256_hash[:16]}...")


def download_with_progress(url: str, desc: str) -> bytes:
    """Download a URL with a simple progress indicator."""
    try:
        with urlopen(url, timeout=60) as response:
            total_size = response.headers.get("Content-Length")
            if total_size:
                total_size = int(total_size)

            data = b""
            downloaded = 0
            block_size = 8192

            while True:
                chunk = response.read(block_size)
                if not chunk:
                    break
                data += chunk
                downloaded += len(chunk)

                if total_size:
                    pct = (downloaded / total_size) * 100
                    mb_down = downloaded / (1024 * 1024)
                    mb_total = total_size / (1024 * 1024)
                    print(
                        f"\r  {desc}: {mb_down:.1f}/{mb_total:.1f} MB ({pct:.0f}%)",
                        end="",
                        flush=True,
                    )

            print()  # Newline after progress
            return data
    except (HTTPError, URLError) as e:
        raise RuntimeError(f"Failed to download from {url}: {e}")


def download_binary(version: str) -> Path:
    """Download the appropriate binary for this platform."""
    os_name, arch_name, ext = get_platform_info()

    archive_name = f"cerebrium_cli_{os_name}_{arch_name}.{ext}"
    url = RELEASE_URL_TEMPLATE.format(
        repo=GITHUB_REPO, version=version, os=os_name, arch=arch_name, ext=ext
    )
    checksums_url = CHECKSUMS_URL_TEMPLATE.format(repo=GITHUB_REPO, version=version)

    print(f"Downloading Cerebrium CLI v{version} for {os_name}/{arch_name}...")

    # Download checksums first
    try:
        with urlopen(checksums_url, timeout=10) as response:
            checksums_data = response.read().decode("utf-8")
    except (HTTPError, URLError) as e:
        raise RuntimeError(
            f"Failed to download checksums: {e}\n"
            f"Please install manually from:\n"
            f"https://github.com/{GITHUB_REPO}/releases"
        )

    # Download the binary archive
    data = download_with_progress(url, "Downloading")

    # Verify checksum
    verify_checksum(data, checksums_data, archive_name)

    # Extract the binary
    bin_dir = get_bin_dir()
    bin_dir.mkdir(parents=True, exist_ok=True)

    binary_name = "cerebrium.exe" if os_name == "windows" else "cerebrium"
    binary_path = bin_dir / binary_name

    print(f"  Extracting to {binary_path}...")

    binary_found = False

    if ext == "zip":
        with zipfile.ZipFile(io.BytesIO(data)) as zf:
            for member in zf.namelist():
                if member.endswith(binary_name) or member == binary_name:
                    with zf.open(member) as src, open(binary_path, "wb") as dst:
                        dst.write(src.read())
                        binary_found = True
                        break
    else:
        with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as tf:
            for member in tf.getmembers():
                if member.name.endswith(binary_name) or member.name == binary_name:
                    src = tf.extractfile(member)
                    if src is None:
                        continue
                    with src, open(binary_path, "wb") as dst:
                        dst.write(src.read())
                        binary_found = True
                        break

    if not binary_found or not binary_path.exists():
        raise RuntimeError(
            f"Failed to extract binary '{binary_name}' from archive.\n"
            f"Please install manually from:\n"
            f"https://github.com/{GITHUB_REPO}/releases"
        )

    # Make executable (Unix only)
    if os_name != "windows":
        binary_path.chmod(binary_path.stat().st_mode | stat.S_IEXEC)

    # Save the installed version
    save_installed_version(version)

    print(f"  Cerebrium CLI v{version} installed successfully!\n")
    return binary_path


def ensure_binary() -> Path:
    """Ensure the binary is installed, downloading if necessary."""
    binary_path = get_binary_path()
    installed_version = get_installed_version()

    # Check if we need to download/update
    if binary_path.exists() and installed_version == VERSION:
        return binary_path

    # Binary missing or version mismatch
    if not binary_path.exists():
        print("Cerebrium CLI binary not found. Installing...\n")
    elif installed_version != VERSION:
        print(f"Updating Cerebrium CLI from v{installed_version} to v{VERSION}...\n")

    try:
        return download_binary(VERSION)
    except RuntimeError as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


def main():
    """Execute the Go binary with the provided arguments."""
    binary = ensure_binary()

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
