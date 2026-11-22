# Doc audit: Python 2 runtime (documentation vs implementation)

## Scope & methodology

Audited the user-facing and invariant documentation for the Python 2 runtime
against the implementation:

- Docs: `docs/languages/python/python2.md`, `docs/languages/python/reference.md`,
  `docs/idl-guide/types.md`, `docs/idl-guide/validation.md`,
  `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` (D1-D16), and the runtime
  `README.md` (`pkg/runtime/runtimes/python2/README.md`).
- Code: `pkg/runtime/runtimes/python2/pulserpc/*`, the Python 2 branch of
  `pkg/generator/python_client_server.go`, `pkg/runtime/embed.go`, and the
  shared harness `tests/integration/servers/python2_conform_server.py`.

Method: every documented claim about Python 2 behavior, waived capabilities,
scalar acceptance, id handling, redaction, limits, and generated output was
checked line-by-line against the source. Where a claim and the code disagree,
the bias applied is that **the documentation should change to match the code**
unless the code is clearly wrong; no source code was modified by this audit.
No Python 2.7 interpreter was available, so runtime behavior is verified by
code review plus the existing test modules
(`pkg/runtime/runtimes/python2/tests/`) and the passing Go generator tests.

## Summary (counts by severity)

- Critical: 0
- High: 0
- Medium: 0
- Low: 5
- Total confirmed doc/code mismatches: 5
- Claims verified as holding: see below
- Test gaps: 4

## Confirmed mismatches (documentation should change to match code)

### doc-python2-01 — Trailing-slash normalization wording overstates the rule
- Severity: low
- Doc locations: `docs/languages/python/python2.md` (D13 bullet),
  `docs/languages/python/reference.md` (Transports section),
  `pulserpc/transport.py` docstring.
- Documented claim: `HttpTransport("http://host/api/")` is normalized to
  `http://host/api` (a single trailing slash is removed).
- Code: `transport.py:52` uses `url.rstrip("/")`, which strips **all**
  trailing slashes, not just one. Python 3 uses the identical implementation
  (`pkg/runtime/runtimes/python/transport.py:52`).
- Bias decision: code is consistent across both Python runtimes and is not
  clearly wrong; the documentation should describe the actual behavior.
- Suggested doc fix: state that `HttpTransport` strips *all* trailing slashes
  from the base URL (e.g. `http://host/api///` → `http://host/api`), or, if
  a single-slash rule is intended, change both runtimes to `rstrip` a single
  character. Recommended: update the docs to match the code.

### doc-python2-02 — Logger name is documented as a literal string
- Severity: low
- Doc location: `docs/languages/python/python2.md` (D4 bullet and
  "Runtime Architecture" section).
- Documented claim: unhandled handler exceptions are logged via
  `logging.getLogger("pulserpc.server")`.
- Code: `server.py:16` creates `logger = logging.getLogger(__name__)`. When the
  module is imported as `pulserpc.server` (the normal generated/imported
  case) the resulting logger name is `pulserpc.server`, so the claim holds in
  practice. If the module is imported under a different name the logger name
  differs.
- Bias decision: the code follows the standard `__name__` convention and is
  not clearly wrong; the documentation should describe the implementation
  accurately.
- Suggested doc fix: say exceptions are logged via `logging.getLogger(__name__)`
  (which resolves to `pulserpc.server` in generated output), rather than
  asserting a hard-coded logger name.

### doc-python2-03 — IDL guide calls `int` a 64-bit integer
- Severity: low
- Doc location: `docs/idl-guide/types.md` (Built-in Types table).
- Documented claim: IDL `int` is a "64-bit integer".
- Code/contract: `RUNTIME_IMPLEMENTATION_GUIDE.md` D1 and `validation.py:12-52`
  define `int` as a **signed 32-bit** value for request/response validation
  (`[-2147483648, 2147483647]`), with native/generated types permitted to be
  wider. `docs/languages/python/reference.md` already documents the 32-bit
  validation range.
- Bias decision: the runtime validation contract (D1) is authoritative; the
  guide entry should not imply a 64-bit wire/validation range.
- Suggested doc fix: change the table entry to "integer (range enforced by
  runtime validation per D1)" or add a footnote pointing at D1, instead of
  "64-bit integers".

### doc-python2-04 — Manual validation guide does not exclude Python 2
- Severity: low
- Doc location: `docs/idl-guide/validation.md` (Manual Validation section).
- Documented claim: arbitrary data can be validated with `Contract.validate()`,
  returning a `ValidationResult`.
- Code: Python 2's `Contract` (`contract.py`) has **no** `validate()` method
  and no `ValidationResult`/`ValidationError` classes; contract-level
  validation raises `RPCError(-32602/-32603)` with a plain-dict `data`
  payload. `reference.md` already notes this Python-3-only limitation, but
  `validation.md` does not.
- Bias decision: the code is correct for the documented carve-out; the guide
  should make the Python 2 limitation discoverable.
- Suggested doc fix: add a short note that `Contract.validate()` and
  `ValidationResult`/`ValidationError` are Python 3 only, and that Python 2
  uses `RPCError`-based validation (see `python2.md`).

### doc-python2-05 — "Generated test servers" claim does not apply to Python 2
- Severity: low
- Doc location: `docs/languages/python/reference.md` (Transports and Resource
  Limits section).
- Documented claim: "Generated test servers (`-generate-test-files`) cap
  request bodies at 1 MiB (`MAX_BODY_SIZE`) and batch entries at 100, and
  return HTTP 405 ... for non-POST methods."
- Code: the Python 2 generator emits only `idl.json` and the runtime package
  (`python_client_server.go`, Python 2 branch returns before
  `generateNamespaceFiles`); there is no generated Python 2 test server. The
  1 MiB cap, 405 handling and batch cap live in the user's adapter or in the
  shared conform harness (`tests/integration/servers/python2_conform_server.py`).
- Bias decision: the code's "metadata + runtime only" output is the documented
  carve-out; the reference text should not imply a generated Python 2 server.
- Suggested doc fix: qualify the sentence as applying to runtimes that
  generate test servers (Go/Java/C#/Python 3), and restate that Python 2 has
  no generated server/adapter.

## Claims verified as holding

- **Feature-reduced notice.** `python2.md` explicitly states Python 2 is
  "actively maintained for legacy systems, not frozen, and not expected to
  reach feature parity", and enumerates waived capabilities. Code and
  `tests/test_api_surface.py` (19 exports, 14 omitted symbols) enforce this.
- **No batch requests.** `Server.call` accepts one request object and rejects
  arrays with `-32600` ("Request must be an object") (`server.py:110-113`);
  no batch dispatch or cap exists in the runtime. The conform harness adds
  batch handling externally, as documented.
- **No contract verification.** No `diff.py`, auditors, checksum helper,
  `Contract.validate`, or client verification API exists in
  `pkg/runtime/runtimes/python2/pulserpc/`; the 14 omitted symbols listed in
  `python2.md`/`test_api_surface.py` are genuinely absent from `__all__`.
- **Structured validation classes.** `ValidationResult`/`ValidationError` are
  absent; contract validation raises `RPCError` with `-32602`/`-32603` and a
  plain-dict `data` payload (`contract.py:13-25`, `:109-189`).
- **Generated output scope.** The generator copies only the runtime and writes
  `idl.json` for Python 2 (`python_client_server.go`, Python 2 branch);
  no stubs, `__init__.py`, server/client/test files are generated. Test files
  are also excluded from the embedded copy (`embed.go`, `IsPythonTestFile`).
- **Type resolution.** `find_struct`/`find_enum`/`get_struct_fields`
  (`rpctypes.py:4-39`) use exact dict keys; `validate_type` (`validation.py:185-207`)
  still performs the unqualified-name suffix scan. Matches the documented
  carve-out.
- **Scalar acceptance (R2/D1).** `validate_int` rejects `bool`, NaN,
  Infinity, fractional floats and out-of-range values, accepts integral
  floats, and range-checks `long` values (`validation.py:30-52`); `validate_float`
  rejects `bool`; `validate_bool` rejects non-bool. Matches `python2.md`.
- **Long ids.** `server.py:13-40` builds `_int_types` from `(int, long)` via
  `2 ** 100` and accepts every legal JSON-RPC numeric id, including
  `long`-sized integers parsed by `json.loads` (`_is_valid_request_id`).
  Booleans, objects, arrays and non-finite numbers are rejected.
- **Handler-exception redaction (D4/R15).** An unhandled handler exception is
  logged server-side (`logger.exception` or `on_error`) and returned as
  `{"code": -32603, "message": "Internal error"}` with no `data`
  (`server.py:201-215`). Declared `RPCError` messages/data and `-32602`
  validation messages are preserved. Matches the documented claim.
- **Response validation is implemented (not a carve-out).** `Server.call`
  validates every result, including `None` (`server.py:219-228`); `Client.call`
  validates the response envelope (version, id match, result XOR error)
  (`client.py:154-177`). The documentation does not claim otherwise.
- **Resource limits (D5/D6).** `DEFAULT_MAX_BODY_SIZE = 1 MiB`
  (`server.py:22`); `HttpTransport` caps response bodies at
  `MAX_RESPONSE_BODY_SIZE = 10 MiB` and raises `ValueError` beyond it
  (`transport.py:16,70-88`); default HTTP timeout is 30 s
  (`transport.py:15`); custom headers are merged over `Content-Type`
  (`transport.py:45-52`).
- **Notification handling (R11/M21).** `is_notification` is computed
  immediately after the dict check; `None` is returned on every path,
  including `pulserpc-idl` and malformed requests (`server.py:115-148`).
- **ctx propagation.** Handler methods receive `ctx` as the first positional
  argument (`server.py`, `call` builds `args = [ctx] + list(params)`), as
  documented.

## Test gaps

1. **Python 2 runtime tests could not be executed in this environment** (no
   `python2.7` interpreter, no Docker daemon). Claims rely on code review plus
   the existing test modules and passing Go generator tests.
2. **Trailing-slash and logger-name behavior is not directly tested.**
   `tests/test_transport.py` covers trailing-slash normalization but not the
   all-slashes case; no test asserts the logger name. The doc/code mismatches
   above are therefore not pinned by tests.
3. **Latent redaction branch is untested.** `server.py:224-227` returns
   `"Response validation failed: %s" % e` as `data` on a bare
   `TypeError`/`ValueError`. It is unreachable today because
   `validate_response` wraps such exceptions into `RPCError`, but no test
   asserts that invariance. (Consistent with the Python 3 audit.)
4. **`get_struct_fields` has no cycle guard and no qualified-parent
   resolution.** Python 3's version guards against cyclic `extends`; the
   Python 2 version does not, relying on the parser rejecting cycles. No test
   covers cyclic or cross-namespace `extends`.

## Unconfirmed hypotheses

- Whether non-finite floats should be rejected by `validate_float` is not
  settled; the current code accepts NaN/Infinity (matching Python 3 and the
  guide's IEEE-754-double definition). The documentation does not claim
  otherwise.
- `HttpTransport.url.rstrip("/")` over-normalizes a base URL that legitimately
  ends in multiple slashes. Not exercised by tests; see doc-python2-01.
- The logger name resolves to `pulserpc.server` only when the module is
  imported under that name; see doc-python2-02.
- `Client.call`'s `except (TypeError, ValueError)` clauses
  (`client.py:102-103, 149-150`) are unreachable for `Contract` validation
  because the contract raises `RPCError`; not documented or tested.
- `Server._signature_cache` is a plain dict mutated from `call`/`add_handler`
  without a lock; concurrent `add_handler` calls (unusual) could race.
- The runtime `README.md` lists `test_validation.py` in the embedded layout,
  but `CopyRuntimeFilesToPackage` skips files matching `IsPythonTestFile`, so
  generated output does not include tests. This is intentional but the README
  wording could be clearer.

## Notes

- No source code was modified by this audit; only this document was written.
- The five mismatches above are all low severity and are documentation wording
  issues rather than implementation defects; the implementation is internally
  consistent with the Python 3 runtime and the shared wire contract.
- Bias applied throughout: documentation should change to match the code unless
  the code is clearly wrong. No code path identified here is clearly wrong.