#!/bin/bash
# Collect normalized JSON-RPC responses from a live PulseRPC server into a
# per-runtime golden file for cross-runtime comparison.
#
# Run against a live PulseRPC server:
#   RUNTIME=python SERVER_URL=http://localhost:9000 \
#     bash tests/integration/test_golden_responses.sh
#
# Each curated case is sent to the server; the response is parsed and
# re-serialized with `json.dumps(sort_keys=True, separators=(',', ':'))` and
# stored in a JSON object keyed by the case name. The result is written to
#   ${GOLDEN_DIR:-/tmp/pulserpc-golden}/$RUNTIME.json
#
# This script does NOT compare against expectations -- that is the job of the
# comparator. It hard-FAILs on a crash / hang (curl status 000), a non-200
# response, or an unparseable JSON body. Live behavior is arbitrated by CI
# across all seven runtimes.

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

SERVER_URL="${SERVER_URL:-${1:-http://localhost:8080}}"
RUNTIME="${RUNTIME:-}"
GOLDEN_DIR="${GOLDEN_DIR:-/tmp/pulserpc-golden}"

if [ -z "$SERVER_URL" ]; then
    echo -e "${RED}ERROR: SERVER_URL not provided${NC}"
    echo "Usage: $0 [SERVER_URL]"
    exit 1
fi

if [ -z "$RUNTIME" ]; then
    echo -e "${RED}ERROR: RUNTIME not provided${NC}"
    echo "Set RUNTIME (e.g. RUNTIME=python) so the golden file can be named."
    exit 1
fi

echo -e "${GREEN}=== Golden Response Collection (${RUNTIME}) ===${NC}"
echo -e "Testing server at: ${YELLOW}$SERVER_URL${NC}"
echo ""

# A candidate only counts as a JSON interpreter when it can actually run. On
# Windows `python3` is often the Microsoft Store stub: it is on PATH but exits
# non-zero with an install hint, so probe functionality before using it.
JSON_PY=""
for candidate in python3 python; do
    if command -v "$candidate" >/dev/null 2>&1 && \
        "$candidate" -c 'import json' >/dev/null 2>&1; then
        JSON_PY="$candidate"
        break
    fi
done

if [ -z "$JSON_PY" ]; then
    echo -e "${RED}ERROR: no usable python interpreter found${NC}"
    exit 1
fi

mkdir -p "$GOLDEN_DIR"
OUT_FILE="${GOLDEN_DIR}/${RUNTIME}.json"
ACCUM=$(mktemp "${TMPDIR:-/tmp}/pulserpc-golden.XXXXXX")
trap 'rm -f "$ACCUM"' EXIT

fail() {
    echo -e "${RED}FATAL${NC}: $1"
    exit 1
}

# Normalize a JSON document into a canonical, compact string. Exits non-zero
# (which, under `set -e`, fails the script) when the body is not valid JSON.
normalize_json() {
    printf '%s' "$1" | "$JSON_PY" -c '
import json, sys
try:
    data = json.loads(sys.stdin.read())
except Exception:
    sys.exit(1)
sys.stdout.write(json.dumps(data, sort_keys=True, separators=(",", ":")))
'
}

# collect_case <case-name> <json-rpc-body>
collect_case() {
    local name="$1"
    local body="$2"
    local status body_file payload_file normalized

    body_file=$(mktemp "${TMPDIR:-/tmp}/pulserpc-golden-body.XXXXXX")
    payload_file=$(mktemp "${TMPDIR:-/tmp}/pulserpc-golden-payload.XXXXXX")
    printf '%s' "$body" > "$payload_file"

    status=$(curl -sS --connect-timeout 5 --max-time 30 -o "$body_file" \
        -w '%{http_code}' -X POST "$SERVER_URL" \
        -H "Content-Type: application/json" \
        --data-binary "@$payload_file" 2>/dev/null || echo "000")
    status="${status:0:3}"

    if [ "$status" = "000" ]; then
        rm -f "$body_file" "$payload_file"
        fail "case '${name}': server crash/hang (curl status 000)"
    fi
    if [ "$status" != "200" ]; then
        rm -f "$body_file" "$payload_file"
        fail "case '${name}': expected HTTP 200, got $status"
    fi

    normalized=$(normalize_json "$(cat "$body_file")") || {
        rm -f "$body_file" "$payload_file"
        fail "case '${name}': unparseable JSON response"
    }
    rm -f "$body_file" "$payload_file"

    printf '%s\t%s\n' "$name" "$normalized" >> "$ACCUM"
    echo -e "${GREEN}✓${NC} collected ${YELLOW}${name}${NC}"
}

# --- Curated cases ---------------------------------------------------------
# Sourced from requests already proven to work across all seven runtimes in
# test_http_api.sh. No expectations are asserted here. IDs are fixed so the
# normalized responses are directly comparable across runtimes.

# 1. Successful int call.
collect_case "int_call" '{"jsonrpc":"2.0","method":"A.add","params":[2,3],"id":1}'

# 2. Successful struct call (RepeatResponse with inherited status field).
collect_case "struct_call" '{"jsonrpc":"2.0","method":"A.repeat","params":[{"to_repeat":"hello","count":3,"force_uppercase":false}],"id":2}'

# 3. Validation error carrying structured data (error.data.invalid_fields).
collect_case "validation_invalid_fields" '{"jsonrpc":"2.0","method":"A.add","params":["five",3],"id":99}'

# 4. Declared error with its IDL code (1001).
collect_case "declared_error_1001" '{"jsonrpc":"2.0","method":"A.divide","params":[5,0],"id":4}'

# 5. Unknown method -> -32601.
collect_case "unknown_method" '{"jsonrpc":"2.0","method":"A.nonexistent","params":[5,3],"id":5}'

# 6. Invalid params (too many) -> -32602.
collect_case "invalid_params" '{"jsonrpc":"2.0","method":"A.add","params":[5,3,7],"id":6}'

# 7. Missing jsonrpc field -> -32600.
collect_case "invalid_jsonrpc" '{"method":"A.add","params":[5,3],"id":7}'

# 8. Enum-returning call (enum serialized as its declared wire string).
collect_case "enum_call" '{"jsonrpc":"2.0","method":"A.pick","params":["add"],"id":8}'

# --- Finalize --------------------------------------------------------------
"$JSON_PY" -c '
import json, sys
out = {}
with open(sys.argv[1]) as fh:
    for line in fh:
        line = line.rstrip("\n")
        if not line:
            continue
        name, value = line.split("\t", 1)
        out[name] = value
json.dump(out, sys.stdout, sort_keys=True, separators=(",", ":"))
' "$ACCUM" > "$OUT_FILE"

echo ""
echo -e "${GREEN}Wrote golden responses for ${RUNTIME} to ${YELLOW}${OUT_FILE}${NC}"
