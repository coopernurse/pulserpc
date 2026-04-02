#!/bin/bash
# Test harness for generator integration tests
# This script generates code, starts a test server, runs client tests, and reports results

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEST_IDL="$PROJECT_ROOT/examples/conform.pulse"
TEST_IDL_INC="$PROJECT_ROOT/examples/conform-inc.pulse"
OUTPUT_DIR="/tmp/pulserpc_test_$$"
BINARY_PATH="$PROJECT_ROOT/target/pulserpc-amd64"
SERVER_PORT=8080
SERVER_URL="http://localhost:$SERVER_PORT"
TIMEOUT=30

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    if [ -n "$SERVER_PID" ]; then
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi
    rm -rf "$OUTPUT_DIR"
}

trap cleanup EXIT

echo -e "${GREEN}=== PulseRPC Generator Integration Test ===${NC}"
echo ""

# Step 1: Build the pulserpc binary (if needed)
# Prefer using pre-built binary if it exists, otherwise build it
if [ -f "$BINARY_PATH" ] && [ -x "$BINARY_PATH" ]; then
    echo -e "${GREEN}Using pre-built pulserpc binary at $BINARY_PATH${NC}"
elif command -v go >/dev/null 2>&1; then
    # We're in a container with Go - build the binary
    echo -e "${YELLOW}Building pulserpc binary in container...${NC}"
    cd "$PROJECT_ROOT"
    go build -o "$BINARY_PATH" cmd/pulserpc/pulserpc.go
    if [ ! -f "$BINARY_PATH" ]; then
        echo -e "${RED}ERROR: Failed to build pulserpc binary${NC}"
        exit 1
    fi
elif [ ! -f "$BINARY_PATH" ]; then
    # No Go and no binary - try to build on host (for local testing)
    echo -e "${YELLOW}Building pulserpc binary on host...${NC}"
    cd "$PROJECT_ROOT"
    if command -v make >/dev/null 2>&1; then
        make build-linux
    else
        echo -e "${RED}ERROR: Cannot build binary - Go not available and binary doesn't exist${NC}"
        exit 1
    fi
fi

if [ ! -f "$BINARY_PATH" ]; then
    echo -e "${RED}ERROR: PulseRPC binary not found at $BINARY_PATH${NC}"
    exit 1
fi

# Step 2: Create output directory
echo -e "${YELLOW}Creating output directory: $OUTPUT_DIR${NC}"
mkdir -p "$OUTPUT_DIR"

# Step 3: Generate code with -generate-test-files flag
echo -e "${YELLOW}Generating code from $TEST_IDL...${NC}"
if ! "$BINARY_PATH" -plugin python-client-server -generate-test-files -dir "$OUTPUT_DIR" "$TEST_IDL"; then
    echo -e "${RED}ERROR: Code generation failed${NC}"
    exit 1
fi

# Verify generated files exist
# Check for test files in namespace subdirectories (since multiple namespaces may be generated)
HAS_TEST_FILES=false
for dir in "$OUTPUT_DIR"/*/; do
    if [ -f "${dir}test_server.py" ] && [ -f "${dir}test_client.py" ]; then
        HAS_TEST_FILES=true
        break
    fi
done

if [ "$HAS_TEST_FILES" = false ]; then
    echo -e "${RED}ERROR: Test files not generated${NC}"
    exit 1
fi

echo -e "${GREEN}Code generation successful${NC}"
echo ""

# Step 4: Start test server in background
echo -e "${YELLOW}Starting test server on port $SERVER_PORT...${NC}"
# Find the namespace subdirectory containing test files
TEST_DIR="$OUTPUT_DIR"
for dir in "$OUTPUT_DIR"/*/; do
    if [ -f "${dir}test_server.py" ] && [ -f "${dir}test_client.py" ]; then
        TEST_DIR="${dir}"
        break
    fi
done
# Extract namespace name from path (e.g., "conform" from "/tmp/.../conform/")
NAMESPACE_NAME=$(basename "$TEST_DIR")
cd "$OUTPUT_DIR"
PYTHONPATH="$OUTPUT_DIR:$PYTHONPATH" python3 -m ${NAMESPACE_NAME}.test_server > server.log 2>&1 &
SERVER_PID=$!

# Step 5: Wait for server to be ready
echo -e "${YELLOW}Waiting for server to be ready...${NC}"
WAIT_COUNT=0
while [ $WAIT_COUNT -lt $TIMEOUT ]; do
    if curl -s -X POST "$SERVER_URL" -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"pulserpc-idl","id":1}' > /dev/null 2>&1; then
        echo -e "${GREEN}Server is ready${NC}"
        break
    fi
    sleep 1
    WAIT_COUNT=$((WAIT_COUNT + 1))
done

if [ $WAIT_COUNT -ge $TIMEOUT ]; then
    echo -e "${RED}ERROR: Server did not become ready within $TIMEOUT seconds${NC}"
    echo "Server log:"
    cat server.log
    exit 1
fi

echo ""

# Step 6: Run test client
echo -e "${YELLOW}Running test client...${NC}"
if PYTHONPATH="$OUTPUT_DIR:$PYTHONPATH" python3 -m ${NAMESPACE_NAME}.test_client; then
    echo ""
    echo -e "${GREEN}Test client passed${NC}"
else
    CLIENT_EXIT_CODE=$?
    echo ""
    echo -e "${RED}=== Tests failed with exit code $CLIENT_EXIT_CODE ===${NC}"
    echo "Server log:"
    cat server.log
    exit $CLIENT_EXIT_CODE
fi

# Step 7: Run HTTP API tests
echo ""
echo -e "${YELLOW}Running HTTP API tests...${NC}"
HTTP_TEST_SCRIPT="$SCRIPT_DIR/test_http_api.sh"
if [ ! -f "$HTTP_TEST_SCRIPT" ]; then
    echo -e "${RED}ERROR: HTTP test script not found at $HTTP_TEST_SCRIPT${NC}"
    exit 1
fi

if bash "$HTTP_TEST_SCRIPT" "$SERVER_URL"; then
    echo ""
    echo -e "${GREEN}=== All tests passed! ===${NC}"
    exit 0
else
    HTTP_TEST_EXIT_CODE=$?
    echo ""
    echo -e "${RED}=== HTTP API tests failed with exit code $HTTP_TEST_EXIT_CODE ===${NC}"
    exit $HTTP_TEST_EXIT_CODE
fi

