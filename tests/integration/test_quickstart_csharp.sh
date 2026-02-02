#!/bin/bash
# Test C# quickstart example from examples/quickstart/csharp/

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
OUTPUT_DIR="$PROJECT_ROOT/.tmp/pulserpc_quickstart_csharp_$$"
SERVER_PORT=8104
SERVER_URL="http://localhost:$SERVER_PORT"
TIMEOUT=45

# Check if we should use Docker
if [ "$USE_DOCKER" = "1" ]; then
    echo -e "${GREEN}=== C# Quickstart Test (Docker Mode) ===${NC}"

    if ! command -v docker >/dev/null 2>&1; then
        echo -e "${RED}ERROR: Docker requested but docker not found${NC}"
        exit 1
    fi

    # Ensure we have the Linux binary
    if [ ! -f "$PROJECT_ROOT/target/pulserpc-amd64" ]; then
        echo -e "${YELLOW}Building pulserpc-amd64...${NC}"
        cd "$PROJECT_ROOT"
        make build-linux
    fi

    echo -e "${YELLOW}Running test in Docker container...${NC}"
    docker run --rm \
        -v "$PROJECT_ROOT:/workspace" \
        -w /workspace \
        -e SERVER_PORT="$SERVER_PORT" \
        mcr.microsoft.com/dotnet/sdk:8.0 \
        /bin/bash -c "bash tests/integration/test_quickstart_csharp.sh"
    exit $?
fi

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

echo -e "${GREEN}=== C# Quickstart Test ===${NC}"
echo ""

# Check for pulserpc binary
# Prefer pulserpc-amd64 (Linux) since it works in Docker containers and on Linux hosts
if [ -f "$PROJECT_ROOT/target/pulserpc-amd64" ]; then
    PULSERPC="$PROJECT_ROOT/target/pulserpc-amd64"
elif [ -f "$PROJECT_ROOT/target/pulserpc" ]; then
    PULSERPC="$PROJECT_ROOT/target/pulserpc"
else
    echo -e "${RED}ERROR: PulseRPC binary not found. Run 'make build' first.${NC}"
    exit 1
fi

# Check for dotnet
if ! command -v dotnet >/dev/null 2>&1; then
    echo -e "${YELLOW}WARNING: dotnet not found, skipping C# quickstart test${NC}"
    echo -e "${YELLOW}Install .NET SDK to run this test: https://dotnet.microsoft.com/download${NC}"
    echo -e "${YELLOW}Or use Docker: make test-quickstart-csharp-docker${NC}"
    exit 0
fi

# 1. Generate code from shared checkout.pulse
echo -e "${YELLOW}Generating code from checkout.pulse...${NC}"
mkdir -p "$OUTPUT_DIR/Shared"
"$PULSERPC" -plugin csharp-client-server -dir "$OUTPUT_DIR/Shared" \
    "$QUICKSTART_DIR/checkout.pulse"

# 2. Copy quickstart implementations
echo -e "${YELLOW}Copying quickstart implementations...${NC}"
mkdir -p "$OUTPUT_DIR/TestServer"
mkdir -p "$OUTPUT_DIR/TestClient"

cp "$QUICKSTART_DIR/csharp/TestServer/TestServer.csproj" "$OUTPUT_DIR/TestServer/"
cp "$QUICKSTART_DIR/csharp/TestServer/MyServer.cs" "$OUTPUT_DIR/TestServer/"
cp "$QUICKSTART_DIR/csharp/TestClient/TestClient.csproj" "$OUTPUT_DIR/TestClient/"
cp "$QUICKSTART_DIR/csharp/TestClient/MyClient.cs" "$OUTPUT_DIR/TestClient/"

# Update server to use test port instead of default 8080
sed -i.bak "s/8080/$SERVER_PORT/g" "$OUTPUT_DIR/TestServer/MyServer.cs"
rm -f "$OUTPUT_DIR/TestServer/MyServer.cs.bak"

# Update client to use test port instead of default 8080
sed -i.bak "s|http://localhost:8080|$SERVER_URL|g" "$OUTPUT_DIR/TestClient/MyClient.cs"
rm -f "$OUTPUT_DIR/TestClient/MyClient.cs.bak"

# 3. Build and start server
cd "$OUTPUT_DIR/TestServer"
echo -e "${YELLOW}Building server...${NC}"
# Note: First build may fail with GlobalUsings.g.cs race condition, retry once
# Also note: dotnet build returns non-zero for warnings, so we check for actual errors
BUILD_OUTPUT=$(dotnet build 2>&1)
if ! echo "$BUILD_OUTPUT" | grep -q "0 Error(s)"; then
    echo -e "${YELLOW}First build had errors, retrying...${NC}"
    BUILD_OUTPUT=$(dotnet build 2>&1)
    if ! echo "$BUILD_OUTPUT" | grep -q "0 Error(s)"; then
        echo -e "${RED}Build failed with errors:${NC}"
        echo "$BUILD_OUTPUT" | tail -20
        exit 1
    fi
fi

echo -e "${YELLOW}Starting server on port $SERVER_PORT...${NC}"
dotnet run > "$OUTPUT_DIR/server.log" 2>&1 &
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
    cat "$OUTPUT_DIR/server.log"
    exit 1
fi

# 5. Run client and verify output
echo -e "${YELLOW}Running client...${NC}"
cd "$OUTPUT_DIR/TestClient"
# Note: First build may fail with GlobalUsings.g.cs race condition, retry once
# Also note: dotnet build returns non-zero for warnings, so we check for actual errors
BUILD_OUTPUT=$(dotnet build 2>&1)
if ! echo "$BUILD_OUTPUT" | grep -q "0 Error(s)"; then
    echo -e "${YELLOW}First build had errors, retrying...${NC}"
    BUILD_OUTPUT=$(dotnet build 2>&1)
    if ! echo "$BUILD_OUTPUT" | grep -q "0 Error(s)"; then
        echo -e "${RED}Client build failed:${NC}"
        echo "$BUILD_OUTPUT" | tail -20
        exit 1
    fi
fi
CLIENT_OUTPUT=$(dotnet run 2>&1)

# Verify expected outputs
echo "$CLIENT_OUTPUT" | grep -q "Products ===" || {
    echo -e "${RED}ERROR: Client output missing 'Products ==='${NC}"
    echo "Client output:"
    echo "$CLIENT_OUTPUT"
    cat "$OUTPUT_DIR/server.log"
    exit 1
}

echo "$CLIENT_OUTPUT" | grep -q "Wireless Mouse" || {
    echo -e "${RED}ERROR: Client output missing 'Wireless Mouse'${NC}"
    echo "Client output:"
    echo "$CLIENT_OUTPUT"
    cat "$OUTPUT_DIR/server.log"
    exit 1
}

echo "$CLIENT_OUTPUT" | grep -q "Order created:" || {
    echo -e "${RED}ERROR: Client output missing 'Order created'${NC}"
    echo "Client output:"
    echo "$CLIENT_OUTPUT"
    cat "$OUTPUT_DIR/server.log"
    exit 1
}

echo "$CLIENT_OUTPUT" | grep -q "Got expected error:" || {
    echo -e "${RED}ERROR: Client output missing expected error message${NC}"
    echo "Client output:"
    echo "$CLIENT_OUTPUT"
    cat "$OUTPUT_DIR/server.log"
    exit 1
}

echo ""
echo -e "${GREEN}✓ C# quickstart test passed!${NC}"
exit 0
