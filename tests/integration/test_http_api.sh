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

# ---------------------------------------------------------------------------
# Shared HTTP / JSON helpers
# ---------------------------------------------------------------------------
REQUEST_BODY=""
REQUEST_STATUS=""
REQUEST_CONTENT_TYPE=""

# Pick a JSON implementation once. python3/python is preferred because the
# probes need real type information; jq is the fallback. A candidate only
# counts when it can actually run: on Windows `python3` is often the Microsoft
# Store stub, which is on PATH but exits non-zero with an install hint.
JSON_PY=""
for candidate in python3 python; do
    if command -v "$candidate" >/dev/null 2>&1 && \
        "$candidate" -c 'import json' >/dev/null 2>&1; then
        JSON_PY="$candidate"
        break
    fi
done
JSON_JQ=""
if command -v jq >/dev/null 2>&1; then
    JSON_JQ="jq"
fi

# json_probe <dot-path> prints "<json-type>\t<compact-value>".
# Reads the last response from $REQUEST_BODY.
json_probe() {
    local path="$1"

    if [ -n "$JSON_PY" ]; then
        printf '%s' "$REQUEST_BODY" | "$JSON_PY" -c '
import json, sys

try:
    data = json.loads(sys.stdin.read())
except Exception:
    print("__INVALID__\t")
    raise SystemExit

path = sys.argv[1]
if path == "":
    cur = data
else:
    cur = data
    for part in path.split("."):
        if isinstance(cur, dict) and part in cur:
            cur = cur[part]
        elif isinstance(cur, list) and part.isdigit() and int(part) < len(cur):
            cur = cur[int(part)]
        else:
            print("__MISSING__\t")
            raise SystemExit

if cur is None:
    kind = "NoneType"
    value = "null"
elif isinstance(cur, bool):
    kind = "bool"
    value = "true" if cur else "false"
elif isinstance(cur, str):
    kind = "str"
    value = cur
elif isinstance(cur, (int, float)):
    kind = "number"
    value = json.dumps(cur, separators=(",", ":"))
elif isinstance(cur, list):
    kind = "list"
    value = json.dumps(cur, separators=(",", ":"))
elif isinstance(cur, dict):
    kind = "dict"
    value = json.dumps(cur, separators=(",", ":"))
else:
    kind = "unknown"
    value = str(cur)
print(kind + "\t" + value)
' "$path"
        return 0
    fi

    # jq fallback (sufficient for the assertions in this harness).
    if [ -n "$JSON_JQ" ]; then
        local value kind
        if [ -z "$path" ]; then
            value=$(printf '%s' "$REQUEST_BODY" | "$JSON_JQ" -c '.' 2>/dev/null) || value=""
            kind=$(printf '%s' "$REQUEST_BODY" | "$JSON_JQ" -r 'type' 2>/dev/null) || kind="__INVALID__"
        else
            value=$(printf '%s' "$REQUEST_BODY" | "$JSON_JQ" -c ".$path" 2>/dev/null) || value=""
            kind=$(printf '%s' "$REQUEST_BODY" | "$JSON_JQ" -r ".$path | type" 2>/dev/null) || kind="__INVALID__"
        fi
        case "$kind" in
            object) kind="dict" ;;
            array) kind="list" ;;
            string) kind="str" ;;
            number) kind="number" ;;
            boolean) kind="bool" ;;
            null) kind="NoneType" ;;
            "") kind="__MISSING__" ;;
        esac
        if [ "$kind" = "str" ]; then
            value=$(printf '%s' "$value" | sed 's/^"//; s/"$//')
        fi
        printf '%s\t%s\n' "$kind" "$value"
        return 0
    fi

    # Last-resort grep/sed fallback. It handles the scalar probes but cannot
    # validate nested structure; python3 or jq is recommended.
    local key="${path##*.}"
    local raw
    if [ -z "$path" ]; then
        raw="$REQUEST_BODY"
        case "$(printf '%s' "$raw" | tr -d '[:space:]' | cut -c1)" in
            '[') printf 'list\t\n' ;;
            '{') printf 'dict\t\n' ;;
            *) printf '__MISSING__\t\n' ;;
        esac
        return 0
    fi
    raw=$(printf '%s' "$REQUEST_BODY" | grep -o "\"$key\"[[:space:]]*:[[:space:]]*[^,}]*" | head -1 | sed 's/^[^:]*:[[:space:]]*//') || raw=""
    if [ -z "$raw" ]; then
        printf '__MISSING__\t\n'
    else
        case "$raw" in
            \"*\") printf 'str\t%s\n' "$(printf '%s' "$raw" | tr -d '"')" ;;
            true|false) printf 'bool\t%s\n' "$raw" ;;
            null) printf 'NoneType\tnull\n' ;;
            \[*) printf 'list\t%s\n' "$raw" ;;
            \{*) printf 'dict\t%s\n' "$raw" ;;
            *) printf 'number\t%s\n' "$raw" ;;
        esac
    fi
}

json_get() {
    json_probe "$1" | cut -f2-
}

json_type() {
    json_probe "$1" | cut -f1
}

# do_request <METHOD> <body> [url] [max-time] sets REQUEST_STATUS/REQUEST_BODY/
# REQUEST_CONTENT_TYPE.
do_request() {
    local method="$1"
    local body="${2:-}"
    local url="${3:-$SERVER_URL}"
    local max_time="${4:-30}"
    local body_file headers_file payload_file

    body_file=$(mktemp "${TMPDIR:-/tmp}/pulserpc-body.XXXXXX")
    headers_file=$(mktemp "${TMPDIR:-/tmp}/pulserpc-headers.XXXXXX")
    payload_file=$(mktemp "${TMPDIR:-/tmp}/pulserpc-payload.XXXXXX")
    printf '%s' "$body" > "$payload_file"

    if [ "$method" = "POST" ]; then
        REQUEST_STATUS=$(curl -sS --connect-timeout 5 --max-time "$max_time" -o "$body_file" -D "$headers_file" -w '%{http_code}' \
            -X POST "$url" -H "Content-Type: application/json" \
            --data-binary "@$payload_file" 2>/dev/null || echo "000")
    else
        REQUEST_STATUS=$(curl -sS --connect-timeout 5 --max-time "$max_time" -o "$body_file" -D "$headers_file" -w '%{http_code}' \
            -X "$method" "$url" 2>/dev/null || echo "000")
    fi

    REQUEST_BODY=$(cat "$body_file")
    REQUEST_CONTENT_TYPE=$(tr -d '\r' < "$headers_file" | awk 'BEGIN{IGNORECASE=1} /^Content-Type:/{sub(/^[^:]*:[ \t]*/, ""); print; exit}')
    rm -f "$body_file" "$headers_file" "$payload_file"
}

record_result() {
    local description="$1"
    local ok="$2"
    local detail="${3:-}"

    if [ "$ok" = "1" ]; then
        echo -e "${GREEN}✓ PASS${NC}: $description"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    fi

    echo -e "${RED}✗ FAIL${NC}: $description"
    if [ -n "$detail" ]; then
        echo "  $detail"
    fi
    TESTS_FAILED=$((TESTS_FAILED + 1))
    FAILED_TESTS+=("$description")
    # Keep running so the summary lists every failing probe; the final exit
    # status is driven by TESTS_FAILED.
    return 0
}

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

    do_request POST "$request_json"

    local ok=1
    local detail=""
    if [ "$REQUEST_STATUS" != "200" ]; then
        ok=0
        detail="${detail}expected HTTP 200, got $REQUEST_STATUS; "
    fi
    if ! printf '%s' "$REQUEST_CONTENT_TYPE" | grep -qi 'application/json'; then
        ok=0
        detail="${detail}expected Content-Type application/json, got '$REQUEST_CONTENT_TYPE'; "
    fi
    if [ "$(json_type "")" != "dict" ]; then
        ok=0
        detail="${detail}response is not a JSON object; "
    else
        if [ "$(json_get jsonrpc)" != "2.0" ]; then
            ok=0
            detail="${detail}jsonrpc != 2.0; "
        fi
        if [ "$(json_get id)" != "1" ]; then
            ok=0
            detail="${detail}id was not echoed as 1; "
        fi
        if [ "$(json_type error)" != "__MISSING__" ]; then
            ok=0
            detail="${detail}unexpected error object; "
        fi
        if [ "$(json_type "$expected_field")" = "__MISSING__" ]; then
            ok=0
            detail="${detail}missing expected field: $expected_field; "
        fi
    fi

    if ! record_result "$description" "$ok" "${detail}Request: $request_json Response: $REQUEST_BODY"; then
        return 1
    fi
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

    do_request POST "$request_json"

    local ok=1
    local detail=""
    if [ "$REQUEST_STATUS" != "200" ]; then
        ok=0
        detail="${detail}expected HTTP 200, got $REQUEST_STATUS; "
    fi
    if ! printf '%s' "$REQUEST_CONTENT_TYPE" | grep -qi 'application/json'; then
        ok=0
        detail="${detail}expected Content-Type application/json, got '$REQUEST_CONTENT_TYPE'; "
    fi
    if [ "$(json_type "")" != "dict" ]; then
        ok=0
        detail="${detail}response is not a JSON object; "
    else
        if [ "$(json_get jsonrpc)" != "2.0" ]; then
            ok=0
            detail="${detail}jsonrpc != 2.0; "
        fi
        if [ "$(json_get id)" != "1" ]; then
            ok=0
            detail="${detail}id was not echoed as 1; "
        fi
        if [ "$(json_type error)" = "__MISSING__" ]; then
            ok=0
            detail="${detail}expected an error object but got success; "
        else
            local actual_code
            actual_code=$(json_get error.code)
            if [ -n "$expected_error_code" ] && [ "$actual_code" != "$expected_error_code" ]; then
                ok=0
                detail="${detail}expected error code $expected_error_code, got $actual_code; "
            fi
            if [ "$(json_type error.message)" != "str" ]; then
                ok=0
                detail="${detail}error.message is not a string; "
            fi
        fi
    fi

    if ! record_result "$description" "$ok" "${detail}Request: $request_json Response: $REQUEST_BODY"; then
        return 1
    fi
    return 0
}

# Wait for server to be ready
echo -e "${YELLOW}Checking if server is ready...${NC}"
if ! curl -sS --connect-timeout 5 --max-time 30 -X POST "$SERVER_URL" -H "Content-Type: application/json" \
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

# Structured validation data (conformance): error.data must be an object with
# invalid_fields listing the offending parameter path. Locks in the shape that
# Python/TS already emit; Go/Java/C# currently emit a plain string (FAIL).
do_request POST '{"jsonrpc":"2.0","method":"A.add","params":["five",3],"id":99}'
probe_ok=1
probe_detail=""
if [ "$REQUEST_STATUS" != "200" ]; then
    probe_ok=0; probe_detail="${probe_detail}expected HTTP 200, got $REQUEST_STATUS; "
fi
if [ "$(json_type "")" != "dict" ]; then
    probe_ok=0; probe_detail="${probe_detail}response is not an object; "
else
    [ "$(json_get error.code)" = "-32602" ] || { probe_ok=0; probe_detail="${probe_detail}error.code != -32602; "; }
    if [ "$(json_type error.data)" != "dict" ]; then
        probe_ok=0; probe_detail="${probe_detail}error.data is not an object (got $(json_type error.data)); "
    else
        [ "$(json_type error.data.invalid_fields)" != "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}error.data.invalid_fields missing; "; }
    fi
fi
record_result "Validation error carries structured data (invalid_fields)" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

# Invalid enum values
test_validation_error "A.calc" "[[33,22],\"subtract\"]" "A.calc with invalid enum value" "-32602"
test_validation_error "A.calc" "[[33,22],\"ADD\"]" "A.calc with wrong case enum" "-32602"

# Missing jsonrpc field
do_request POST '{"method":"A.add","params":[5,3],"id":1}'
probe_ok=1
probe_detail=""
if [ "$REQUEST_STATUS" != "200" ]; then
    probe_ok=0; probe_detail="${probe_detail}expected HTTP 200, got $REQUEST_STATUS; "
fi
if [ "$(json_type "")" != "dict" ]; then
    probe_ok=0; probe_detail="${probe_detail}response is not a JSON object; "
else
    [ "$(json_get jsonrpc)" = "2.0" ] || { probe_ok=0; probe_detail="${probe_detail}jsonrpc != 2.0; "; }
    [ "$(json_get id)" = "null" ] || { probe_ok=0; probe_detail="${probe_detail}id is not null; "; }
    [ "$(json_get error.code)" = "-32600" ] || { probe_ok=0; probe_detail="${probe_detail}error.code != -32600; "; }
fi
record_result "Request without jsonrpc field returns -32600" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

# Invalid method names
test_validation_error "A.nonexistent" "[5,3]" "A.nonexistent method" "-32601"
test_validation_error "Invalid.method" "[5,3]" "Invalid interface.method" "-32601"
test_validation_error "A" "[5,3]" "Method without dot separator" "-32601"

# Wrong number of parameters
test_validation_error "A.add" "[5,3,7]" "A.add with too many parameters" "-32602"
test_validation_error "A.say_hi" "[5]" "A.say_hi with parameters (should have none)" "-32602"

echo ""

# ============================================================================
# Protocol Envelope Tests (M18-M28, R22-R24)
# ============================================================================

echo -e "${BLUE}=== Protocol Envelope Tests ===${NC}"

# C2/C4/C5: pulserpc-idl returns the canonical IDL artifact as an object.
do_request POST '{"jsonrpc":"2.0","method":"pulserpc-idl","id":1}'
probe_ok=1
probe_detail=""
if [ "$(json_type "")" != "dict" ]; then
    probe_ok=0; probe_detail="${probe_detail}response is not an object; "
elif [ "$(json_type result)" = "__MISSING__" ]; then
    probe_ok=0; probe_detail="${probe_detail}response has no result; "
elif [ "$(json_type result)" != "dict" ]; then
    probe_ok=0; probe_detail="${probe_detail}pulserpc-idl result must be an object, got $(json_type result); "
else
    [ "$(json_get result.rootNamespace)" = "conform" ] || { probe_ok=0; probe_detail="${probe_detail}result.rootNamespace != conform; "; }
    [ "$(json_type result.checksum)" = "str" ] || { probe_ok=0; probe_detail="${probe_detail}result.checksum is not a string; "; }
    [ "$(json_type result.interfaces)" != "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}result.interfaces is missing; "; }
    [ "$(json_type result.structs)" != "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}result.structs is missing; "; }
    [ "$(json_type result.enums)" != "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}result.enums is missing; "; }
    [ "$(json_type result.errors)" != "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}result.errors is missing; "; }
    checksum=$(json_get result.checksum)
    [ "${#checksum}" = "43" ] || { probe_ok=0; probe_detail="${probe_detail}result.checksum is not 43 characters; "; }
fi
record_result "pulserpc-idl serves the canonical IDL object" "$probe_ok" "${probe_detail}Response: $(printf '%s' "$REQUEST_BODY" | cut -c1-300)"

probe_ok=0
probe_detail=""

# M26: malformed JSON -> -32700
do_request POST '{"jsonrpc":"2.0",'
probe_ok=1
probe_detail=""
if [ "$REQUEST_STATUS" != "200" ]; then
    probe_ok=0; probe_detail="${probe_detail}expected HTTP 200, got $REQUEST_STATUS; "
fi
if [ "$(json_type "")" != "dict" ]; then
    probe_ok=0; probe_detail="${probe_detail}response is not an object; "
else
    [ "$(json_get jsonrpc)" = "2.0" ] || { probe_ok=0; probe_detail="${probe_detail}jsonrpc != 2.0; "; }
    [ "$(json_get error.code)" = "-32700" ] || { probe_ok=0; probe_detail="${probe_detail}error.code != -32700; "; }
    [ "$(json_get id)" = "null" ] || { probe_ok=0; probe_detail="${probe_detail}id is not null; "; }
fi
record_result "Malformed JSON returns -32700" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

# M18: empty batch -> a single -32600 object (not an array)
do_request POST '[]'
probe_ok=1
probe_detail=""
if [ "$REQUEST_STATUS" != "200" ]; then
    probe_ok=0; probe_detail="${probe_detail}expected HTTP 200, got $REQUEST_STATUS; "
fi
if [ "$(json_type "")" != "dict" ]; then
    probe_ok=0; probe_detail="${probe_detail}empty batch must return a single object, got $(json_type ""); "
else
    [ "$(json_get error.code)" = "-32600" ] || { probe_ok=0; probe_detail="${probe_detail}error.code != -32600; "; }
    [ "$(json_get id)" = "null" ] || { probe_ok=0; probe_detail="${probe_detail}id is not null; "; }
fi
record_result "Empty batch returns one -32600 object" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

# M19: non-object batch entry -> per-entry -32600, response stays an array
do_request POST '[5]'
probe_ok=1
probe_detail=""
[ "$(json_type "")" = "list" ] || { probe_ok=0; probe_detail="${probe_detail}response is not an array; "; }
[ "$(json_get 0.error.code)" = "-32600" ] || { probe_ok=0; probe_detail="${probe_detail}entry error.code != -32600; "; }
[ "$(json_get 0.id)" = "null" ] || { probe_ok=0; probe_detail="${probe_detail}entry id is not null; "; }
record_result "Non-object batch entry returns per-entry -32600" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

# R23/M28: valid notification -> no body
do_request POST '{"jsonrpc":"2.0","method":"A.add","params":[1,2]}'
probe_ok=1
probe_detail=""
case "$REQUEST_STATUS" in 200|204) ;; *) probe_ok=0; probe_detail="${probe_detail}unexpected status $REQUEST_STATUS; ";; esac
[ -z "$REQUEST_BODY" ] || { probe_ok=0; probe_detail="${probe_detail}notification returned a body: $REQUEST_BODY; "; }
record_result "Valid notification returns no body" "$probe_ok" "$probe_detail"

# R23/M28: invalid-method notification -> still no body
do_request POST '{"jsonrpc":"2.0","method":"No.such"}'
probe_ok=1
probe_detail=""
case "$REQUEST_STATUS" in 200|204) ;; *) probe_ok=0; probe_detail="${probe_detail}unexpected status $REQUEST_STATUS; ";; esac
[ -z "$REQUEST_BODY" ] || { probe_ok=0; probe_detail="${probe_detail}notification error returned a body: $REQUEST_BODY; "; }
record_result "Invalid notification returns no body" "$probe_ok" "$probe_detail"

# M28: mixed batch -> only the request response
do_request POST '[{"jsonrpc":"2.0","method":"A.add","params":[1,2],"id":7},{"jsonrpc":"2.0","method":"A.add","params":[3,4]}]'
probe_ok=1
probe_detail=""
[ "$(json_type "")" = "list" ] || { probe_ok=0; probe_detail="${probe_detail}response is not an array; "; }
[ "$(json_get 0.id)" = "7" ] || { probe_ok=0; probe_detail="${probe_detail}request response id != 7; "; }
[ "$(json_type 0.result)" != "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}request response has no result; "; }
[ "$(json_type 1)" = "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}notification produced a second response; "; }
record_result "Mixed batch returns only the request response" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

# M21: id:null is a notification
do_request POST '{"jsonrpc":"2.0","method":"A.add","params":[1,2],"id":null}'
probe_ok=1
probe_detail=""
case "$REQUEST_STATUS" in 200|204) ;; *) probe_ok=0; probe_detail="${probe_detail}unexpected status $REQUEST_STATUS; ";; esac
[ -z "$REQUEST_BODY" ] || { probe_ok=0; probe_detail="${probe_detail}id:null returned a body: $REQUEST_BODY; "; }
record_result "id:null is treated as a notification" "$probe_ok" "$probe_detail"

# M20: object-form params are handled explicitly (result or -32602)
do_request POST '{"jsonrpc":"2.0","method":"A.add","params":{"a":5,"b":3},"id":11}'
probe_ok=1
probe_detail=""
[ "$(json_get id)" = "11" ] || { probe_ok=0; probe_detail="${probe_detail}id was not echoed as 11; "; }
if [ "$(json_type result)" != "__MISSING__" ]; then
    :
elif [ "$(json_get error.code)" = "-32602" ]; then
    [ "$(json_type error.message)" = "str" ] || { probe_ok=0; probe_detail="${probe_detail}error.message is not a string; "; }
else
    probe_ok=0
    probe_detail="${probe_detail}object-form params were silently dropped (no result and no -32602); "
fi
record_result "Object-form params get an explicit result or -32602" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

# M12: extra struct fields and extra request fields are accepted
do_request POST '{"jsonrpc":"2.0","method":"A.repeat","params":[{"to_repeat":"hi","count":2,"force_uppercase":false,"unexpected":"ignored"}],"id":12,"trace":"ignored"}'
probe_ok=1
probe_detail=""
[ "$(json_get id)" = "12" ] || { probe_ok=0; probe_detail="${probe_detail}id was not echoed as 12; "; }
[ "$(json_type result)" != "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}extra fields caused the call to fail: $REQUEST_BODY; "; }
record_result "Extra struct/request fields are accepted" "$probe_ok" "$probe_detail"

# R24/D2: boolean id -> -32600
do_request POST '{"jsonrpc":"2.0","method":"A.add","params":[1,2],"id":true}'
probe_ok=1
probe_detail=""
if [ "$(json_type "")" != "dict" ]; then
    probe_ok=0; probe_detail="${probe_detail}response is not an object; "
else
    [ "$(json_get error.code)" = "-32600" ] || { probe_ok=0; probe_detail="${probe_detail}boolean id did not return -32600; "; }
    [ "$(json_get id)" = "null" ] || { probe_ok=0; probe_detail="${probe_detail}error response id is not null; "; }
fi
record_result "Boolean id returns -32600" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

# R24/D2: object id -> -32600
do_request POST '{"jsonrpc":"2.0","method":"A.add","params":[1,2],"id":{"x":1}}'
probe_ok=1
probe_detail=""
if [ "$(json_type "")" != "dict" ]; then
    probe_ok=0; probe_detail="${probe_detail}response is not an object; "
else
    [ "$(json_get error.code)" = "-32600" ] || { probe_ok=0; probe_detail="${probe_detail}object id did not return -32600; "; }
    [ "$(json_get id)" = "null" ] || { probe_ok=0; probe_detail="${probe_detail}error response id is not null; "; }
fi
record_result "Object id returns -32600" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

# D3/R22/S8: non-POST requests -> HTTP 405 with a JSON-RPC -32600 body
for http_method in GET PUT; do
    do_request "$http_method" ""
    probe_ok=1
    probe_detail=""
    [ "$REQUEST_STATUS" = "405" ] || { probe_ok=0; probe_detail="${probe_detail}expected HTTP 405, got $REQUEST_STATUS; "; }
    printf '%s' "$REQUEST_CONTENT_TYPE" | grep -qi 'application/json' || { probe_ok=0; probe_detail="${probe_detail}Content-Type is not application/json; "; }
    if [ "$(json_type "")" != "dict" ]; then
        probe_ok=0; probe_detail="${probe_detail}body is not a JSON-RPC error object; "
    else
        [ "$(json_get jsonrpc)" = "2.0" ] || { probe_ok=0; probe_detail="${probe_detail}jsonrpc != 2.0; "; }
        [ "$(json_get error.code)" = "-32600" ] || { probe_ok=0; probe_detail="${probe_detail}error.code != -32600; "; }
        [ "$(json_get id)" = "null" ] || { probe_ok=0; probe_detail="${probe_detail}id is not null; "; }
    fi
    record_result "$http_method is rejected with 405 and -32600" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"
done

# D4/R15/S6: if a runtime can be induced to return -32603, it must not leak
# details. A negative array allocation triggers an internal error in several
# runtimes; runtimes that validate it away are skipped.
do_request POST '{"jsonrpc":"2.0","method":"A.repeat","params":[{"to_repeat":"x","count":-1,"force_uppercase":false}],"id":13}'
if [ "$(json_get error.code)" = "-32603" ]; then
    probe_ok=1
    probe_detail=""
    [ "$(json_get id)" = "13" ] || { probe_ok=0; probe_detail="${probe_detail}id was not echoed; "; }
    [ "$(json_get error.message)" = "Internal error" ] || { probe_ok=0; probe_detail="${probe_detail}error.message is not the fixed 'Internal error' text; "; }
    [ "$(json_type error.data)" = "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}error.data must be absent for -32603; "; }
    record_result "-32603 is redacted (message, no data)" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"
else
    echo -e "${YELLOW}SKIP${NC}: -32603 redaction (runtime did not produce an internal error for the probe)"
fi

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

