"""
Python wrapper that executes the Go-based Cerebrium CLI binary.
"""

import os
import subprocess
import sys
from pathlib import Path


def get_binary_path():
    """Get the path to the cerebrium binary."""
    bin_dir = Path.home() / ".cerebrium" / "bin"
    binary_name = "cerebrium.exe" if os.name == "nt" else "cerebrium"
    binary_path = bin_dir / binary_name

    if not binary_path.exists():
        print(
            f"Error: Cerebrium CLI binary not found at {binary_path}\n"
            f"Please reinstall: pip install --force-reinstall cerebrium",
            file=sys.stderr,
        )
        sys.exit(1)

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
