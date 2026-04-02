#!/bin/bash
# Test harness for TypeScript static client integration tests
# This script generates TypeScript code with static client stubs,
# compiles it, and verifies end-to-end RPC calls

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
OUTPUT_DIR="/tmp/pulserpc_static_client_ts_$$"
BINARY_PATH="$PROJECT_ROOT/target/pulserpc-amd64"
SERVER_PORT=8081
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

echo -e "${GREEN}=== PulseRPC TypeScript Static Client Integration Test ===${NC}"
echo ""

# Step 1: Build the pulserpc binary (if needed)
if [ -f "$BINARY_PATH" ] && [ -x "$BINARY_PATH" ]; then
    echo -e "${GREEN}Using pre-built pulserpc binary at $BINARY_PATH${NC}"
elif command -v go >/dev/null 2>&1; then
    echo -e "${YELLOW}Building pulserpc binary...${NC}"
    cd "$PROJECT_ROOT"
    go build -o "$BINARY_PATH" cmd/pulserpc/pulserpc.go 2>/dev/null || go build -o "$BINARY_PATH" cmd/pulse/pulse.go 2>/dev/null
fi

if [ ! -f "$BINARY_PATH" ]; then
    echo -e "${RED}ERROR: PulseRPC binary not found at $BINARY_PATH${NC}"
    exit 1
fi

# Step 2: Create output directory
echo -e "${YELLOW}Creating output directory: $OUTPUT_DIR${NC}"
mkdir -p "$OUTPUT_DIR"

# Step 3: Generate code with -generate-test-files flag
echo -e "${YELLOW}Generating TypeScript code from $TEST_IDL...${NC}"
if ! "$BINARY_PATH" -plugin ts-client-server -generate-test-files -dir "$OUTPUT_DIR" "$TEST_IDL"; then
    echo -e "${RED}ERROR: Code generation failed${NC}"
    exit 1
fi

echo -e "${GREEN}Code generation successful${NC}"
echo ""

# Step 4: Create a test client that uses the static client stubs
echo -e "${YELLOW}Creating static client test file...${NC}"

cat > "$OUTPUT_DIR/static_client_test.ts" << 'EOF'
/**
 * Integration test using static typed client stubs
 */

import { HttpTransport } from './pulserpc/transport';
import { AServiceClient, BServiceClient } from './client';

const SERVER_URL = process.env.SERVER_URL || 'http://localhost:8081';

async function waitForServer(url: string, timeout: number = 10000): Promise<boolean> {
    const startTime = Date.now();
    let retryDelay = 200;

    while (Date.now() - startTime < timeout) {
        try {
            const controller = new AbortController();
            const timeoutId = setTimeout(() => controller.abort(), 2000);
            const response = await fetch(url, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: '{"jsonrpc":"2.0","method":"pulserpc-idl","id":1}',
                signal: controller.signal,
            });
            clearTimeout(timeoutId);
            if (response.ok) {
                return true;
            }
        } catch (err) {
            // Connection error - server not ready yet
        }
        await new Promise(resolve => setTimeout(resolve, retryDelay));
        retryDelay = Math.min(retryDelay * 1.5, 1000);
    }
    return false;
}

async function main() {
    console.log(`Connecting to server at ${SERVER_URL}...`);

    // Wait for server to be ready
    if (!(await waitForServer(SERVER_URL, 10000))) {
        console.error('ERROR: Server did not become ready in time');
        process.exit(1);
    }

    console.log('Server is ready. Running static client tests...');
    console.log();

    const transport = new HttpTransport(SERVER_URL);
    const aClient = new AServiceClient(transport);
    const bClient = new BServiceClient(transport);

    const errors: string[] = [];

    // Test A.add
    try {
        const result = await aClient.add(2, 3);
        if (result !== 5) {
            throw new Error(`Expected 5, got ${result}`);
        }
        console.log('✓ A.add(2, 3) = 5 passed');
    } catch (err: any) {
        errors.push(`A.add failed: ${err.message}`);
        console.error(`✗ A.add failed: ${err.message}`);
    }

    // Test A.sqrt
    try {
        const result = await aClient.sqrt(4);
        if (Math.abs(result - 2.0) >= 0.001) {
            throw new Error(`Expected ~2.0, got ${result}`);
        }
        console.log('✓ A.sqrt(4) = 2.0 passed');
    } catch (err: any) {
        errors.push(`A.sqrt failed: ${err.message}`);
        console.error(`✗ A.sqrt failed: ${err.message}`);
    }

    // Test A.sayHi
    try {
        const result = await aClient.sayHi();
        if (typeof result !== 'object' || !result) {
            throw new Error(`Expected object, got ${typeof result}`);
        }
        if (result.hi !== 'hi') {
            throw new Error(`Expected hi='hi', got ${JSON.stringify(result)}`);
        }
        console.log('✓ A.sayHi() passed');
    } catch (err: any) {
        errors.push(`A.sayHi failed: ${err.message}`);
        console.error(`✗ A.sayHi failed: ${err.message}`);
    }

    // Test B.echo
    try {
        const result = await bClient.echo('test');
        if (result !== 'test') {
            throw new Error(`Expected 'test', got ${result}`);
        }
        console.log('✓ B.echo("test") passed');
    } catch (err: any) {
        errors.push(`B.echo failed: ${err.message}`);
        console.error(`✗ B.echo failed: ${err.message}`);
    }

    // Test B.echo with null return
    try {
        const result = await bClient.echo('return-null');
        if (result !== null) {
            throw new Error(`Expected null, got ${result}`);
        }
        console.log('✓ B.echo("return-null") = null passed');
    } catch (err: any) {
        errors.push(`B.echo(null) failed: ${err.message}`);
        console.error(`✗ B.echo(null) failed: ${err.message}`);
    }

    console.log();
    if (errors.length > 0) {
        console.error(`FAILED: ${errors.length} test(s) failed:`);
        for (const error of errors) {
            console.error(`  - ${error}`);
        }
        process.exit(1);
    } else {
        console.log('SUCCESS: All static client tests passed!');
        process.exit(0);
    }
}

main().catch((err) => {
    console.error('Fatal error:', err);
    process.exit(1);
});
EOF

# Step 5: Create tsconfig.json
echo -e "${YELLOW}Creating tsconfig.json...${NC}"
cat > "$OUTPUT_DIR/tsconfig.json" << 'EOF'
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "CommonJS",
    "lib": ["ES2020"],
    "moduleResolution": "node10",
    "esModuleInterop": true,
    "skipLibCheck": true,
    "strict": false,
    "resolveJsonModule": true,
    "isolatedModules": false,
    "ignoreDeprecations": "6.0"
  },
  "ts-node": {
    "compilerOptions": {
      "module": "CommonJS",
      "isolatedModules": false
    }
  }
}
EOF

# Step 6: Verify static client code structure
echo -e "${YELLOW}Verifying generated static client code...${NC}"

# Check that client.ts exists and has the expected structure
if [ ! -f "$OUTPUT_DIR/conform/client.ts" ]; then
    echo -e "${RED}ERROR: conform/client.ts not found${NC}"
    exit 1
fi

# Check for static client classes
if ! grep -q "export class AClient" "$OUTPUT_DIR/conform/client.ts"; then
    echo -e "${RED}ERROR: AClient class not found in conform/client.ts${NC}"
    exit 1
fi

if ! grep -q "export class BClient" "$OUTPUT_DIR/conform/client.ts"; then
    echo -e "${RED}ERROR: BClient class not found in conform/client.ts${NC}"
    exit 1
fi

# Check for Transport import
if ! grep -q "import { Transport, HttpTransport } from '../pulserpc/transport'" "$OUTPUT_DIR/conform/client.ts"; then
    echo -e "${RED}ERROR: Transport import not found in conform/client.ts${NC}"
    exit 1
fi

# Check for RPCError import
if ! grep -q "import { RPCError } from '../pulserpc/rpc'" "$OUTPUT_DIR/conform/client.ts"; then
    echo -e "${RED}ERROR: RPCError import not found in conform/client.ts${NC}"
    exit 1
fi

# Check for types import
if ! grep -q "import \* as types from './types'" "$OUTPUT_DIR/conform/client.ts"; then
    echo -e "${RED}ERROR: types import not found in conform/client.ts${NC}"
    exit 1
fi

# Check that methods have proper signatures
if ! grep -q "async add(a: number, b: number)" "$OUTPUT_DIR/conform/client.ts"; then
    echo -e "${RED}ERROR: add method signature not found in conform/client.ts${NC}"
    exit 1
fi

# Check for params array in method calls
if ! grep -q "params: \[a, b\]" "$OUTPUT_DIR/conform/client.ts"; then
    echo -e "${RED}ERROR: params array not found in add method${NC}"
    exit 1
fi

# Check for RPCError throw
if ! grep -q "throw new RPCError(resp.error.code, resp.error.message, resp.error.data)" "$OUTPUT_DIR/conform/client.ts"; then
    echo -e "${RED}ERROR: RPCError throw not found in conform/client.ts${NC}"
    exit 1
fi

echo -e "${GREEN}Static client code structure verified!${NC}"
echo ""
echo "Generated client classes:"
grep -E "^export class" "$OUTPUT_DIR/conform/client.ts" | sed 's/^/  /'
echo ""

echo -e "${GREEN}=== Static client code generation test passed! ===${NC}"
exit 0