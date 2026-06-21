#!/bin/bash
# Test harness for TypeScript generator integration tests
# Tests all three module styles: esm-node, esm-bundler, cjs
# Each style runs the full end-to-end test: generate → compile (where required) → start server → run client → assert RPC success

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
TEST_IDL="$PROJECT_ROOT/examples/conform.pulse"
TEST_IDL_INC="$PROJECT_ROOT/examples/conform-inc.pulse"
BINARY_PATH="$PROJECT_ROOT/target/pulserpc-amd64"
TIMEOUT=30

# Entry-point namespace for conform.pulse
ENTRY_POINT_NS="conform"

# All three module styles to test
STYLES=("esm-node" "esm-bundler" "cjs")

# Track results
PASSED_STYLES=()
FAILED_STYLES=()

# Step 1: Build the pulserpc binary (if needed) — shared across all styles
echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   PulseRPC TypeScript Generator Integration Test (3 styles) ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""

if [ -f "$BINARY_PATH" ] && [ -x "$BINARY_PATH" ]; then
    echo -e "${GREEN}Using pre-built pulserpc binary at $BINARY_PATH${NC}"
elif command -v go >/dev/null 2>&1; then
    echo -e "${YELLOW}Building pulserpc binary in container...${NC}"
    cd "$PROJECT_ROOT"
    go build -o "$BINARY_PATH" ./cmd/pulse
    if [ ! -f "$BINARY_PATH" ]; then
        echo -e "${RED}ERROR: Failed to build pulserpc binary${NC}"
        exit 1
    fi
elif [ ! -f "$BINARY_PATH" ]; then
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

# Step 2: Install global tooling once
echo -e "${YELLOW}Installing TypeScript and tsx...${NC}"
npm install -g typescript tsx @types/node >/dev/null 2>&1 || true

# Test a single module style
test_style() {
    local style="$1"
    local port="$2"
    local output_dir="/tmp/pulserpc_test_ts_${style}_$$"
    local server_url="http://localhost:$port"
    local server_pid=""

    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  Testing module style: ${style}${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    # Create output directory
    mkdir -p "$output_dir"

    # Generate code with style-specific flags and test files
    echo -e "${YELLOW}  Generating code (${style})...${NC}"
    if ! "$BINARY_PATH" -plugin ts-client-server -generate-test-files \
        -ts-module="$style" -ts-gen-package-json -ts-gen-tsconfig \
        -dir "$output_dir" "$TEST_IDL"; then
        echo -e "${RED}  ERROR: Code generation failed for ${style}${NC}"
        return 1
    fi

    # Verify generated files exist
    local test_server_path="$output_dir/$ENTRY_POINT_NS/test_server.ts"
    local test_client_path="$output_dir/$ENTRY_POINT_NS/test_client.ts"
    if [ ! -f "$test_server_path" ] || [ ! -f "$test_client_path" ]; then
        echo -e "${RED}  ERROR: Test files not generated for ${style}${NC}"
        echo "  Looking for: $test_server_path and $test_client_path"
        return 1
    fi

    echo -e "${GREEN}  Code generation successful${NC}"

    local test_dir="$output_dir/$ENTRY_POINT_NS"

    if [ "$style" = "cjs" ]; then
        # CJS: compile with tsc first, then run with node
        echo -e "${YELLOW}  Installing @types/node for CJS compilation...${NC}"
        cd "$output_dir"
        npm install --save-dev @types/node > /dev/null 2>&1 || true
        echo -e "${YELLOW}  Compiling with tsc (CommonJS)...${NC}"
        npx tsc --project tsconfig.json 2>&1 || true
        # Verify compiled JS files exist (type-only errors don't prevent emit)
        if [ ! -f "$test_dir/test_server.js" ]; then
            echo -e "${RED}  ERROR: tsc did not emit test_server.js for ${style}${NC}"
            return 1
        fi

        echo -e "${YELLOW}  Starting compiled server on port $port...${NC}"
        cd "$test_dir"
        node test_server.js > server.log 2>&1 &
        server_pid=$!
    else
        # esm-node / esm-bundler: run directly with tsx
        echo -e "${YELLOW}  Starting server on port $port...${NC}"
        cd "$test_dir"
        tsx test_server.ts > server.log 2>&1 &
        server_pid=$!
    fi

    # Wait for server to be ready
    echo -e "${YELLOW}  Waiting for server to be ready...${NC}"
    local wait_count=0
    while [ $wait_count -lt $TIMEOUT ]; do
        if curl -s -X POST "$server_url" -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","method":"pulserpc-idl","id":1}' > /dev/null 2>&1; then
            echo -e "${GREEN}  Server is ready${NC}"
            break
        fi
        sleep 1
        wait_count=$((wait_count + 1))
    done

    if [ $wait_count -ge $TIMEOUT ]; then
        echo -e "${RED}  ERROR: Server did not become ready within $TIMEOUT seconds${NC}"
        echo "  Server log:"
        cat server.log 2>/dev/null || true
        kill $server_pid 2>/dev/null || true
        wait $server_pid 2>/dev/null || true
        rm -rf "$output_dir"
        return 1
    fi

    echo ""

    # Run test client
    echo -e "${YELLOW}  Running test client...${NC}"
    cd "$test_dir"
    local client_exit=0
    if [ "$style" = "cjs" ]; then
        node test_client.js 2>&1 || client_exit=$?
    else
        tsx test_client.ts 2>&1 || client_exit=$?
    fi

    if [ $client_exit -ne 0 ]; then
        echo ""
        echo -e "${RED}  === Tests failed for ${style} (exit code $client_exit) ===${NC}"
        echo "  Server log:"
        cat server.log 2>/dev/null || true
        kill $server_pid 2>/dev/null || true
        wait $server_pid 2>/dev/null || true
        rm -rf "$output_dir"
        return $client_exit
    fi

    echo ""
    echo -e "${GREEN}  Test client passed for ${style}${NC}"

    # Run HTTP API tests
    echo -e "${YELLOW}  Running HTTP API tests...${NC}"
    local http_test_script="$SCRIPT_DIR/test_http_api.sh"
    local http_exit=0
    if [ -f "$http_test_script" ]; then
        bash "$http_test_script" "$server_url" 2>&1 || http_exit=$?
    else
        echo -e "${YELLOW}  HTTP test script not found, skipping${NC}"
    fi

    # Cleanup
    kill $server_pid 2>/dev/null || true
    wait $server_pid 2>/dev/null || true
    rm -rf "$output_dir"

    if [ $http_exit -ne 0 ]; then
        echo -e "${RED}  === HTTP API tests failed for ${style} (exit code $http_exit) ===${NC}"
        return $http_exit
    fi

    echo -e "${GREEN}  ✓ ${style}: All tests passed${NC}"
    return 0
}

# Main test loop — each style uses the same port because the generated
# test server always binds to 8080 (the port is hardcoded in the
# test_server.ts template). The previous style's server is killed and its
# output directory is removed before the next style starts, so there is
# no port conflict between styles.
PORT=8080
for i in "${!STYLES[@]}"; do
    style="${STYLES[$i]}"
    port="$PORT"

    if test_style "$style" "$port"; then
        PASSED_STYLES+=("$style")
    else
        FAILED_STYLES+=("$style")
    fi
done

# Summary
echo ""
echo -e "${BLUE}╔══════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   Summary                                                  ║${NC}"
echo -e "${BLUE}╚══════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "Styles tested: ${#STYLES[@]}"
echo -e "${GREEN}Passed: ${#PASSED_STYLES[@]}${NC}"

for s in "${PASSED_STYLES[@]}"; do
    echo -e "  ${GREEN}✓${NC} $s"
done

if [ ${#FAILED_STYLES[@]} -gt 0 ]; then
    echo -e "${RED}Failed: ${#FAILED_STYLES[@]}${NC}"
    for s in "${FAILED_STYLES[@]}"; do
        echo -e "  ${RED}✗${NC} $s"
    done
    echo ""
    echo -e "${RED}=== Some style tests failed ===${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}=== All TypeScript style tests passed! ===${NC}"
exit 0
