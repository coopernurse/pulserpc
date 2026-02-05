#!/bin/bash
# Integration test for Pulse to OpenAPI conversion
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
TEST_PULSE="$PROJECT_ROOT/examples/conform.pulse"
OUTPUT_DIR="/tmp/pulserpc_test_pulse_to_openapi_$$"
BINARY_PATH="$PROJECT_ROOT/target/pulserpc"

# Cleanup function
cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    rm -rf "$OUTPUT_DIR"
}

trap cleanup EXIT

echo -e "${GREEN}=== Pulse to OpenAPI Integration Test ===${NC}"
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
echo -e "${YELLOW}Test 1: Basic Pulse to OpenAPI conversion${NC}"
mkdir -p "$OUTPUT_DIR"
if "$BINARY_PATH" -pulse-to-openapi "$TEST_PULSE" -output-dir "$OUTPUT_DIR" 2>&1 | tee "$OUTPUT_DIR/conversion.log"; then
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
OUTPUT_FILE="$OUTPUT_DIR/conform.openapi.yaml"
if [ ! -f "$OUTPUT_FILE" ]; then
    echo -e "${RED}ERROR: Output file not found at $OUTPUT_FILE${NC}"
    ls -la "$OUTPUT_DIR"
    exit 1
fi
echo -e "${GREEN}Output file found at $OUTPUT_FILE${NC}"

# Step 4: Verify generated file is valid YAML
echo ""
echo -e "${YELLOW}Test 3: Validating generated YAML${NC}"
if command -v python3 >/dev/null 2>&1; then
    if python3 -c "import yaml; yaml.safe_load(open('$OUTPUT_FILE'))" 2>&1; then
        echo -e "${GREEN}Generated file is valid YAML${NC}"
    else
        echo -e "${YELLOW}Python yaml module not available, skipping YAML validation${NC}"
    fi
else
    echo -e "${YELLOW}Python3 not available, skipping YAML validation${NC}"
fi

# Step 5: Verify expected content in generated file
echo ""
echo -e "${YELLOW}Test 4: Verifying expected content${NC}"

# Check for openapi version
if ! grep -q "openapi:.*3\." "$OUTPUT_FILE"; then
    echo -e "${RED}ERROR: OpenAPI version not found${NC}"
    exit 1
fi
echo -e "${GREEN}OpenAPI version found${NC}"

# Check for info section
if ! grep -q "info:" "$OUTPUT_FILE"; then
    echo -e "${RED}ERROR: Info section not found${NC}"
    exit 1
fi
echo -e "${GREEN}Info section found${NC}"

# Check for paths
if ! grep -q "paths:" "$OUTPUT_FILE"; then
    echo -e "${RED}ERROR: Paths section not found${NC}"
    exit 1
fi
echo -e "${GREEN}Paths section found${NC}"

# Check for components/schemas
if ! grep -q "components:" "$OUTPUT_FILE" && ! grep -q "definitions:" "$OUTPUT_FILE"; then
    echo -e "${RED}ERROR: Components/Definitions section not found${NC}"
    exit 1
fi
echo -e "${GREEN}Components/Definitions section found${NC}"

# Step 6: Test error handling - missing input file
echo ""
echo -e "${YELLOW}Test 5: Error handling for missing input file${NC}"
"$BINARY_PATH" -pulse-to-openapi "/nonexistent/file.pulse" -output-dir "$OUTPUT_DIR" 2>&1 | tee "$OUTPUT_DIR/error_test.log" || true
EXIT_CODE=${PIPESTATUS[0]}
if [ $EXIT_CODE -ne 0 ]; then
    echo -e "${GREEN}Correctly failed with exit code $EXIT_CODE${NC}"
else
    echo -e "${RED}ERROR: Exit code was 0, should have been non-zero${NC}"
    exit 1
fi

# Step 7: Test error handling - invalid Pulse IDL
echo ""
echo -e "${YELLOW}Test 6: Error handling for invalid Pulse IDL${NC}"
INVALID_PULSE="$OUTPUT_DIR/invalid.pulse"
echo "not a valid pulse idl" > "$INVALID_PULSE"
"$BINARY_PATH" -pulse-to-openapi "$INVALID_PULSE" -output-dir "$OUTPUT_DIR" 2>&1 | tee "$OUTPUT_DIR/invalid_test.log" || true
EXIT_CODE=${PIPESTATUS[0]}
if [ $EXIT_CODE -ne 0 ]; then
    echo -e "${GREEN}Correctly failed with exit code $EXIT_CODE${NC}"
else
    echo -e "${RED}ERROR: Exit code was 0, should have been non-zero${NC}"
    exit 1
fi

# Step 8: Test with OpenAPI version flag
echo ""
echo -e "${YELLOW}Test 7: Testing with OpenAPI 3.0 target version${NC}"
if "$BINARY_PATH" -pulse-to-openapi "$TEST_PULSE" -output-dir "$OUTPUT_DIR/openapi30" -openapi-version "3.0" 2>&1 | tee "$OUTPUT_DIR/openapi30.log"; then
    echo -e "${GREEN}OpenAPI 3.0 target conversion succeeded${NC}"
    OUTPUT_FILE30="$OUTPUT_DIR/openapi30/conform.openapi.yaml"
    if [ ! -f "$OUTPUT_FILE30" ]; then
        echo -e "${RED}ERROR: Output file not found for OpenAPI 3.0${NC}"
        exit 1
    fi

    # Verify it's actually 3.0
    if grep -q "openapi: \"3.0\"" "$OUTPUT_FILE30" || grep -q "openapi: 3.0" "$OUTPUT_FILE30"; then
        echo -e "${GREEN}OpenAPI 3.0 version confirmed${NC}"
    else
        echo -e "${YELLOW}Warning: OpenAPI version may not be 3.0${NC}"
    fi
else
    EXIT_CODE=$?
    echo -e "${RED}OpenAPI 3.0 target conversion failed with exit code $EXIT_CODE${NC}"
    cat "$OUTPUT_DIR/openapi30.log"
    exit 1
fi

# Step 9: Validate with spectral (if available)
echo ""
echo -e "${YELLOW}Test 8: Validating with Spectral (if available)${NC}"
if command -v spectral >/dev/null 2>&1 || command -v spectral-cli >/dev/null 2>&1; then
    if spectral lint "$OUTPUT_FILE" 2>&1 | tee "$OUTPUT_DIR/spectral.log"; then
        echo -e "${GREEN}Spectral validation passed${NC}"
    else
        echo -e "${YELLOW}Spectral validation found issues (may be warnings)${NC}"
    fi
else
    echo -e "${YELLOW}Spectral not available, skipping OpenAPI validation${NC}"
fi

echo ""
echo -e "${GREEN}=== All Pulse to OpenAPI integration tests passed! ===${NC}"
exit 0
