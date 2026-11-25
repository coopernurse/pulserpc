#!/bin/bash
# Test integer field validation across all runtime implementations
# Tests that int fields correctly accept/reject various JSON number formats

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Runtime ports
declare -A RUNTIME_PORTS=(
    ["go"]="9004"
    ["python"]="9000"
    ["ts"]="9001"
    ["csharp"]="9002"
    ["java"]="9003"
)

# Test helper
test_int_value() {
    local runtime="$1"
    local port="${RUNTIME_PORTS[$runtime]}"
    local value="$2"
    local expected="$3"  # "pass" or "fail"
    local description="$4"
    
    local response=$(curl -s -X POST "http://localhost:$port" \
        -H "Content-Type: application/json" \
        -d "{\"jsonrpc\":\"2.0\",\"method\":\"A.add\",\"params\":[$value,0],\"id\":1}")
    
    if [ "$expected" = "pass" ]; then
        if echo "$response" | grep -q '"result"'; then
            echo -e "${GREEN}✓${NC} $runtime: $description (value=$value)"
            return 0
        else
            echo -e "${RED}✗${NC} $runtime: $description (value=$value) - expected pass, got error"
            echo "  Response: $response"
            return 1
        fi
    else
        if echo "$response" | grep -q '"error"'; then
            echo -e "${GREEN}✓${NC} $runtime: $description (value=$value)"
            return 0
        else
            echo -e "${RED}✗${NC} $runtime: $description (value=$value) - expected fail, got result"
            echo "  Response: $response"
            return 1
        fi
    fi
}

echo -e "${BLUE}╔═══════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   Integer Validation Test Suite                     ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"
echo ""

# Start test servers if not running
echo -e "${YELLOW}Checking if test servers are running...${NC}"
for runtime in "${!RUNTIME_PORTS[@]}"; do
    port="${RUNTIME_PORTS[$runtime]}"
    if curl -s "http://localhost:$port" > /dev/null 2>&1; then
        echo -e "  $runtime (port $port): ${GREEN}running${NC}"
    else
        echo -e "  $runtime (port $port): ${YELLOW}not running - will skip${NC}"
    fi
done
echo ""

# Track results
PASSED=0
FAILED=0

# Runtime-specific expectation overrides.
#
# IDL `int` is signed 64-bit on the wire, but JavaScript `number` is an IEEE-754
# double: the TypeScript runtimes accept only safe integers (|x| <= 2^53 - 1)
# and reject larger values instead of silently rounding them. Every other
# runtime supports the full signed 64-bit range.
declare -A TS_EXPECTED_OVERRIDES=(
    ["9223372036854775807"]="fail"
    ["-9223372036854775808"]="fail"
)

# Test cases: value, expected, description
TEST_CASES=(
    "5:pass:integer 5"
    "5.0:pass:float 5.0 (whole number)"
    "-3:pass:negative integer -3"
    "-3.0:pass:negative float -3.0 (whole number)"
    "5.1:fail:float 5.1 (fractional part)"
    "-3.5:fail:float -3.5 (fractional part)"
    "0:pass:zero"
    "0.0:pass:float 0.0"
    "2147483648:pass:above int32 max (valid int64)"
    "-2147483649:pass:below int32 min (valid int64)"
    "9223372036854775807:pass:int64 max (TS rejects: unsafe JavaScript number)"
    "-9223372036854775808:pass:int64 min (TS rejects: unsafe JavaScript number)"
    "9223372036854775808:fail:above int64 max"
    "-9223372036854775809:fail:below int64 min"
)

for runtime in go python ts csharp java; do
    port="${RUNTIME_PORTS[$runtime]}"
    
    # Skip if server not running
    if ! curl -s "http://localhost:$port" > /dev/null 2>&1; then
        echo -e "${YELLOW}Skipping $runtime (server not running)${NC}"
        continue
    fi
    
    echo -e "${BLUE}--- Testing $runtime runtime ---${NC}"
    
    for test_case in "${TEST_CASES[@]}"; do
        IFS=':' read -r value expected description <<< "$test_case"
        expected_for_runtime="$expected"
        if [ "$runtime" = "ts" ] && [ -n "${TS_EXPECTED_OVERRIDES[$value]:-}" ]; then
            expected_for_runtime="${TS_EXPECTED_OVERRIDES[$value]}"
        fi
        if test_int_value "$runtime" "$value" "$expected_for_runtime" "$description"; then
            PASSED=$((PASSED + 1))
        else
            FAILED=$((FAILED + 1))
        fi
    done
    echo ""
done

echo -e "${BLUE}=== Summary ===${NC}"
echo -e "Passed: ${GREEN}$PASSED${NC}"
echo -e "Failed: ${RED}$FAILED${NC}"

if [ $FAILED -gt 0 ]; then
    exit 1
fi
echo -e "${GREEN}All tests passed!${NC}"
exit 0