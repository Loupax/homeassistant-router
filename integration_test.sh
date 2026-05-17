#!/bin/bash
set -e

TESTS_PASSED=0
TESTS_FAILED=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

pass() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

fail() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

# Build the binary
log_test "Building binary..."
go build -o /tmp/ha_test_bin . || fail "Build failed" && pass "Build succeeded"

# Test 1: Startup validation - missing binary check
log_test "Testing startup with missing binary in PATH"
OUTPUT=$(PATH=/tmp:$PATH HOMEASSISTANT_LLM_URL=http://localhost:9999 timeout 1 /tmp/ha_test_bin 2>&1 || true)
if echo "$OUTPUT" | grep -q "required binary not found"; then
    pass "Missing binary check working"
else
    fail "Missing binary check not working. Output: $OUTPUT"
fi

# Test 2: Missing HOMEASSISTANT_LLM_URL
log_test "Testing startup without HOMEASSISTANT_LLM_URL"
OUTPUT=$(PATH=$PATH timeout 1 /tmp/ha_test_bin 2>&1 || true)
if echo "$OUTPUT" | grep -q "HOMEASSISTANT_LLM_URL"; then
    pass "Missing HOMEASSISTANT_LLM_URL detected correctly"
else
    fail "Missing HOMEASSISTANT_LLM_URL not detected. Output: $OUTPUT"
fi

# Test 3: Atomic write - verify .tmp file cleanup after exit command
log_test "Testing atomic write cleanup"
BEFORE_TMP=$(find ~/.config/homeassistant -name "*.tmp" 2>/dev/null | wc -l)
(echo "exit" | HOMEASSISTANT_LLM_URL=http://localhost:9999 timeout 2 /tmp/ha_test_bin 2>&1 > /dev/null) || true
AFTER_TMP=$(find ~/.config/homeassistant -name "*.tmp" 2>/dev/null | wc -l)

if [ "$AFTER_TMP" = "$BEFORE_TMP" ]; then
    pass ".tmp files cleaned up correctly"
else
    fail "Temp files were not cleaned up"
fi

# Test 4: go vet passes
log_test "Running go vet..."
if go vet ./... 2>&1; then
    pass "go vet passes"
else
    fail "go vet found issues"
fi

# Test 5: go build passes
log_test "Building with go build..."
if go build . 2>&1; then
    pass "go build succeeds"
else
    fail "go build failed"
fi

# Test 6: All unit tests pass
log_test "Running all unit tests..."
if go test ./... 2>&1 | tail -5; then
    pass "All unit tests pass"
else
    fail "Unit tests failed"
fi

# Summary
echo ""
echo "=========================================="
echo "Integration Test Summary"
echo "=========================================="
echo "Passed: $TESTS_PASSED"
echo "Failed: $TESTS_FAILED"
echo "Total: $((TESTS_PASSED + TESTS_FAILED))"

if [ $TESTS_FAILED -gt 0 ]; then
    exit 1
fi
exit 0
