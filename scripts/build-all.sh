#!/bin/bash
# Build all binaries for GoReleaser to package
set -e

VERSION=${1:-dev}
COMMIT=$(git rev-parse --short HEAD)
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Platforms to build
PLATFORMS=(
    "darwin/amd64"
    "darwin/arm64"
    "linux/amd64"
    "linux/arm64"
    "windows/amd64"
    "windows/arm64"
)

echo "Building Cerebrium CLI ${VERSION}"
echo "Commit: ${COMMIT}"
echo "Date: ${BUILD_DATE}"

# Create dist directory
mkdir -p dist

# Build for each platform
for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}
    OUTPUT="dist/cerebrium_${GOOS}_${GOARCH}"

    if [ "$GOOS" = "windows" ]; then
        OUTPUT="${OUTPUT}.exe"
    fi

    echo "Building for $GOOS/$GOARCH..."

    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags "-X github.com/cerebriumai/cerebrium/internal/version.Version=${VERSION} \
                  -X github.com/cerebriumai/cerebrium/internal/version.Commit=${COMMIT} \
                  -X github.com/cerebriumai/cerebrium/internal/version.BuildDate=${BUILD_DATE} \
                  -X github.com/cerebriumai/cerebrium/pkg/bugsnag.BugsnagAPIKey=${BUGSNAG_API_KEY}" \
        -o "$OUTPUT" \
        ./cmd/cerebrium
done

echo "✓ All binaries built successfully"
