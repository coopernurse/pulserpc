# Doc audit: Java runtime (documentation vs implementation)

## Scope & methodology

Audited the user-facing Java documentation
(`docs/languages/java/reference.md`, `docs/languages/java/quickstart.md`),
the Java-relevant sections of `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` (D1-D16
and the Java runtime/generator notes), `docs/idl-guide/*` (notably
`validation.md`), `docs/get-started/cli-reference.md`, and the openspec
contract-compatibility spec, against the Java runtime implementation
(`pkg/runtime/runtimes/java/com/bitmechanic/pulserpc/*`) and the Java
sections of `pkg/generator/java_client_server.go`.

Focus areas: validation data shape, severity, notification handling, response
validation, id types, batch/limits, scalar acceptance, void returns, meta,
contract verification, namespace resolution, and transport.

Method: read every runtime source file and the generated-adapter sections of
the generator, then compared each documented claim in the focus areas to the
current code. Claims were confirmed against source (and, where the existing
runtime tests already cover them, against those tests). No source code was
modified; only this report was written. Bias applied throughout:
documentation should change to match the implementation unless the
implementation is clearly wrong, in which case the code defect is called out
separately.

## Summary (counts by severity)

- Critical: 0
- High: 0
- Medium: 2
- Low: 2
- Total confirmed doc/code mismatches: 4
- Test gaps: 5
- Verified as holding: 12 focus areas

## Confirmed issues

### doc-java-01 - `idl-guide/validation.md` claims validation `data` is a plain string

- Severity: medium
- Location: `docs/idl-guide/validation.md` ("Validation Errors" section,
  lines ~229-236)
- Doc claim: `data` is a plain string in the Go, Java and C# runtimes; only
  TypeScript and Python expose a structured validation result.
- Code reality: the Java runtime now emits a structured `ValidationResult` as
  `RPCError.data`. `Contract.validateRequest` throws
  `new RPCError(-32602, ..., vr.toMap())` and `Contract.validateResponse`
  throws `new RPCError(-32603, ..., vr.toMap())`
  (`Contract.java:170-176, 205-215`); `ValidationResult.toMap()` produces
  `{"valid": false, "error": "<joined>", "invalid_fields": [...]}`
  (`ValidationResult.java:44-52`). The generated Java server passes that map
  through as `data` (`java_client_server.go:1141, 1285`). The Go and C#
  runtimes emit the same shape.
- Recommendation: update the validation guide so the Java (and Go/C#) example
  shows the structured `valid`/`error`/`invalid_fields` object instead of a
  plain string, and drop the "plain string in Go, Java and C#" sentence.
  (Docs change.)

### doc-java-02 - Contract-verification example uses the wrong `Client` package

- Severity: medium
- Location: `docs/languages/java/reference.md` ("Contract Compatibility
  Verification" -> "Using Auditors")
- Doc claim: the example imports `com.bitmechanic.pulserpc.*` and then calls
  `new Client("http://localhost:8080", jsonParser)` with `.withAuditor(...)`
  and `.verifyOnBootstrap()`.
- Code reality: the runtime `Client` in `com.bitmechanic.pulserpc` is
  constructed as `Client(Transport, boolean, boolean)` (or via
  `Client.create(...)`) and fetches its IDL over a `Transport`
  (`Client.java:24-52`). The constructor `Client(String baseUrl,
  JsonParser jsonParser)` with auditor/verification methods belongs to the
  *generated* `Client` class, which lives in the generated base package
  (e.g. `com.example.myapp`), not in `pulserpc`
  (`java_client_server.go:1358-1395`). As written, the snippet does not
  compile against the runtime import.
- Recommendation: change the import to the generated base package (or show a
  fully-qualified generated `Client`) and keep the `pulserpc` import only for
  `ContractAuditor`/`VerificationResult`/`ContractDelta`. (Docs change.)

### doc-java-03 - Guide's `Contract` class documentation omits `meta`

- Severity: low
- Location: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` ("Runtime Library
  Components" -> "Contract Class", ~lines 234-260)
- Doc claim: `Contract` exposes only `interfaces`, `structs`, `enums`; no
  `meta` field is documented.
- Code reality: `Contract.java` declares `public Map<String, Object> meta`
  and populates it when parsing the Barrister list form (items whose
  `type == "meta"`) (`Contract.java:26-27, 115-121`). The PulseRPC dict
  form does not carry `meta` (it is not part of the D8 key set).
- Recommendation: document `meta` on `Contract` and state that it is populated
  from the Barrister list form only. (Docs change.)

### doc-java-04 - D1 wording can be read as rejecting `NaN`/`Infinity` for `float`

- Severity: low
- Location: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` D1 (~line 55)
- Doc claim: "Fractional numbers, NaN/Infinity, booleans, and out-of-range
  values are invalid in every runtime" (in the context of `int`).
- Code reality: `Validation.validateInt` rejects non-integral, non-finite and
  out-of-range values (`Validation.java:24-60`), but
  `Validation.validateFloat` accepts any `Number`, including `NaN` and
  `Infinity` (`Validation.java:62-72`), consistent with the IEEE-754-double
  statement (D26/R26). Python's `validate_float` behaves the same way.
- Recommendation: clarify in D1 that the rejection list applies to `int`
  only, and that `float` follows IEEE-754 (so `NaN`/`Infinity` pass runtime
  validation even though they are not representable in standard JSON).
  (Docs change.)

## Test gaps

- `tests/ServerTest.java` has no case for a malformed notification returning
  `null` (the runtime path that java-01 of the code audit covered).
- `tests/ValidationTest.java` has no assertion on the shape of validation
  `data` (`valid`/`error`/`invalid_fields`).
- No generated-client test covers the contract-verification import path or
  the `Client` constructor/auditor flow documented in `reference.md`.
- No test asserts that the generated Java server returns HTTP 200 with an
  empty body for a notification, nor the 405 status for a non-POST request,
  in the Java integration harness.
- No test covers `HttpLimits` boundary values (exactly 1 MiB request,
  exactly 100 batch entries) for the Java generated server.

## Verified as holding

- **Validation data shape (code side)** - Java now emits a structured
  `ValidationResult` (`valid`/`error`/`invalid_fields`) from both the runtime
  `Contract` and the generated server; the guide's plain-string example is the
  stale part (doc-java-01).
- **Severity** - `Error`/`Warning`/`Info` levels exist and `isCompatible()`
  is "no Error-level deltas"; `DiffEngine` is the source of truth
  (`ContractDelta.java`, `DiffEngine.java:587-646`).
- **Notification handling** - runtime `Server.call` computes
  `isNotification = (reqId == null)` up front and suppresses every error and
  success path (`Server.java:28-148`); the generated server wraps
  `handleJsonRpcRequest` and drops error envelopes for notifications, returning
  HTTP 200 with an empty body (`java_client_server.go:1070-1074, 1160-1168`).
- **Response validation** - runtime `Server` validates requests and responses
  when `validateRequests`/`validateResponses` are set and rejects a `null`
  result for a non-optional return (`Contract.validateResponse`,
  `Server.java:95-115`); the generated server always validates both
  (`java_client_server.go:1133-1149, 1275-1295`).
- **id types** - server rejects boolean/object/array ids with `-32600`
  (`Server.java:33-36`, `java_client_server.go:1188-1191`); numeric ids are
  compared by value in `Response.idsMatch` (`Response.java:78-88`);
  `id: null`/absent is a notification (M21).
- **Batch/limits** - request body 1 MiB, response body 10 MiB, batch entries
  100 (`HttpLimits.java`); generated server enforces all three and returns
  `-32600 "Request body too large"` / `"Batch exceeds maximum size"`
  (`java_client_server.go:1046-1058, 1080-1088`); empty batch returns a single
  `-32600` error object (M18).
- **Scalar acceptance** - `int` is signed 32-bit and rejects fractions,
  `NaN`/`Infinity`, booleans and out-of-range values (`Validation.validateInt`);
  `float` accepts any `Number` including `NaN`/`Infinity`
  (`Validation.validateFloat`); `bool` rejects `1`/`"true"`
  (`Validation.validateBool`).
- **Void returns** - `Contract.validateResponse` requires `null` when there is
  no `returnType` and rejects a non-null result (`Contract.java:195-200`).
- **meta** - `Contract` parses `meta` items in the Barrister list form
  (`Contract.java:115-121`); the PulseRPC dict form has no `meta` key (D8).
- **Contract verification** - generated `Client` fetches `pulserpc-idl`,
  diffs against the embedded IDL, reports an explicit incompatible result
  (with an `Error` message) when the fetch fails, and runs the configured
  auditor (`java_client_server.go:1388-1440`); severity is `Error`/`Warning`/
  `Info` per `DiffEngine`.
- **Namespace resolution** - `Types.findStruct`/`findEnum` try the exact key,
  then a qualified-name suffix match, then a base-name match over sorted keys
  (`Types.java:38-72`), matching D16.
- **Transport** - `HTTPTransport` accepts an optional header map, strips all
  trailing slashes from the base URL, applies a 10 s connect / 30 s request
  timeout, and bounds the response body at 10 MiB
  (`HTTPTransport.java:28-75`, `HttpLimits.java`).

## Unconfirmed hypotheses

- The generated standalone `Client` does not accept custom headers (it builds
  its own `HttpClient`), unlike `HTTPTransport`; the docs do not claim that it
  does, but users moving from `HTTPTransport` + interface clients to the
  standalone `Client` may expect parity. Severity low; not covered by tests.
- `Response.hasResult()` treats a non-null `result` as present even when the
  `resultPresent` flag is false (Gson bypasses `setResult`); the HTTP and
  in-process paths use `fromMap`/`setResult`, so this is likely benign, but
  direct Gson deserialization into `Response` of a null result may report no
  result. Not covered by tests.
- `Contract.findInterfaceName` and `Types.resolve` rely on `WeakHashMap`
  caches keyed by the definition map; a mutated map after construction could
  return a stale sorted key list. Construction-time warming in `Contract`
  mitigates this for the normal path.

## Notes

- The user-facing Java docs (`reference.md`, `quickstart.md`) are otherwise
  consistent with the current implementation for the focus areas; the main
  documentation defects are the stale validation-data-shape statement in
  `idl-guide/validation.md` (doc-java-01) and the wrong `Client` package in
  the contract-verification example (doc-java-02).
- Older audit documents under `docs/audit/` (e.g. `invariants.md`,
  `invariants-additional.md`, `problems-java.md`, `audit-java.md`) still
  describe pre-remediation Java behavior in places; those are historical
  audit artifacts, not the user-facing docs in scope here, and were not
  modified.
- No source files were modified by this audit; only this document was written.
EOF