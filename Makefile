.PHONY: help build install clean test lint snapshot

# Build variables
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo "dev")
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
	go build $(LDFLAGS) -o bin/tiki .

# Build, sign, and install to ~/.local/bin
install: build
	@echo "Installing tiki to ~/.local/bin..."
	@mkdir -p ~/.local/bin
	cp bin/tiki ~/.local/bin/tiki
ifeq ($(shell uname),Darwin)
	codesign -s - ~/.local/bin/tiki
endif

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
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
