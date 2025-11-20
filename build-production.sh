#!/bin/bash
# Production build script for Cerebrium CLI with Bugsnag error reporting

# Set your Bugsnag API key here or pass as environment variable
# BUGSNAG_API_KEY=${BUGSNAG_API_KEY:-"your-production-api-key"}

# Example production build with Bugsnag enabled
# Uncomment and set your API key to enable error reporting
# make build \
#   VERSION=$(git describe --tags --always) \
#   BUGSNAG_API_KEY="your-production-api-key" \
#   BUGSNAG_RELEASE_STAGE="prod"

# Example production build without Bugsnag (error reporting disabled)
make build VERSION=$(git describe --tags --always)

# Example development build with Bugsnag
# make build \
#   VERSION="dev" \
#   BUGSNAG_API_KEY="your-dev-api-key" \
#   BUGSNAG_RELEASE_STAGE="dev"

echo "Build complete. Binary location: ./bin/cerebrium"
echo ""
echo "To enable error reporting, rebuild with:"
echo "  make build BUGSNAG_API_KEY=your-api-key"
echo ""
echo "Or set via environment variable at runtime:"
echo "  export BUGSNAG_API_KEY=your-api-key"
echo "  ./bin/cerebrium"
