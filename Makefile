.PHONY: build clean test test-coverage test-pkg test-update test-short run install help release fmt lint tidy release-dry

# Version information (can be overridden: make build VERSION=1.0.0)
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Bugsnag configuration (optional: make build BUGSNAG_API_KEY=your-key)
BUGSNAG_API_KEY ?=
BUGSNAG_RELEASE_STAGE ?= prod

# Linker flags to inject version information and optional Bugsnag configuration
LDFLAGS := -X 'github.com/cerebriumai/cerebrium/internal/version.Version=$(VERSION)' \
           -X 'github.com/cerebriumai/cerebrium/internal/version.Commit=$(COMMIT)' \
           -X 'github.com/cerebriumai/cerebrium/internal/version.BuildDate=$(BUILD_DATE)'

# Add Bugsnag flags if API key is provided
ifdef BUGSNAG_API_KEY
LDFLAGS += -X 'github.com/cerebriumai/cerebrium/pkg/bugsnag.BugsnagAPIKey=$(BUGSNAG_API_KEY)' \
           -X 'github.com/cerebriumai/cerebrium/pkg/bugsnag.DefaultReleaseStage=$(BUGSNAG_RELEASE_STAGE)'
endif

# Build the binary
build:
	@echo "Building cerebrium CLI..."
	@go build -ldflags "$(LDFLAGS)" -o ./bin/cerebrium ./cmd/cerebrium
	@echo "Build complete: ./bin/cerebrium"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf ./bin
	@rm -f coverage.out coverage.html
	@go clean
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	@go test -v -shuffle=on ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests for a specific package
test-pkg:
	@echo "Running tests for specific package..."
	@go test -v ./$(PKG)

# Update golden files for UI tests
generate:
	@echo "Running go generates..."
	@go generate ./...
	@mockery

# Run short tests only (skip long-running tests)
test-short:
	@echo "Running short tests..."
	@go test -v -short ./...

# Run the binary
run: build
	@./bin/cerebrium version

# Install the binary to GOPATH/bin
install:
	@echo "Installing cerebrium..."
	@go install ./cmd/cerebrium
	@echo "Install complete"

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete"

# Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	@go mod tidy
	@echo "Tidy complete"

# Release using GoReleaser (dry-run)
release-dry:
	@echo "Running GoReleaser in dry-run mode..."
	@goreleaser release --snapshot --clean --skip=publish

# Release using GoReleaser (requires git tag)
release:
	@echo "Running GoReleaser..."
	@goreleaser release --clean

# Help command
help:
	@echo "Available targets:"
	@echo "  build         - Build the cerebrium binary to ./bin/cerebrium"
	@echo "  clean         - Remove build artifacts"
	@echo "  test          - Run all tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  test-pkg      - Run tests for specific package (make test-pkg PKG=internal/api)"
	@echo "  test-update   - Update golden test files for UI tests"
	@echo "  test-short    - Run short tests only (skip long-running tests)"
	@echo "  run           - Build and run the binary"
	@echo "  install       - Install the binary to GOPATH/bin"
	@echo "  fmt           - Format code"
	@echo "  lint          - Run linter"
	@echo "  tidy          - Tidy dependencies"
	@echo "  release-dry   - Test release build with GoReleaser (no publish)"
	@echo "  release       - Create release with GoReleaser (requires git tag)"
	@echo "  help          - Show this help message"
