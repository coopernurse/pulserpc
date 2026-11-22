# Doc Audit: Python 3 runtime (documentation vs implementation)

## Scope & methodology

Audited the Python 3 runtime library (`pkg/runtime/runtimes/python/pulserpc/`) and the
Python code generator (`pkg/generator/python_client_server.go`) against the
user-facing and spec documentation: `docs/languages/python/reference.md`,
`docs/languages/python/quickstart.md`, `docs/languages/python/python2.md`,
`docs/RUNTIME_IMPLEMENTATION_GUIDE.md` (Python sections), `docs/idl-guide/*`,
and `openspec/specs/python-*`. The code was treated as authoritative unless it
is clearly wrong; every finding below cites the documentation claim and the
implementing code. No source code was modified by this audit; only this document
was written.

Method: read every runtime source file and the generator sections named above;
cross-checked each documented claim in the focus areas (validation data shape,
severity, notification handling, response validation, id types, batch/limits,
scalar acceptance, void returns, meta, contract verification, namespace
resolution, transport, accumulation) against the implementation.

## Summary (counts by severity)

- Critical: 0
- High: 1
- Medium: 4
- Low: 2
- Total confirmed doc mismatches: 7
- Verified as matching: validation data shape, severity classification,
  notification handling, id types, scalar acceptance, void returns, meta
  handling, namespace resolution, accumulation, transport limits.

## Confirmed doc mismatches

### python-doc-01 - Guide says generated Python servers do not support batch requests

- Severity: High
- Doc: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` ("JSON-RPC 2.0 Compliance" section)
  states "Batch requests (an array of requests) are optional for generated
  adapters. The generated Go, C# and Java servers support them; generated
  TypeScript and Python servers do not."
- Code: `pkg/runtime/runtimes/python/pulserpc/server.py` implements
  `_call_batch` with `DEFAULT_MAX_BATCH_SIZE = 100`, and the generated test
  server delegates to `rpc_server.call(req)` (`python_client_server.go`,
  `do_POST`), so arrays are dispatched and bounded. `Server.call` accepts
  `Union[Dict, List]`.
- Suggested doc fix: move Python into the batch-supporting list (and keep the
  100-entry cap documented), or reword to "generated Python servers support
  batches via the runtime `Server`".

### python-doc-02 - Quickstart/reference use `PulseRPCServer`, which the generator does not emit

- Severity: Medium
- Doc: `docs/languages/python/quickstart.md` ("This creates: `server.py` -
  PulseRPCServer framework ..."), `docs/languages/python/reference.md`
  (`from server import PulseRPCServer, CatalogService` and
  `PulseRPCServer(host=..., port=...)`), and
  `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` describe a generated `PulseRPCServer`
  HTTP wrapper.
- Code: `pkg/generator/python_client_server.go` generates abstract service
  classes plus a module-level `rpc_server = Server(contract, ...)` and a
  `TestRPCHandler` only in `-generate-test-files` output; there is no
  `PulseRPCServer` class. HTTP serving is the user's (or test harness's)
  responsibility.
- Suggested doc fix: replace `PulseRPCServer` examples with construction of an
  `http.server.HTTPServer` feeding `rpc_server.call(req)`, or with the
  generated `TestRPCHandler` pattern, and document that `Server` is
  transport-independent.

### python-doc-03 - "Automatic validation" implies client-side validation that is opt-in

- Severity: Medium
- Doc: `docs/languages/python/reference.md` ("Automatic Validation: PulseRPC
  automatically validates requests and responses against your IDL") and
  `docs/idl-guide/validation.md` ("Validation occurs at two points: Client-side
  - Before sending requests; Server-side - Before and after processing
  requests").
- Code: `pkg/runtime/runtimes/python/pulserpc/server.py` defaults
  `validate_requests=True, validate_responses=True`, but
  `pkg/runtime/runtimes/python/pulserpc/client.py` defaults
  `validate_request=False, validate_response=False`. Client validation runs only
  when explicitly enabled.
- Suggested doc fix: state that server-side validation is on by default while
  client-side validation is opt-in via `Client(transport, validate_request=True,
  validate_response=True)`.

### python-doc-04 - Spec claims verification falls back to server IDL when no local IDL exists

- Severity: Medium
- Doc: `openspec/specs/python-contract-verification/spec.md` ("Client Loads
  Local IDL from idl.json": "IF idl.json is not found, verification uses
  server IDL only (no local comparison)").
- Code: `pkg/runtime/runtimes/python/pulserpc/client.py` `verify_compatibility`
  raises `RuntimeError("No local IDL available - call set_local_idl() or ensure
  idl.json exists")` when `self._local_idl` is falsy; it never compares the
  server IDL against itself.
- Suggested doc fix: change the scenario to state that verification raises
  `RuntimeError` unless a local IDL is supplied via `idl.json` or
  `set_local_idl()`.

### python-doc-05 - Spec says `ctx` is the last positional handler argument

- Severity: Medium
- Doc: `openspec/specs/python-ctx-support/spec.md` ("Server.call accepts ctx
  parameter ... passed as the last positional argument to all JSON-RPC method
  handler invocations"; generated stubs include `ctx=None` "as the last
  parameter").
- Code: `pkg/runtime/runtimes/python/pulserpc/server.py` invokes
  `func(ctx, *params)` and binds `signature.bind(ctx, *params)`; the
  documentation in `reference.md` and `python2.md` and the quickstart examples
  all place `ctx` first (after `self`).
- Suggested doc fix: amend the spec to "first positional argument after self",
  matching the implementation and the other Python docs.

### python-doc-06 - Reference uses wrong `HttpTransport` capitalization

- Severity: Low
- Doc: `docs/languages/python/reference.md` Client Usage section:
  `from client import HTTPTransport, CatalogServiceClient` and
  `transport = HTTPTransport("http://localhost:8080")`.
- Code: `pkg/generator/python_client_server.go` imports
  `HttpTransport` (e.g. `from ... import HttpTransport, Client`) and generated
  clients use `HttpTransport(server_url)`.
- Suggested doc fix: use `HttpTransport` everywhere in the Python docs.

### python-doc-07 - Docs say HttpTransport normalizes a single trailing slash

- Severity: Low
- Doc: `docs/languages/python/reference.md` ("HttpTransport ... normalizes a
  single trailing `/` on the base URL") and the transport table.
- Code: `pkg/runtime/runtimes/python/pulserpc/transport.py`
  `self.url = url.rstrip('/') if url else url` removes every trailing slash,
  not just one.
- Suggested doc fix: change "a single trailing `/`" to "trailing `/` characters",
  or change the code to strip exactly one slash if single-slash normalization is
  intended.

## Test gaps

- No test asserts the `RPCError` (rather than `ValueError`) raised by
  `Client.call` when `validate_response=True` rejects a response, because
  `Contract.validate_response` raises `RPCError`, which is not caught by the
  `except (TypeError, ValueError)` clauses in `client.py`.
- No test exercises the generated Python HTTP adapter's batch path through the
  documented quickstart flow; the runtime `_call_batch` is unit-tested only
  indirectly.
- No test pins `HttpTransport` URL normalization beyond a single trailing
  slash.

## Verified as matching (focus areas)

- **Validation data shape** - Python emits the structured `ValidationResult`
  (`valid`, `error`, `invalid_fields`) as `RPCError.data` for both request and
  response validation; `Contract.validate()` returns the same shape. Matches
  `docs/idl-guide/validation.md` and `reference.md`.
- **Severity** - removed fields are `Info`, added required fields are `Error`,
  type changes are `Error`; matches the amended spec and code.
- **Notification handling** - requests without an `id` (including `null`) produce
  no response on every path, including `pulserpc-idl` and malformed requests.
- **Response validation** - no `result is not None` guard remains; null results
  for required returns are rejected with `INTERNAL_ERROR`.
- **id types** - `_is_valid_request_id` accepts string/number/null only and
  rejects bool/object/array/non-finite numbers, matching D2.
- **Batch/limits** - runtime batch cap 100 and 1 MiB body cap in the generated
  adapter match D5 and `reference.md`.
- **Scalar acceptance** - `int` rejects bool and non-integral/non-finite floats
  with 32-bit range enforcement; `float` rejects bool; matches R2 and the
  idl-guide.
- **Void returns** - `validate_response` requires `None` for methods without a
  return type.
- **meta** - parsed only in the legacy Barrister list format; generated
  `idl.json` has no `meta` key, so `Contract.meta` is empty in generated
  projects (matches the parser schema; not documented as a user feature).
- **Contract verification** - compares the local IDL against the server IDL
  fetched at bootstrap; severity and auditor invocation match the spec except
  for the two spec scenarios corrected above.
- **Namespace resolution** - suffix/short-name resolution with deterministic
  sorted-key tie-breaking applies to structs and enums, including `extends`.
- **Transport** - custom headers, 30 s default timeout, 10 MiB response cap, and
  405 for non-POST in the generated adapter match D6/D13/D5.
- **Accumulation** - validation collects all errors across fields, array
  elements, map values, and parameters.

## Notes

- This audit covers documentation vs implementation only. Code defects already
  tracked in `docs/audit/problems-python.md` and `docs/audit/audit-python.md`
  (and fixed in the current tree) are not restated here.
- Where a documented behavior and the implementation disagree, this report
  recommends updating the documentation to match the code unless the code is
  clearly wrong (notably `python-doc-07`, where either side could be
  intentional).

No source code was modified by this audit; only this document was written.
