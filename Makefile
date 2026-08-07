# MinusSync Makefile
# Cross-platform build system for msync and msyncd

BINARY_MSYNC=msync
BINARY_MSYNCD=msyncd
CMD_MSYNC=./cmd/msync
CMD_MSYNCD=./cmd/msyncd

VERSION?=1.0.0
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X github.com/MinusSync/internal/cmd.Version=$(VERSION) -X github.com/MinusSync/internal/cmd.GitCommit=$(GIT_COMMIT) -X github.com/MinusSync/internal/cmd.BuildDate=$(BUILD_DATE)"

GO=go
GOOS?=$(shell go env GOOS)
GOARCH?=$(shell go env GOARCH)
CGO_ENABLED?=0

.PHONY: all build build-all test clean install fmt lint

all: build

# Build for current platform
build:
	$(GO) build $(LDFLAGS) -o $(BINARY_MSYNC) $(CMD_MSYNC)
	$(GO) build $(LDFLAGS) -o $(BINARY_MSYNCD) $(CMD_MSYNCD)

# Cross-compile for all target platforms
build-all: build-linux-amd64 build-linux-arm64 build-windows-amd64

build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(LDFLAGS) -o $(BINARY_MSYNC)-linux-amd64 $(CMD_MSYNC)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(LDFLAGS) -o $(BINARY_MSYNCD)-linux-amd64 $(CMD_MSYNCD)

build-linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(LDFLAGS) -o $(BINARY_MSYNC)-linux-arm64 $(CMD_MSYNC)
	GOOS=linux GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(LDFLAGS) -o $(BINARY_MSYNCD)-linux-arm64 $(CMD_MSYNCD)

build-windows-amd64:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(LDFLAGS) -o $(BINARY_MSYNC)-windows-amd64.exe $(CMD_MSYNC)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(LDFLAGS) -o $(BINARY_MSYNCD)-windows-amd64.exe $(CMD_MSYNCD)

# Run tests
test:
	$(GO) test ./... -v -count=1

# Run tests with race detector
test-race:
	$(GO) test ./... -race -count=1

# Clean build artifacts
clean:
	rm -f $(BINARY_MSYNC) $(BINARY_MSYNCD)
	rm -f $(BINARY_MSYNC)-* $(BINARY_MSYNCD)-*

# Install to system
install: build
	install -d $(DESTDIR)/usr/local/bin
	install -m 755 $(BINARY_MSYNC) $(DESTDIR)/usr/local/bin/$(BINARY_MSYNC)
	install -m 755 $(BINARY_MSYNCD) $(DESTDIR)/usr/local/bin/$(BINARY_MSYNCD)
	install -d $(DESTDIR)/etc/msyncd
	install -d $(DESTDIR)/usr/lib/systemd/system
	install -m 644 contrib/systemd/msyncd@.service $(DESTDIR)/usr/lib/systemd/system/
	install -m 644 contrib/systemd/msyncd.socket $(DESTDIR)/usr/lib/systemd/system/

# Format code
fmt:
	$(GO) fmt ./...

# Lint (requires golangci-lint)
lint:
	golangci-lint run ./...

# Show help
help:
	@echo "MinusSync Build System"
	@echo ""
	@echo "Targets:"
	@echo "  build            Build for current platform"
	@echo "  build-all        Cross-compile for all targets"
	@echo "  build-linux-amd64   Linux x86_64"
	@echo "  build-linux-arm64   Linux ARM64 (Termux/Android)"
	@echo "  build-windows-amd64 Windows x86_64"
	@echo "  test             Run unit tests"
	@echo "  test-race        Run tests with race detector"
	@echo "  clean            Remove build artifacts"
	@echo "  install          Install binaries and systemd units"
	@echo "  fmt              Format Go code"
	@echo "  lint             Run linter"
