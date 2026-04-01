#!/bin/bash
# Test Server Management Script
# Manages Docker containers running test servers for all PulseRPC runtimes

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TEST_IDL="$PROJECT_ROOT/examples/conform.pulse"
BINARY_PATH="$PROJECT_ROOT/target/pulserpc"
CONTAINER_PREFIX="pulserpc-test"
TIMEOUT=30

# Runtime configuration: name:plugin:image:port:start_command
# name: short name for the runtime
# plugin: plugin name to use with -plugin flag
# image: Docker image to use
# port: host port to map (container always uses 8080)
# start_command: command to run the test server in the container
RUNTIMES=(
    "python:python-client-server:python:3.11-slim:9000:python3 test_server.py"
    "ts:ts-client-server:node:18-slim:9001:ts-node --project tsconfig.json test_server.ts"
    "csharp:csharp-client-server:mcr.microsoft.com/dotnet/sdk:8.0:9002:dotnet run --project TestServer.csproj"
    "java:java-client-server:maven:3.9-eclipse-temurin-17:9003:mvn exec:java -Dexec.mainClass=TestServer"
    "go:go-client-server:golang:1.21-alpine:9004:rm -f client.go test_client.go && go build -o test-server-bin ./... && ./test-server-bin"
)

# Parse runtime config
# Format: name:plugin:image:port:start_command
# Note: image may contain colons (e.g., python:3.11-slim), so we need to parse carefully
parse_runtime() {
    local config="$1"
    
    # Extract name and plugin (first two fields)
    local name="${config%%:*}"
    config="${config#*:}"
    local plugin="${config%%:*}"
    config="${config#*:}"
    
    # Now we have: image:port:start_command
    # The port is always numeric, so we can find the pattern :number:
    # But we need to handle the case where image has colons (e.g., python:3.11-slim)
    # Strategy: find the last colon followed by a number, that's where port starts
    
    local image=""
    local port=""
    local start_cmd=""
    
    # Use regex to match: image (may contain colons) : port (numeric) : start_command
    # The regex looks for a colon followed by digits followed by a colon
    if [[ "$config" =~ ^(.+):([0-9]+):(.+)$ ]]; then
        image="${BASH_REMATCH[1]}"
        port="${BASH_REMATCH[2]}"
        start_cmd="${BASH_REMATCH[3]}"
    else
        echo "ERROR: Failed to parse runtime config (missing port): $config" >&2
        return 1
    fi
    
    echo "$name|$plugin|$image|$port|$start_cmd"
}

# Check if Docker is available
check_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        echo -e "${RED}ERROR: Docker is not installed or not in PATH${NC}"
        exit 1
    fi
}

# Build pulserpc binary if needed
build_binary() {
    if [ ! -f "$BINARY_PATH" ]; then
        echo -e "${YELLOW}Building pulserpc binary...${NC}"
        cd "$PROJECT_ROOT"
        if ! make build; then
            echo -e "${RED}ERROR: Failed to build pulserpc binary${NC}"
            exit 1
        fi
    fi
}

# Generate code for a runtime
generate_code() {
    local runtime_config="$1"
    local output_dir="$2"
    
    IFS='|' read -r name plugin image port start_cmd <<< "$(parse_runtime "$runtime_config")"
    
    echo -e "${YELLOW}Generating $name code...${NC}"
    # For Java code generation, the generator requires a package flag.
    if [ "$plugin" = "java-client-server" ]; then
        JAVA_BASE_PACKAGE="com.pulserpc.test"
        if ! "$BINARY_PATH" -plugin "$plugin" -package "$JAVA_BASE_PACKAGE" -generate-test-files -dir "$output_dir" "$TEST_IDL"; then
            echo -e "${RED}ERROR: Code generation failed for $name${NC}"
            return 1
        fi
    else
        if ! "$BINARY_PATH" -plugin "$plugin" -generate-test-files -dir "$output_dir" "$TEST_IDL"; then
            echo -e "${RED}ERROR: Code generation failed for $name${NC}"
            return 1
        fi
    fi
    
    # Verify test_server file exists
    local test_server_file=""
    case "$name" in
        python)
            test_server_file="$output_dir/test_server.py"
            ;;
        ts)
            test_server_file="$output_dir/test_server.ts"
            # Create tsconfig.json if it doesn't exist
            if [ ! -f "$output_dir/tsconfig.json" ]; then
                cat > "$output_dir/tsconfig.json" << 'EOF'
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "CommonJS",
    "lib": ["ES2020"],
    "types": ["node"],
    "moduleResolution": "node",
    "esModuleInterop": true,
    "skipLibCheck": true,
    "strict": false,
    "resolveJsonModule": true,
    "isolatedModules": false
  },
  "ts-node": {
    "compilerOptions": {
      "module": "CommonJS",
      "types": ["node"],
      "isolatedModules": false
    }
  }
}
EOF
            fi
            ;;
        csharp)
            test_server_file="$output_dir/TestServer.cs"
            ;;
        java)
            test_server_file="$output_dir/TestServer.java"
            ;;
        go)
            test_server_file="$output_dir/test_server.go"
            ;;
    esac
    
    if [ ! -f "$test_server_file" ]; then
        echo -e "${RED}ERROR: Test server file not generated: $test_server_file${NC}"
        return 1
    fi
    
    return 0
}

# Wait for server to be ready
wait_for_server() {
    local url="$1"
    local name="$2"
    
    echo -e "${YELLOW}Waiting for $name server to be ready...${NC}"
    local wait_count=0
    while [ $wait_count -lt $TIMEOUT ]; do
        if curl -s -X POST "$url" \
            -H "Content-Type: application/json" \
            -d '{"jsonrpc":"2.0","method":"pulserpc-idl","id":1}' > /dev/null 2>&1; then
            echo -e "${GREEN}$name server is ready${NC}"
            return 0
        fi
        sleep 1
        wait_count=$((wait_count + 1))
    done
    
    echo -e "${RED}ERROR: $name server did not become ready within $TIMEOUT seconds${NC}"
    return 1
}

# Start a test server container
start_runtime() {
    local runtime_config="$1"
    
    IFS='|' read -r name plugin image port start_cmd <<< "$(parse_runtime "$runtime_config")"
    local container_name="${CONTAINER_PREFIX}-${name}"
    local output_dir="/tmp/pulserpc_test_${name}_$$"
    
    # Check if container already exists
    if docker ps -a --format '{{.Names}}' | grep -q "^${container_name}$"; then
        echo -e "${YELLOW}Container $container_name already exists, removing...${NC}"
        docker rm -f "$container_name" >/dev/null 2>&1 || true
    fi
    
    # Create output directory
    mkdir -p "$output_dir"
    
    # Generate code
    if ! generate_code "$runtime_config" "$output_dir"; then
        rm -rf "$output_dir"
        return 1
    fi
    
    # Prepare container command
    local container_cmd=""
    case "$name" in
        python)
            # Install dependencies if needed (test_server.py should work standalone)
            container_cmd="cd /workspace && $start_cmd"
            ;;
        ts)
            # Install TypeScript tools and run server
            container_cmd="cd /workspace && npm install -g typescript ts-node @types/node >/dev/null 2>&1 && $start_cmd"
            ;;
        csharp)
            # Build and run C# server
            container_cmd="cd /workspace && dotnet build TestServer.csproj >/dev/null 2>&1 && $start_cmd"
            ;;
        java)
            # If the image already contains Maven (e.g., maven:...), just run mvn; otherwise install it
            if echo "$image" | grep -q "maven"; then
                container_cmd="cd /workspace && mvn -q package -DskipTests >/dev/null 2>&1 && $start_cmd"
            else
                container_cmd="cd /workspace && apt-get update >/dev/null 2>&1 && apt-get install -y maven >/dev/null 2>&1 && mvn -q package -DskipTests >/dev/null 2>&1 && $start_cmd"
            fi
            ;;
        go)
            # Initialize Go module and run server
            container_cmd="cd /workspace && go mod init test-server 2>/dev/null || true && $start_cmd"
            ;;
    esac
    
    # Start container
    echo -e "${YELLOW}Starting $name server container on port $port...${NC}"
    # Run container and capture docker output so failures are visible (don't discard stderr)
    # Use /bin/sh for Alpine images, /bin/bash for others
    local shell="/bin/bash"
    if [[ "$image" == *"alpine"* ]]; then
        shell="/bin/sh"
    fi
    container_run_output=$(docker run -d \
        --name "$container_name" \
        -p "$port:8080" \
        -v "$output_dir:/workspace" \
        -w /workspace \
        "$image" \
        $shell -c "$container_cmd" 2>&1) || {
        echo -e "${RED}ERROR: Failed to start $name container: ${container_run_output}${NC}"
        rm -rf "$output_dir"
        return 1
    }

    # container_run_output should contain the container id on success
    container_id="$container_run_output"
    
    # Wait for server to be ready
    local url="http://localhost:$port"
    if ! wait_for_server "$url" "$name"; then
        echo -e "${YELLOW}Checking container logs...${NC}"
        docker logs "$container_name"
        docker rm -f "$container_name" >/dev/null 2>&1 || true
        rm -rf "$output_dir"
        return 1
    fi
    
    echo "$name|$port|$url|$container_name|$output_dir"
    return 0
}

# Start all test servers
start_all() {
    echo -e "${GREEN}=== Starting PulseRPC Test Servers ===${NC}"
    echo ""
    
    check_docker
    build_binary
    
    local started=()
    local failed=()
    
    for runtime_config in "${RUNTIMES[@]}"; do
        IFS='|' read -r name plugin image port start_cmd <<< "$(parse_runtime "$runtime_config")"
        
        if result=$(start_runtime "$runtime_config"); then
            started+=("$result")
            echo ""
        else
            failed+=("$name")
            echo ""
        fi
    done
    
    # Print summary
    echo -e "${GREEN}=== Test Server Summary ===${NC}"
    echo ""
    printf "%-15s %-10s %-30s\n" "Runtime" "Port" "URL"
    echo "------------------------------------------------------------"
    
    for result in "${started[@]}"; do
        IFS='|' read -r name port url container_name output_dir <<< "$result"
        printf "%-15s %-10s %-30s\n" "$name" "$port" "$url"
    done
    
    if [ ${#failed[@]} -gt 0 ]; then
        echo ""
        echo -e "${RED}Failed to start: ${failed[*]}${NC}"
        exit 1
    fi
    
    echo ""
    echo -e "${GREEN}All test servers are running!${NC}"
    echo -e "${BLUE}Use './scripts/test-servers.sh stop' to stop all servers${NC}"
}

# Stop all test servers
stop_all() {
    echo -e "${YELLOW}=== Stopping PulseRPC Test Servers ===${NC}"
    echo ""
    
    check_docker
    
    local containers=$(docker ps -a --format '{{.Names}}' | grep "^${CONTAINER_PREFIX}-" || true)
    
    if [ -z "$containers" ]; then
        echo -e "${YELLOW}No test server containers found${NC}"
        return 0
    fi
    
    echo "Stopping containers:"
    echo "$containers" | while read -r container; do
        echo -e "  ${YELLOW}Stopping $container...${NC}"
        docker stop "$container" >/dev/null 2>&1 || true
        docker rm "$container" >/dev/null 2>&1 || true
    done
    
    echo ""
    echo -e "${GREEN}All test server containers stopped and removed${NC}"
}

# Show status of test servers
show_status() {
    echo -e "${BLUE}=== Test Server Status ===${NC}"
    echo ""
    
    check_docker
    
    local running=$(docker ps --format '{{.Names}}\t{{.Status}}\t{{.Ports}}' | grep "^${CONTAINER_PREFIX}-" || true)
    
    if [ -z "$running" ]; then
        echo -e "${YELLOW}No test servers are currently running${NC}"
        return 0
    fi
    
    printf "%-30s %-30s %s\n" "Container" "Status" "Ports"
    echo "--------------------------------------------------------------------------------"
    echo "$running" | while IFS=$'\t' read -r name status ports; do
        printf "%-30s %-30s %s\n" "$name" "$status" "$ports"
    done
    
    echo ""
    
    # Show runtime URLs
    echo -e "${BLUE}Runtime URLs:${NC}"
    for runtime_config in "${RUNTIMES[@]}"; do
        IFS='|' read -r name plugin image port start_cmd <<< "$(parse_runtime "$runtime_config")"
        local container_name="${CONTAINER_PREFIX}-${name}"
        if docker ps --format '{{.Names}}' | grep -q "^${container_name}$"; then
            echo -e "  ${GREEN}$name${NC}: http://localhost:$port"
        fi
    done
}

# Main command handler
main() {
    case "${1:-}" in
        start)
            start_all
            ;;
        stop)
            stop_all
            ;;
        status)
            show_status
            ;;
        *)
            echo "Usage: $0 {start|stop|status}"
            echo ""
            echo "Commands:"
            echo "  start   - Start all test server containers"
            echo "  stop    - Stop all test server containers"
            echo "  status  - Show status of running containers"
            exit 1
            ;;
    esac
}

main "$@"

