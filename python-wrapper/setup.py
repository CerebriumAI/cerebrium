"""
Python wrapper for the Cerebrium Go CLI.

This package downloads and executes the Go-based Cerebrium CLI,
maintaining backward compatibility with 'pip install cerebrium'.
"""

import hashlib
import io
import os
import platform
import stat
import subprocess
import sys
import tarfile
import zipfile
from pathlib import Path
from urllib.request import urlopen

from setuptools import setup
from setuptools.command.install import install

# Version should match the Go CLI version
VERSION = "2.0.0"

# GitHub release URL pattern
# Note: Archive names don't include version (for /latest/ compatibility)
RELEASE_URL_TEMPLATE = (
    "https://github.com/CerebriumAI/cerebrium/releases/download/"
    "v{version}/cerebrium_cli_{os}_{arch}.{ext}"
)

# Checksums URL
CHECKSUMS_URL_TEMPLATE = (
    "https://github.com/CerebriumAI/cerebrium/releases/download/"
    "v{version}/checksums.txt"
)


def get_platform_info():
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
        raise RuntimeError(
            f"Unsupported platform: {system} {machine}. "
            f"Please install the Go binary manually from "
            f"https://github.com/CerebriumAI/cerebrium/releases"
        )

    ext = "zip" if system == "windows" else "tar.gz"

    return os_name, arch_name, ext


def verify_checksum(data, expected_checksums, archive_name):
    """Verify the SHA256 checksum of downloaded data."""
    # Calculate SHA256 hash of the downloaded data
    sha256_hash = hashlib.sha256(data).hexdigest()

    # Find the expected checksum for this archive
    expected_checksum = None
    for line in expected_checksums.split('\n'):
        if archive_name in line:
            # Format: "<checksum>  <filename>"
            parts = line.split()
            if len(parts) >= 2:
                expected_checksum = parts[0]
                break

    if not expected_checksum:
        raise RuntimeError(
            f"Checksum not found for {archive_name} in checksums.txt. "
            f"This may indicate a compromised release. "
            f"Please report this issue."
        )

    # Verify checksums match
    if sha256_hash != expected_checksum:
        raise RuntimeError(
            f"Checksum verification failed for {archive_name}!\n"
            f"Expected: {expected_checksum}\n"
            f"Got:      {sha256_hash}\n"
            f"This may indicate a corrupted download or security issue. "
            f"Please try again or report this issue."
        )

    print(f"✓ Checksum verified: {sha256_hash[:16]}...")


def download_binary(version):
    """Download the appropriate binary for this platform."""
    os_name, arch_name, ext = get_platform_info()

    archive_name = f"cerebrium_cli_{os_name}_{arch_name}.{ext}"
    url = RELEASE_URL_TEMPLATE.format(
        version=version, os=os_name, arch=arch_name, ext=ext
    )

    print(f"Downloading Cerebrium CLI v{version} for {os_name}/{arch_name}...")
    print(f"URL: {url}")

    # Download checksums.txt for verification
    checksums_url = CHECKSUMS_URL_TEMPLATE.format(version=version)
    try:
        with urlopen(checksums_url) as response:
            checksums_data = response.read().decode('utf-8')
    except Exception as e:
        raise RuntimeError(
            f"Failed to download checksums from {checksums_url}: {e}\n"
            f"Please install manually from "
            f"https://github.com/CerebriumAI/cerebrium/releases"
        )

    # Download the binary archive
    try:
        with urlopen(url) as response:
            data = response.read()
    except Exception as e:
        raise RuntimeError(
            f"Failed to download binary from {url}: {e}\n"
            f"Please install manually from "
            f"https://github.com/CerebriumAI/cerebrium/releases"
        )

    # Verify checksum before extracting
    verify_checksum(data, checksums_data, archive_name)

    # Extract the binary
    bin_dir = Path.home() / ".cerebrium" / "bin"
    bin_dir.mkdir(parents=True, exist_ok=True)

    binary_name = "cerebrium.exe" if os_name == "windows" else "cerebrium"
    binary_path = bin_dir / binary_name

    print(f"Extracting to {binary_path}...")

    binary_found = False

    if ext == "zip":
        with zipfile.ZipFile(io.BytesIO(data)) as zf:
            # Extract the binary (search for it in case of subdirectories)
            for member in zf.namelist():
                if member.endswith(binary_name) or member == binary_name:
                    with zf.open(member) as src, open(binary_path, "wb") as dst:
                        dst.write(src.read())
                        binary_found = True
                        break
    else:
        with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as tf:
            # Extract the binary (search for it in case of subdirectories)
            for member in tf.getmembers():
                if member.name.endswith(binary_name) or member.name == binary_name:
                    src = tf.extractfile(member)
                    if src is None:
                        continue
                    with src, open(binary_path, "wb") as dst:
                        dst.write(src.read())
                        binary_found = True
                        break

    # Verify binary was extracted
    if not binary_found or not binary_path.exists():
        raise RuntimeError(
            f"Failed to extract binary '{binary_name}' from archive. "
            f"Please install manually from "
            f"https://github.com/CerebriumAI/cerebrium/releases"
        )

    # Make executable (Unix only)
    if os_name != "windows":
        binary_path.chmod(binary_path.stat().st_mode | stat.S_IEXEC)

    print(f"✓ Cerebrium CLI v{version} installed to {binary_path}")
    return binary_path


class PostInstallCommand(install):
    """Post-installation hook to download the Go binary."""

    def run(self):
        install.run(self)
        download_binary(VERSION)


setup(
    name="cerebrium",
    version=VERSION,
    description="CLI for deploying and managing Cerebrium apps",
    long_description=open("README.md").read() if Path("README.md").exists() else "",
    long_description_content_type="text/markdown",
    author="Cerebrium AI",
    author_email="support@cerebrium.ai",
    url="https://www.cerebrium.ai",
    project_urls={
        "Documentation": "https://docs.cerebrium.ai/",
        "Source": "https://github.com/CerebriumAI/cerebrium",
        "Bug Tracker": "https://github.com/CerebriumAI/cerebrium/issues",
    },
    license="MIT",
    classifiers=[
        "Development Status :: 5 - Production/Stable",
        "Intended Audience :: Developers",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
    ],
    python_requires=">=3.8",
    py_modules=["cerebrium_cli"],
    entry_points={
        "console_scripts": [
            "cerebrium=cerebrium_cli:main",
        ],
    },
    cmdclass={
        "install": PostInstallCommand,
    },
)
