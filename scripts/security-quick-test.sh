#!/bin/bash
# SoHoLINK Security Quick Test
# Run this before every commit to catch security issues early

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_ROOT"

echo "=================================="
echo "SoHoLINK Security Quick Test"
echo "=================================="
echo ""

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counters
PASS=0
FAIL=0
WARN=0

# Helper functions
pass() {
    echo -e "${GREEN}✅ PASS${NC}: $1"
    ((PASS++))
}

fail() {
    echo -e "${RED}❌ FAIL${NC}: $1"
    ((FAIL++))
}

warn() {
    echo -e "${YELLOW}⚠️  WARN${NC}: $1"
    ((WARN++))
}

# Test 1: Check for dangerous command patterns
echo "[1/8] Checking for dangerous command execution patterns..."
if grep -r "exec.Command.*sh.*-c" internal/ 2>/dev/null | grep -v "_test.go"; then
    fail "Found 'sh -c' pattern in exec.Command (command injection risk)"
else
    pass "No dangerous 'sh -c' patterns found"
fi

# Test 2: Check for SQL string concatenation
echo "[2/8] Checking for SQL injection risks..."
if grep -r "\"SELECT.*+.*\"" internal/ 2>/dev/null; then
    fail "Found SQL string concatenation (SQL injection risk)"
else
    pass "No SQL string concatenation found"
fi

# Test 3: Check for hardcoded secrets
echo "[3/8] Checking for hardcoded secrets..."
SECRETS_FOUND=0
if grep -ri "password.*=.*\"" internal/ 2>/dev/null | grep -v "_test.go" | grep -v "User-Password"; then
    ((SECRETS_FOUND++))
fi
if grep -ri "api.*key.*=.*\"" internal/ 2>/dev/null | grep -v "_test.go"; then
    ((SECRETS_FOUND++))
fi
if grep -ri "secret.*=.*\"" internal/ 2>/dev/null | grep -v "_test.go" | grep -v "shared_secret.*testing123" | grep -v "SharedSecret"; then
    ((SECRETS_FOUND++))
fi

if [ $SECRETS_FOUND -gt 0 ]; then
    warn "Found potential hardcoded secrets (review manually)"
else
    pass "No obvious hardcoded secrets found"
fi

# Test 4: Check for insecure random number generation
echo "[4/8] Checking for insecure random number generation..."
if grep -r "math/rand" internal/ 2>/dev/null | grep -v "_test.go" | grep -v "// crypto/rand"; then
    fail "Found math/rand usage (use crypto/rand for security)"
else
    pass "No insecure random number generation found"
fi

# Test 5: Check for timing attack vulnerabilities
echo "[5/8] Checking for timing attack vulnerabilities..."
if grep -r "== .*signature\|signature.*==" internal/verifier/ 2>/dev/null | grep -v "_test.go" | grep -v "//"; then
    warn "Found direct signature comparison (use subtle.ConstantTimeCompare)"
elif grep -r "bytes.Equal.*signature\|signature.*bytes.Equal" internal/verifier/ 2>/dev/null | grep -v "_test.go"; then
    warn "Found bytes.Equal for signature (use subtle.ConstantTimeCompare)"
else
    pass "No obvious timing attack vulnerabilities in signature verification"
fi

# Test 6: Check for missing error handling
echo "[6/8] Checking for missing error handling..."
MISSING_ERROR_CHECKS=$(grep -r "err :=" internal/ 2>/dev/null | grep -v "_test.go" | wc -l)
ERROR_CHECKS=$(grep -r "if err != nil" internal/ 2>/dev/null | grep -v "_test.go" | wc -l)

if [ $ERROR_CHECKS -lt $((MISSING_ERROR_CHECKS / 2)) ]; then
    warn "Fewer error checks than error assignments (review error handling)"
else
    pass "Error handling looks reasonable"
fi

# Test 7: Check for file permission issues
echo "[7/8] Checking for file permission issues..."
if grep -r "os.WriteFile.*0666\|os.WriteFile.*0777" internal/ 2>/dev/null | grep -v "_test.go"; then
    fail "Found overly permissive file permissions (0666 or 0777)"
else
    pass "No overly permissive file permissions found"
fi

# Test 8: Check for path traversal risks
echo "[8/8] Checking for path traversal protection..."
HAS_FILEPATH_CLEAN=$(grep -r "filepath.Clean" internal/ 2>/dev/null | wc -l)
HAS_FILE_OPS=$(grep -r "os.ReadFile\|os.WriteFile\|os.Open\|os.Create" internal/ 2>/dev/null | grep -v "_test.go" | wc -l)

if [ $HAS_FILE_OPS -gt 10 ] && [ $HAS_FILEPATH_CLEAN -lt 3 ]; then
    warn "Many file operations but few filepath.Clean() calls (check for path traversal protection)"
else
    pass "Path handling looks reasonable"
fi

echo ""
echo "=================================="
echo "Security Quick Test Summary"
echo "=================================="
echo -e "${GREEN}Passed: $PASS${NC}"
echo -e "${YELLOW}Warnings: $WARN${NC}"
echo -e "${RED}Failed: $FAIL${NC}"
echo ""

if [ $FAIL -gt 0 ]; then
    echo -e "${RED}❌ SECURITY ISSUES FOUND - MUST FIX BEFORE COMMIT${NC}"
    echo "See SECURITY_FINDINGS.md for details and remediation steps"
    exit 1
elif [ $WARN -gt 2 ]; then
    echo -e "${YELLOW}⚠️  MULTIPLE WARNINGS - REVIEW BEFORE COMMIT${NC}"
    echo "See SECURITY_FINDINGS.md for best practices"
    exit 0
else
    echo -e "${GREEN}✅ SECURITY QUICK TEST PASSED${NC}"
    echo "Run full security audit with: gosec ./..."
    exit 0
fi
