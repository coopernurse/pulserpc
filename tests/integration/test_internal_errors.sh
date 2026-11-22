#!/bin/bash
# D4/R15/S6: deterministic, hard-gating internal-error redaction probe.
# Sends A.repeat with count=-1; the generated/hand-written test server raises a
# distinctive sentinel error, which the generic runtime error path must turn
# into a redacted -32603 ("Internal error", no error.data).
# Can be run against any runtime implementation.

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

echo -e "${GREEN}=== Internal Error Redaction Probe ===${NC}"
echo -e "Testing server at: ${YELLOW}$SERVER_URL${NC}"
echo ""

TESTS_PASSED=0
TESTS_FAILED=0
FAILED_TESTS=()

# ---------------------------------------------------------------------------
# Shared HTTP / JSON helpers (copied from test_http_api.sh)
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

# do_request <METHOD> <body> sets REQUEST_STATUS/REQUEST_BODY/REQUEST_CONTENT_TYPE.
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
    return 0
}

# ---------------------------------------------------------------------------
# The probe
# ---------------------------------------------------------------------------
do_request POST '{"jsonrpc":"2.0","method":"A.repeat","params":[{"to_repeat":"x","count":-1,"force_uppercase":false}],"id":13}'

probe_ok=1
probe_detail=""

[ "$REQUEST_STATUS" = "200" ] || { probe_ok=0; probe_detail="${probe_detail}expected HTTP 200, got $REQUEST_STATUS; "; }

if [ "$(json_type "")" != "dict" ]; then
    probe_ok=0
    probe_detail="${probe_detail}response body is not a JSON object; "
else
    [ "$(json_get jsonrpc)" = "2.0" ] || { probe_ok=0; probe_detail="${probe_detail}jsonrpc != 2.0; "; }
    [ "$(json_get id)" = "13" ] || { probe_ok=0; probe_detail="${probe_detail}id was not echoed as 13; "; }
    [ "$(json_get error.code)" = "-32603" ] || { probe_ok=0; probe_detail="${probe_detail}error.code != -32603 (got $(json_get error.code)); "; }
    [ "$(json_get error.message)" = "Internal error" ] || { probe_ok=0; probe_detail="${probe_detail}error.message is not exactly 'Internal error'; "; }
    [ "$(json_type error.data)" = "__MISSING__" ] || { probe_ok=0; probe_detail="${probe_detail}error.data must be absent; "; }
fi

if printf '%s' "$REQUEST_BODY" | grep -q 'PULSERPC_SECRET_TOKEN_7F3A'; then
    probe_ok=0
    probe_detail="${probe_detail}sentinel token leaked to client; "
fi

record_result "-32603 is redacted (fixed message, no data, no sentinel)" "$probe_ok" "${probe_detail}Response: $REQUEST_BODY"

echo ""
echo -e "${BLUE}=== Probe Summary ===${NC}"
echo -e "Total probes: $((TESTS_PASSED + TESTS_FAILED))"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
    for test in "${FAILED_TESTS[@]}"; do
        echo -e "  ${RED}✗${NC} $test"
    done
    exit 1
fi
exit 0
