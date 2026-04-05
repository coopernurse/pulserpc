#!/bin/bash
# Test enum case sensitivity across all runtime implementations
# Enums should be case-sensitive - "add" != "Add" != "ADD"

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
test_enum_value() {
    local runtime="$1"
    local port="${RUNTIME_PORTS[$runtime]}"
    local value="$2"
    local expected="$3"  # "pass" or "fail"
    local description="$4"
    
    # A.calc takes []float and MathOp enum
    local response=$(curl -s -X POST "http://localhost:$port" \
        -H "Content-Type: application/json" \
        -d "{\"jsonrpc\":\"2.0\",\"method\":\"A.calc\",\"params\":[[10,5],\"$value\"],\"id\":1}")
    
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
echo -e "${BLUE}║   Enum Case Sensitivity Test Suite                   ║${NC}"
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

# Test cases: value, expected, description
# conform-inc.pulse defines MathOp with values: add, multiply
TEST_CASES=(
    "add:pass:lowercase 'add'"
    "multiply:pass:lowercase 'multiply'"
    "Add:fail:wrong case 'Add'"
    "ADD:fail:uppercase 'ADD'"
    "Multiply:fail:wrong case 'Multiply'"
    "MULTIPLY:fail:uppercase 'MULTIPLY'"
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
        if test_enum_value "$runtime" "$port" "$value" "$expected" "$description"; then
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