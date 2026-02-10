#!/bin/bash
# Test script for GoLeapAI Configure Tool

set -e

echo "🧪 GoLeapAI Configure Tool - Test Suite"
echo "========================================"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
pass() {
    echo -e "${GREEN}✓ PASS${NC}: $1"
    ((TESTS_PASSED++))
}

fail() {
    echo -e "${RED}✗ FAIL${NC}: $1"
    ((TESTS_FAILED++))
}

info() {
    echo -e "${YELLOW}ℹ${NC} $1"
}

# Build the tool
echo "1. Building configure tool..."
if go build -o goleapai-configure 2>/dev/null; then
    pass "Build successful"
else
    fail "Build failed"
    exit 1
fi
echo ""

# Test 1: Help flag
echo "2. Testing --help flag..."
if ./goleapai-configure --help > /dev/null 2>&1; then
    pass "Help flag works"
else
    fail "Help flag failed"
fi
echo ""

# Test 2: Dry run mode
echo "3. Testing --dry-run mode..."
if ./goleapai-configure --dry-run 2>&1 | grep -q "DRY RUN"; then
    pass "Dry run mode works"
else
    fail "Dry run mode failed"
fi
echo ""

# Test 3: Detection
echo "4. Testing tool detection..."
OUTPUT=$(./goleapai-configure 2>&1)
if echo "$OUTPUT" | grep -q "Detecting"; then
    pass "Detection logic runs"
else
    fail "Detection logic failed"
fi
echo ""

# Test 4: Binary exists and is executable
echo "5. Testing binary..."
if [ -x "./goleapai-configure" ]; then
    pass "Binary is executable"
else
    fail "Binary is not executable"
fi
echo ""

# Test 5: Verbose flag
echo "6. Testing verbose mode..."
if ./goleapai-configure -v --dry-run 2>&1 | grep -q "Detecting"; then
    pass "Verbose mode works"
else
    fail "Verbose mode failed"
fi
echo ""

# Test 6: Test mode (without gateway)
echo "7. Testing --test mode..."
if ./goleapai-configure --test 2>&1 | grep -q "Testing connectivity"; then
    pass "Test mode works"
else
    fail "Test mode failed"
fi
echo ""

# Test 7: Check for proper error handling
echo "8. Testing error handling..."
# This should gracefully handle missing gateway
if ./goleapai-configure --test 2>&1; then
    pass "Error handling works (no crashes)"
else
    fail "Error handling failed"
fi
echo ""

# Summary
echo ""
echo "========================================"
echo "Test Summary"
echo "========================================"
echo -e "Passed: ${GREEN}${TESTS_PASSED}${NC}"
echo -e "Failed: ${RED}${TESTS_FAILED}${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo ""
    echo "You can now use the configure tool:"
    echo "  ./goleapai-configure --help"
    echo "  ./goleapai-configure --all"
    echo "  ./goleapai-configure --test"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi
