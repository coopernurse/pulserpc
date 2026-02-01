#!/bin/bash
# Test TypeScript quickstart example from examples/quickstart/typescript/

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
OUTPUT_DIR="/tmp/pulserpc_quickstart_ts_$$"
SERVER_PORT=8103
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

echo -e "${GREEN}=== TypeScript Quickstart Test ===${NC}"
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

# Check for Node.js
if ! command -v node >/dev/null 2>&1; then
    echo -e "${RED}ERROR: node not found${NC}"
    exit 1
fi

# Check for npm
if ! command -v npm >/dev/null 2>&1; then
    echo -e "${RED}ERROR: npm not found${NC}"
    exit 1
fi

# Check for tsc
if ! command -v npx tsc >/dev/null 2>&1; then
    echo -e "${YELLOW}Installing TypeScript globally...${NC}"
    npm install -g typescript >/dev/null 2>&1 || {
        echo -e "${RED}ERROR: Failed to install TypeScript${NC}"
        exit 1
    }
fi

# 1. Generate code from shared checkout.pulse
echo -e "${YELLOW}Generating code from checkout.pulse...${NC}"
mkdir -p "$OUTPUT_DIR"
"$PULSERPC" -plugin ts-client-server -dir "$OUTPUT_DIR" \
    "$QUICKSTART_DIR/checkout.pulse"

# 2. Copy quickstart implementations
echo -e "${YELLOW}Copying quickstart implementations...${NC}"
cp "$QUICKSTART_DIR/typescript/my_server.ts" "$OUTPUT_DIR/"
cp "$QUICKSTART_DIR/typescript/my_client.ts" "$OUTPUT_DIR/"
cp "$QUICKSTART_DIR/typescript/package.json" "$OUTPUT_DIR/"
cp "$QUICKSTART_DIR/typescript/tsconfig.json" "$OUTPUT_DIR/"

# 3. Install dependencies and build
cd "$OUTPUT_DIR"
echo -e "${YELLOW}Installing dependencies...${NC}"
npm install --silent >/dev/null 2>&1

echo -e "${YELLOW}Building TypeScript...${NC}"
npx tsc >/dev/null 2>&1

# 4. Start server
echo -e "${YELLOW}Starting server on port $SERVER_PORT...${NC}"
node dist/my_server.js > server.log 2>&1 &
SERVER_PID=$!

# 5. Wait for server ready
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

# 6. Run client and verify output
echo -e "${YELLOW}Running client...${NC}"
CLIENT_OUTPUT=$(node dist/my_client.js 2>&1)

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
echo -e "${GREEN}✓ TypeScript quickstart test passed!${NC}"
exit 0
