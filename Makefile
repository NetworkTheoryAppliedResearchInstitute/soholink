.PHONY: all build build-cli build-gui build-vendor build-pi test test-short lint clean install vendor

VERSION ?= 0.1.0
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

all: build-cli

deps:
	go mod download
	go mod tidy

# Vendor dependencies for offline builds
vendor:
	go mod vendor
	@echo "Dependencies vendored to ./vendor/"
	@echo "To build with vendored deps: make build-vendor"

# CLI build (no GUI dependencies)
build:
	go build $(LDFLAGS) -o bin/fedaaa ./cmd/fedaaa

# Build using vendored dependencies (offline-capable)
build-vendor:
	go build $(LDFLAGS) -mod=vendor -o bin/fedaaa ./cmd/fedaaa

# Alias for clarity
build-cli: build

# GUI build (includes Fyne GUI toolkit)
build-gui:
	go build $(LDFLAGS) -tags gui -o bin/fedaaa-gui ./cmd/fedaaa-gui

build-pi:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/fedaaa-linux-arm64 ./cmd/fedaaa

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/fedaaa-linux-amd64 ./cmd/fedaaa

# GUI builds for different platforms
build-gui-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -tags gui -o bin/fedaaa-gui.exe ./cmd/fedaaa-gui

build-gui-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -tags gui -o bin/fedaaa-gui-linux ./cmd/fedaaa-gui

build-gui-macos:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -tags gui -o bin/fedaaa-gui-macos ./cmd/fedaaa-gui

test:
	go test -v -race -coverprofile=coverage.out ./internal/...
	go tool cover -html=coverage.out -o coverage.html

test-short:
	go test -v -short ./internal/...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

install: build
	sudo cp bin/fedaaa /usr/local/bin/
