#!/bin/bash
# Test all runtime implementations using test-servers.sh and HTTP API tests
# This script starts all test servers, runs HTTP tests against each, and reports results

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEST_SERVERS_SCRIPT="$PROJECT_ROOT/scripts/test-servers.sh"
HTTP_TEST_SCRIPT="$SCRIPT_DIR/test_http_api.sh"
LIMITS_TEST_SCRIPT="$SCRIPT_DIR/test_limits.sh"
INTERNAL_ERRORS_TEST_SCRIPT="$SCRIPT_DIR/test_internal_errors.sh"
CONCURRENCY_TEST_SCRIPT="$SCRIPT_DIR/test_concurrency.sh"
INPUT_HARDENING_TEST_SCRIPT="$SCRIPT_DIR/test_input_hardening.sh"
GOLDEN_TEST_SCRIPT="$SCRIPT_DIR/test_golden_responses.sh"

# Per-runtime golden response files are written here for cross-runtime
# comparison. Allow CI/operators to override the location.
export GOLDEN_DIR="${GOLDEN_DIR:-/tmp/pulserpc-golden}"

# Runtime ports (matching test-servers.sh configuration)
# All seven runtimes advertised in docs/RUNTIME_IMPLEMENTATION_GUIDE.md.
declare -A RUNTIME_PORTS=(
    ["python"]="9000"
    ["ts"]="9001"
    ["csharp"]="9002"
    ["java"]="9003"
    ["go"]="9004"
    ["python2"]="9005"
    ["ts-cjs"]="9006"
)

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up test servers...${NC}"
    if [ -f "$TEST_SERVERS_SCRIPT" ]; then
        bash "$TEST_SERVERS_SCRIPT" stop >/dev/null 2>&1 || true
    fi
}

trap cleanup EXIT

echo -e "${GREEN}=== Multi-Runtime HTTP API Integration Tests ===${NC}"
echo ""

# Check if test-servers.sh exists
if [ ! -f "$TEST_SERVERS_SCRIPT" ]; then
    echo -e "${RED}ERROR: test-servers.sh not found at $TEST_SERVERS_SCRIPT${NC}"
    exit 1
fi

# Check if HTTP test script exists
if [ ! -f "$HTTP_TEST_SCRIPT" ]; then
    echo -e "${RED}ERROR: HTTP test script not found at $HTTP_TEST_SCRIPT${NC}"
    exit 1
fi

if [ ! -f "$LIMITS_TEST_SCRIPT" ]; then
    echo -e "${RED}ERROR: Limit test script not found at $LIMITS_TEST_SCRIPT${NC}"
    exit 1
fi

for required_script in "$INTERNAL_ERRORS_TEST_SCRIPT" "$CONCURRENCY_TEST_SCRIPT" "$INPUT_HARDENING_TEST_SCRIPT" "$GOLDEN_TEST_SCRIPT"; do
    if [ ! -f "$required_script" ]; then
        echo -e "${RED}ERROR: Conformance script not found at $required_script${NC}"
        exit 1
    fi
done

# Start all test servers
echo -e "${YELLOW}Starting all test servers...${NC}"
if ! bash "$TEST_SERVERS_SCRIPT" start; then
    echo -e "${RED}ERROR: Failed to start test servers${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}Waiting for servers to be fully ready...${NC}"
sleep 5

# Track results
TOTAL_RUNTIMES=0
PASSED_RUNTIMES=0
FAILED_RUNTIMES=()

# Test each runtime
for runtime in python ts csharp java go python2 ts-cjs; do
    port="${RUNTIME_PORTS[$runtime]}"
    url="http://localhost:$port"

    TOTAL_RUNTIMES=$((TOTAL_RUNTIMES + 1))

    # Variants not launched by test-servers.sh are reported as skipped rather
    # than silently omitted from the matrix.
    if ! grep -q "\"$runtime:" "$TEST_SERVERS_SCRIPT"; then
        echo -e "${YELLOW}SKIP: $runtime has no test server configured in test-servers.sh${NC}"
        continue
    fi
    
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Testing ${runtime} runtime (port $port)${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
    
    # Wait for server to be ready
    echo -e "${YELLOW}Checking if $runtime server is ready...${NC}"
    WAIT_COUNT=0
    MAX_WAIT=30
    while [ $WAIT_COUNT -lt $MAX_WAIT ]; do
        if curl -s -X POST "$url" -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","method":"pulserpc-idl","id":1}' > /dev/null 2>&1; then
            echo -e "${GREEN}$runtime server is ready${NC}"
            break
        fi
        sleep 1
        WAIT_COUNT=$((WAIT_COUNT + 1))
    done
    
    if [ $WAIT_COUNT -ge $MAX_WAIT ]; then
        echo -e "${RED}ERROR: $runtime server did not become ready${NC}"
        FAILED_RUNTIMES+=("$runtime")
        continue
    fi
    
    # Run HTTP protocol tests and resource-limit probes
    runtime_ok=1
    echo ""
    if ! bash "$HTTP_TEST_SCRIPT" "$url"; then
        echo ""
        echo -e "${RED}✗ $runtime runtime: HTTP protocol tests failed${NC}"
        runtime_ok=0
    fi

    echo ""
    if ! SERVER_URL="$url" bash "$LIMITS_TEST_SCRIPT"; then
        echo ""
        echo -e "${RED}✗ $runtime runtime: limit tests failed${NC}"
        runtime_ok=0
    fi

    echo ""
    if ! SERVER_URL="$url" bash "$INTERNAL_ERRORS_TEST_SCRIPT"; then
        echo ""
        echo -e "${RED}✗ $runtime runtime: internal-error redaction tests failed${NC}"
        runtime_ok=0
    fi

    echo ""
    if ! SERVER_URL="$url" bash "$CONCURRENCY_TEST_SCRIPT"; then
        echo ""
        echo -e "${RED}✗ $runtime runtime: concurrency tests failed${NC}"
        runtime_ok=0
    fi

    echo ""
    if ! SERVER_URL="$url" bash "$INPUT_HARDENING_TEST_SCRIPT"; then
        echo ""
        echo -e "${RED}✗- $runtime runtime: input-hardening tests failed${NC}"
        runtime_ok=0
    fi

    echo ""
    if ! RUNTIME="$runtime" SERVER_URL="$url" bash "$GOLDEN_TEST_SCRIPT"; then
        echo ""
        echo -e "${RED}✗ $runtime runtime: golden response collection failed${NC}"
        runtime_ok=0
    fi

    echo ""
    if [ "$runtime_ok" = "1" ]; then
        echo -e "${GREEN}✓ $runtime runtime: All HTTP and limit tests passed${NC}"
        PASSED_RUNTIMES=$((PASSED_RUNTIMES + 1))
    else
        echo -e "${RED}✗ $runtime runtime: integration tests failed${NC}"
        FAILED_RUNTIMES+=("$runtime")
    fi
done

# Print summary
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Test Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "Total runtimes tested: $TOTAL_RUNTIMES"
echo -e "${GREEN}Passed: $PASSED_RUNTIMES${NC}"

if [ ${#FAILED_RUNTIMES[@]} -gt 0 ]; then
    echo -e "${RED}Failed: ${#FAILED_RUNTIMES[@]}${NC}"
    echo ""
    echo -e "${RED}Failed runtimes:${NC}"
    for runtime in "${FAILED_RUNTIMES[@]}"; do
        echo -e "  ${RED}✗${NC} $runtime"
    done
    exit 1
else
    echo -e "${GREEN}Failed: 0${NC}"
    echo ""
    echo -e "${GREEN}=== All runtime tests passed! ===${NC}"
    exit 0
fi

