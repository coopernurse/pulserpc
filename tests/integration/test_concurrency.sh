#!/bin/bash
# Parallel-request concurrency probe for PulseRPC conformance servers (R20/R21).
#
# Fires N=20 simultaneous A.add POSTs with distinct integer ids and asserts that
# every response is a well-formed JSON-RPC 2.0 success whose echoed id matches
# the request and whose result matches the sum. A separate distinctness check
# catches cross-talk / duplicate-id bugs (e.g. a shared, non-atomic id counter).
#
# This is raw HTTP only: no runtime-specific client code is involved, so the
# same probe runs against all seven conformance servers.
#
# Usage:
#   SERVER_URL=http://localhost:9000 bash tests/integration/test_concurrency.sh
#   bash tests/integration/test_concurrency.sh http://localhost:9000

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SERVER_URL="${SERVER_URL:-${1:-}}"
if [ -z "$SERVER_URL" ]; then
    echo -e "${RED}ERROR: SERVER_URL not provided${NC}"
    echo "Usage: $0 [SERVER_URL]"
    echo "   or: SERVER_URL=http://localhost:9000 $0"
    exit 1
fi

N=20
FIRST_ID=1000
LAST_ID=$((FIRST_ID + N - 1))

echo -e "${GREEN}=== HTTP Concurrency Tests (R20/R21) ===${NC}"
echo -e "Testing server at: ${YELLOW}$SERVER_URL${NC}"
echo -e "Firing ${YELLOW}$N${NC} simultaneous A.add requests (ids $FIRST_ID..$LAST_ID)"
echo ""

# ---------------------------------------------------------------------------
# JSON parsing helpers. A candidate only counts when it can actually run: on
# Windows `python3` is often the Microsoft Store stub, which is on PATH but
# exits non-zero with an install hint. jq is the fallback (mirrors
# test_http_api.sh).
# ---------------------------------------------------------------------------
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
if [ -z "$JSON_PY" ] && [ -z "$JSON_JQ" ]; then
    echo -e "${RED}ERROR: no usable JSON parser (need python3/python or jq)${NC}"
    exit 1
fi

# json_probe <file> <dot-path> prints "<json-type>\t<compact-value>".
# An empty path probes the root. A missing key yields type __MISSING__ and
# unparseable JSON yields __INVALID__.
json_probe() {
    local file="$1"
    local path="$2"

    if [ -n "$JSON_PY" ]; then
        "$JSON_PY" -c '
import json, sys

try:
    with open(sys.argv[1], "r", encoding="utf-8") as fh:
        data = json.load(fh)
except Exception:
    print("__INVALID__\t")
    raise SystemExit

cur = data
path = sys.argv[2]
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
    kind, value = "number", json.dumps(cur, separators=(",", ":"))
elif isinstance(cur, list):
    kind, value = "list", json.dumps(cur, separators=(",", ":"))
elif isinstance(cur, dict):
    kind, value = "dict", json.dumps(cur, separators=(",", ":"))
else:
    kind, value = "unknown", str(cur)
print(kind + "\t" + value)
' "$file" "$path" || printf '__INVALID__\t\n'
        return 0
    fi

    local raw kind
    if [ -z "$path" ]; then
        raw=$("$JSON_JQ" -c '.' "$file" 2>/dev/null) || raw=""
        kind=$("$JSON_JQ" -r 'type' "$file" 2>/dev/null) || kind=""
    else
        raw=$("$JSON_JQ" -c ".$path" "$file" 2>/dev/null) || raw=""
        kind=$("$JSON_JQ" -r ".$path | type" "$file" 2>/dev/null) || kind=""
    fi
    case "$kind" in
        object)  kind="dict" ;;
        array)   kind="list" ;;
        string)  kind="str" ;;
        number)  kind="number" ;;
        boolean) kind="bool" ;;
        null)    kind="NoneType" ;;
        "")      kind="__MISSING__" ;;
    esac
    if [ "$kind" = "str" ]; then
        raw=$(printf '%s' "$raw" | sed 's/^"//; s/"$//')
    fi
    printf '%s\t%s\n' "$kind" "$raw"
}

# json_has_key <file> <key> prints "yes" when the root object has <key>.
# Used for the "no error" assertion; relying on a type probe alone would treat
# a missing "error" as jq's null.
json_has_key() {
    local file="$1"
    local key="$2"

    if [ -n "$JSON_PY" ]; then
        "$JSON_PY" -c '
import json, sys
try:
    with open(sys.argv[1], "r", encoding="utf-8") as fh:
        data = json.load(fh)
    print("yes" if isinstance(data, dict) and sys.argv[2] in data else "no")
except Exception:
    print("no")
' "$file" "$key" || printf 'no\n'
        return 0
    fi

    if "$JSON_JQ" -e "has(\"$key\")" "$file" >/dev/null 2>&1; then
        printf 'yes\n'
    else
        printf 'no\n'
    fi
}

# probe_field <file> <path> sets PROBE_KIND and PROBE_VALUE.
probe_field() {
    local line
    line=$(json_probe "$1" "$2")
    PROBE_KIND=${line%%$'\t'*}
    PROBE_VALUE=${line#*$'\t'}
}

TESTS_PASSED=0
TESTS_FAILED=0
FAILED_TESTS=()

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
# Fire all N requests concurrently. Each request owns its body/status files so
# parallel curl jobs cannot overwrite one another.
# ---------------------------------------------------------------------------
WORKDIR=$(mktemp -d "${TMPDIR:-/tmp}/pulserpc-concurrency.XXXXXX")
cleanup() {
    rm -rf "$WORKDIR"
}
trap cleanup EXIT INT TERM

PIDS=()
for id in $(seq "$FIRST_ID" "$LAST_ID"); do
    request="{\"jsonrpc\":\"2.0\",\"method\":\"A.add\",\"params\":[$id,1],\"id\":$id}"
    printf '%s' "$request" > "$WORKDIR/request_$id"
    curl -sS --connect-timeout 5 --max-time 30 \
        -o "$WORKDIR/body_$id" -w '%{http_code}' \
        -X POST "$SERVER_URL" -H 'Content-Type: application/json' \
        --data-binary "$request" \
        > "$WORKDIR/status_$id" 2> "$WORKDIR/curlerr_$id" &
    PIDS+=("$!")
done
for pid in "${PIDS[@]}"; do
    wait "$pid" || true
done

# ---------------------------------------------------------------------------
# Validate every response.
# ---------------------------------------------------------------------------
ECHOED_IDS=()
for id in $(seq "$FIRST_ID" "$LAST_ID"); do
    status=$(cat "$WORKDIR/status_$id" 2>/dev/null || printf '000')
    body=$(cat "$WORKDIR/body_$id" 2>/dev/null || printf '')
    curl_err=$(cat "$WORKDIR/curlerr_$id" 2>/dev/null || printf '')

    ok=1
    detail=""
    if [ "$status" != "200" ]; then
        ok=0
        detail="${detail}expected HTTP 200, got '$status'; "
    fi

    probe_field "$WORKDIR/body_$id" ""
    if [ "$PROBE_KIND" != "dict" ]; then
        ok=0
        detail="${detail}response is not a JSON object (got $PROBE_KIND); "
    else
        probe_field "$WORKDIR/body_$id" "jsonrpc"
        if [ "$PROBE_KIND" != "str" ] || [ "$PROBE_VALUE" != "2.0" ]; then
            ok=0
            detail="${detail}jsonrpc != '2.0'; "
        fi

        probe_field "$WORKDIR/body_$id" "id"
        echoed="$PROBE_VALUE"
        ECHOED_IDS+=("$echoed")
        if [ "$PROBE_KIND" != "number" ] || [ "$echoed" != "$id" ]; then
            ok=0
            detail="${detail}id was not echoed as $id (got '$echoed'); "
        fi

        if [ "$(json_has_key "$WORKDIR/body_$id" "error")" != "no" ]; then
            ok=0
            detail="${detail}unexpected error object; "
        fi

        probe_field "$WORKDIR/body_$id" "result"
        expected_result=$((id + 1))
        if [ "$PROBE_KIND" != "number" ] || [ "$PROBE_VALUE" != "$expected_result" ]; then
            ok=0
            detail="${detail}result != $expected_result (got '$PROBE_VALUE'); "
        fi
    fi

    if [ -n "$curl_err" ]; then
        detail="${detail}curl: $curl_err; "
    fi
    record_result "A.add id=$id: 200, jsonrpc 2.0, id echoed, no error, result=$((id + 1))" \
        "$ok" "${detail}Response: $body"
done

# Cross-talk check: two distinct requests must never produce the same id.
duplicate=""
declare -A seen_ids=()
for echoed in "${ECHOED_IDS[@]}"; do
    if [ -n "${seen_ids[$echoed]+x}" ]; then
        duplicate="$echoed"
        break
    fi
    seen_ids[$echoed]=1
done
if [ -z "$duplicate" ]; then
    record_result "All $N echoed ids are distinct (no cross-talk)" "1"
else
    record_result "All $N echoed ids are distinct (no cross-talk)" "0" \
        "duplicate echoed id: '$duplicate'"
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
echo ""
echo -e "${BLUE}=== Concurrency Test Summary ===${NC}"
echo -e "Requests: $N"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
if [ "$TESTS_FAILED" -gt 0 ]; then
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
    echo ""
    echo -e "${RED}Failed checks:${NC}"
    for test in "${FAILED_TESTS[@]}"; do
        echo -e "  ${RED}✗${NC} $test"
    done
    echo ""
    echo -e "${RED}=== Concurrency tests FAILED ===${NC}"
    exit 1
fi
echo -e "${GREEN}Failed: 0${NC}"
echo ""
echo -e "${GREEN}=== All concurrency tests passed! ===${NC}"
exit 0
