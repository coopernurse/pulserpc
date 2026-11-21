#!/bin/bash
# HTTP API integration test suite for PulseRPC servers
# Tests all conform.pulse methods via raw JSON-RPC requests
# Can be run against any runtime implementation (Python, TypeScript, C#)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get SERVER_URL from environment or first argument
SERVER_URL="${SERVER_URL:-${1:-http://localhost:8080}}"

if [ -z "$SERVER_URL" ]; then
    echo -e "${RED}ERROR: SERVER_URL not provided${NC}"
    echo "Usage: $0 [SERVER_URL]"
    echo "   or: SERVER_URL=http://localhost:8080 $0"
    exit 1
fi

echo -e "${GREEN}=== HTTP API Integration Tests ===${NC}"
echo -e "Testing server at: ${YELLOW}$SERVER_URL${NC}"
echo ""

# Track test results
TESTS_PASSED=0
TESTS_FAILED=0
FAILED_TESTS=()

# Test helper function
test_method() {
    local method=$1
    local params=$2
    local description="${3:-$method}"
    local expected_field="${4:-result}"
    
    local request_json
    if [ "$params" = "null" ] || [ -z "$params" ]; then
        request_json="{\"jsonrpc\":\"2.0\",\"method\":\"$method\",\"id\":1}"
    else
        request_json="{\"jsonrpc\":\"2.0\",\"method\":\"$method\",\"params\":$params,\"id\":1}"
    fi
    
    local response=$(curl -s -X POST "$SERVER_URL" \
        -H "Content-Type: application/json" \
        -d "$request_json")
    
    if echo "$response" | grep -q '"error"'; then
        echo -e "${RED}✗ FAIL${NC}: $description"
        echo "  Request: $request_json"
        echo "  Response: $response"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        FAILED_TESTS+=("$description")
        return 1
    fi
    
    if ! echo "$response" | grep -q "\"$expected_field\""; then
        echo -e "${RED}✗ FAIL${NC}: $description (missing expected field: $expected_field)"
        echo "  Request: $request_json"
        echo "  Response: $response"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        FAILED_TESTS+=("$description")
        return 1
    fi
    
    echo -e "${GREEN}✓ PASS${NC}: $description"
    TESTS_PASSED=$((TESTS_PASSED + 1))
    return 0
}

# Test helper for validation tests (expects error)
test_validation_error() {
    local method=$1
    local params=$2
    local description="${3:-$method (validation)}"
    local expected_error_code="${4:--32602}"
    
    local request_json
    if [ "$params" = "null" ] || [ -z "$params" ]; then
        request_json="{\"jsonrpc\":\"2.0\",\"method\":\"$method\",\"id\":1}"
    else
        request_json="{\"jsonrpc\":\"2.0\",\"method\":\"$method\",\"params\":$params,\"id\":1}"
    fi
    
    local response=$(curl -s -X POST "$SERVER_URL" \
        -H "Content-Type: application/json" \
        -d "$request_json")
    
    if ! echo "$response" | grep -q '"error"'; then
        echo -e "${RED}✗ FAIL${NC}: $description (expected error but got success)"
        echo "  Request: $request_json"
        echo "  Response: $response"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        FAILED_TESTS+=("$description")
        return 1
    fi
    
    # Check error code if specified
    if [ "$expected_error_code" != "" ]; then
        # Check if error code matches (handle optional spaces in JSON)
        if ! echo "$response" | grep -qE "\"code\"[[:space:]]*:[[:space:]]*$expected_error_code"; then
            # Extract actual error code from response
            local actual_code=""
            if command -v jq >/dev/null 2>&1; then
                actual_code=$(echo "$response" | jq -r '.error.code // "unknown"' 2>/dev/null || echo "unknown")
            else
                # Fallback: extract code using grep
                # Match "code": (with optional spaces) followed by optional minus sign and digits
                local code_match=$(echo "$response" | grep -o '"code"[[:space:]]*:[[:space:]]*-[0-9]*' 2>/dev/null || echo "")
                if [ -n "$code_match" ]; then
                    actual_code=$(echo "$code_match" | sed 's/.*://' | tr -d '[:space:]')
                else
                    # Try positive numbers too
                    code_match=$(echo "$response" | grep -o '"code"[[:space:]]*:[[:space:]]*[0-9]*' 2>/dev/null || echo "")
                    if [ -n "$code_match" ]; then
                        actual_code=$(echo "$code_match" | sed 's/.*://' | tr -d '[:space:]')
                    else
                        actual_code="unknown"
                    fi
                fi
            fi
            
            # A wrong error code is a real failure, not a warning (core-11).
            if [ -z "$actual_code" ] || [ "$actual_code" = "unknown" ]; then
                echo -e "${RED}✗ FAIL${NC}: $description (got error but wrong code: expected $expected_error_code, got empty/unknown)"
                echo "  Request: $request_json"
                echo "  Full response: $response"
            else
                echo -e "${RED}✗ FAIL${NC}: $description (got error but wrong code: expected $expected_error_code, got $actual_code)"
                echo "  Request: $request_json"
                echo "  Response: $response"
            fi
            TESTS_FAILED=$((TESTS_FAILED + 1))
            FAILED_TESTS+=("$description (expected error code $expected_error_code)")
            return 1
        fi
    fi
    
    echo -e "${GREEN}✓ PASS${NC}: $description"
    TESTS_PASSED=$((TESTS_PASSED + 1))
    return 0
}

# Wait for server to be ready
echo -e "${YELLOW}Checking if server is ready...${NC}"
if ! curl -s -X POST "$SERVER_URL" -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"pulserpc-idl","id":1}' > /dev/null 2>&1; then
    echo -e "${RED}ERROR: Server at $SERVER_URL is not responding${NC}"
    exit 1
fi
echo -e "${GREEN}Server is ready${NC}"
echo ""

# ============================================================================
# Interface A Tests
# ============================================================================

echo -e "${BLUE}=== Interface A Tests ===${NC}"

# A.add - Simple int addition
test_method "A.add" "[5,3]" "A.add(5, 3)" "result"

# A.calc - Array + enum parameter
# CRITICAL: This exact test case reproduces the C# server bug
# Original failing request: {"jsonrpc":"2.0","method":"A.calc","params":[[33,22],"add"],"id":1766337019052}
# Error: "The JSON value could not be converted to MathOp"
test_method "A.calc" "[[33,22],\"add\"]" "A.calc([33,22], 'add') - CRITICAL BUG TEST" "result"
test_method "A.calc" "[[10,5],\"multiply\"]" "A.calc([10,5], 'multiply')" "result"
test_method "A.calc" "[[2.5,4.0,1.5],\"add\"]" "A.calc([2.5,4.0,1.5], 'add')" "result"

# A.sqrt - Float calculation
test_method "A.sqrt" "[16.0]" "A.sqrt(16.0)" "result"
test_method "A.sqrt" "[2.25]" "A.sqrt(2.25)" "result"

# A.repeat - Struct parameter
test_method "A.repeat" "[{\"to_repeat\":\"hello\",\"count\":3,\"force_uppercase\":false}]" "A.repeat with lowercase" "result"
test_method "A.repeat" "[{\"to_repeat\":\"world\",\"count\":2,\"force_uppercase\":true}]" "A.repeat with uppercase" "result"

# A.say_hi - No parameters
test_method "A.say_hi" "null" "A.say_hi()" "result"

# A.repeat_num - Array return
test_method "A.repeat_num" "[42,5]" "A.repeat_num(42, 5)" "result"

# A.putPerson - Optional field handling (email is optional, test with null)
test_method "A.putPerson" "[{\"personId\":\"123\",\"firstName\":\"John\",\"lastName\":\"Doe\",\"email\":null}]" "A.putPerson with null email" "result"
test_method "A.putPerson" "[{\"personId\":\"456\",\"firstName\":\"Jane\",\"lastName\":\"Smith\"}]" "A.putPerson without email field" "result"

echo ""

# ============================================================================
# Interface B Tests
# ============================================================================

echo -e "${BLUE}=== Interface B Tests ===${NC}"

# B.echo - String parameter, optional return
test_method "B.echo" "[\"hello\"]" "B.echo('hello')" "result"
test_method "B.echo" "[\"return-null\"]" "B.echo('return-null') - should return null" "result"

echo ""

# ============================================================================
# Validation Tests (should return errors)
# ============================================================================

echo -e "${BLUE}=== Validation Tests (expecting errors) ===${NC}"

# Missing required parameters
test_validation_error "A.add" "[5]" "A.add with missing parameter" "-32602"
test_validation_error "A.calc" "[[33,22]]" "A.calc with missing enum parameter" "-32602"

# Wrong parameter types
test_validation_error "A.add" "[\"five\",3]" "A.add with string instead of int" "-32602"
test_validation_error "A.sqrt" "[\"not-a-number\"]" "A.sqrt with string instead of float" "-32602"

# Invalid enum values
test_validation_error "A.calc" "[[33,22],\"subtract\"]" "A.calc with invalid enum value" "-32602"
test_validation_error "A.calc" "[[33,22],\"ADD\"]" "A.calc with wrong case enum" "-32602"

# Missing jsonrpc field
bad_request='{"method":"A.add","params":[5,3],"id":1}'
response=$(curl -s -X POST "$SERVER_URL" -H "Content-Type: application/json" -d "$bad_request")
if echo "$response" | grep -q '"error"'; then
    echo -e "${GREEN}✓ PASS${NC}: Request without jsonrpc field (validation)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}✗ FAIL${NC}: Request without jsonrpc field (should return error)"
    TESTS_FAILED=$((TESTS_FAILED + 1))
    FAILED_TESTS+=("Request without jsonrpc field")
fi

# Invalid method names
test_validation_error "A.nonexistent" "[5,3]" "A.nonexistent method" "-32601"
test_validation_error "Invalid.method" "[5,3]" "Invalid interface.method" "-32601"
test_validation_error "A" "[5,3]" "Method without dot separator" "-32601"

# Wrong number of parameters
test_validation_error "A.add" "[5,3,7]" "A.add with too many parameters" "-32602"
test_validation_error "A.say_hi" "[5]" "A.say_hi with parameters (should have none)" "-32602"

echo ""

# ============================================================================
# Summary
# ============================================================================

echo -e "${BLUE}=== Test Summary ===${NC}"
echo -e "Total tests: $((TESTS_PASSED + TESTS_FAILED))"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
    echo ""
    echo -e "${RED}Failed tests:${NC}"
    for test in "${FAILED_TESTS[@]}"; do
        echo -e "  ${RED}✗${NC} $test"
    done
    exit 1
else
    echo -e "${GREEN}Failed: 0${NC}"
    echo ""
    echo -e "${GREEN}=== All tests passed! ===${NC}"
    exit 0
fi

