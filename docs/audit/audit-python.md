# Audit: Python runtime

## Scope & methodology

Audited the Python 3 runtime library (`pkg/runtime/runtimes/python/pulserpc/`), the
Python code generator (`pkg/generator/python_client_server.go`), and the Python
Makefile/test wiring against the invariants in `docs/audit/invariants.md`,
`docs/audit/invariants-additional.md`, and `docs/RUNTIME_IMPLEMENTATION_GUIDE.md`
(D1-D16). The stale baseline `docs/audit/problems-python.md` (python-01..python-11)
was re-verified line-by-line against the current tree; all of its confirmed
violations have since been fixed (commits `9253bb8` and follow-ups).

Method: read every runtime source file (`rpc.py`, `rpctypes.py`, `validation.py`,
`contract.py`, `server.py`, `client.py`, `transport.py`, `diff.py`) and the
generator sections; ran `python -m pytest tests/ pulserpc/tests/` (174 tests, all
passing) and `go test ./pkg/generator/ -run Python` (all passing); generated
`examples/conform.pulse` twice and compared byte-for-byte (deterministic). Only
findings with concrete file:line evidence are reported; suspicions that could not
be confirmed are listed separately.

## Summary (counts by severity)

- Critical: 0
- High: 0
- Medium: 0
- Low: 0
- Total confirmed: 0
- Test gaps: 3 (see below)
- Invariants verified as holding: R1-R32 (Python-relevant), M1-M28, C1-C5, C8,
  C11-C13, S6-S8, D1-D9, D12-D13, D15-D16, G11, G20, G34, G35.

## Confirmed issues

None. Every baseline finding in `docs/audit/problems-python.md` (python-01..python-11)
is resolved in the current tree:

- python-01 (qualified `extends`) — `rpctypes.py:162-179`, `:201-243` now resolve
  parents through `resolve_qualified_name`; covered by
  `tests/test_qualified_inheritance.py`.
- python-02 (float accepts bool) — `validation.py:66-73` rejects `bool`.
- python-03 (null results skip response validation) — `server.py:289-298` and
  `client.py:234-242` no longer guard on `result is not None`.
- python-04 (client envelope) — `client.py:207-210`, `:244-267` validates
  version, id and result-XOR-error.
- python-05 (notifications) — `server.py:172-213` suppresses every path.
- python-06 (excluded tests) — `Makefile:14` / `:22` now collect
  `tests/ pulserpc/tests/`.
- python-07 (kwargs) — `client.py:49-61`, `:167-175` send positional arrays.
- python-08 (missing unit tests) — `tests/test_server.py`, `test_client.py`,
  `test_transport.py`, `test_error_constants.py` added.
- python-09 (batch) — `server.py:130-163` implements bounded batches.
- python-10 (M13 data shape) — resolved in favor of the structured
  `ValidationResult` shape (see Notes).
- python-11 (constants) — `rpc.py:8-12` defines and exports all five codes.

## Test gaps

- **Client-side request validation failure path.** `Client.call` converts a dict to
  positional params and calls `Contract.validate_request` when `validate_request=True`,
  but no test asserts the raised exception type/code. The `except (TypeError, ValueError)`
  clauses at `client.py:182-183` are effectively dead because `Contract.validate_request`
  raises `RPCError` (`contract.py:267`, `:271`, `:276-281`); behavior is unverified.
- **Generated HTTP adapter.** Parse-error (`-32700`), 405, body cap, batch and
  notification handling live in the generated `test_server.py` produced by
  `python_client_server.go:1020-1100`. They are exercised only by
  `tests/integration/test_http_api.sh`, which is not part of
  `make test-runtime-python`.
- **Qualified-enum resolution.** `tests/test_qualified_inheritance.py` covers
  qualified struct `extends` but there is no equivalent case for a qualified
  enum reference in `validate_type`/`find_enum`.

## Verified as holding

- **R1** — `validate_type`, `find_struct`, `find_enum`, `get_struct_fields` exist and
  are exported (`validation.py:173`, `rpctypes.py:181,191,201`,
  `__init__.py:15-39`).
- **R2** — Scalar sets: `int` rejects `bool` and non-finite floats, accepts integral
  floats only (`validation.py:48-64`); `float` rejects `bool` (`:66-73`); `bool`
  rejects non-bool (`:76-79`). 32-bit range enforced (`INT_MIN`/`INT_MAX` at `:20-21`).
- **R3** — Validation accumulates `ValidationError[]` with paths
  (`validation.py:89-93`, `contract.py:283-296`); covered by
  `pulserpc/tests/test_validation.py:192-208`.
- **R4** — Missing required / null required → error, missing optional → OK
  (`validation.py:144-168`); covered by `tests/test_validation.py`.
- **R5 / R6** — Parent fields first, child overrides (`rpctypes.py:222-242`);
  qualified resolution via suffix/short-name scan (`:144-179`) with D16 ordering;
  thread-safe cached index (`:100-142`). Covered by
  `tests/test_qualified_inheritance.py` and `tests/test_resolution_index.py`.
- **R7** — `RPCError(code, message, data=None)` (`rpc.py:15-22`).
- **R8** — Named constants `PARSE_ERROR`..`INTERNAL_ERROR` in `rpc.py:8-12`,
  exported (`__init__.py:7-14`); no bare literals in runtime modules
  (`tests/test_error_constants.py`).
- **R9** — Transport is internally consistent: `Transport.request(dict)->dict`,
  `HttpTransport`, `InProcTransport` (`transport.py:15-129`).
- **R10** — Server response validation implemented and invoked with no `result is not None`
  guard (`server.py:293-298`); covered by `tests/test_server.py`.
- **R11 / M4 / M21** — Notifications produce no response on every path, including
  `pulserpc-idl` and malformed requests (`server.py:172-213`); covered by
  `tests/test_server.py:223-240`.
- **R12** — `meta` parsed in the legacy barrister list format
  (`contract.py:179-183`); generated `idl.json` has no `meta` key, so `Contract.meta`
  is empty in generated projects (matches parser schema).
- **R13 / M15** — `diff_idl`, four auditors, `Contract.validate`, `extract_checksum`,
  and `Client.verify_compatibility` exist and are tested
  (`pulserpc/tests/test_diff.py`, `tests/test_client.py`). Python compares local vs.
  server IDL, so the self-comparison problem (M15) does not apply.
- **R14** — Void returns validated as `None` (`contract.py:328-338`).
- **R15** — Unhandled handler exceptions are logged server-side and returned as
  `INTERNAL_ERROR` / `"Internal error"` with no `data`
  (`server.py:273-287`); covered by `tests/test_server.py`. Response-validation
  failures keep the structured `ValidationResult` as `data` per the documented
  decision (see Notes).
- **R16-R21** — Body 1 MiB cap in generated adapter
  (`python_client_server.go:1034-1036`); response 10 MiB cap
  (`transport.py:11-12`, `:84-93`); batch 100 cap (`server.py:23`, `:148-152`);
  30s default timeout (`transport.py:11`); atomic request ids via lock
  (`client.py:90-94`, `:186-188`); thread-safe resolution cache
  (`rpctypes.py:100-142`).
- **R22-R24** — Generated adapter accepts only POST with 405 + `-32600`
  (`python_client_server.go:1055-1073`); notifications yield 204
  (`:1051-1054`); id validated as string/number/null (`server.py:26-39`,
  `:176-182`).
- **R25-R32** — 32-bit int range (`validation.py:20-31`); IEEE-754 double
  (`float`); enums as wire strings (generated `rpctypes.py`); custom headers and
  trailing-slash normalization (`transport.py:38-52`); `pulserpc-idl` bootstrap
  (`client.py:111-135`); 43-char checksum in `idl.json`
  (`python_client_server.go:279-283`).
- **M1-M3, M5-M12, M14, M16-M28** — `jsonrpc`/`method`/params validation,
  exact param count, per-param type validation, enum membership, recursive
  validation, extra fields permitted, response validation, structured `data`,
  severity classification, batch semantics, notification handling, envelope
  validation, and error-object shape are implemented and covered by
  `tests/test_server.py` / `tests/test_client.py`.
- **M13** — Python keeps the structured `ValidationResult`
  (`{valid, error, invalid_fields}`) as `RPCError.data` on both request and
  response validation paths (`contract.py:260-264`, `:316-318`, `:296-302`,
  `:344-349`).
- **C1** — Generation is byte-deterministic (verified by two runs of
  `examples/conform.pulse` producing identical bytes).
- **C2** — `idl.json` has `rootNamespace`, `checksum`, `interfaces`, `structs`,
  `enums`, `errors` (`python_client_server.go:277-296`).
- **C3** — Python writes `idl.json` under the entry-point namespace directory
  (documented divergence).
- **C4 / C5** — Checksum is 43 chars; served `pulserpc-idl` result is an object.
- **C8 / C11 / C12 / C13** — Stable namespace layout; enums as strings; flag
  defaults; generated code passes its test suite.
- **D1-D9, D12-D13, D15-D16** — 32-bit int, id types, (generated) 405,
  redaction, limits, 30s timeout, optional dynamism, uniform IDL JSON, 43-char
  checksum, determinism, header/trailing-slash, contract verification, type-name
  resolution.
- **G11 / G20 / G34 / G35** — Negative error codes, 43-char checksum,
  checksum excludes comments/error text, includes parameter position (parser side;
  Python consumes the resulting `idl.json`).

## Unconfirmed hypotheses

- `HttpTransport.url.rstrip('/')` (`transport.py:52`) strips *all* trailing
  slashes rather than a single one; a base URL that legitimately ends in
  multiple slashes would be over-normalized. Not exercised by tests.
- `Client.call`'s `except (TypeError, ValueError)` blocks (`client.py:182-183`,
  `:239-240`) are unreachable for `Contract` validation because the contract raises
  `RPCError`; the intended conversion (if any) is not documented or tested.
- Generated production `server.py` contains only abstract interface stubs; the
  HTTP adapter and `Server` wiring are generated only for the test server. Whether
  a first-time user can assemble a production server without reading the runtime
  guide is unverified, but this is by design for the dynamic Python runtime.

## Notes

- **Documented decision verified (M13 / R15).** Python intentionally keeps the
  structured `ValidationResult` (`{valid, error, invalid_fields}`) as `RPCError.data`
  for validation errors and for response-validation failures. This is documented in
  `docs/idl-guide/validation.md:245-247` ("TypeScript and Python runtimes expose a
  structured validation result instead"), in the runtime docstrings
  (`contract.py:260-264`, `:316-318`), and asserted by
  `tests/test_server.py:356-375`. The unhandled-exception path remains opaque
  (`server.py:278-287`, "Internal error", no `data`), so D4/S6 are not violated.
- `docs/audit/problems-python.md` is a stale baseline: its python-01..python-11
  findings are resolved in the current tree; this document supersedes it for the
  Python 3 runtime.
- No source code was modified by this audit; only this document was written.