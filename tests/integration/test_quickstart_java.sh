#!/bin/bash
# Test Java quickstart example from examples/quickstart/java/

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
OUTPUT_DIR="/tmp/pulserpc_quickstart_java_$$"
SERVER_PORT=$((8100 + RANDOM % 1000))
SERVER_URL="http://localhost:$SERVER_PORT"
TIMEOUT=45

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

echo -e "${GREEN}=== Java Quickstart Test ===${NC}"
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

# Check for Maven
if ! command -v mvn >/dev/null 2>&1; then
    echo -e "${YELLOW}WARNING: Maven (mvn) not found, skipping Java quickstart test${NC}"
    echo -e "${YELLOW}Install Maven to run this test: https://maven.apache.org/download.cgi${NC}"
    exit 0
fi

# 1. Generate code from shared checkout.pulse
# Note: Java generator expects -dir to be project root and appends src/main/java automatically
# Note: Use com.example.myapp as base-package so namespace (checkout) is appended correctly
echo -e "${YELLOW}Generating code from checkout.pulse...${NC}"
mkdir -p "$OUTPUT_DIR/src/main/java/com/example/myapp"
"$PULSERPC" -plugin java-client-server -dir "$OUTPUT_DIR" -base-package com.example.myapp \
    "$QUICKSTART_DIR/checkout.pulse"

# 2. Copy quickstart implementations
echo -e "${YELLOW}Copying quickstart implementations...${NC}"
cp "$QUICKSTART_DIR/java/MyServer.java" "$OUTPUT_DIR/src/main/java/com/example/myapp/"
cp "$QUICKSTART_DIR/java/MyClient.java" "$OUTPUT_DIR/src/main/java/com/example/myapp/"
cp "$QUICKSTART_DIR/java/pom.xml" "$OUTPUT_DIR/"

# Update server to use test port instead of default 8080
sed -i.bak "s/8080/$SERVER_PORT/g" "$OUTPUT_DIR/src/main/java/com/example/myapp/MyServer.java"
rm -f "$OUTPUT_DIR/src/main/java/com/example/myapp/MyServer.java.bak"

# Update client to use test port instead of default 8080
sed -i.bak "s|http://localhost:8080|$SERVER_URL|g" "$OUTPUT_DIR/src/main/java/com/example/myapp/MyClient.java"
rm -f "$OUTPUT_DIR/src/main/java/com/example/myapp/MyClient.java.bak"

# 3. Build and start server
cd "$OUTPUT_DIR"
echo -e "${YELLOW}Building project...${NC}"
mvn -q compile

echo -e "${YELLOW}Starting server on port $SERVER_PORT...${NC}"
mvn -q exec:java -Dexec.mainClass="com.example.myapp.MyServer" > server.log 2>&1 &
SERVER_PID=$!

# Give the server a moment to start
sleep 2

# 4. Wait for server ready
echo -e "${YELLOW}Waiting for server to be ready...${NC}"
WAIT_COUNT=0
while [ $WAIT_COUNT -lt $TIMEOUT ]; do
    # The Java server only handles POST requests, so use a simple JSON-RPC ping
    if curl -s -X POST -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"pulserpc-idl","id":1}' \
        "$SERVER_URL" > /dev/null 2>&1; then
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
CLIENT_OUTPUT=$(mvn -q exec:java -Dexec.mainClass="com.example.myapp.MyClient" 2>&1)

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
echo -e "${GREEN}✓ Java quickstart test passed!${NC}"
exit 0
