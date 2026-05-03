#!/bin/bash
# Test Python quickstart example from examples/quickstart/python/

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
QUICKSTART_DIR="$PROJECT_ROOT/examples/quickstart"
OUTPUT_DIR="/tmp/pulserpc_quickstart_python_$$"
SERVER_PORT=8101
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

echo -e "${GREEN}=== Python Quickstart Test ===${NC}"
echo ""

# Check for pulserpc binary
if [ -f "$PROJECT_ROOT/target/pulserpc" ]; then
    PULSERPC="$PROJECT_ROOT/target/pulserpc"
elif [ -f "$PROJECT_ROOT/target/pulserpc-amd64" ]; then
    PULSERPC="$PROJECT_ROOT/target/pulserpc-amd64"
else
    echo -e "${RED}ERROR: PulseRPC binary not found. Run 'make build' first.${NC}"
    exit 1
fi

# Check for Python
if ! command -v python3 >/dev/null 2>&1; then
    echo -e "${YELLOW}WARNING: python3 not found, skipping Python quickstart test${NC}"
    exit 0
fi

# 1. Generate code from shared checkout.pulse
echo -e "${YELLOW}Generating code from checkout.pulse...${NC}"
mkdir -p "$OUTPUT_DIR"
"$PULSERPC" -plugin python-client-server -dir "$OUTPUT_DIR" \
    "$QUICKSTART_DIR/checkout.pulse"

# Verify idl.json is in checkout namespace directory
if [ ! -f "$OUTPUT_DIR/checkout/idl.json" ]; then
    echo -e "${RED}ERROR: checkout/idl.json not generated${NC}"
    exit 1
fi

# Copy idl.json to root for backwards compatibility with quickstart servers
cp "$OUTPUT_DIR/checkout/idl.json" "$OUTPUT_DIR/idl.json"

# 1b. Test generation with -package flag
echo -e "${YELLOW}Testing generation with -package flag...${NC}"
PKG_OUTPUT_DIR="$OUTPUT_DIR/pkg_test"
mkdir -p "$PKG_OUTPUT_DIR"
"$PULSERPC" -plugin python-client-server -dir "$PKG_OUTPUT_DIR" -package "myapp.rpc" \
    "$QUICKSTART_DIR/checkout.pulse"

# Verify package structure (nested directories split by dots)
if [ ! -d "$PKG_OUTPUT_DIR/myapp/rpc/checkout" ]; then
    echo -e "${RED}ERROR: Expected namespace at myapp/rpc/checkout/, found:${NC}"
    ls -la "$PKG_OUTPUT_DIR"
    ls -la "$PKG_OUTPUT_DIR/myapp" 2>/dev/null || echo "myapp dir missing"
    exit 1
fi
echo -e "${GREEN}✓ Package flag creates correct directory structure${NC}"

# Verify runtime is in correct location
if [ ! -d "$PKG_OUTPUT_DIR/myapp/rpc/pulserpc" ]; then
    echo -e "${RED}ERROR: Expected runtime at myapp/rpc/pulserpc/${NC}"
    ls -la "$PKG_OUTPUT_DIR/myapp/rpc" 2>/dev/null || echo "myapp/rpc dir missing"
    exit 1
fi
echo -e "${GREEN}✓ Runtime in correct location with package flag${NC}"

# Verify namespace files are in correct location
for f in "rpctypes.py" "server.py" "client.py" "__init__.py"; do
    if [ ! -f "$PKG_OUTPUT_DIR/myapp/rpc/checkout/$f" ]; then
        echo -e "${RED}ERROR: Expected $f at myapp/rpc/checkout/$f${NC}"
        exit 1
    fi
done
echo -e "${GREEN}✓ Namespace files in correct location with package flag${NC}"

# 2. Copy quickstart implementations
echo -e "${YELLOW}Copying quickstart implementations...${NC}"
cp "$QUICKSTART_DIR/python/my_server.py" "$OUTPUT_DIR/my_server.py"
cp "$QUICKSTART_DIR/python/my_client.py" "$OUTPUT_DIR/my_client.py"

# 3. Start server
cd "$OUTPUT_DIR"
echo -e "${YELLOW}Starting server on port $SERVER_PORT...${NC}"
PYTHONPATH="$OUTPUT_DIR:$PYTHONPATH" SERVER_PORT=$SERVER_PORT python3 my_server.py > server.log 2>&1 &
SERVER_PID=$!

# 4. Wait for server ready
echo -e "${YELLOW}Waiting for server to be ready...${NC}"
WAIT_COUNT=0
while [ $WAIT_COUNT -lt $TIMEOUT ]; do
    if curl -s "$SERVER_URL" > /dev/null 2>&1; then
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

# 5. Run client and verify output
echo -e "${YELLOW}Running client...${NC}"
CLIENT_OUTPUT=$(PYTHONPATH="$OUTPUT_DIR:$PYTHONPATH" SERVER_PORT=$SERVER_PORT python3 my_client.py 2>&1)

# Verify expected outputs
echo "$CLIENT_OUTPUT" | grep -q "Products ===" || {
    echo -e "${RED}ERROR: Client output missing 'Products ==='${NC}"
    echo "Client output:"
    echo "$CLIENT_OUTPUT"
    cat server.log
    exit 1
}

echo "$CLIENT_OUTPUT" | grep -q "Wireless Mouse" || {
    echo -e "${RED}ERROR: Client output missing 'Wireless Mouse'${NC}"
    echo "Client output:"
    echo "$CLIENT_OUTPUT"
    cat server.log
    exit 1
}

echo "$CLIENT_OUTPUT" | grep -q "Order created:" || {
    echo -e "${RED}ERROR: Client output missing 'Order created'${NC}"
    echo "Client output:"
    echo "$CLIENT_OUTPUT"
    cat server.log
    exit 1
}

echo "$CLIENT_OUTPUT" | grep -q "Got expected error:" || {
    echo -e "${RED}ERROR: Client output missing expected error message${NC}"
    echo "Client output:"
    echo "$CLIENT_OUTPUT"
    cat server.log
    exit 1
}

echo ""
echo -e "${GREEN}✓ Python quickstart test passed!${NC}"
exit 0
