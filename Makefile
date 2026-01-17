.PHONY: help build install clean test lint snapshot

# Build variables
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X github.com/boolean-maybe/tiki/config.Version=$(VERSION) -X github.com/boolean-maybe/tiki/config.GitCommit=$(COMMIT) -X github.com/boolean-maybe/tiki/config.BuildDate=$(DATE)"

# Default target
help:
	@echo "Available targets:"
	@echo "  build      - Build the tiki binary with version injection"
	@echo "  install    - Build and install to GOPATH/bin"
	@echo "  clean      - Remove built binaries and dist directory"
	@echo "  test       - Run all tests"
	@echo "  lint       - Run golangci-lint"
	@echo "  snapshot   - Create a snapshot release with GoReleaser"
	@echo "  help       - Show this help message"

# Build the binary
build:
	@echo "Building tiki $(VERSION)..."
	go build $(LDFLAGS) -o tiki .

# Build and install to GOPATH/bin
install:
	@echo "Installing tiki $(VERSION)..."
	go install $(LDFLAGS) .

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f tiki
	rm -rf dist/

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Create a snapshot release (local testing)
snapshot:
	@echo "Creating snapshot release..."
	goreleaser release --snapshot --clean
