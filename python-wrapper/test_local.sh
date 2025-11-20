#!/bin/bash
# Test the Python package locally before publishing

set -e

echo "=== Testing Cerebrium Python Package Locally ==="

# Clean previous builds
rm -rf dist/ build/ *.egg-info

# Build the package
echo "Building package..."
python -m build

# Create a test virtual environment
echo "Creating test environment..."
python -m venv test_env
source test_env/bin/activate

# Install the package from the local wheel
echo "Installing package..."
pip install dist/*.whl

# Test that the command works
echo "Testing cerebrium command..."
cerebrium --version || echo "Note: --version flag might not work until binary is downloaded"

# Show where it installed
echo "Package installed at:"
which cerebrium
pip show cerebrium

# Clean up
deactivate
rm -rf test_env

echo "=== Local testing complete ==="
echo ""
echo "To publish to Test PyPI:"
echo "  1. Get Test PyPI token from https://test.pypi.org/manage/account/token/"
echo "  2. Run: twine upload --repository testpypi dist/*"
echo ""
echo "To publish to PyPI:"
echo "  1. Get PyPI token from https://pypi.org/manage/account/token/"
echo "  2. Run: twine upload dist/*"
