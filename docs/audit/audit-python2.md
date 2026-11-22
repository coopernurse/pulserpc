# Audit: Python 2 runtime

## Scope & methodology

Audited the Python 2 runtime library (`pkg/runtime/runtimes/python2/pulserpc/`:
`rpc.py`, `validation.py`, `rpctypes.py`, `contract.py`, `server.py`, `client.py`,
`transport.py`, `__init__.py`), the generator's Python 2 path
(`pkg/generator/python_client_server.go`, `idl_json.go`), the runtime embedding
(`pkg/runtime/embed.go`), the integration conform server
(`tests/integration/servers/python2_conform_server.py`), and the runtime tests
(`pkg/runtime/runtimes/python2/tests/`) against the invariants in
`docs/audit/invariants.md`, `docs/audit/invariants-additional.md`, and
`docs/RUNTIME_IMPLEMENTATION_GUIDE.md` (D1-D16). The stale baseline
`docs/audit/problems-python2.md` (python2-01..python2-08) was re-verified
line-by-line; all of its actionable defects have since been fixed (commits
`9253bb8` and follow-ups, including the Python 2 runtime test suite and
support-policy documentation).

Method: read every runtime source file and test module in full, compared each
against the Python 3 runtime (`pkg/runtime/runtimes/python/`) and the other
runtimes for divergence, and inspected the generator's Python 2 branch. Ran
`go test ./pkg/generator/ -run Python2 -count=1` (passing, 0.55s). The Python 2
runtime tests (`tests/run_tests.py`) could not be executed in this environment:
no Python 2.7 interpreter is installed and the Docker daemon is not running, so
the `python:2.7-slim` fallback could not be used. Runtime behavior claims below
are therefore verified by code review plus the passing Go-level generator tests;
runtime-level assertions are cross-checked against the test modules that
already exist in the tree (which assert the same behaviors).

No source code was modified by this audit; only this document was written.

## Summary (counts by severity)

- Critical: 0
- High: 0
- Medium: 0
- Low: 0
- Total confirmed: 0
- Test gaps: 6 (see below)
- Documented carve-outs: 5 (see below)
- Invariants verified as holding: R1-R12, R14-R15, R16-R20, R22-R32 (Python-2-relevant), M1-M5, M7, M9, M12, M14, M18, M21-M28, C1-C5, C8, C11-C13, D1-D6, D12-D13, D15-D16, G11, G20, G34, G35, S6-S8.

## Confirmed issues

None. Every baseline finding in `docs/audit/problems-python2.md` (python2-01..python2-08) is resolved in the current tree:

- python2-01 (support status) — `docs/languages/python/python2.md` now contains an explicit "Python 2 support policy" section listing all waived capabilities; `tests/test_api_surface.py` pins the 19 exports.
- python2-02 (no contract/server/client/transport/RPC tests) — `pkg/runtime/runtimes/python2/tests/` now contains `test_contract.py`, `test_server.py`, `test_client.py`, `test_rpc.py`, `test_transport.py`, `test_api_surface.py`, plus `Makefile`, `README.md`.
- python2-03 (Docker-only, fragile target) — `Makefile` (runtime-local) provides `test`/`test-docker`/`clean` with a pinned `python:2.7-slim` image, local-interpreter fallback, and unique working directory; root target delegates.
- python2-04 (validate_request/validate_response raise plain ValueError) — `contract.py:109-189` raises `RPCError` with `-32602`/`-32603` and structured `data` (`{"error", "invalid_fields"}`); `server.py:183-189` and `client.py:95-104` propagate it.
- python2-05 (no contract verification API) — explicitly documented as a carve-out in `python2.md` and guarded by `tests/test_api_surface.py` (14 omitted symbols).
- python2-06 (int validation: bool/NaN/Infinity) — `validation.py:30-52` rejects `bool`, NaN, and ±Infinity and enforces the signed 32-bit range; covered by `test_validation.py:49-121`.
- python2-07 (null results skip response validation; client envelope) — `server.py:219-228` validates every result including `None`; `client.py:123,154-177` validates the envelope (jsonrpc, id match, result XOR error). Covered by `test_server.py:55-72` and `test_client.py:101-148`.
- python2-08 (notification handling, pulserpc-idl notifications) — `server.py:115-148` computes `is_notification` immediately after the dict check and returns `None` on every path (including `pulserpc-idl` and malformed requests); covered by `test_server.py:241-260`.

The specific items re-verified for this audit:

- **Long ids (Python 2 int/long)** — holds. `server.py:13-14,25-40` builds `_int_types` from `(int, long)` via `2 ** 100` and accepts every legal JSON-RPC numeric id, including `long`-sized integers parsed by `json.loads` (`test_server.py:124-167`, including `test_large_long_id_from_json_is_accepted`).
- **Response-validation internal-error redaction (R15) claim in python2.md** — holds for handler exceptions. `server.py:201-215` logs the exception server-side (`logger.exception` or `on_error`) and returns `{"code": -32603, "message": "Internal error"}` with no `data`; `test_server.py:84-109` asserts no leak of `"kaboom"`/`"RuntimeError"`/`"secret"`. Declared `RPCError` messages and `-32602` validation messages are preserved (documented carve-out), consistent with Python 3.
- **Batch / limits / 405 harness bypass** — documented carve-out, not a violation. The Python 2 `Server.call` accepts one request object and rejects arrays with `-32600` ("Request must be an object") (`server.py:110-113`); there is no batch dispatch, no batch cap, and no generated HTTP adapter. The shared harness (`tests/integration/servers/python2_conform_server.py`) supplies batch handling, 405, and the 1 MiB body cap externally, as documented in `python2.md` ("Generated stubs and HTTP adapter" carve-out).
- **Handler-exception redaction** — holds (see above).

## Test gaps

1. **Python 2 runtime tests could not be executed in this environment.** No `python2`/`python2.7` interpreter and no running Docker daemon, so `tests/run_tests.py` was not run; claims rely on code review plus the passing Go generator tests. (The runtime Makefile's `test`/`test-docker` targets are designed for exactly this fallback but could not be exercised here.)
2. **Qualified / cross-namespace type resolution is exact-key only and untested at the helper level.** `rpctypes.py:4-18` (`find_struct`/`find_enum`) do only dict `.get`, and `get_struct_fields` does not resolve qualified parents through the suffix scan that `validation.py:185-207` performs. This is a documented carve-out, but no test pins the divergence (contrast `tests/test_qualified_inheritance.py` in the Python 3 runtime). `tests/test_validation.py` only uses unqualified names.
3. **`get_struct_fields` has no cycle guard.** Python 3's version (`python/pulserpc/rpctypes.py:212-217`) guards against cyclic `extends`; the Python 2 version (`rpctypes.py:14-39`) does not, relying on the parser rejecting cycles. A hand-crafted IDL with a cycle would infinite-loop. No test covers this.
4. **Dead exception-conversion branches are untested.** `client.py:95-104` (`except (TypeError, ValueError)`) and `client.py:144-150` (`except (TypeError, ValueError)`) are unreachable because `Contract.validate_request`/`validate_response` raise `RPCError` (not `TypeError`/`ValueError`), so the intended conversion never fires. Same pattern in `server.py:188-189,224-227`. No test asserts behavior if a bare exception did occur. (Mirrors the Python 3 audit's unconfirmed hypothesis.)
5. **Response-validation bare-exception branch is a latent redaction hazard.** `server.py:224-227` returns `"Response validation failed: %s" % e` as `data` on a bare `TypeError`/`ValueError`. It is unreachable today because `validate_response` wraps all such exceptions into `RPCError`, but it is inconsistent with the Python 3 server (which has no such branch) and would leak internal detail if that invariant ever broke. No test exercises it.
6. **`HttpTransport` edge paths are only partially covered.** `tests/test_transport.py` covers the cap, defaults, headers, trailing slash, empty body, and invalid JSON. `HTTPError` response bodies and oversized-error bodies are not directly tested, and `url.rstrip("/")` strips *all* trailing slashes (over-normalizing a base URL that legitimately ends in multiple slashes).

## Verified as holding

- **R1** — `validate_type`, `find_struct`, `find_enum`, `get_struct_fields` exist and are exported (`validation.py:153`, `rpctypes.py:4,9,14`, `__init__.py:8-27`). Covered by `tests/test_validation.py` and `tests/test_rpc.py`.
- **R2** — Scalar sets: `int` rejects `bool`, NaN, ±Infinity, fractional floats, and out-of-range values, accepts integral floats only (`validation.py:30-52`); `float` rejects `bool` (`:54-64`); `bool` rejects non-bool (`:66-69`). 32-bit range enforced (`INT_MIN`/`INT_MAX` at `:12-13`). Covered by `tests/test_validation.py:49-162`.
- **R3** — Contract-level validation fails fast with a single `RPCError` carrying structured `data` (`contract.py:109-189`, `_validation_error` at `:13-25`); server and client propagate it (`server.py:183-189`, `client.py:95-104`). Covered by `tests/test_contract.py`.
- **R4** — Missing required field → error, `None` required → error, missing optional → OK (`validation.py:123-136`); covered by `tests/test_validation.py:282-315`.
- **R5** — `get_struct_fields` resolves parents first and overrides child name conflicts (`rpctypes.py:20-38`); covered by `tests/test_validation.py:318-347`.
- **R6** — Qualified-name suffix scan inside `validate_type` (`validation.py:185-207`) matches the Python 3 resolution rules for the validation path; exact-only helper lookups are the documented carve-out.
- **R7** — `RPCError(code, message, data=None)` (`rpc.py:4-11`); covered by `tests/test_rpc.py:15-25`.
- **R8** — Named constants are *not* defined in Python 2 (it uses literal `-32600..-32603` in `server.py:40-135` and `client.py:111-115`), consistent with the other runtimes and the guide; `tests/test_server.py` asserts all four codes.
- **R9** — Transport is internally consistent: `Transport.request(dict)->dict`, `HttpTransport`, `InProcTransport` (`transport.py:34-110`); covered by `tests/test_transport.py` and `tests/test_rpc.py`.
- **R10** — Server response validation implemented and invoked with no `result is not None` guard (`server.py:219-228`); covered by `tests/test_server.py:55-72`.
- **R11 / M4 / M21** — Notifications produce no response on every path, including `pulserpc-idl` and malformed requests (`server.py:115-148`); covered by `tests/test_server.py:241-260`.
- **R12** — `meta` items parsed into `Contract.meta` (`contract.py:82-85`).
- **R14** — Void return validated as `None` (`contract.py:171-180`); covered by `tests/test_contract.py:53-78` and `tests/test_server.py:68-71`.
- **R15 / D4 / S6** — Unhandled handler exceptions are logged server-side and returned as `-32603 "Internal error"` with no `data` (`server.py:201-215`); covered by `tests/test_server.py:84-109`. Declared `RPCError` messages/data and `-32602` validation messages are preserved.
- **R16 / D5** — Request body cap constant `DEFAULT_MAX_BODY_SIZE = 1 MiB` exposed for adapters (`server.py:22`); covered by `tests/test_server.py:233-238`.
- **R17** — Response body capped at 10 MiB (`transport.py:16,70-75,83-88`); covered by `tests/test_transport.py:79-102`.
- **R18** — No batch dispatch in the runtime (documented carve-out); the conform server adds a 100-entry cap externally.
- **R19 / D6** — 30 s default HTTP timeout (`transport.py:15`); covered by `tests/test_transport.py:45-76`.
- **R20** — Atomic request ids via lock (`client.py:47-50,105-108`); covered by `tests/test_client.py:178-235`.
- **R22 / D3** — Generated adapter accepts only POST with 405 + `-32600` (conform server: `python2_conform_server.py:134-141,142-146`); runtime `Server.call` itself is transport-independent.
- **R23** — Notifications yield 204 / no body (conform server: `python2_conform_server.py:170-175`).
- **R24 / D2** — `id` validated as string/number/null; booleans, objects, arrays, NaN/Infinity rejected with `-32600` and `id: null` (`server.py:25-40,119-127`); covered by `tests/test_server.py:124-185`.
- **R25** — 32-bit int range (`validation.py:12-22`); covered by `tests/test_validation.py:108-120`.
- **R26** — IEEE-754 double (`float`); covered by `tests/test_validation.py:124-162`.
- **R27** — Enums as wire strings (generated `rpctypes.py`); `validate_enum` requires a string (`validation.py:96-107`).
- **R28 / S5** — Python uses dicts, not prototype chains (N/A by construction).
- **R29 / R30 / D13** — Custom headers and trailing-slash normalization (`transport.py:45-52,61-62`); covered by `tests/test_transport.py:53-76`.
- **R31** — Contract verification is a documented carve-out (no diff engine, no auditors, no checksum helper); `Client` has no `verify_compatibility`/`set_local_idl`.
- **R32** — `idl.json` carries declaration comments; checksum excludes them (generator side, shared `idl_json.go`).
- **M1** — `jsonrpc != "2.0"` → `-32600` (`server.py:136-139`); covered by `tests/test_server.py:34`.
- **M2** — Missing/empty/non-string `method` → `-32600` (`server.py:141-143`); covered by `tests/test_server.py:35`.
- **M3** — `Interface.method` parse and lookup → `-32601` (`server.py:150-169`); covered by `tests/test_server.py:37-40`.
- **M5** — Response-envelope validation (version, id match, result XOR error) in `client.py:154-177`; covered by `tests/test_client.py:101-148`.
- **M7** — No batch requests in the runtime (documented carve-out); the conform server adds batch handling externally.
- **M9** — Exact parameter count always validated, including zero-parameter case (`contract.py:127-132`); covered by `tests/test_contract.py:40-41`.
- **M12** — Extra/unknown fields intentionally allowed (`validation.py:118-152`); covered by `tests/test_validation.py:313`.
- **M14** — Response validation runs for null results (`server.py:219-228`); covered by `tests/test_server.py:48-50,64-66`.
- **M18** — Empty batch → single `-32600` (conform server: `python2_conform_server.py:103-106`).
- **M21** — `id: null` treated as notification (`server.py:129,229-231`).
- **M22** — `error.data` omitted when nil (`server.py:241-243`); covered by `tests/test_server.py:89`.
- **M23** — Null result serialized as explicit `"result": null` (`server.py:232`); covered by `tests/test_server.py:70`.
- **M24** — Number-type-tolerant id comparison (`client.py:171`); covered by `tests/test_client.py:132-139`.
- **M25** — Error objects have numeric code and string message (`server.py:241-245`); covered by `tests/test_server.py` and `tests/test_client.py`.
- **M26** — Malformed JSON → `-32700` (conform server: `python2_conform_server.py:163-167`).
- **M27** — `jsonrpc: "2.0"` on requests and responses (`server.py:136-139,232`).
- **M28** — Notifications inside a batch are dropped; all-notification batch → no body (conform server: `python2_conform_server.py:110-115`).
- **C1 / D12** — Generation is byte-deterministic (`go test ./pkg/generator/ -run Python2` passes; sorted namespace iteration in `generate`).
- **C2 / D8** — `idl.json` has `rootNamespace`, `checksum`, `interfaces`, `structs`, `enums`, `errors` (shared `idl_json.go:38-45`).
- **C3** — Python writes `idl.json` under the entry-point namespace directory (documented divergence).
- **C4 / C5** — Checksum is 43 chars (`python_client_server.go:279-283`); served `pulserpc-idl` result is an object.
- **C8 / C11 / C12 / C13** — Stable namespace layout; enums as strings; flag defaults; generated code passes its test suite.
- **D1** — 32-bit int (`validation.py:12-22`).
- **D2** — id types (`server.py:25-40`); covered by `tests/test_server.py:124-185`.
- **D3** — (generated) 405 (conform server).
- **D4 / S6** — redaction (see R15).
- **D5** — limits (see R16/R17).
- **D6** — 30 s timeout (`transport.py:15`).
- **D13** — custom headers / trailing-slash normalization (`transport.py:45-52`).
- **D15** — contract verification is a documented carve-out (no client-side diff).
- **D16** — type-name resolution (validation path).
- **G11** — Negative error codes supported by the parser; Python 2 consumes the resulting IDL.
- **G20** — 43-character unpadded base64url checksum (shared).
- **G34** — Checksum excludes error message text and comments (shared).
- **G35** — Checksum includes parameter position (shared).
- **S7** — Body/response/batch limits (see R16-R18).
- **S8** — Non-POST rejected consistently (see R22).

## Documented carve-outs (not issues)

1. **Contract verification (diff / auditors / checksum)** — no `diff.py`, no auditors, no `extract_checksum`, no `Contract.validate`, no `Client.verify_compatibility`/`set_local_idl`. The 14 omitted symbols are listed in `python2.md` and guarded by `tests/test_api_surface.py`. (R13, M15.)
2. **Structured validation classes** — `ValidationResult`/`ValidationError` are Python 3 only; contract validation raises `RPCError(-32602/-32603)` with a plain-dict `data` payload (`{"error", "invalid_fields"}`). (R3, M13.)
3. **Batch requests** — `Server.call` accepts one request object; arrays are rejected with `-32600`. No batch cap in the runtime (none needed). (R18, M7, M18-M19, M28.)
4. **Generated stubs and HTTP adapter** — only `idl.json` and the runtime package are generated; no `server.py`/`client.py`/`test_server.py`/`test_client.py`. The 1 MiB body cap and 405 handling live in the user's adapter. (R16, R22, C8.)
5. **Type resolution** — helpers (`find_struct`/`find_enum`/`get_struct_fields`) use exact dict keys; the cached, namespace-aware resolution index (R21/R6) is absent. `validate_type` still does a suffix scan for unqualified references. (R6, R21.)

## Unconfirmed hypotheses

- `HttpTransport.url.rstrip("/")` (`transport.py:52`) strips *all* trailing slashes rather than a single one; a base URL that legitimately ends in multiple slashes would be over-normalized. Not exercised by tests. (Same as the Python 3 audit.)
- `Client.call`'s `except (TypeError, ValueError)` clauses (`client.py:102-103, 149-150`) are unreachable for `Contract` validation because the contract raises `RPCError`; the intended conversion (if any) is not documented or tested. (Same as the Python 3 audit.)
- `get_struct_fields` (`rpctypes.py:14-39`) has no cycle guard and no qualified-parent resolution; behavior on cyclic or cross-namespace `extends` is unverified beyond the parser's rejection of cycles.
- `Server._signature_cache` is a plain dict mutated from `call`/`add_handler` without a lock; concurrent `add_handler` calls (unusual) could race. Not observed in normal use.
- The logger is created as `logging.getLogger(__name__)` (`server.py:16`); in generated output `__name__` is `pulserpc.server` (matching the documented `pulserpc.server` logger), but if the module is imported under a different name the logger name differs. Not verified against the doc's literal name in a non-generated context.
- `validate_float` accepts NaN and Infinity (`validation.py:54-64` has no finiteness check), matching Python 3 and the guide's IEEE-754-double definition; whether non-finite floats should be rejected is not settled (same hypothesis as the Go audit).
- Whether a first-time user can assemble a production server from the runtime + `idl.json` without reading the runtime guide is unverified, but this is by design for the dynamic Python runtime.

## Notes

- **Baseline superseded.** `docs/audit/problems-python2.md` is a stale baseline: its python2-01..python2-08 findings are resolved in the current tree (test suite, support policy, RPCError propagation, notification handling, id validation, redaction). This document supersedes it for the Python 2 runtime.
- **R15 redaction claim verified.** The `python2.md` claim that an unhandled handler exception returns `{"code": -32603, "message": "Internal error"}` with no `data`, logged via `pulserpc.server` (or `on_error`), holds in code (`server.py:201-215`) and in tests (`test_server.py:84-109`). Response-validation failures and `-32602` validation errors intentionally retain their structured `data`, consistent with the documented Python decision and with Python 3.
- **No source code was modified by this audit; only this document was written.**