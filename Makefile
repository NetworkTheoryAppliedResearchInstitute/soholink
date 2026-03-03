.PHONY: all build build-cli build-gui build-vendor build-pi build-wizards build-installer-windows \
        build-static-windows fyne-package-windows fyne-package-linux fyne-package-macos \
        dist dist-release test test-short lint clean install vendor

VERSION ?= 0.1.0
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"
LDFLAGS_GUI := -ldflags "-s -w -H=windowsgui -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)"

all: build-cli build-wizards

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

# Build wizards (GUI and CLI)
build-wizards:
	go build $(LDFLAGS_GUI) -o bin/soholink-wizard.exe ./cmd/soholink-wizard
	go build $(LDFLAGS) -o bin/soholink-wizard-cli.exe ./cmd/soholink-wizard-cli

# Build wizard demo
build-wizard-demo:
	go build $(LDFLAGS) -o bin/wizard-demo.exe ./cmd/wizard-demo

# Build complete Windows installer package
build-installer-windows: build-wizards
	@echo "Building Windows installer package..."
	powershell -ExecutionPolicy Bypass -File ./scripts/build-installer-windows.ps1 -Version $(VERSION)

# Build complete Windows installer with embedded Go
build-installer-windows-portable: build-wizards
	@echo "Building portable Windows installer package (with embedded Go)..."
	powershell -ExecutionPolicy Bypass -File ./scripts/build-installer-windows.ps1 -Version $(VERSION)

# Quick build for testing (skips Go download)
build-installer-windows-quick: build-wizards
	@echo "Building Windows installer package (quick mode)..."
	powershell -ExecutionPolicy Bypass -File ./scripts/build-installer-windows.ps1 -Version $(VERSION) -SkipGoDownload

# ── Static Windows .exe — no MinGW DLLs needed on user machine ──────
#    Requires MinGW-w64 GCC in PATH (winget install msys2 → pacman -S mingw-w64-x86_64-gcc)
#    The -static-libgcc/-static-libstdc++ flags embed the C++ runtime directly into
#    the .exe so end-users need ZERO extra DLLs or runtimes.
LDFLAGS_STATIC_WIN := -ldflags "-s -w -H=windowsgui \
  -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME) \
  -extldflags \"-static-libgcc -static-libstdc++ -static-libpthread\""

build-static-windows:
	CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ \
	GOOS=windows GOARCH=amd64 \
	go build -tags gui $(LDFLAGS_STATIC_WIN) \
	  -o bin/soholink.exe ./cmd/soholink
	@echo "Static Windows binary ready: bin/soholink.exe (no DLL dependencies)"

# ── Fyne package — embeds icon + manifest, produces platform-native bundle ──
#    Requires: go install fyne.io/fyne/v2/cmd/fyne@latest
#    FyneApp.toml is read automatically from the working directory.
fyne-package-windows:
	fyne package -os windows -icon assets/logo.png -name SoHoLINK -appID io.soholink.app
	@echo "Windows bundle ready (look for SoHoLINK.exe with embedded manifest)"

fyne-package-linux:
	fyne package -os linux -icon assets/logo.png -name SoHoLINK -appID io.soholink.app
	@echo "Linux bundle ready"

fyne-package-macos:
	fyne package -os darwin -icon assets/logo.png -name SoHoLINK -appID io.soholink.app
	@echo "macOS .app bundle ready"

# ── One-command release build — all platforms via GoReleaser ────────
#    Requires: go install github.com/goreleaser/goreleaser/v2@latest
#    Snapshot mode: no git tag required, version gets a -dev-<commit> suffix.
dist:
	goreleaser release --snapshot --clean
	@echo ""
	@echo "All platform packages are in dist/:"
	@ls dist/*.zip dist/*.tar.gz dist/*.deb dist/*.rpm dist/*.exe 2>/dev/null || true

# Real release — requires a git tag (e.g. git tag v0.1.0 && make dist-release)
dist-release:
	goreleaser release --clean

install: build
	sudo cp bin/fedaaa /usr/local/bin/

# Help target
help:
	@echo "SoHoLINK Build Targets:"
	@echo ""
	@echo "  ── Standard builds ─────────────────────────────────────────────────────"
	@echo "  make build-cli                        Build headless CLI binary (no CGO)"
	@echo "  make build-gui                        Build GUI binary (requires CGO + GCC)"
	@echo "  make build-wizards                    Build configuration wizards (GUI + CLI)"
	@echo "  make build-pi                         Cross-compile for Raspberry Pi (ARM64)"
	@echo ""
	@echo "  ── Zero-dependency Windows .exe ────────────────────────────────────────"
	@echo "  make build-static-windows             Statically links MinGW runtime into .exe"
	@echo "                                        → bin/soholink.exe (no DLLs needed)"
	@echo "  make build-installer-windows          Full NSIS setup wizard (.exe installer)"
	@echo "  make build-installer-windows-quick    Same, skips Go download (faster)"
	@echo ""
	@echo "  ── Fyne platform bundles ───────────────────────────────────────────────"
	@echo "  make fyne-package-windows             .exe with embedded icon + manifest"
	@echo "  make fyne-package-linux               Linux bundle with desktop integration"
	@echo "  make fyne-package-macos               macOS .app bundle"
	@echo "  (requires: go install fyne.io/fyne/v2/cmd/fyne@latest)"
	@echo ""
	@echo "  ── One-command all-platform release (GoReleaser) ───────────────────────"
	@echo "  make dist                             Snapshot build, all platforms → dist/"
	@echo "                                        Produces: .exe, .zip, .tar.gz, .deb, .rpm"
	@echo "  make dist-release                     Real release (requires git tag v*)"
	@echo "  (requires: go install github.com/goreleaser/goreleaser/v2@latest)"
	@echo ""
	@echo "  ── Testing & quality ───────────────────────────────────────────────────"
	@echo "  make test                             Run all tests with race detector + coverage"
	@echo "  make test-short                       Run quick tests (no race detector)"
	@echo "  make lint                             Run golangci-lint"
	@echo ""
	@echo "  ── Utilities ───────────────────────────────────────────────────────────"
	@echo "  make clean                            Remove build artifacts (bin/, coverage)"
	@echo "  make vendor                           Vendor dependencies for offline builds"
	@echo "  make deps                             Download and tidy Go modules"
	@echo ""
