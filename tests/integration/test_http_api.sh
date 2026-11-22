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

# json_encode_string <raw-string> prints a JSON string literal for the value.
# json_get deliberately returns strings without their JSON quotes (other probes
# depend on that), so a probe comparing against a JSON literal must re-encode
# string values first. Numbers, booleans, arrays and objects are already valid
# JSON from json_get and are passed through unchanged.
json_encode_string() {
    local value="$1"

    if [ -n "$JSON_PY" ]; then
        "$JSON_PY" -c 'import json, sys; print(json.dumps(sys.argv[1]))' "$value"
        return 0
    fi
    if [ -n "$JSON_JQ" ]; then
        "$JSON_JQ" -n --arg v "$value" '$v'
        return 0
    fi
    # Last-resort fallback: escape backslashes and double quotes.
    printf '"%s"' "$(printf '%s' "$value" | sed 's/\\/\\\\/g; s/"/\\"/g')"
}

# values_equal <actual-json> <expected-json> compares two JSON values by type
# and structure. Integers, booleans and strings must match exactly; numbers
# that are declared as floats (written with a fraction or exponent in the
# expected value) are compared with a numeric tolerance so that JSON renderings
# such as 5 and 5.0 are equivalent. Arrays and objects are compared deeply.
values_equal() {
    local actual="$1"
    local expected="$2"

    if [ -n "$JSON_PY" ]; then
        "$JSON_PY" -c '
import json, math, sys

def number_equal(a, b):
    if isinstance(a, float) or isinstance(b, float):
        x, y = float(a), float(b)
        if math.isnan(x) or math.isnan(y):
            return math.isnan(x) and math.isnan(y)
        if math.isinf(x) or math.isinf(y):
            return x == y
        return math.isclose(x, y, rel_tol=1e-9, abs_tol=1e-9)
    return a == b

def equal(a, b):
    if isinstance(a, bool) or isinstance(b, bool):
        return type(a) is type(b) and a == b
    if a is None or b is None:
        return a is None and b is None
    if isinstance(a, (int, float)) and isinstance(b, (int, float)):
        return number_equal(a, b)
    if isinstance(a, str) and isinstance(b, str):
        return a == b
    if isinstance(a, list) and isinstance(b, list):
        return len(a) == len(b) and all(equal(x, y) for x, y in zip(a, b))
    if isinstance(a, dict) and isinstance(b, dict):
        return a.keys() == b.keys() and all(equal(a[k], b[k]) for k in a)
    return False

try:
    actual = json.loads(sys.argv[1])
    expected = json.loads(sys.argv[2])
except Exception:
    raise SystemExit(1)
raise SystemExit(0 if equal(actual, expected) else 1)
' "$actual" "$expected"
        return $?
    fi

    if [ -n "$JSON_JQ" ]; then
        local result
        result=$("$JSON_JQ" -n --argjson a "$actual" --argjson b "$expected" '$a == $b' 2>/dev/null) || return 1
        [ "$result" = "true" ]
        return $?
    fi

    # Last-resort fallback: the grep/sed json_probe cannot reconstruct nested
    # values, so only compact scalar equality can be checked here.
    local actual_clean expected_clean
    actual_clean=$(printf '%s' "$actual" | tr -d '[:space:]')
    expected_clean=$(printf '%s' "$expected" | tr -d '[:space:]')
    [ "$actual_clean" = "$expected_clean" ]
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
    # curl's %{http_code} is 3 digits, but the `|| echo "000"` fallback can
    # append to it (e.g. 405000). Keep only the leading status code.
    REQUEST_STATUS="${REQUEST_STATUS:0:3}"

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
    local expected_value="${5:-}"

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
        local value_type
        value_type=$(json_type "$expected_field")
        if [ "$value_type" = "__MISSING__" ]; then
            ok=0
            detail="${detail}missing expected field: $expected_field; "
        elif [ -n "$expected_value" ]; then
            local actual_value actual_json
            actual_value=$(json_get "$expected_field")
            if [ "$value_type" = "str" ]; then
                # json_get strips JSON quotes from strings; re-quote before
                # comparing so `"hello"` matches the unquoted value `hello`.
                # Numeric/bool/array/object values stay byte-for-byte exact.
                actual_json=$(json_encode_string "$actual_value")
            else
                actual_json="$actual_value"
            fi
            if ! values_equal "$actual_json" "$expected_value"; then
                ok=0
                detail="${detail}$expected_field value was $actual_value, expected $expected_value; "
            fi
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
test_method "A.add" "[5,3]" "A.add(5, 3)" "result" "8"

# A.calc - Array + enum parameter
# CRITICAL: This exact test case reproduces the C# server bug
# Original failing request: {"jsonrpc":"2.0","method":"A.calc","params":[[33,22],"add"],"id":1766337019052}
# Error: "The JSON value could not be converted to MathOp"
test_method "A.calc" "[[33,22],\"add\"]" "A.calc([33,22], 'add') - CRITICAL BUG TEST" "result" "55.0"
test_method "A.calc" "[[10,5],\"multiply\"]" "A.calc([10,5], 'multiply')" "result" "50.0"
test_method "A.calc" "[[2.5,4.0,1.5],\"add\"]" "A.calc([2.5,4.0,1.5], 'add')" "result" "8.0"

# A.sqrt - Float calculation
test_method "A.sqrt" "[16.0]" "A.sqrt(16.0)" "result" "4.0"
test_method "A.sqrt" "[2.25]" "A.sqrt(2.25)" "result" "1.5"

# A.repeat - Struct parameter (RepeatResponse inherits status from inc.Response)
test_method "A.repeat" "[{\"to_repeat\":\"hello\",\"count\":3,\"force_uppercase\":false}]" "A.repeat with lowercase" "result" "{\"status\":\"ok\",\"count\":3,\"items\":[\"hello\",\"hello\",\"hello\"]}"
test_method "A.repeat" "[{\"to_repeat\":\"world\",\"count\":2,\"force_uppercase\":true}]" "A.repeat with uppercase" "result" "{\"status\":\"ok\",\"count\":2,\"items\":[\"WORLD\",\"WORLD\"]}"

# A.say_hi - No parameters (HiResponse has no inherited status field)
test_method "A.say_hi" "null" "A.say_hi()" "result" "{\"hi\":\"hi\"}"

# A.repeat_num - Array return
test_method "A.repeat_num" "[42,5]" "A.repeat_num(42, 5)" "result" "[42,42,42,42,42]"

# A.putPerson - Optional field handling (email is optional, test with null)
test_method "A.putPerson" "[{\"personId\":\"123\",\"firstName\":\"John\",\"lastName\":\"Doe\",\"email\":null}]" "A.putPerson with null email" "result" "\"123\""
test_method "A.putPerson" "[{\"personId\":\"456\",\"firstName\":\"Jane\",\"lastName\":\"Smith\"}]" "A.putPerson without email field" "result" "\"456\""

# A.echoOptional - optional built-ins (score/active/ratio) must be nullable in
# statically typed structs and round-trip over the wire (D7/C10).
# D7 allows either absence or an explicit null for optional fields, so assert
# the required fields exactly and treat each optional field individually.
do_request POST '{"jsonrpc":"2.0","method":"A.echoOptional","params":[{"personId":"1","firstName":"A","lastName":"B","score":null,"active":null,"ratio":null}],"id":1}'
probe_ok=1
probe_detail=""
if [ "$REQUEST_STATUS" != "200" ]; then
    probe_ok=0; probe_detail="${probe_detail}expected HTTP 200, got $REQUEST_STATUS; "
elif [ "$(json_type "")" != "dict" ]; then
    probe_ok=0; probe_detail="${probe_detail}response is not a JSON object; "
else
    [ "$(json_get result.personId)" = "1" ] || { probe_ok=0; probe_detail="${probe_detail}personId not echoed; "; }
    [ "$(json_get result.firstName)" = "A" ] || { probe_ok=0; probe_detail="${probe_detail}firstName not echoed; "; }
    [ "$(json_get result.lastName)" = "B" ] || { probe_ok=0; probe_detail="${probe_detail}lastName not echoed; "; }
    for optional_field in score active ratio email; do
        optional_field_type=$(json_type "result.$optional_field")
        if [ "$optional_field_type" != "NoneType" ] && [ "$optional_field_type" != "__MISSING__" ]; then
            probe_ok=0; probe_detail="${probe_detail}$optional_field was $optional_field_type, expected absent-or-null; "
        fi
    done
fi
record_result "A.echoOptional with null built-ins" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

do_request POST '{"jsonrpc":"2.0","method":"A.echoOptional","params":[{"personId":"2","firstName":"C","lastName":"D","score":7,"active":true,"ratio":2.5}],"id":1}'
probe_ok=1
probe_detail=""
if [ "$REQUEST_STATUS" != "200" ]; then
    probe_ok=0; probe_detail="${probe_detail}expected HTTP 200, got $REQUEST_STATUS; "
elif [ "$(json_type "")" != "dict" ]; then
    probe_ok=0; probe_detail="${probe_detail}response is not a JSON object; "
else
    [ "$(json_get result.personId)" = "2" ] || { probe_ok=0; probe_detail="${probe_detail}personId not echoed; "; }
    [ "$(json_get result.score)" = "7" ] || { probe_ok=0; probe_detail="${probe_detail}score was $(json_get result.score), expected 7; "; }
    [ "$(json_get result.active)" = "true" ] || { probe_ok=0; probe_detail="${probe_detail}active was $(json_get result.active), expected true; "; }
    [ "$(json_get result.ratio)" = "2.5" ] || { probe_ok=0; probe_detail="${probe_detail}ratio was $(json_get result.ratio), expected 2.5; "; }
fi
record_result "A.echoOptional with populated built-ins" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

# All three optional built-ins omitted: each echoed field must be absent OR
# explicitly null (runtimes differ on whether they emit null optional fields).
do_request POST '{"jsonrpc":"2.0","method":"A.echoOptional","params":[{"personId":"3","firstName":"E","lastName":"F"}],"id":1}'
probe_ok=1
probe_detail=""
if [ "$REQUEST_STATUS" != "200" ]; then
    probe_ok=0; probe_detail="${probe_detail}expected HTTP 200, got $REQUEST_STATUS; "
fi
if [ "$(json_type "")" != "dict" ]; then
    probe_ok=0; probe_detail="${probe_detail}response is not a JSON object; "
else
    if [ "$(json_type error)" != "__MISSING__" ]; then
        probe_ok=0; probe_detail="${probe_detail}unexpected error object; "
    fi
    for optional_field in score active ratio; do
        optional_field_type=$(json_type "result.$optional_field")
        if [ "$optional_field_type" != "NoneType" ] && [ "$optional_field_type" != "__MISSING__" ]; then
            probe_ok=0; probe_detail="${probe_detail}$optional_field was $optional_field_type, expected absent-or-null; "
        fi
    done
fi
record_result "A.echoOptional with omitted built-ins" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

echo ""

# ============================================================================
# Interface B Tests
# ============================================================================

echo -e "${BLUE}=== Interface B Tests ===${NC}"

# B.echo - String parameter, optional return
test_method "B.echo" "[\"hello\"]" "B.echo('hello')" "result" "\"hello\""
# M23: the optional return must be an explicit JSON null, not a missing result.
test_method "B.echo" "[\"return-null\"]" "B.echo('return-null') - should return null" "result" "null"

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
# invalid_fields listing the offending parameter path. All seven runtimes emit
# this canonical structured shape.
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

# R3: invalid_fields entries are `param[<index>]` selectors. Accumulation is
# intentionally divergent per R3: Python3/TS collect every offending param,
# while Go/Java/Python2/C# fail fast and report only the first one. Only one
# entry is therefore required here, but every present entry must be well formed.
do_request POST '{"jsonrpc":"2.0","method":"A.add","params":["five","three"],"id":98}'
probe_ok=1
probe_detail=""
if [ "$REQUEST_STATUS" != "200" ]; then
    probe_ok=0; probe_detail="${probe_detail}expected HTTP 200, got $REQUEST_STATUS; "
fi
if [ "$(json_type "")" != "dict" ]; then
    probe_ok=0; probe_detail="${probe_detail}response is not an object; "
else
    [ "$(json_get error.code)" = "-32602" ] || { probe_ok=0; probe_detail="${probe_detail}error.code != -32602; "; }
    if [ "$(json_type error.data.invalid_fields)" = "__MISSING__" ]; then
        probe_ok=0; probe_detail="${probe_detail}error.data.invalid_fields missing; "
    elif [ "$(json_type error.data.invalid_fields)" != "list" ]; then
        probe_ok=0; probe_detail="${probe_detail}error.data.invalid_fields is not a list; "
    elif [ "$(json_type error.data.invalid_fields.0)" = "__MISSING__" ]; then
        probe_ok=0; probe_detail="${probe_detail}invalid_fields has no entries; "
    else
        entry_index=0
        while [ "$(json_type "error.data.invalid_fields.$entry_index")" != "__MISSING__" ]; do
            field_entry=$(json_get "error.data.invalid_fields.$entry_index")
            if ! printf '%s' "$field_entry" | grep -Eq '^param\[[0-9]+\]$'; then
                probe_ok=0; probe_detail="${probe_detail}invalid_fields[$entry_index] is not a param index selector: $field_entry; "
            fi
            entry_index=$((entry_index + 1))
        done
    fi
fi
record_result "Invalid params list well-formed param[index] selectors" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

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
# Numeric Boundaries, Declared Errors, and Invalid Requests (D1, R7, M2)
# ============================================================================

echo -e "${BLUE}=== Numeric Boundaries, Declared Errors, and Invalid Requests ===${NC}"

# D1: int32 boundaries for int parameters (inclusive at both ends).
test_method "A.add" "[2147483647,0]" "A.add accepts int32 maximum (2147483647)" "result" "2147483647"
test_validation_error "A.add" "[2147483648,0]" "A.add rejects int32 overflow (2147483648)" "-32602"
test_method "A.add" "[-2147483648,0]" "A.add accepts int32 minimum (-2147483648)" "result" "-2147483648"
test_validation_error "A.add" "[-2147483649,0]" "A.add rejects int32 underflow (-2147483649)" "-32602"

# D1: int parameters reject fractional numbers and booleans.
test_validation_error "A.add" "[5.5,3]" "A.add rejects fractional int parameter" "-32602"
test_validation_error "A.add" "[true,3]" "A.add rejects boolean for int parameter" "-32602"

# D1: float parameters accept integer literals (5 vs 5.0 rendering) but not
# booleans. NaN/Infinity literals are only valid if the runtime JSON parser
# produces them, so that case is informational rather than a hard failure.
test_method "A.sqrt" "[4]" "A.sqrt accepts integer literal for float parameter" "result" "2.0"
test_validation_error "A.sqrt" "[true]" "A.sqrt rejects boolean for float parameter" "-32602"
for float_literal in NaN Infinity -Infinity; do
    do_request POST "{\"jsonrpc\":\"2.0\",\"method\":\"A.sqrt\",\"params\":[$float_literal],\"id\":1}"
    if [ "$(json_type error)" = "__MISSING__" ] && [ "$(json_type result)" = "number" ]; then
        record_result "A.sqrt($float_literal) accepted when the JSON parser produces it" 1 "Response: $REQUEST_BODY"
    else
        echo -e "${YELLOW}NOTE${NC}: A.sqrt($float_literal) is parser-dependent (no hard assertion); Response: $REQUEST_BODY"
    fi
done

# R7: declared errors use their IDL code; the success path still returns 5.0.
test_validation_error "A.divide" "[5,0]" "A.divide(5, 0) returns declared error 1001" "1001"
test_method "A.divide" "[10,2]" "A.divide(10, 2)" "result" "5.0"

# M2: a request without a usable method is an Invalid Request with a null id.
while IFS='|' read -r method_label method_payload; do
    [ -n "$method_label" ] || continue
    do_request POST "$method_payload"
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
    record_result "$method_label returns -32600 with null id" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"
done <<'M2_METHOD_CASES'
Missing method|{"jsonrpc":"2.0","id":31}
Empty-string method|{"jsonrpc":"2.0","method":"","id":32}
Numeric method|{"jsonrpc":"2.0","method":123,"id":33}
M2_METHOD_CASES

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

# D5: a batch at the documented upper size limit (100) must still be served.
batch_100="["
for ((batch_i = 0; batch_i < 100; batch_i++)); do
    [ "$batch_i" -eq 0 ] || batch_100+=","
    batch_100+="{\"jsonrpc\":\"2.0\",\"method\":\"A.add\",\"params\":[1,2],\"id\":$batch_i}"
done
batch_100+="]"
do_request POST "$batch_100"
probe_ok=1
probe_detail=""
[ "$(json_type "")" = "list" ] || { probe_ok=0; probe_detail="${probe_detail}response is not an array; "; }
[ "$(json_type 99)" != "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}batch returned fewer than 100 responses; "; }
[ "$(json_get 0.id)" = "0" ] || { probe_ok=0; probe_detail="${probe_detail}first response id != 0; "; }
[ "$(json_get 99.id)" = "99" ] || { probe_ok=0; probe_detail="${probe_detail}last response id != 99; "; }
[ "$(json_get 0.result)" = "3" ] || { probe_ok=0; probe_detail="${probe_detail}first response result != 3; "; }
[ "$(json_get 99.result)" = "3" ] || { probe_ok=0; probe_detail="${probe_detail}last response result != 3; "; }
record_result "Batch of exactly 100 requests is accepted" "$probe_ok" "${probe_detail}Response: $(printf '%s' "$REQUEST_BODY" | cut -c1-300)"

# A batch of two valid requests returns one response per id, with real values.
do_request POST '[{"jsonrpc":"2.0","method":"A.add","params":[5,3],"id":21},{"jsonrpc":"2.0","method":"A.add","params":[10,5],"id":22}]'
probe_ok=1
probe_detail=""
[ "$(json_type "")" = "list" ] || { probe_ok=0; probe_detail="${probe_detail}response is not an array; "; }
[ "$(json_type 2)" = "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}batch produced a third response; "; }
[ "$(json_get 0.id)" = "21" ] || { probe_ok=0; probe_detail="${probe_detail}first id != 21; "; }
[ "$(json_get 0.result)" = "8" ] || { probe_ok=0; probe_detail="${probe_detail}first result != 8; "; }
[ "$(json_get 1.id)" = "22" ] || { probe_ok=0; probe_detail="${probe_detail}second id != 22; "; }
[ "$(json_get 1.result)" = "15" ] || { probe_ok=0; probe_detail="${probe_detail}second result != 15; "; }
record_result "Batch of two requests returns two matched responses" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

# Duplicate ids are allowed; each request still gets its own response.
do_request POST '[{"jsonrpc":"2.0","method":"A.add","params":[1,2],"id":7},{"jsonrpc":"2.0","method":"A.add","params":[3,4],"id":7}]'
probe_ok=1
probe_detail=""
[ "$(json_type "")" = "list" ] || { probe_ok=0; probe_detail="${probe_detail}response is not an array; "; }
[ "$(json_type 2)" = "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}duplicate-id batch produced a third response; "; }
[ "$(json_get 0.id)" = "7" ] || { probe_ok=0; probe_detail="${probe_detail}first id != 7; "; }
[ "$(json_get 1.id)" = "7" ] || { probe_ok=0; probe_detail="${probe_detail}second id != 7; "; }
case "$(json_get 0.result),$(json_get 1.result)" in
    3,7|7,3) ;;
    *) probe_ok=0; probe_detail="${probe_detail}duplicate-id results were not 3 and 7; " ;;
esac
record_result "Batch with duplicate ids returns one response per request" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

# A batch made only of invalid entries stays an array of per-entry errors.
do_request POST '[{"jsonrpc":"2.0","method":"No.such","id":41},{"jsonrpc":"2.0","method":"A.add","params":[1],"id":42}]'
probe_ok=1
probe_detail=""
[ "$(json_type "")" = "list" ] || { probe_ok=0; probe_detail="${probe_detail}response is not an array; "; }
[ "$(json_type 0.error)" != "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}first entry has no error; "; }
[ "$(json_type 1.error)" != "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}second entry has no error; "; }
[ "$(json_get 0.id)" = "41" ] || { probe_ok=0; probe_detail="${probe_detail}first id != 41; "; }
[ "$(json_get 1.id)" = "42" ] || { probe_ok=0; probe_detail="${probe_detail}second id != 42; "; }
case "$(json_get 0.error.code),$(json_get 1.error.code)" in
    -32601,-32602|-32602,-32601) ;;
    *) probe_ok=0; probe_detail="${probe_detail}expected error codes -32601 and -32602; " ;;
esac
record_result "Batch of only invalid entries returns per-entry errors" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

# A syntactically malformed entry makes the whole batch a parse error.
do_request POST '[{"jsonrpc":"2.0","method":"A.add","params":[1,2],"id":51},{"jsonrpc":"2.0",]'
probe_ok=1
probe_detail=""
if [ "$(json_type "")" != "dict" ]; then
    probe_ok=0; probe_detail="${probe_detail}malformed batch must return a single object, got $(json_type ""); "
else
    [ "$(json_get error.code)" = "-32700" ] || { probe_ok=0; probe_detail="${probe_detail}error.code != -32700; "; }
    [ "$(json_get id)" = "null" ] || { probe_ok=0; probe_detail="${probe_detail}id is not null; "; }
fi
record_result "Malformed JSON entry inside a batch returns -32700" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

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

# D3/R22/S8: non-POST requests -> HTTP 405 with a JSON-RPC -32600 body and the
# exact documented reason text.
for http_method in GET PUT DELETE PATCH HEAD OPTIONS; do
    do_request "$http_method" ""
    probe_ok=1
    probe_detail=""
    [ "$REQUEST_STATUS" = "405" ] || { probe_ok=0; probe_detail="${probe_detail}expected HTTP 405, got $REQUEST_STATUS; "; }
    if [ "$http_method" = "HEAD" ]; then
        # A HEAD response must not carry a body, so only the status can be
        # asserted; Content-Type/JSON-RPC body checks would be meaningless.
        :
    else
        printf '%s' "$REQUEST_CONTENT_TYPE" | grep -qi 'application/json' || { probe_ok=0; probe_detail="${probe_detail}Content-Type is not application/json; "; }
        if [ "$(json_type "")" != "dict" ]; then
            probe_ok=0; probe_detail="${probe_detail}body is not a JSON-RPC error object; "
        else
            [ "$(json_get jsonrpc)" = "2.0" ] || { probe_ok=0; probe_detail="${probe_detail}jsonrpc != 2.0; "; }
            [ "$(json_get error.code)" = "-32600" ] || { probe_ok=0; probe_detail="${probe_detail}error.code != -32600; "; }
            [ "$(json_get error.message)" = "Invalid Request - only POST allowed" ] || { probe_ok=0; probe_detail="${probe_detail}error.message is not the exact D3 text; "; }
            [ "$(json_get id)" = "null" ] || { probe_ok=0; probe_detail="${probe_detail}id is not null; "; }
        fi
    fi
    record_result "$http_method is rejected with 405 and -32600" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"
done

# D4/R15/S6: internal-error redaction is asserted as a hard gate by
# `test_internal_errors.sh`, which drives the same negative-count A.repeat
# handler against a deterministic throwing implementation in every runtime.

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

