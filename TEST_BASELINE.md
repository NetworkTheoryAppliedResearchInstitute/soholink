# SoHoLINK Test Baseline

This document tracks the current state of the test suite and serves as a baseline for measuring progress.

## Test Infrastructure

### Test Runner Scripts

- **scripts/test.sh** - Unix/Linux/macOS test runner
- **scripts/test.bat** - Windows test runner
- **Makefile targets**:
  - `make test` - Full test suite with coverage
  - `make test-short` - Quick tests only

### Running Tests

```bash
# Run all tests with coverage
make test

# Run short tests only (skip slow integration tests)
make test-short

# Run with custom script
./scripts/test.sh --short
./scripts/test.sh --no-race --no-coverage

# Windows
scripts\test.bat --short
```

## Current Test Coverage

### Packages with Tests

As of Phase 0 completion (2026-02-09):

| Package | Test File | Status | Coverage Target |
|---------|-----------|--------|-----------------|
| `internal/did` | `didkey_test.go` | ✅ Present | 80%+ |
| `internal/store` | `store_test.go` | ✅ Present | 75%+ |
| `internal/accounting` | `collector_test.go` | ✅ Present | 80%+ |
| `internal/merkle` | `tree_test.go` | ✅ Present | 85%+ |
| `internal/policy` | `engine_test.go` | ✅ Present | 75%+ |
| `internal/verifier` | `verifier_test.go` | ✅ Present | 85%+ |
| `internal/blockchain` | `local_test.go` | ✅ Present | 70%+ |

### Packages Needing Tests

Priority packages that need test coverage:

| Package | Priority | Reason |
|---------|----------|--------|
| `internal/radius` | HIGH | Core authentication protocol |
| `internal/app` | HIGH | Application entry point |
| `internal/config` | MEDIUM | Configuration parsing |
| `internal/cli` | MEDIUM | CLI commands |
| `internal/thinclient` | HIGH | P2P mesh networking |
| `internal/orchestrator` | HIGH | Workload scheduling |
| `internal/compute` | HIGH | Container/VM isolation |
| `internal/payment` | HIGH | Payment processing |
| `internal/services` | MEDIUM | Managed services |
| `internal/sla` | MEDIUM | SLA monitoring |
| `internal/cdn` | LOW | CDN routing |
| `internal/lbtas` | MEDIUM | Reputation system |

## Test Execution Baseline

### Expected Test Results (Phase 0)

**As of 2026-02-09:**

```
Package                                              Status
-------                                              ------
github.com/...soholink/internal/did                  PASS
github.com/...soholink/internal/store                PASS (with minor import warnings)
github.com/...soholink/internal/accounting           PASS
github.com/...soholink/internal/merkle               PASS
github.com/...soholink/internal/policy               PASS
github.com/...soholink/internal/verifier             PASS
github.com/...soholink/internal/blockchain           PASS
```

**Known Issues:**
- GUI tests require `-tags gui` build flag (intentionally excluded from standard test run)
- Some integration tests may be skipped in `-short` mode
- Race detection may slow down tests significantly (expect 2-5x longer runtime)

### Test Performance

**Typical runtimes:**

- **Short mode** (`make test-short`): ~5-15 seconds
- **Full mode** (`make test`): ~30-60 seconds
- **With race detection**: ~60-180 seconds

## Coverage Goals

### Phase 0 Baseline (Current)

- **Overall Coverage**: Establish baseline (likely 40-60%)
- **Critical Paths**: 70%+ coverage for authentication/authorization

### Phase 1 Target

- **Overall Coverage**: 60%+
- **Critical Packages**: 80%+ for `radius`, `verifier`, `store`, `merkle`

### Phase 2 Target

- **Overall Coverage**: 70%+
- **All Core Packages**: 75%+ coverage

### Phase 5 (Production Ready)

- **Overall Coverage**: 75%+
- **Critical Packages**: 90%+ for security-sensitive code
- **Integration Tests**: Full end-to-end scenarios

## Test Categories

### Unit Tests

Focus on individual functions and methods in isolation.

**Example packages:**
- `internal/did` - DID parsing and validation
- `internal/verifier` - Credential verification logic
- `internal/merkle` - Merkle tree construction

### Integration Tests

Test interactions between components.

**Example scenarios:**
- RADIUS authentication flow
- Database operations
- Policy engine evaluation

### End-to-End Tests

Full system tests (future phase).

**Planned scenarios:**
- Complete authentication request from radclient → RADIUS → verification → response
- User creation → credential generation → authentication → accounting
- P2P mesh discovery → block synchronization → consensus

## Testing Best Practices

### Writing Tests

1. **Use table-driven tests** for multiple input scenarios
2. **Test edge cases**: empty inputs, nil values, boundary conditions
3. **Use meaningful test names**: `TestVerifyCredential_ExpiredToken_ReturnsError`
4. **Mock external dependencies**: database, network, filesystem
5. **Keep tests fast**: < 1 second per test in unit tests

### Test Organization

```
package mypackage

func TestFunctionName(t *testing.T) {
	// Setup
	// Execute
	// Assert
}

func TestFunctionName_EdgeCase(t *testing.T) {
	// ...
}
```

### Running Specific Tests

```bash
# Single package
go test -v ./internal/verifier/

# Single test
go test -v -run TestVerifyCredential ./internal/verifier/

# Verbose with race detection
go test -v -race -run TestVerifyCredential ./internal/verifier/
```

## Continuous Integration

### GitHub Actions Integration

Future CI/CD will run tests on:

- **Every commit** to feature branches
- **Every pull request**
- **Before merging** to main

**Test matrix:**
- Go 1.22, 1.23, 1.24
- Linux (Ubuntu), macOS, Windows
- With and without race detection

### Pre-Commit Hooks

Recommended local pre-commit hook:

```bash
#!/bin/bash
# .git/hooks/pre-commit

echo "Running tests before commit..."
make test-short

if [ $? -ne 0 ]; then
    echo "Tests failed! Commit aborted."
    exit 1
fi
```

## Updating This Baseline

**When to update:**

1. After completing a major phase (Phase 1, 2, 3, etc.)
2. After adding significant test coverage
3. When test infrastructure changes
4. When coverage targets are adjusted

**What to update:**

- Test count and package coverage
- Performance benchmarks
- Known issues
- Coverage goals

## Resources

- [Go Testing Package](https://pkg.go.dev/testing)
- [Table-Driven Tests in Go](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [Advanced Testing with Go](https://www.youtube.com/watch?v=8hQG7QlcLBk)
- [Go Test Coverage](https://go.dev/blog/cover)

---

**Last Updated:** 2026-02-09 (Phase 0 Completion)
**Next Review:** After Phase 1 completion
