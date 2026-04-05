#!/bin/bash
# Test raises() error propagation across all runtime implementations
# When a server method raises an error declared in raises() clause,
# the client should receive a proper JSON-RPC error response

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

# conform.pulse defines: divide(a int, b int) float raises(ValidationError)
# ValidationError has code 1001

echo -e "${BLUE}╔═══════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   raises() Error Propagation Test Suite                ║${NC}"
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

# Test: A.divide(5, 0) should return error because 5/0 is ValidationError
# The server implementation should catch this and return RPCError with code 1001
test_raises_propagation() {
    local runtime="$1"
    local port="${RUNTIME_PORTS[$runtime]}"
    
    echo -e "${BLUE}--- Testing $runtime runtime ---${NC}"
    
    # Call A.divide(5, 0) - should trigger ValidationError
    local response=$(curl -s -X POST "http://localhost:$port" \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"A.divide","params":[5,0],"id":1}')
    
    echo "  Request: A.divide(5, 0)"
    echo "  Response: $response"
    
    # Check for error response
    if echo "$response" | grep -q '"error"'; then
        # Check error code - should be 1001 (ValidationError)
        if echo "$response" | grep -q '"code":1001'; then
            echo -e "${GREEN}✓${NC} $runtime: Returns error with code 1001 (ValidationError)"
            PASSED=$((PASSED + 1))
            return 0
        else
            echo -e "${RED}✗${NC} $runtime: Error returned but wrong code"
            echo "  Expected code: 1001"
            FAILED=$((FAILED + 1))
            return 1
        fi
    else
        echo -e "${RED}✗${NC} $runtime: Expected error response for divide(5, 0), got result instead"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Test successful case (no error)
test_successful_division() {
    local runtime="$1"
    local port="${RUNTIME_PORTS[$runtime]}"
    
    echo -e "${BLUE}--- Testing $runtime runtime (success case) ---${NC}"
    
    # Call A.divide(10, 2) - should return 5.0 successfully
    local response=$(curl -s -X POST "http://localhost:$port" \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"A.divide","params":[10,2],"id":1}')
    
    echo "  Request: A.divide(10, 2)"
    echo "  Response: $response"
    
    # Check for successful result
    if echo "$response" | grep -q '"result"'; then
        echo -e "${GREEN}✓${NC} $runtime: Returns successful result"
        PASSED=$((PASSED + 1))
        return 0
    else
        echo -e "${RED}✗${NC} $runtime: Expected result for divide(10, 2), got error instead"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

for runtime in go python ts csharp java; do
    port="${RUNTIME_PORTS[$runtime]}"
    
    # Skip if server not running
    if ! curl -s "http://localhost:$port" > /dev/null 2>&1; then
        echo -e "${YELLOW}Skipping $runtime (server not running)${NC}"
        continue
    fi
    
    # Test error propagation
    test_raises_propagation "$runtime"
    echo ""
    
    # Test success case
    test_successful_division "$runtime"
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