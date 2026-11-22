#!/bin/bash
# Resource-limit probes shared by every generated server (D5/S7).
#
# Run against a live PulseRPC server:
#   SERVER_URL=http://localhost:9000 bash tests/integration/test_limits.sh
#
# Covered here:
#   - request body > 1 MiB  -> -32600 "Request body too large"
#   - batch entries > 100   -> -32600 "Batch exceeds maximum size"
#
# The 10 MiB response cap is enforced inside generated *clients*. It cannot be
# asserted by a raw curl probe without a per-runtime client wrapper, so that
# check lives in each runtime's generated-client integration test.

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SERVER_URL="${SERVER_URL:-${1:-http://localhost:8080}}"

if [ -z "$SERVER_URL" ]; then
    echo -e "${RED}ERROR: SERVER_URL not provided${NC}"
    echo "Usage: $0 [SERVER_URL]"
    exit 1
fi

echo -e "${GREEN}=== Resource Limit Tests ===${NC}"
echo -e "Testing server at: ${YELLOW}$SERVER_URL${NC}"
echo ""

TESTS_PASSED=0
TESTS_FAILED=0
FAILED_TESTS=()

JSON_PY=""
if command -v python3 >/dev/null 2>&1; then
    JSON_PY="python3"
elif command -v python >/dev/null 2>&1; then
    JSON_PY="python"
fi

json_probe() {
    local path="$1"

    if [ -n "$JSON_PY" ]; then
        printf '%s' "$RESPONSE_BODY" | "$JSON_PY" -c '
import json, sys

try:
    data = json.loads(sys.stdin.read())
except Exception:
    print("__INVALID__\t")
    raise SystemExit

cur = data
path = sys.argv[1]
if path:
    for part in path.split("."):
        if isinstance(cur, dict) and part in cur:
            cur = cur[part]
        elif isinstance(cur, list) and part.isdigit() and int(part) < len(cur):
            cur = cur[int(part)]
        else:
            print("__MISSING__\t")
            raise SystemExit

if cur is None:
    kind, value = "NoneType", "null"
elif isinstance(cur, bool):
    kind, value = "bool", ("true" if cur else "false")
elif isinstance(cur, str):
    kind, value = "str", cur
elif isinstance(cur, (int, float)):
    kind, value = "number", json.dumps(cur)
elif isinstance(cur, list):
    kind, value = "list", json.dumps(cur, separators=(",", ":"))
elif isinstance(cur, dict):
    kind, value = "dict", json.dumps(cur, separators=(",", ":"))
else:
    kind, value = "unknown", str(cur)
print(kind + "\t" + value)
' "$path"
        return 0
    fi

    # Minimal fallback: top-level type and direct field extraction.
    local trimmed key raw
    trimmed=$(printf '%s' "$RESPONSE_BODY" | tr -d '[:space:]')
    if [ -z "$path" ]; then
        case "${trimmed:0:1}" in
            '[') printf 'list\t\n' ;;
            '{') printf 'dict\t\n' ;;
            *) printf '__MISSING__\t\n' ;;
        esac
        return 0
    fi
    key="${path##*.}"
    raw=$(printf '%s' "$RESPONSE_BODY" | grep -o "\"$key\"[[:space:]]*:[[:space:]]*[^,}]*" | head -1 | sed 's/^[^:]*:[[:space:]]*//') || raw=""
    if [ -z "$raw" ]; then
        printf '__MISSING__\t\n'
    else
        case "$raw" in
            \"*\") printf 'str\t%s\n' "$(printf '%s' "$raw" | tr -d '"')" ;;
            true|false) printf 'bool\t%s\n' "$raw" ;;
            null) printf 'NoneType\tnull\n' ;;
            *) printf 'number\t%s\n' "$raw" ;;
        esac
    fi
}

json_get() { json_probe "$1" | cut -f2-; }
json_type() { json_probe "$1" | cut -f1; }

RESPONSE_BODY=""
RESPONSE_STATUS=""

do_post() {
    local body="$1"
    local body_file payload_file
    body_file=$(mktemp "${TMPDIR:-/tmp}/pulserpc-limit-response.XXXXXX")
    payload_file=$(mktemp "${TMPDIR:-/tmp}/pulserpc-limit-payload.XXXXXX")
    printf '%s' "$body" > "$payload_file"
    RESPONSE_STATUS=$(curl -sS -o "$body_file" -w '%{http_code}' \
        -X POST "$SERVER_URL" -H "Content-Type: application/json" \
        --data-binary "@$payload_file" 2>/dev/null || echo "000")
    RESPONSE_BODY=$(cat "$body_file")
    rm -f "$body_file" "$payload_file"
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
    return 0
}

echo -e "${YELLOW}Checking if server is ready...${NC}"
if ! curl -s -X POST "$SERVER_URL" -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0","method":"pulserpc-idl","id":1}' > /dev/null 2>&1; then
    echo -e "${RED}ERROR: Server at $SERVER_URL is not responding${NC}"
    exit 1
fi
echo -e "${GREEN}Server is ready${NC}"
echo ""

# ---------------------------------------------------------------------------
# D5: request body > 1 MiB
# ---------------------------------------------------------------------------
echo -e "${BLUE}=== Request Body Limit (1 MiB) ===${NC}"

if command -v python3 >/dev/null 2>&1 || command -v python >/dev/null 2>&1; then
    LARGE_BODY=$("$JSON_PY" -c 'import json; print(json.dumps({"jsonrpc": "2.0", "method": "A.add", "params": [1, 2], "id": 1, "pad": "A" * 1048576}, separators=(",", ":")))')
else
    PAD=$(head -c 1048576 /dev/zero | tr '\0' 'A')
    LARGE_BODY="{\"jsonrpc\":\"2.0\",\"method\":\"A.add\",\"params\":[1,2],\"id\":1,\"pad\":\"$PAD\"}"
fi

do_post "$LARGE_BODY"
probe_ok=1
probe_detail=""
if [ "$RESPONSE_STATUS" = "413" ]; then
    # Some HTTP front ends reject at the transport layer. The documented
    # PulseRPC behavior is a JSON-RPC -32600 body, so only accept 413 when it
    # also carries an error object.
    [ "$(json_get error.code)" = "-32600" ] || {
        probe_ok=0
        probe_detail="${probe_detail}HTTP 413 without a -32600 JSON-RPC body; "
    }
else
    [ "$RESPONSE_STATUS" = "200" ] || {
        probe_ok=0
        probe_detail="${probe_detail}expected HTTP 200, got $RESPONSE_STATUS; "
    }
    [ "$(json_get error.code)" = "-32600" ] || {
        probe_ok=0
        probe_detail="${probe_detail}expected -32600 for an oversize body; "
    }
    [ "$(json_get error.message)" = "Request body too large" ] || {
        probe_ok=0
        probe_detail="${probe_detail}error.message is not 'Request body too large'; "
    }
fi
record_result "Oversize request body is rejected with -32600" "$probe_ok" "${probe_detail}Response: $(printf '%s' "$RESPONSE_BODY" | cut -c1-300)"

# ---------------------------------------------------------------------------
# D5: batch entries > 100
# ---------------------------------------------------------------------------
echo -e "${BLUE}=== Batch Size Limit (100) ===${NC}"

BATCH_BODY="["
for i in $(seq 1 101); do
    if [ "$i" -gt 1 ]; then
        BATCH_BODY="$BATCH_BODY,"
    fi
    BATCH_BODY="$BATCH_BODY{\"jsonrpc\":\"2.0\",\"method\":\"A.add\",\"params\":[1,2],\"id\":$i}"
done
BATCH_BODY="$BATCH_BODY]"

do_post "$BATCH_BODY"
probe_ok=1
probe_detail=""
if [ "$RESPONSE_STATUS" != "200" ]; then
    probe_ok=0
    probe_detail="${probe_detail}expected HTTP 200, got $RESPONSE_STATUS; "
fi
if [ "$(json_type "")" != "dict" ]; then
    probe_ok=0
    probe_detail="${probe_detail}over-limit batch must return a single error object; "
else
    [ "$(json_get error.code)" = "-32600" ] || {
        probe_ok=0
        probe_detail="${probe_detail}expected -32600 for an over-limit batch; "
    }
    [ "$(json_get error.message)" = "Batch exceeds maximum size" ] || {
        probe_ok=0
        probe_detail="${probe_detail}error.message is not 'Batch exceeds maximum size'; "
    }
fi
record_result "Batch with more than 100 entries is rejected" "$probe_ok" "${probe_detail}Response: $(printf '%s' "$RESPONSE_BODY" | cut -c1-300)"

echo ""
echo -e "${BLUE}=== Test Summary ===${NC}"
echo -e "Total tests: $((TESTS_PASSED + TESTS_FAILED))"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
if [ "$TESTS_FAILED" -gt 0 ]; then
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
    for test in "${FAILED_TESTS[@]}"; do
        echo -e "  ${RED}✗${NC} $test"
    done
    exit 1
fi

echo -e "${GREEN}Failed: 0${NC}"
echo -e "${GREEN}=== All limit tests passed! ===${NC}"
exit 0
