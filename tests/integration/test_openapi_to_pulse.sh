#!/bin/bash
# Integration test for OpenAPI to Pulse conversion
# This script validates the CLI exit codes and output file creation

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEST_OPENAPI="$PROJECT_ROOT/pkg/openapi/testdata/petstore-openapi30.yaml"
OUTPUT_DIR="/tmp/pulserpc_test_openapi_to_pulse_$$"
BINARY_PATH="$PROJECT_ROOT/target/pulserpc"

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    rm -rf "$OUTPUT_DIR"
}

trap cleanup EXIT

echo -e "${GREEN}=== OpenAPI to Pulse Integration Test ===${NC}"
echo ""

# Step 1: Build the pulserpc binary if needed
if [ ! -f "$BINARY_PATH" ]; then
    echo -e "${YELLOW}Building pulserpc binary...${NC}"
    cd "$PROJECT_ROOT"
    make build
fi

if [ ! -f "$BINARY_PATH" ]; then
    echo -e "${RED}ERROR: PulseRPC binary not found at $BINARY_PATH${NC}"
    exit 1
fi

echo -e "${GREEN}Using pulserpc binary at $BINARY_PATH${NC}"
echo ""

# Step 2: Test basic conversion
echo -e "${YELLOW}Test 1: Basic OpenAPI to Pulse conversion${NC}"
mkdir -p "$OUTPUT_DIR"
if "$BINARY_PATH" -openapi-to-pulse "$TEST_OPENAPI" -output-dir "$OUTPUT_DIR" 2>&1 | tee "$OUTPUT_DIR/conversion.log"; then
    echo -e "${GREEN}Conversion succeeded${NC}"
else
    EXIT_CODE=$?
    echo -e "${RED}Conversion failed with exit code $EXIT_CODE${NC}"
    cat "$OUTPUT_DIR/conversion.log"
    exit 1
fi

# Step 3: Verify output file exists
echo ""
echo -e "${YELLOW}Test 2: Verifying output file exists${NC}"
OUTPUT_FILE="$OUTPUT_DIR/petstore-openapi30.pulse"
if [ ! -f "$OUTPUT_FILE" ]; then
    echo -e "${RED}ERROR: Output file not found at $OUTPUT_FILE${NC}"
    ls -la "$OUTPUT_DIR"
    exit 1
fi
echo -e "${GREEN}Output file found at $OUTPUT_FILE${NC}"

# Step 4: Verify generated file is valid Pulse IDL
echo ""
echo -e "${YELLOW}Test 3: Validating generated Pulse IDL${NC}"
mkdir -p "$OUTPUT_DIR/generated"
cd "$OUTPUT_DIR/generated"
go mod init pulserpc_test 2>/dev/null || true
cd - > /dev/null
if "$BINARY_PATH" -plugin go-client-server -dir "$OUTPUT_DIR/generated" "$OUTPUT_FILE" 2>&1 | tee "$OUTPUT_DIR/validation.log"; then
    echo -e "${GREEN}Generated IDL is valid${NC}"
else
    EXIT_CODE=$?
    echo -e "${RED}Generated IDL is invalid (exit code $EXIT_CODE)${NC}"
    cat "$OUTPUT_DIR/validation.log"
    exit 1
fi

# Step 5: Verify expected content in generated file
echo ""
echo -e "${YELLOW}Test 4: Verifying expected content${NC}"

# Check for namespace
if ! grep -q "namespace" "$OUTPUT_FILE"; then
    echo -e "${RED}ERROR: No namespace declaration found${NC}"
    exit 1
fi
echo -e "${GREEN}Namespace declaration found${NC}"

# Check for struct (Petstore should have Pet type)
if ! grep -q "struct Pet" "$OUTPUT_FILE"; then
    echo -e "${RED}ERROR: Expected struct 'Pet' not found${NC}"
    exit 1
fi
echo -e "${GREEN}Struct 'Pet' found${NC}"

# Check for interface
if ! grep -q "interface" "$OUTPUT_FILE"; then
    echo -e "${RED}ERROR: No interface declaration found${NC}"
    exit 1
fi
echo -e "${GREEN}Interface declaration found${NC}"

# Step 6: Test error handling - missing input file
echo ""
echo -e "${YELLOW}Test 5: Error handling for missing input file${NC}"
"$BINARY_PATH" -openapi-to-pulse "/nonexistent/file.yaml" -output-dir "$OUTPUT_DIR" 2>&1 | tee "$OUTPUT_DIR/error_test.log" || true
EXIT_CODE=${PIPESTATUS[0]}
if [ $EXIT_CODE -ne 0 ]; then
    echo -e "${GREEN}Correctly failed with exit code $EXIT_CODE${NC}"
else
    echo -e "${RED}ERROR: Exit code was 0, should have been non-zero${NC}"
    exit 1
fi

# Step 7: Test error handling - invalid OpenAPI spec
echo ""
echo -e "${YELLOW}Test 6: Error handling for invalid OpenAPI spec${NC}"
INVALID_SPEC="$OUTPUT_DIR/invalid.yaml"
echo "not a valid openapi spec" > "$INVALID_SPEC"
"$BINARY_PATH" -openapi-to-pulse "$INVALID_SPEC" -output-dir "$OUTPUT_DIR" 2>&1 | tee "$OUTPUT_DIR/invalid_test.log" || true
EXIT_CODE=${PIPESTATUS[0]}
if [ $EXIT_CODE -ne 0 ]; then
    echo -e "${GREEN}Correctly failed with exit code $EXIT_CODE${NC}"
else
    echo -e "${RED}ERROR: Exit code was 0, should have been non-zero${NC}"
    exit 1
fi

# Step 8: Test with OpenAPI 3.1 spec
echo ""
echo -e "${YELLOW}Test 7: Testing with OpenAPI 3.1 spec${NC}"
TEST_OPENAPI31="$PROJECT_ROOT/pkg/openapi/testdata/petstore-openapi31.yaml"
if "$BINARY_PATH" -openapi-to-pulse "$TEST_OPENAPI31" -output-dir "$OUTPUT_DIR/openapi31" 2>&1 | tee "$OUTPUT_DIR/openapi31.log"; then
    echo -e "${GREEN}OpenAPI 3.1 conversion succeeded${NC}"
    OUTPUT_FILE31="$OUTPUT_DIR/openapi31/petstore-openapi31.pulse"
    if [ ! -f "$OUTPUT_FILE31" ]; then
        echo -e "${RED}ERROR: Output file not found for OpenAPI 3.1${NC}"
        exit 1
    fi
    echo -e "${GREEN}OpenAPI 3.1 output file found${NC}"
else
    EXIT_CODE=$?
    echo -e "${RED}OpenAPI 3.1 conversion failed with exit code $EXIT_CODE${NC}"
    cat "$OUTPUT_DIR/openapi31.log"
    exit 1
fi

echo ""
echo -e "${GREEN}=== All OpenAPI to Pulse integration tests passed! ===${NC}"
exit 0
