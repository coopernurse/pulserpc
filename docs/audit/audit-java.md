# Audit: Java runtime

## Scope & methodology

Audited the Java runtime library (`pkg/runtime/runtimes/java/com/bitmechanic/pulserpc/`), the
generated Java server/client (`pkg/generator/java_client_server.go`), and the Java Makefile/test
wiring, against the invariants in `docs/audit/invariants.md`,
`docs/audit/invariants-additional.md`, and `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` (D1-D16).
The stale baseline `docs/audit/problems-java.md` was re-verified line-by-line against the current
tree; most of its findings have since been fixed (commits `8ddaebc`, `a81042e`, `13c7525`).

Method: read every runtime source file and the generated-server/client sections of the generator;
ran `mvn test` (101 tests, all passing) and inspected the generator audit tests
(`pkg/generator/java_audit_test.go`, `java_invariants_test.go`). Only findings with concrete
file:line evidence are reported below; suspicions that could not be confirmed are listed
separately.

## Summary (counts by severity)

- Critical: 0
- High: 0
- Medium: 2
- Low: 2
- Total confirmed: 4

## Confirmed issues

### java-01 — Runtime `Server` leaks error responses for notifications
- Severity: medium
- Invariant: M4 / M21 / R11 (notifications never receive a response, even for malformed input)
- Evidence: `pkg/runtime/runtimes/java/com/bitmechanic/pulserpc/Server.java:40-59` returns
  `errorResponse(...)` for a null request, a bad `id`, a wrong `jsonrpc` version, and a missing
  `method` without consulting `isNotification`; `:70-88` does the same for unknown interface/method.
  `isNotification` is only computed at `:68` and honored at `:146-148`. By contrast the generated
  server suppresses these via `handleJsonRpcRequest` (`java_client_server.go:1162-1168`), and the
  Go/Python/C# runtimes suppress every error path for notifications.
- Current behavior: a POST such as `{"jsonrpc":"2.0","method":"Svc.oneArg"}` (no `id`) with a bad
  or missing field yields a JSON-RPC error envelope instead of an empty body.
- Expected: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:902-904` — notifications receive no response,
  not even for malformed input.
- Suggested fix: compute `isNotification = (reqId == null)` before the early returns and route
  every error path through a helper that returns `null` when `isNotification` is true (mirror
  `respondError` in the Go/Python/C# runtimes).
- Test status: `tests/ServerTest.java:208-217` only covers a well-formed notification; no test sends
  a malformed notification.
- Acceptance criteria:
  - [ ] `server.call` with a notification that has a wrong/missing `jsonrpc` returns `null`.
  - [ ] `server.call` with a notification that has a missing/empty `method` returns `null`.
  - [ ] The handler is still invoked for a valid notification.

### java-02 — Validation errors carry a plain string as `data`, not `{field, message}`
- Severity: medium
- Invariant: M13 (`docs/idl-guide/validation.md:239-243`)
- Evidence: `Validation.java:19,44,52,62,82,100,110,115,142,148,154,171,193,236,240` throw
  `IllegalArgumentException` with free-text messages; `Contract.java:186-188,223-225` re-wraps them
  into another string; `Server.java:100-101,108` passes that string as `data` via `errorResponse`
  (`:158-171`). No Java code produces a structured object. Same pattern in the generated server
  (`java_client_server.go:1232,1265`).
- Current behavior: a missing required field yields `"data": "Function '...' invalid param '...':
  Missing required field 'email'..."` rather than `{"field":..., "message":...}`.
- Expected: documented `data` shape `{"field": "email", "message": "Required field 'email' is
  missing"}`.
- Suggested fix: introduce a `ValidationException`/structured error carrying field path + message
  and emit it as `data` from both runtime and generated server/client paths, while keeping the
  human-readable message.
- Test status: `tests/ValidationTest.java` asserts only `getMessage().contains(...)`; no test checks
  the `data` shape. (All other runtimes share this gap; not Java-specific in cause.)
- Acceptance criteria:
  - [ ] A missing required field error has `data` with `field` and `message` keys.
  - [ ] Existing message text remains human-readable.

### java-03 — Severity classification contradicts the breaking-change spec for removed fields
- Severity: low
- Invariant: M16 (`openspec/specs/contract-compatibility-verification/spec.md:115-118`)
- Evidence: `DiffEngine.java:587-620` (`classifySeverity`) returns `Info` for
  `(Field, Removed, ClientHasMore)`; `tests/SeverityTest.java:29-36` and
  `tests/DiffEngineTest.java:48-58` assert `Info`. The spec says removing a required field yields
  `Compatible=false` with an `Error`-level delta.
- Current behavior: a client that has a field the server removed is still reported compatible.
- Expected: either the spec or the implementation must change; if the spec wins, `classifySeverity`
  must return `Error` and the tests updated.
- Suggested fix: decide the intended severity once (all runtimes currently agree on `Info`), then
  either amend the spec or change `classifySeverity` + `SeverityTest`/`DiffEngineTest`.
- Test status: asserting tests exist and pass, but encode the non-spec behavior.
- Acceptance criteria:
  - [ ] Removing a required field produces an `Error` delta and `compatible=false` (if spec wins), or
  - [ ] The spec is amended to match the implementation.

### java-04 — Generated interface client leaks exception text in `error.data`
- Severity: low
- Invariant: R15 / D4 / S6 (internal error details redacted from clients)
- Evidence: `pkg/generator/java_client_server.go:592` emits
  `throw new RPCError(-32603, "Internal error", e.getMessage());` in the generated interface
  client's catch block. The runtime server and generated server redact via `InternalErrors.log`
  (`InternalErrors.java:23-30`; `java_client_server.go:1083,1288,1305,1312`), but this client path
  does not.
- Current behavior: a transport/serialization failure on the client side surfaces the underlying
  exception message as `RPCError.data`.
- Expected: `InternalErrors`-style redaction — fixed message, no `data`.
- Suggested fix: change the emitted line to
  `throw new RPCError(-32603, "Internal error", null);` (and log via `System.err` if desired).
- Test status: no generated-client unit test; `tests/integration/test_generator_java.sh` exercises
  the happy path only.
- Acceptance criteria:
  - [ ] The generated interface client throws `RPCError(-32603, "Internal error")` with null `data`
    on a non-RPC exception.
  - [ ] The exception message does not appear in `data`.

## Test gaps

- `tests/ServerTest.java` has no case for a malformed notification returning `null` (java-01).
- `tests/ValidationTest.java` has no assertion on the shape of validation `data` (java-02).
- No generated-client test covers the internal-error redaction path or non-object return types.
- `tests/integration/test_http_api.sh` still treats an unexpected error code as a warning in a few
  paths; Java is now in the `test_all_runtimes.sh` loop but coverage of notification/parse-error
  cases for Java is limited.

## Verified as holding

- R1, R2, R3, R4 (code + new `testValidateStruct`/`testValidateType` coverage), R5, R6, R7, R8,
  R10, R12, R13, R14; R11 for the generated server (runtime gap is java-01).
- R15, R16, R17, R18, R19, R20, R22, R23, R24, R25, R26, R27, R28, R29 (runtime transport),
  R30, R31, R32 for Java.
- M1, M2, M3, M5, M6 (generated), M7, M8, M9, M10, M11, M12, M14, M15 (generated), M17, M18,
  M19, M20, M22, M23, M24, M25, M26, M27, M28.
- C1, C2, C4, C5, C8, C9, C10, C11, C12, C13.
- D1-D16 (notably D3 405, D4 redaction on server, D5 limits, D6 30s/10s, D7 boxed optionals,
  D8 uniform IDL JSON, D9 checksum, D12 determinism, D13 trailing slash, D15 contract verification).
- Makefile now runs all 13 Java test classes (`pkg/runtime/runtimes/java/Makefile:7`); root
  `Makefile` wires `test-runtime-java`, `test-generator-java`, `test-java-compile`.

## Unconfirmed hypotheses

- `pkg/generator/java_client_server.go:1522` casts the low-level `Client.call` result to
  `Map<String, Object>`; a method returning a scalar or array would raise `ClassCastException`.
  The interface clients do not use this class, so impact is unclear.
- The generated standalone `Client` does not accept custom headers, unlike `HTTPTransport`; it is
  unclear whether callers rely on this.
- `Response.hasResult()` (`Response.java:37-41`) treats `result != null` as present for
  parser-populated objects; direct Gson deserialization into `Response` of a null result may report
  no result. HTTP/InProc paths use `fromMap`/`setResult`, so this is likely benign.

## Notes

- `docs/audit/problems-java.md` is a stale baseline: its java-01..java-10 and java-12..java-15
  findings are resolved in the current tree; java-11 and java-16 remain (re-reported above as
  java-02 and java-03).
- No source code was modified by this audit; only this document was written.