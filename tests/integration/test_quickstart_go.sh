#!/bin/bash
# Test Go quickstart example from examples/quickstart/go/

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
OUTPUT_DIR="/tmp/pulserpc_quickstart_go_$$"
SERVER_PORT=8100
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

echo -e "${GREEN}=== Go Quickstart Test ===${NC}"
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

# Check for Go
if ! command -v go >/dev/null 2>&1; then
    echo -e "${YELLOW}WARNING: go not found, skipping Go quickstart test${NC}"
    echo -e "${YELLOW}Install Go to run this test: https://go.dev/dl/${NC}"
    exit 0
fi

# 1. Generate code from shared checkout.pulse
echo -e "${YELLOW}Generating code from checkout.pulse...${NC}"
mkdir -p "$OUTPUT_DIR"
cd "$OUTPUT_DIR"
go mod init checkout-service
mkdir -p pkg/checkout
# Use -package flag to specify base import path for generated code
"$PULSERPC" -plugin go-client-server -dir pkg/checkout -package checkout-service/pkg/checkout \
    "$QUICKSTART_DIR/checkout.pulse"

# 2. Copy quickstart implementations
echo -e "${YELLOW}Copying quickstart implementations...${NC}"
mkdir -p cmd/server cmd/client
cp "$QUICKSTART_DIR/go/server.go" cmd/server/main.go
cp "$QUICKSTART_DIR/go/client.go" cmd/client/main.go

# Update import paths: generated code is now in <outputDir>/<namespace>/ instead of <outputDir>/
# e.g., checkout-service/pkg/checkout becomes checkout-service/pkg/checkout/checkout
# and pulserpc runtime moves from checkout-service/pkg/pulserpc to checkout-service/pkg/checkout/pulserpc
sed -i.bak "s|checkout-service/pkg/checkout\"|checkout-service/pkg/checkout/checkout\"|g" cmd/server/main.go
sed -i.bak "s|checkout-service/pkg/pulserpc\"|checkout-service/pkg/checkout/pulserpc\"|g" cmd/server/main.go
rm -f cmd/server/main.go.bak
sed -i.bak "s|checkout-service/pkg/checkout\"|checkout-service/pkg/checkout/checkout\"|g" cmd/client/main.go
sed -i.bak "s|checkout-service/pkg/pulserpc\"|checkout-service/pkg/checkout/pulserpc\"|g" cmd/client/main.go
rm -f cmd/client/main.go.bak

# Update server and client to use test port instead of default 8080
sed -i.bak "s/8080/$SERVER_PORT/g" cmd/server/main.go
rm -f cmd/server/main.go.bak
sed -i.bak "s/8080/$SERVER_PORT/g" cmd/client/main.go
rm -f cmd/client/main.go.bak

# 3. Build and start server
echo -e "${YELLOW}Starting server on port $SERVER_PORT...${NC}"
go mod tidy 2>/dev/null || true
go run -tags server_only cmd/server/main.go > server.log 2>&1 &
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
CLIENT_OUTPUT=$(go run -tags client_only cmd/client/main.go 2>&1)

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

echo "$CLIENT_OUTPUT" | grep -q "Order created" || {
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
echo -e "${GREEN}✓ Go quickstart test passed!${NC}"
exit 0
