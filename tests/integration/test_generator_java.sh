#!/bin/bash
# Test harness for Java generator integration tests
# This script generates code, starts a test server in Docker, runs client tests, and reports results

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
OUTPUT_DIR="/tmp/pulserpc_test_java_$$"
NATIVE_BINARY="$PROJECT_ROOT/target/pulserpc"
LINUX_BINARY="$PROJECT_ROOT/target/pulserpc-amd64"
SERVER_PORT=8080
SERVER_URL="http://localhost:$SERVER_PORT"
TIMEOUT=30
DOCKER_IMAGE="maven:3.9-eclipse-temurin-17"
CONTAINER_NAME="pulserpc-test-java-$$"
M2_CACHE_DIR="$PROJECT_ROOT/.m2-cache"

# Check if Docker is available
check_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        echo -e "${RED}ERROR: Docker is required but not installed${NC}"
        exit 1
    fi
}

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
    #rm -rf "$OUTPUT_DIR"
}

trap cleanup EXIT

echo -e "${GREEN}=== PulseRPC Java Generator Integration Test ===${NC}"
echo ""

check_docker

# Create Maven cache directory if it doesn't exist
mkdir -p "$M2_CACHE_DIR"

# Step 1: Determine which binary to use
# Prefer native binary for local testing, fall back to Linux binary for containers
BINARY_PATH=""
if [ -f "$NATIVE_BINARY" ] && [ -x "$NATIVE_BINARY" ]; then
    BINARY_PATH="$NATIVE_BINARY"
    echo -e "${GREEN}Using native pulserpc binary at $BINARY_PATH${NC}"
elif [ -f "$LINUX_BINARY" ] && [ -x "$LINUX_BINARY" ]; then
    BINARY_PATH="$LINUX_BINARY"
    echo -e "${GREEN}Using Linux pulserpc binary at $BINARY_PATH${NC}"
elif command -v go >/dev/null 2>&1; then
    # Build the native binary
    echo -e "${YELLOW}Building pulserpc binary...${NC}"
    cd "$PROJECT_ROOT"
    go build -o "$NATIVE_BINARY" cmd/pulserpc/pulserpc.go
    if [ -f "$NATIVE_BINARY" ] && [ -x "$NATIVE_BINARY" ]; then
        BINARY_PATH="$NATIVE_BINARY"
        echo -e "${GREEN}Built native pulserpc binary at $BINARY_PATH${NC}"
    else
        echo -e "${RED}ERROR: Failed to build pulserpc binary${NC}"
        exit 1
    fi
else
    echo -e "${RED}ERROR: Neither pulserpc binary nor Go compiler found${NC}"
    echo -e "${RED}Please build the binary first with 'make build' or ensure Go is installed${NC}"
    exit 1
fi

if [ -z "$BINARY_PATH" ] || [ ! -f "$BINARY_PATH" ]; then
    echo -e "${RED}ERROR: PulseRPC binary not found${NC}"
    exit 1
fi

# Step 2: Create output directory
echo -e "${YELLOW}Creating output directory: $OUTPUT_DIR${NC}"
mkdir -p "$OUTPUT_DIR"

# Step 3: Generate Java code with Jackson (default)
echo -e "${YELLOW}Generating Java code with Jackson...${NC}"
cd "$PROJECT_ROOT"
"$BINARY_PATH" -plugin java-client-server -package com.example.server -generate-test-files -dir "$OUTPUT_DIR" "$TEST_IDL"

# Verify generated files
echo -e "${YELLOW}Verifying generated files...${NC}"
REQUIRED_FILES=(
    "src/main/java/com/example/server/Server.java"
    "src/main/java/com/example/server/Client.java"
    "src/test/java/com/example/server/TestServer.java"
    "src/test/java/com/example/server/TestClient.java"
    "pom.xml"
    "src/main/java/com/bitmechanic/pulserpc/RPCError.java"
    "src/main/java/com/bitmechanic/pulserpc/Validation.java"
    "src/main/java/com/bitmechanic/pulserpc/Types.java"
    "src/main/java/com/bitmechanic/pulserpc/JsonParser.java"
    "src/main/java/com/bitmechanic/pulserpc/JacksonJsonParser.java"
    "src/main/java/com/bitmechanic/pulserpc/Transport.java"
    "src/main/java/com/bitmechanic/pulserpc/Request.java"
    "src/main/java/com/bitmechanic/pulserpc/Response.java"
    "src/main/java/com/bitmechanic/pulserpc/HTTPTransport.java"
)

for file in "${REQUIRED_FILES[@]}"; do
    if [ ! -f "$OUTPUT_DIR/$file" ]; then
        echo -e "${RED}ERROR: Required file $file not found in output directory${NC}"
        ls -la "$OUTPUT_DIR"
        exit 1
    fi
done

# Verify GSON parser is NOT included (only Jackson should be)
if [ -f "$OUTPUT_DIR/src/main/java/com/bitmechanic/pulserpc/GsonJsonParser.java" ]; then
    echo -e "${RED}ERROR: GsonJsonParser.java should not be generated when using Jackson${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Generated files verified${NC}"

# Step 4: Build the Java project using Docker
echo -e "${YELLOW}Building Java project with Maven in Docker...${NC}"
cd "$OUTPUT_DIR"
docker run --rm \
    -v "$OUTPUT_DIR:/workspace" \
    -v "$M2_CACHE_DIR:/root/.m2" \
    -w /workspace \
    "$DOCKER_IMAGE" \
    mvn clean test-compile

echo -e "${GREEN}✓ Java project built successfully${NC}"

# Step 5: Start test server in Docker
echo -e "${YELLOW}Starting test server in Docker on port $SERVER_PORT...${NC}"
docker run -d \
    --name "$CONTAINER_NAME" \
    -p "$SERVER_PORT:8080" \
    -v "$OUTPUT_DIR:/workspace" \
    -v "$M2_CACHE_DIR:/root/.m2" \
    -w /workspace \
    "$DOCKER_IMAGE" \
    mvn exec:java -Dexec.mainClass="com.example.server.TestServer" -Dexec.classpathScope=test

# Give container a moment to start, then check if it's still running
sleep 2
if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo -e "${RED}ERROR: Container exited immediately${NC}"
    echo "Container logs:"
    docker logs "$CONTAINER_NAME" 2>&1
    exit 1
fi

# Wait for server to start
echo -e "${YELLOW}Waiting for server to start...${NC}"
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
    echo "Container logs:"
    docker logs "$CONTAINER_NAME" 2>&1
    exit 1
fi

echo ""

# Step 6: Test HTTP API
echo -e "${YELLOW}Running HTTP API tests...${NC}"
cd "$SCRIPT_DIR"
if ! bash test_http_api.sh "$SERVER_URL"; then
    echo -e "${RED}ERROR: HTTP API tests failed${NC}"
    echo "Container logs:"
    docker logs "$CONTAINER_NAME" 2>&1
    exit 1
fi
echo -e "${GREEN}✓ HTTP API tests passed${NC}"

# Step 7: Test client execution in Docker
echo -e "${YELLOW}Running test client in Docker...${NC}"
# Use host.docker.internal to connect to the host's exposed port
# This works on Docker Desktop (Mac/Windows) and newer Linux Docker versions
CLIENT_SERVER_URL="http://host.docker.internal:$SERVER_PORT"
# Pass the server URL as an argument to TestClient
docker run --rm \
    --add-host=host.docker.internal:host-gateway \
    -v "$OUTPUT_DIR:/workspace" \
    -v "$M2_CACHE_DIR:/root/.m2" \
    -w /workspace \
    "$DOCKER_IMAGE" \
    mvn exec:java -Dexec.mainClass="com.example.server.TestClient" -Dexec.args="$CLIENT_SERVER_URL" -Dexec.classpathScope=test
echo -e "${GREEN}✓ Test client executed successfully${NC}"

# Step 8: Test with GSON instead of Jackson
echo -e "${YELLOW}Testing GSON code generation...${NC}"
cd "$PROJECT_ROOT"
GSON_OUTPUT_DIR="/tmp/pulserpc_test_java_gson_$$"
mkdir -p "$GSON_OUTPUT_DIR"
"$BINARY_PATH" -plugin java-client-server -package com.example.server -json-lib gson -generate-test-files -dir "$GSON_OUTPUT_DIR" "$TEST_IDL"

# Verify GSON files
if [ ! -f "$GSON_OUTPUT_DIR/src/main/java/com/bitmechanic/pulserpc/GsonJsonParser.java" ]; then
    echo -e "${RED}ERROR: GsonJsonParser.java not found when using GSON${NC}"
    exit 1
fi

if [ -f "$GSON_OUTPUT_DIR/src/main/java/com/bitmechanic/pulserpc/JacksonJsonParser.java" ]; then
    echo -e "${RED}ERROR: JacksonJsonParser.java should not be generated when using GSON${NC}"
    exit 1
fi

# Build GSON version in Docker
echo -e "${YELLOW}Building GSON version in Docker...${NC}"
cd "$GSON_OUTPUT_DIR"
docker run --rm \
    -v "$GSON_OUTPUT_DIR:/workspace" \
    -v "$M2_CACHE_DIR:/root/.m2" \
    -w /workspace \
    "$DOCKER_IMAGE" \
    mvn clean test-compile
echo -e "${GREEN}✓ GSON version built successfully${NC}"

# Cleanup GSON test
rm -rf "$GSON_OUTPUT_DIR"

echo ""
echo -e "${GREEN}=== Java Generator Integration Test PASSED ===${NC}"
echo -e "${GREEN}✓ Code generation with Jackson${NC}"
echo -e "${GREEN}✓ Code generation with GSON${NC}"
echo -e "${GREEN}✓ Maven build${NC}"
echo -e "${GREEN}✓ Server startup${NC}"
echo -e "${GREEN}✓ HTTP API compliance${NC}"
echo -e "${GREEN}✓ Client execution${NC}"
