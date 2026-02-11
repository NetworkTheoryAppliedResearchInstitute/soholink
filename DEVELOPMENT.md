# SoHoLINK Development Guide

This guide covers development workflows, dependency management, and best practices for contributing to SoHoLINK.

## Development Setup

### Prerequisites

1. **Go 1.22 or later** - [Download](https://go.dev/dl/)
2. **Git** - For version control
3. **Make** - Build automation (optional)
4. **golangci-lint** - Code linting (optional): `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`

### Initial Setup

```bash
# Clone the repository
git clone https://github.com/NetworkTheoryAppliedResearchInstitute/soholink.git
cd soholink

# Download dependencies
make deps

# Build the CLI
make build-cli

# Run tests
make test-short
```

## Dependency Management

SoHoLINK uses Go modules for dependency management and supports **vendoring** for offline-first development.

### Working with Dependencies

#### Adding a New Dependency

```bash
# Add the dependency in your code
import "github.com/example/package"

# Download and update go.mod/go.sum
go get github.com/example/package@latest

# Tidy up
go mod tidy
```

#### Updating Dependencies

```bash
# Update all dependencies to latest compatible versions
go get -u ./...
go mod tidy

# Update a specific dependency
go get -u github.com/example/package@v1.2.3
go mod tidy
```

#### Checking for Outdated Dependencies

```bash
# List available updates
go list -u -m all

# Or use go-mod-outdated
go install github.com/psampaz/go-mod-outdated@latest
go list -u -m -json all | go-mod-outdated -update -direct
```

### Vendor Mode (Offline Development)

Vendoring copies all dependencies into the `vendor/` directory, allowing builds without network access.

#### Creating Vendor Directory

```bash
# Vendor all dependencies
make vendor
# or
go mod vendor
```

This creates a `vendor/` directory with all dependencies. **Note:** The `vendor/` directory is in `.gitignore` and should NOT be committed.

#### Building with Vendored Dependencies

```bash
# Build using vendor directory
make build-vendor

# Or manually
go build -mod=vendor -o bin/fedaaa ./cmd/fedaaa
```

#### When to Use Vendor Mode

- **Air-gapped environments** - No internet connectivity
- **CI/CD caching** - Faster builds by caching vendor directory
- **Reproducible builds** - Guaranteed dependency versions
- **Offline development** - Working without network access

#### Vendor Workflow

```bash
# 1. Update dependencies normally
go get -u github.com/some/package
go mod tidy

# 2. Re-vendor after any go.mod changes
make vendor

# 3. Build using vendored deps
make build-vendor

# 4. Verify everything works
./bin/fedaaa version
```

## Build Modes

See [BUILD.md](BUILD.md) for comprehensive build documentation.

### Quick Reference

```bash
# CLI-only (no GUI)
make build-cli

# GUI build
make build-gui

# Offline build (uses vendor/)
make build-vendor

# Cross-compilation
make build-pi          # Raspberry Pi ARM64
make build-linux-amd64 # Linux x86-64
```

## Testing

### Running Tests

```bash
# All tests with coverage
make test

# Quick tests only
make test-short

# Specific package
go test -v ./internal/radius/...

# With race detection
go test -race ./...

# Verbose output
go test -v ./...
```

### Writing Tests

#### Unit Test Example

```go
// internal/verifier/verifier_test.go
package verifier

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestVerifyCredential(t *testing.T) {
	v := NewVerifier()

	t.Run("valid credential", func(t *testing.T) {
		cred := "valid-token"
		err := v.Verify(cred)
		assert.NoError(t, err)
	})

	t.Run("invalid credential", func(t *testing.T) {
		cred := "invalid-token"
		err := v.Verify(cred)
		assert.Error(t, err)
	})
}
```

#### Table-Driven Tests

```go
func TestParseCredential(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Credential
		wantErr bool
	}{
		{
			name:    "valid credential",
			input:   "base64-encoded-token",
			want:    &Credential{...},
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   "",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCredential(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
```

### Test Coverage

```bash
# Generate coverage report
make test

# View HTML coverage report
open coverage.html  # macOS
xdg-open coverage.html  # Linux
start coverage.html  # Windows
```

## Code Style and Linting

### Running Linters

```bash
# Run all linters
make lint

# Or manually
golangci-lint run ./...
```

### Pre-Commit Checks

Before committing code, ensure:

1. **Code compiles**: `make build-cli && make build-gui`
2. **Tests pass**: `make test-short`
3. **Linting clean**: `make lint`
4. **go.mod tidy**: `go mod tidy`

### Formatting

```bash
# Format all Go files
go fmt ./...

# Or use gofmt directly
gofmt -s -w .
```

## Project Structure

```
soholink/
├── cmd/
│   ├── fedaaa/           # CLI entry point (no GUI)
│   └── fedaaa-gui/       # GUI entry point (Fyne)
├── internal/
│   ├── app/              # Application core
│   ├── auth/             # Authentication logic
│   ├── cli/              # CLI commands
│   ├── config/           # Configuration management
│   ├── gui/              # GUI components (Fyne)
│   ├── radius/           # RADIUS protocol
│   ├── store/            # Database layer
│   └── verifier/         # Credential verification
├── configs/              # Default configuration files
├── docs/                 # Documentation
├── go.mod                # Go module definition
├── go.sum                # Dependency checksums
├── Makefile              # Build automation
├── BUILD.md              # Build instructions
├── DEVELOPMENT.md        # This file
├── PLAN.md               # Implementation roadmap
└── README.md             # Project overview
```

## Common Tasks

### Adding a New CLI Command

1. Create command file in `internal/cli/`:

```go
// internal/cli/mycommand.go
package cli

import "github.com/spf13/cobra"

func newMyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mycommand",
		Short: "Description of my command",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Implementation
			return nil
		},
	}
	return cmd
}
```

2. Register in `internal/cli/root.go`:

```go
rootCmd.AddCommand(newMyCommand())
```

### Adding a Database Migration

1. Add SQL to `internal/store/schema.go`:

```go
const schema = `
-- Existing tables...

-- New table
CREATE TABLE IF NOT EXISTS my_new_table (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`
```

2. Database is auto-migrated on startup.

### Adding Configuration Options

1. Update `internal/config/config.go`:

```go
type Config struct {
	// Existing fields...

	MyFeature MyFeatureConfig `mapstructure:"my_feature"`
}

type MyFeatureConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Option  string `mapstructure:"option"`
}
```

2. Add defaults in `embed.go`:

```yaml
my_feature:
  enabled: false
  option: "default_value"
```

## Troubleshooting

### "package fyne.io/fyne/v2 is not in GOROOT"

**Solution:** Run `make deps` or `go mod download`

### "build constraints exclude all Go files"

**Problem:** Trying to build GUI code without the `gui` tag.

**Solution:** Use `make build-gui` or add `-tags gui` to your build command.

### Tests Failing After Dependency Update

**Solution:**
```bash
# Clear module cache
go clean -modcache

# Re-download dependencies
go mod download

# Re-vendor if using vendor mode
make vendor

# Re-run tests
make test-short
```

### Vendor Directory Out of Sync

**Solution:**
```bash
# Remove vendor directory
rm -rf vendor/

# Re-vendor
make vendor
```

## Contributing

### Pull Request Process

1. **Fork** the repository
2. **Create a feature branch**: `git checkout -b feature/my-feature`
3. **Make your changes**
4. **Run tests**: `make test-short`
5. **Run linters**: `make lint`
6. **Commit** with clear messages: `git commit -m "feat: add feature X"`
7. **Push** to your fork: `git push origin feature/my-feature`
8. **Open a Pull Request** against `main`

### Commit Message Format

Follow [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): subject

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Formatting changes
- `refactor`: Code restructuring
- `test`: Adding tests
- `chore`: Build/tooling changes

**Examples:**
```
feat(radius): add support for RADIUS accounting
fix(verifier): correct signature validation logic
docs(readme): update installation instructions
```

## Resources

- [Go Documentation](https://go.dev/doc/)
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Fyne Documentation](https://developer.fyne.io/)
- [RADIUS Protocol RFC 2865](https://datatracker.ietf.org/doc/html/rfc2865)
