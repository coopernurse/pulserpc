# Python — audit findings

## Summary
- Invariants checked: 31 (R1–R14; M1–M17), plus the canonical guide's runtime/JSON-RPC requirements.
- Confirmed violations: 9 (python-01…05, 07, 09, 10, 11)
- Test gaps: 2 primary (python-06, python-08); every confirmed violation is also untested by a test that runs.
- Scope note: `python2` excluded (separate agent). Survey claims that are false for Python 3 were dropped; they are listed at the end with evidence.

## Problems

### python-01 — Qualified `extends` parent fields are silently skipped
- Severity: high
- Invariant: R5, R6
- Current behavior: `get_struct_fields()` resolves the parent with exact-only `find_struct()` (`pkg/runtime/runtimes/python/pulserpc/rpctypes.py:68`, `:75-77`) while `find_struct` is a bare dict lookup (`:56-58`). `Contract` keys structs by plain `name` (`pkg/runtime/runtimes/python/pulserpc/contract.py:157-158`), but the conformance IDL declares `struct RepeatResponse extends inc.Response` (`examples/conform.pulse:26`) and `inc.Response` has a required `status` field (`examples/conform-inc.pulse:13-15`). Validating a `RepeatResponse` therefore never checks the inherited `status`. `validate_type` *does* suffix-match qualified `userDefined` names (`pkg/runtime/runtimes/python/pulserpc/validation.py:169-177`), so qualified resolution is inconsistent between the two paths.
- Expected behavior: R5 parent fields resolved first (`docs/audit/invariants.md:60`); guide requires inheritance field resolution and child override (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:879-883`).
- Test status: `tests/test_types.py:39-56` and `pulserpc/tests/test_validation.py:174-190` use only unqualified `extends: 'Base'`; both files don't even run the latter (see python-06). No qualified-extends test anywhere.
- Suggested fix: Make `get_struct_fields` resolve `extends` with the same suffix-scan used in `validate_type` (or store fully-qualified keys in `Contract.structs` and resolve names consistently). Add a test using the `examples/conform.pulse` shape.
- Acceptance criteria:
  - [ ] `get_struct_fields('RepeatResponse', {'Response': {'fields':[...]}, 'RepeatResponse': {'extends':'inc.Response', ...}})` returns the parent `status` field.
  - [ ] `validate_type` on a `RepeatResponse` missing `status` returns a `ValidationError` with path `.status`.

### python-02 — `validate_float` accepts booleans (R2 divergence)
- Severity: medium
- Invariant: R2
- Current behavior: `validate_float` uses `isinstance(value, (int, float))` (`pkg/runtime/runtimes/python/pulserpc/validation.py:41-44`). In Python `bool` subclasses `int`, so `validate_float(True)` / `validate_float(False)` return `[]`. The sibling `validate_int` explicitly rejects bool (`:26-28`). Separately, `validate_int` accepts integral floats with exact equality (`:31-37`), unlike Go's 1e-9 tolerance.
- Expected behavior: R2 "Scalar acceptance sets should be identical" (`docs/audit/invariants.md:57`); guide requires clear built-in validation (`:128-143`). JSON `true` is not a float.
- Test status: `tests/test_validation.py:62-74` tests only `3.14`, `42`, `-1.5` and string/None failures; the excluded `pulserpc/tests/test_validation.py:48-56` is identical. `tests/test_validation.py:47-50` asserts bool rejection only for `int`.
- Suggested fix: Add `if isinstance(value, bool): return [_make_error(...)]` at the top of `validate_float`; add explicit bool cases to the float tests. If integral-float acceptance is intentional, document the tolerance policy.
- Acceptance criteria:
  - [ ] `validate_float(True)` and `validate_float(False)` each return one `ValidationError` containing "Expected float".
  - [ ] The new cases are in `tests/test_validation.py` so `make test-runtime-python` executes them.

### python-03 — Null results skip required-response validation
- Severity: high
- Invariant: M14, R10
- Current behavior: `Server.call` guards response validation with `if self.validate_responses and result is not None` (`pkg/runtime/runtimes/python/pulserpc/server.py:147`) and `Client.call` with `if self.validate_response and self.contract and result is not None` (`pkg/runtime/runtimes/python/pulserpc/client.py:214`). `Contract.validate_response` would reject `None` for a required return (`pkg/runtime/runtimes/python/pulserpc/contract.py:331-339`), but a handler returning `None` for a non-void method yields `{"result": null}` with no error.
- Expected behavior: M14 "`result != nil` guards mean null results for required returns skip validation everywhere" (`docs/audit/invariants.md:88`); guide requires return values validated against IDL (`:885-889`).
- Test status: `tests/test_rpc.py:41-166` only covers ctx passing; `tests/integration/test_http_api.sh:200` exercises `B.echo('return-null')`, an *optional* return where null is legal — it does not cover a required return.
- Suggested fix: Drop the `result is not None` guard; let `Contract.validate_response` decide via `returnType`/`returnOptional` (it already handles void at `contract.py:320-329`). Do the same in `Client.call`.
- Acceptance criteria:
  - [ ] A handler returning `None` for a method with required return type produces an error response with code `-32603`.
  - [ ] With `validate_response=True`, `Client.call` rejects a `result: null` for a required return with `RPCError`.

### python-04 — Client does not validate the JSON-RPC response envelope
- Severity: medium
- Invariant: M5
- Current behavior: `Client.call` checks only `if 'error' in response` (`pkg/runtime/runtimes/python/pulserpc/client.py:202-208`) then `response.get('result')` (`:211`). It never checks `jsonrpc == '2.0'`, never checks the response `id` against the request id (`:180-192`), and has no result-XOR-error rule — a response with both fields silently returns the error. `HttpTransport.request` similarly returns any parsed JSON unchanged (`pkg/runtime/runtimes/python/pulserpc/transport.py:80-84`).
- Expected behavior: M5 "id type/range and response-envelope validation … not enforced anywhere" (`docs/audit/invariants.md:79`); guide requires response format and result XOR error (`:810-815`).
- Test status: none — no client test exists in `tests/`; shell integration tests only grep for `"error"`/`"result"` presence (`tests/integration/test_http_api.sh:52-73`).
- Suggested fix: In `Client.call`, verify `jsonrpc`, matching `id`, and exactly one of `result`/`error`; raise `RPCError(-32600, ...)` on violation. Add an `InProcTransport` unit test with a deliberately malformed response.
- Acceptance criteria:
  - [ ] A response containing both `result` and `error` is rejected, not returned.
  - [ ] A response with a mismatched id or wrong `jsonrpc` is rejected.

### python-05 — Notifications still receive responses on two paths
- Severity: medium
- Invariant: R11, M4
- Current behavior: `Server.call` handles `pulserpc-idl` at `pkg/runtime/runtimes/python/pulserpc/server.py:76-83`, *before* `is_notification` is computed at `:85-87`; a request with no `id` returns `{'jsonrpc':'2.0','result':...,'id':None}`. Early format errors at `:62-74` likewise return an error body for a notification (the id is merely omitted by `_error_response` at `:187-188`). Only normal methods suppress at `:153-155`.
- Expected behavior: R11/M4 notifications produce no response (`docs/audit/invariants.md:66`, `:78`); guide `:814`.
- Test status: none — `tests/test_rpc.py` has no notification case. (The generated HTTP adapter does send 204 for `Server.call() is None`, `pkg/generator/python_client_server.go:1004-1007`, so the bug is in the shared runtime.)
- Suggested fix: Compute `is_notification` immediately after the dict check and return `None` for every notification path, including `pulserpc-idl` and malformed requests.
- Acceptance criteria:
  - [ ] `Server.call({'jsonrpc':'2.0','method':'pulserpc-idl'}) is None`.
  - [ ] A notification with bad `jsonrpc`/`method` also returns `None`.

### python-06 — Contract-verification and richer validation tests are excluded from the default make target
- Severity: high (test gap)
- Invariant: R13, R3
- Current behavior: `pkg/runtime/runtimes/python/Makefile:14` runs `python3 -m pytest tests/ -v` and `:22` runs `python -m pytest tests/ -v` in Docker. The top-level `tests/` directory contains only `test_rpc.py`, `test_types.py`, `test_validation.py`. The diff/auditor suite lives at `pulserpc/tests/test_diff.py:1` and a second, richer validation suite at `pulserpc/tests/test_validation.py:1` (multi-error accumulation at `:192-208`, nested validation at `:235-262`, `Contract.validate` at `:292-347`). Neither is collected: `pyproject.toml:1-10` has no pytest config that would add them. Root `Makefile:99-101` delegates to this target, so `make test-runtimes` / `make quality-full` also skip them.
- Expected behavior: R13 "Test files for Go/Java/Python exist but are excluded from their runtime Makefiles" (`docs/audit/invariants.md:68`); guide requires runtime tests for all validation and edge cases (`:843-847`).
- Test status: `pulserpc/tests/test_diff.py:14-242` exists but never runs under `make test-runtime-python`; same for `pulserpc/tests/test_validation.py`. R3's multi-error assertion is therefore only in a dead test.
- Suggested fix: Change both Makefile commands to `pytest tests/ pulserpc/tests/ -v` (or move both files into `tests/` and add `__init__.py`), and add a CI check that a known test id from `test_diff.py` appears in pytest output.
- Acceptance criteria:
  - [ ] `make test-runtime-python` output includes `test_added_required_field_returns_error` from `test_diff.py:68`.
  - [ ] `make test-runtime-python` output includes `test_collects_multiple_errors` from `pulserpc/tests/test_validation.py:192`.

### python-07 — Keyword-argument client calls are rejected by the Python server
- Severity: medium
- Invariant: M8
- Current behavior: `InterfaceClientProxy._create_methodcaller` sends kwargs as a **dict** `params` (`pkg/runtime/runtimes/python/pulserpc/client.py:48-55`) while `Client.call` only converts named→positional for *validation* (`:165-178`) and still puts the dict on the wire (`:189-190`). `Server.call` rejects any non-list params with `-32602` (`pkg/runtime/runtimes/python/pulserpc/server.py:113-118`). The generated example explicitly advertises `client.Iface.method(param=...)` (`pkg/generator/python_client_server.go:354-360`), so documented usage always fails against the companion server.
- Expected behavior: M8 "params shape inconsistent … Python requires list only" (`docs/audit/invariants.md:82`); guide request format uses `params: [...]` (`:811`).
- Test status: none. Generated test clients call methods with positional args only (`pkg/generator/python_client_server.go:1363-1364`); `tests/integration/test_http_api.sh` always sends arrays (e.g. `:211`).
- Suggested fix: In `InterfaceClientProxy` (or `Client.call`), pass named params through `_named_to_positional` before building the request, and send a list. Add an `InProcTransport` round-trip test using kwargs.
- Acceptance criteria:
  - [ ] `client.SomeService.method(name=value)` succeeds against a Python `Server` with `validate_requests=True`.
  - [ ] The outgoing request's `params` is a JSON array, verified in the new test.

### python-08 — No unit tests for Contract/Server/Client behavior; integration assertions are weak
- Severity: medium (test gap)
- Invariant: R7, R10, M1, M3, M9, M17
- Current behavior: `tests/` exercises only validation helpers, type helpers, `RPCError` construction and ctx passing (`tests/test_validation.py:1`, `tests/test_types.py:1`, `tests/test_rpc.py:1-166`). No test asserts `Contract.validate_request/validate_response` (`pkg/runtime/runtimes/python/pulserpc/contract.py:248-340`), the `-32600/-32601/-32602/-32603` branches of `Server.call` (`pkg/runtime/runtimes/python/pulserpc/server.py:63-143`), or a client round trip. The only behavioral coverage is `tests/integration/test_http_api.sh`, which greps for `"error"` presence (`:93`) and prints `WARN` + `PASS` when the error code is wrong (`:105-142`).
- Expected behavior: guide requires tests for error handling and edge cases (`:843-847`); catalog flags the WARN-not-failure harness (M17, `docs/audit/invariants.md:91`; also `problems-core.md` core-11).
- Test status: none in the Python unit suite; the shell suite is not part of `make test-runtime-python` (root `Makefile:99-101`).
- Suggested fix: Add `tests/test_contract.py`, `tests/test_server.py`, `tests/test_client.py` covering all four codes, notification silence, request/response validation and an `InProcTransport` round trip; make the shell helper fail on a code mismatch.
- Acceptance criteria:
  - [ ] `tests/test_server.py` asserts `-32600` (bad jsonrpc/method), `-32601` (unknown method), `-32602` (bad params) and `-32603` (handler exception).
  - [ ] A wrong error code makes `test_http_api.sh` increment `TESTS_FAILED` and exit non-zero.

### python-09 — Batch requests unsupported
- Severity: medium
- Invariant: M7
- Current behavior: `Server.call` immediately rejects any non-dict request with `-32600` (`pkg/runtime/runtimes/python/pulserpc/server.py:62-63`); the generated HTTP adapter passes the parsed body straight through (`pkg/generator/python_client_server.go:998-1004`). There is no array branch anywhere in the Python runtime or generator.
- Expected behavior: guide requires batch requests and "Server handles batch requests" (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:813`, `:1212`); catalog M7 (`docs/audit/invariants.md:86`) and `problems-core.md` core-10.
- Test status: none — no batch case in `tests/` or `tests/integration/test_http_api.sh`.
- Suggested fix: Add an array branch to `Server.call` returning a list of responses and `None` when all entries are notifications; add a unit test with mixed valid/invalid entries.
- Acceptance criteria:
  - [ ] `Server.call([req1, req2])` returns a two-element response list.
  - [ ] A batch of only notifications returns `None`/no HTTP body.

### python-10 — Validation-error `data` does not match the documented shape
- Severity: low
- Invariant: M13
- Current behavior: `Contract.validate_request/validate_response` embed `asdict(build_validation_result(...))` (`pkg/runtime/runtimes/python/pulserpc/contract.py:296`, `:339`), i.e. `{valid: false, error: "<joined string>", invalid_fields: [...]}` built at `:110-127`. No `{field, message}` object is ever produced.
- Expected behavior: docs show `data: {"field": "email", "message": "Required field 'email' is missing"}` (`docs/idl-guide/validation.md:229-246`); catalog M13 (`docs/audit/invariants.md:87`; doc-level fix tracked in `problems-core.md` core-13).
- Test status: the excluded `pulserpc/tests/test_validation.py:265-289` asserts the `ValidationResult` fields, not the wire `data`; no integration assertion on `data`.
- Suggested fix: Either emit a list of `{"field": path, "message": message}` objects as `RPCError.data`, or update the docs to show the actual `ValidationResult` shape — pick one and make Python consistent with it.
- Acceptance criteria:
  - [ ] A missing-required-field error's `data` is byte-for-byte reproducible from the documented example (or the docs show `valid`/`error`/`invalid_fields`).
  - [ ] A test asserts the `data` shape on a real `Server.call` error response.

### python-11 — No named error-code constants in the runtime itself
- Severity: low
- Invariant: R8
- Current behavior: Literal codes are used throughout `pkg/runtime/runtimes/python/pulserpc/contract.py:262,266,273,293,313,317,325,336`, `server.py:63,67,73,93,98,105,130,143` and `client.py:205`. Generated `ErrJsonRpc` constants exist (`pkg/generator/python_client_server.go:549-558`) but the runtime never imports or uses them.
- Expected behavior: R8 flags the lack of shared named constants (`docs/audit/invariants.md:63`); guide lists the standard codes (`:815`).
- Test status: `tests/test_rpc.py:7-28` asserts raw integers, so constants would be trivially verifiable but none are exported.
- Suggested fix: Define module-level constants (e.g. `INVALID_REQUEST = -32600`, …) in `pulserpc/rpc.py`, export them, and replace literals in `server.py`/`client.py`/`contract.py`.
- Acceptance criteria:
  - [ ] `from pulserpc import INVALID_REQUEST, METHOD_NOT_FOUND, INVALID_PARAMS, INTERNAL_ERROR` succeeds and the server emits those exact values.
  - [ ] No bare `-32600`/`-32601`/`-32602`/`-32603` literals remain in the three runtime modules.

## Verified-correct invariants
- R1 — `validate_type`, `find_struct`, `find_enum`, `get_struct_fields` all exist and are exported (`pulserpc/validation.py:139`, `pulserpc/rpctypes.py:56,61,66`, `pulserpc/__init__.py:8-32`).
- R3 — Validation accumulates `ValidationError[]` with paths (`pulserpc/validation.py:60-64,108-136`, `pulserpc/contract.py:278-297`); coverage gap only (python-06).
- R4 — Missing required / null required → error, missing optional → OK (`pulserpc/validation.py:117-134`); asserted in `tests/test_validation.py:195-234`.
- R5 (unqualified case) — Parent-first field order and child override work (`pulserpc/rpctypes.py:74-92`; `tests/test_types.py:39-78`); only qualified `extends` is broken (python-01).
- R7 — `RPCError` has `code`, `message`, optional `data` (`pulserpc/rpc.py:6-13`; `tests/test_rpc.py:7-28`).
- R9 — Python's transport is internally consistent (`Transport.request(dict)->dict`, `HttpTransport`, `InProcTransport` at `pulserpc/transport.py:10-109`); cross-runtime API divergence is a catalog-wide finding, not a Python defect.
- R10 — Server response validation is implemented and invoked (`pulserpc/server.py:146-151`), subject to python-03.
- R12 — Survey claim overstated: Python parses `meta` only for the legacy barrister list format (`pulserpc/contract.py:179-183`), and generated `idl.json` has no `meta` key at all (`pkg/parser/idl.go:8-15`), so `Contract.meta` is always empty in generated projects. No test.
- R14 — Void return validated as `None` (`pulserpc/contract.py:320-329`).
- M1 — `jsonrpc != '2.0'` → `-32600` (`pulserpc/server.py:65-68`); integration coverage only checks error presence (`tests/integration/test_http_api.sh:222-232`).
- M2 — `method` must be a non-empty string (`pulserpc/server.py:70-74`); no Python NPE risk.
- M3 — Unknown/malformed method → `-32601` (`pulserpc/server.py:89-106`); `tests/integration/test_http_api.sh:235-237`.
- M6 — Parse error `-32700` is emitted by the generated test-server HTTP adapter (`pkg/generator/python_client_server.go:992-1001`).
- M9 — Exact parameter count always checked, including zero params (`pulserpc/contract.py:270-276`; `tests/integration/test_http_api.sh:240-241`) — unlike Go/Java/C#.
- M10 — Per-parameter type validation against IDL (`pulserpc/contract.py:278-297`).
- M11 — Enum membership (`pulserpc/validation.py:84-94`), recursive/nested validation (`:161-168`, `pulserpc/tests/test_validation.py:235-262`), fractional-int rejection (`:31-38`; `tests/test_validation.py:43-45`).
- M12 — Extra/unknown fields intentionally permitted (`pulserpc/validation.py:111-136` iterates IDL fields only).
- M15 — Not applicable to Python: the generated `client.py` is only commented example code (`pkg/generator/python_client_server.go:334-370`) and `Client.verify_compatibility` compares local vs server IDL (`pulserpc/client.py:300-317`), not an embedded IDL against itself.
- M16 — Python matches the spec's classification table (removed field → `Info`, `pulserpc/diff.py:253-262`, `_classify_severity` at `:421-422`); the contradictory scenario text is a spec/core issue (`problems-core.md` core-12), not a Python violation.
