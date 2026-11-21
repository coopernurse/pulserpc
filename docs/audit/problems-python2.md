# Python2 — audit findings

## Summary
- Invariants checked: 23 (R1–R14; M1, M3, M4, M5, M7, M9, M12, M13, M14) plus the canonical guide's runtime structure, error, testing, JSON-RPC and build-integration requirements.
- Confirmed violations: 5
- Test gaps: 2 primary (plus every behavior problem below is untested — see `pkg/runtime/runtimes/python2/pulserpc/test_validation.py:425-450`)
- Deprecation vs parity assessment: Python2 is an **actively maintained, deliberately feature-reduced legacy runtime**, not a deprecated one left alone and not a runtime expected to reach parity. Evidence: `openspec/changes/archive/2026-04-09-python2-runtime/design.md:10` calls out that Python 2 reached EOL in 2020 "but legacy systems persist", and `:17-26` set the original scope to `idl.json` + runtime only. `openspec/changes/archive/2026-04-17-simplify-python-positional-params/design.md:85-86` records the mitigation "Python2 runtime is maintained for legacy systems". It still receives changes (`openspec/changes/archive/2026-05-04-add-ctx-to-python-runtimes/tasks.md:8-12`, in-flight `openspec/changes/python-runtime-types-rename/tasks.md:31-32`). Missing features are an explicit, documented carve-out: `docs/languages/python/reference.md:191` says `Contract.validate()`, `ValidationResult` and `ValidationError` are Python-3-only, and `docs/languages/python/python2.md:27` says no stubs are generated. Therefore "Python3 has a larger API" is **not by itself a violation**; the actionable problems below are either (a) undocumented gaps not covered by that carve-out, (b) genuine divergences from the shared JSON-RPC/validation contract, or (c) absent tests for code that is supposed to work.

## Problems

### python2-01 — Support status (legacy fork vs parity) is only partially documented
- Severity: low
- Invariant: R13 (scope decision)
- Current behavior: Documentation states three Python-3-only APIs (`docs/languages/python/reference.md:191`) and that no stubs are generated (`docs/languages/python/python2.md:27`), but no document states a general support policy. The original scope (`openspec/changes/archive/2026-04-09-python2-runtime/design.md:17-26`) and unresolved questions (`:142-143`, error-handling parity and batch requests) were never resolved in a user-facing doc. The runtime still receives feature work (`openspec/changes/archive/2026-05-04-add-ctx-to-python-runtimes/tasks.md:8-12`), so it is neither frozen nor at parity.
- Expected behavior: The canonical contract treats seven runtimes uniformly (`docs/audit/invariants.md:14-15`) and `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:1191-1221` is a single checklist for "each runtime". A reader cannot tell which checklist items are intentionally waived for Python2.
- Test status: none — no test or doc-check asserts a declared support status.
- Suggested fix: Add a short "Python 2 support policy" section to `docs/languages/python/python2.md` stating "maintained for legacy systems, not feature-parity with Python 3", enumerate the waived capabilities (verification/diff/auditors, structured validation, batch requests, generated stubs), and close the two open questions in the archived design.
- Acceptance criteria:
  - [ ] `docs/languages/python/python2.md` contains an explicit maintained-legacy scope statement listing every waived capability.
  - [ ] The error-handling-parity and batch-request open questions in `openspec/changes/archive/2026-04-09-python2-runtime/design.md:142-143` have a recorded decision.

### python2-02 — Only validation is tested; contract/server/client/transport/RPC have zero tests
- Severity: high
- Invariant: R7, R10, R11, M1, M3, M4, M9
- Current behavior: The only Python2 test file is `pkg/runtime/runtimes/python2/pulserpc/test_validation.py:1-450`; `main()` at `:425-450` calls only validation/type-helper tests (`:430-442`). No test exercises `Contract.validate_request/validate_response` (`pulserpc/contract.py:93-153`), `Server.call` (`pulserpc/server.py:30-153`), `Client`/`InterfaceClientProxy` (`pulserpc/client.py:8-151`), `Transport`/`HttpTransport`/`InProcTransport` (`pulserpc/transport.py:15-69`), or `RPCError` (`pulserpc/rpc.py:4-11`). The generator deliberately strips Python test files for this runtime (`pkg/generator/python_client_server.go:251-254`), and the root target copies only `test_validation.py` (`Makefile:109`). There is no `python2/Makefile` and no `python2/README.md` (directory contains only `pulserpc/`), unlike every other runtime.
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:81-86` requires `tests/` + `Makefile` + `README.md` per runtime; `:843-847` requires tests for validation, error handling, type helpers and edge cases; the original Python2 design promises "Comprehensive tests for both generator and runtime" (`openspec/changes/archive/2026-04-09-python2-runtime/design.md:20`) and specifically "Test HTTP transport with actual server" / "Test both HttpTransport and InProcTransport" (`:134`, `:136`). Catalog R7 flags "Python2 has no RPC test" (`docs/audit/invariants.md:62`).
- Test status: none for the listed modules; `test_validation.py:243-260` covers only struct/optional validation, and `test_validation.py:265-294` has no child-override case for `get_struct_fields` (`pulserpc/rpctypes.py:29-37`).
- Suggested fix: Add `pkg/runtime/runtimes/python2/tests/` with `test_contract.py`, `test_server.py`, `test_client.py`, `test_rpc.py` and a local `Makefile`/`README.md` per the guide; cover `-32600/-32601/-32602/-32603`, notification suppression, request/response validation, `RPCError` propagation and an `InProcTransport` round trip.
- Acceptance criteria:
  - [ ] New tests assert all four standard error codes and notification silence from `Server.call`.
  - [ ] A `Client` round trip over `InProcTransport` asserts `RPCError` is raised on a server error response.
  - [ ] `pkg/runtime/runtimes/python2/` contains `Makefile`, `README.md` and `tests/`.

### python2-03 — Python2 runtime tests are Docker-only and the root target is fragile
- Severity: medium
- Invariant: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:765-788` (Docker testing pattern)
- Current behavior: `Makefile:104-113` (`test-runtime-python2`) hard-depends on `build` (which runs `npm install && npm run build` at `Makefile:15-21`), creates a fixed `/tmp/pulserpc_py2_test` path (`:106`, `:108-112`), chooses an arbitrary IDL via `find examples -name "*.pulse" | head -1` (`:108`), and unconditionally runs `docker run ... moxel/python2` (`:111`) — an unpinned third-party image, not an official `python:2.7-slim` per the guide (`:796-804`). There is no local-interpreter fallback and no `test-docker` target. `make test` (`Makefile:41-43`) does not include it; only `make test-runtimes` (`:141`) → `quality-full` (`:82`) does. The companion quickstart test also hard-fails without Docker (`tests/integration/test_quickstart_python2.sh:40-43`).
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:765-788` requires a per-runtime `test` target that uses Docker only when available and otherwise runs locally, plus a separate `test-docker` target; official images are recommended at `:796-804`.
- Test status: the target is the sole runtime-level test entry point, so without Docker no Python2 runtime test can run at all.
- Suggested fix: Move testing into `pkg/runtime/runtimes/python2/Makefile` with `test`/`test-docker`/`clean`; use a pinned `python:2.7-slim`, `mktemp -d`, and an explicit test IDL; have the root target delegate to it.
- Acceptance criteria:
  - [ ] `make test-runtime-python2` passes with a local Python 2.7 and no Docker installed.
  - [ ] The Docker path uses a version-pinned official image and a unique temp directory.

### python2-04 — `validate_request`/`validate_response` raise plain `ValueError` instead of `RPCError` with code/data
- Severity: medium
- Invariant: R3, R7, M13
- Current behavior: `pkg/runtime/runtimes/python2/pulserpc/contract.py:97-125` raises `ValueError` for unknown interface/function, wrong parameter count and invalid params; `:131-153` does the same for responses. `pulserpc/client.py:88-90` and `:121-123` re-wrap those as plain `ValueError`, so callers get no JSON-RPC code or structured data. Validation helpers raise bare `TypeError`/`ValueError` (`pulserpc/validation.py:14,23,29,35,46,59,69,91,99,113,125,173`). Python3 raises `RPCError(-32602/-32603, ..., ValidationResult-as-data)` (`pkg/runtime/runtimes/python/pulserpc/contract.py:248-297`, `:299-340`) and exports `ValidationError`/`ValidationResult` (`:15`, `python/pulserpc/__init__.py:30-31`). The archived design leaves this as an open question (`openspec/changes/archive/2026-04-09-python2-runtime/design.md:142`), and `docs/languages/python/reference.md:191` only carves out the classes, not the error type.
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:108-117` defines `RPCError` with `code`/`message`/`data` for all runtimes; `:545` requires clients to "Raise `RPCError` on JSON-RPC errors"; catalog R3 flags "Python2 raises plain ValueError with no code/data" (`docs/audit/invariants.md:58`).
- Test status: none — `test_validation.py:8-18` imports only validation/type helpers, and no test imports `contract`, `client` or `rpc`.
- Suggested fix: Make Python2 `validate_request`/`validate_response` raise `RPCError` with `-32602`/`-32603` and structured `data` (a Py2-compatible dict such as `{"invalid_fields": [...], "error": "..."}`); update `Client.call` and `Server.call` catch clauses (`pulserpc/server.py:101,130`) accordingly. If this is intentionally waived, document it explicitly per python2-01.
- Acceptance criteria:
  - [ ] Contract violations surface as `RPCError` with code `-32602` (request) or `-32603` (response).
  - [ ] `RPCError.data` contains structured field/message information rather than only a string.

### python2-05 — No contract-verification API (diff, auditors, checksum, `Contract.validate`)
- Severity: medium
- Invariant: R13
- Current behavior: `pkg/runtime/runtimes/python2/pulserpc/` has no `diff.py` and a grep for `extract_checksum|verify_compatibility|set_local_idl|ValidationResult|VerificationResult|ContractDelta|Severity|ContractAuditor|diff_idl` returns zero matches. `__all__` has 19 symbols (`pulserpc/__init__.py:29-49`) versus 33 in Python3 (`pkg/runtime/runtimes/python/pulserpc/__init__.py:45-79`). Missing relative to Python3: `extract_checksum` and the diff enums/dataclasses (`python/pulserpc/rpctypes.py:9-54`, `:108`), `diff_idl`, four auditors (`python/pulserpc/contract.py:19-81`), `Contract.validate` (`:220-246`), and `Client.verify_compatibility`/`set_local_idl` (`python/pulserpc/client.py:106`, `:292`, `:300`). Only `validate()` / `ValidationResult` / `ValidationError` are documented as Python3-only (`docs/languages/python/reference.md:191`); the verification API is not mentioned there. The canonical guide does **not** require diff/auditors (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:156-188`), so this is a parity gap plus a documentation gap, not a guide violation.
- Expected behavior: Catalog R13 — "Contract verification (diff/auditors/checksum). Python2 has none" (`docs/audit/invariants.md:68`); seven runtimes treated uniformly (`:14-15`).
- Test status: none — there is no python2 diff/verification test at all.
- Suggested fix: Decide explicitly: either port a Py2-compatible subset (plain classes instead of `dataclass`/`Enum`) with tests, or list every missing verification symbol in the Python2 support-policy doc so the API difference is intentional and discoverable. Add a test asserting the exported API matches the documented set.
- Acceptance criteria:
  - [ ] The support doc lists all 14 omitted Python3 symbols, or the symbols are present in Python2 `__all__`.
  - [ ] An API-surface test compares Python2 `__all__` against the documented set and fails on drift.

### python2-06 — `int` validation diverges: booleans and NaN/Infinity are mishandled
- Severity: medium
- Invariant: R2
- Current behavior: `pkg/runtime/runtimes/python2/pulserpc/validation.py:5-8` builds `_int_types` from `(int, long)`, and in Python 2 `bool` subclasses `int`, so `validate_int(True)` passes (`:17-23`); `test_validation.py:75-78` codifies this as "expected behavior in Python 2". Python3 explicitly rejects bool and non-finite floats (`pkg/runtime/runtimes/python/pulserpc/validation.py:26-38`). In Python2, `validate_int(float('nan'))` / `validate_int(float('inf'))` reach `value == int(value)` (`validation.py:21`), where `int(nan)`/`int(inf)` raises an unwrapped `ValueError`/`OverflowError` instead of the intended `TypeError`; no test covers this.
- Expected behavior: Catalog R2 — "Scalar acceptance sets should be identical" (`docs/audit/invariants.md:57`); `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:128-143` requires consistent built-in validation and clear errors.
- Test status: `test_validation.py:49-80` tests ints but asserts the divergent bool acceptance at `:75-78`; no NaN/Infinity case exists.
- Suggested fix: Add an explicit `isinstance(value, bool)` rejection before the int branch and explicit `NaN`/`±inf` checks in `validate_int`/`validate_float`, matching Python3's messages; update `test_validation.py` accordingly.
- Acceptance criteria:
  - [ ] `validate_int(True)` and `validate_int(False)` raise `TypeError`.
  - [ ] `validate_int(float('nan'))` and `validate_int(float('inf'))` raise `TypeError` (not `ValueError`/`OverflowError`).

### python2-07 — Null results skip response validation; client does not validate response envelope
- Severity: medium
- Invariant: M5, M14
- Current behavior: `pkg/runtime/runtimes/python2/pulserpc/server.py:127` guards response validation with `and result is not None`, so a handler returning `None` for a required (non-void) return never reaches `Contract.validate_response` — which would otherwise reject it (`pulserpc/validation.py:121-125`, `contract.py:137-144`). The client has the same guard (`pulserpc/client.py:119`) and at `:109-123` checks only `if "error" in response` (error silently wins if both `result` and `error` are present); it never checks `jsonrpc == "2.0"` or that the response `id` matches the request id (`:92-104`).
- Expected behavior: Catalog M14 — "`result != nil` guards mean null results for required returns skip validation everywhere" (`docs/audit/invariants.md:88`); M5 — "`id` type/range and response-envelope validation ... not enforced anywhere" (`:79`); `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:808-815` requires response format and result-XOR-error.
- Test status: none — no client or server test file exists (see python2-02).
- Suggested fix: Remove the `result is not None` guard when the IDL return is required, and add an envelope check in `Client.call` (jsonrpc version, id match, exactly one of `result`/`error` → `-32600`).
- Acceptance criteria:
  - [ ] A handler returning `None` for a required return produces a `-32603` response.
  - [ ] A response containing both `result` and `error`, or a mismatched id, is rejected with `-32600`.

### python2-08 — Notification handling is untested and `pulserpc-idl` notifications get a response
- Severity: low
- Invariant: R11, M4
- Current behavior: `pkg/runtime/runtimes/python2/pulserpc/server.py:57-59` handles `pulserpc-idl` **before** `is_notification` is computed at `:61-62`, so a method-less-id request for `pulserpc-idl` returns `{"jsonrpc":"2.0","result":...,"id":None}` instead of no response. Likewise, early validation failures at `:38-55` return an error response even when `req.get("id")` is `None` (the id is merely omitted by `_error_response` at `:150-151`). Normal methods do suppress correctly (`:138-139`). `Client.notify` exists at `pulserpc/client.py:127-129` but is untested.
- Expected behavior: Catalog R11/M4 — notifications produce no response (`docs/audit/invariants.md:66`, `:78`); `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:814` — "Notifications: Requests without `id` field (no response sent)".
- Test status: none; no notification test in `test_validation.py:425-450`.
- Suggested fix: Compute `is_notification` immediately after the dict check and return `None` for all notification paths (including `pulserpc-idl` and validation errors); add client/server notification tests.
- Acceptance criteria:
  - [ ] `Server.call({"jsonrpc":"2.0","method":"pulserpc-idl"})` returns `None`.
  - [ ] A notification with a malformed `jsonrpc`/`method` also returns `None`.

## Verified-correct invariants
- R1 — `validate_type` (`pulserpc/validation.py:119`) and `find_struct`/`find_enum`/`get_struct_fields` (`pulserpc/rpctypes.py:4,9,14`) all exist and are exported (`pulserpc/__init__.py:8-23`).
- R4 — Missing required field → error (`validation.py:89-94`), `None` required → error (`:97-102`), missing optional → allowed; covered by `test_validation.py:243-260`.
- R5 — `get_struct_fields` resolves parents first and overrides child name conflicts (`rpctypes.py:20-37`); matches Python3 (`python/pulserpc/rpctypes.py:66-92`).
- R6 — Qualified-name suffix scan inside `validate_type` (`validation.py:151-173`) matches Python3; exact-only helper lookups are shared with Python3 (`python/pulserpc/rpctypes.py:56-68`) — no Python2-specific divergence.
- R7 — `RPCError` carries `code`, `message`, optional `data` (`pulserpc/rpc.py:4-11`), though untested.
- R8 — Literal `-32600..-32603` codes in `server.py:40-135` and `client.py:111-115`, consistent with the other runtimes.
- R10 — Server response validation is implemented and invoked (`server.py:127-136`), subject to M14 (python2-07).
- R12 — `meta` items are parsed into `Contract.meta` (`contract.py:66-69`).
- R14 — Void return is validated as `None` (`contract.py:137-144`), consistent with non-TS runtimes.
- M1 — Incoming `jsonrpc` must equal `"2.0"` → `-32600` (`server.py:43-49`).
- M3 — Malformed/unknown method → `-32601` (`server.py:65-84`).
- M9 — Exact parameter count is always validated, including the zero-parameter case (`contract.py:105-109`) — unlike Go/Java/C#.
- M12 — Extra/unknown fields intentionally allowed (`validation.py:75-117`; asserted by `test_validation.py:259-260`).
- M7 — No batch requests in Python2, but Python3 also lacks them and the design leaves it open (`openspec/changes/archive/2026-04-09-python2-runtime/design.md:143`) — not a parity gap.
