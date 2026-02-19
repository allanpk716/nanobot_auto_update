# Makefile for nanobot-auto-updater
# Provides both console (debug) and GUI (release) builds

.PHONY: build build-release clean test help

# Version from git tags, or "dev" if unavailable
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Build flags for release (GUI subsystem + version embedding)
LDFLAGS_RELEASE = -H=windowsgui -X main.Version=$(VERSION)

# Default: console build (easier debugging)
build:
	go build -o nanobot-auto-updater.exe ./cmd/nanobot-auto-updater
	@echo "Built console version: nanobot-auto-updater.exe"

# Release: GUI build (no console window)
build-release:
	go build -ldflags="$(LDFLAGS_RELEASE)" -o nanobot-auto-updater.exe ./cmd/nanobot-auto-updater
	@echo "Built release version (no console): nanobot-auto-updater.exe"

# Clean build artifacts
clean:
	rm -f nanobot-auto-updater.exe
	@echo "Cleaned build artifacts"

# Run all tests
test:
	go test ./...

# Show help
help:
	@echo "Available targets:"
	@echo "  make build         - Build console version (for debugging)"
	@echo "  make build-release - Build GUI version (for distribution)"
	@echo "  make test          - Run tests"
	@echo "  make clean         - Remove build artifacts"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION=x.x.x      - Set version (default: git tag or 'dev')"
