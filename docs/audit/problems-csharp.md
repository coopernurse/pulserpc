# C# — audit findings

## Summary
- Invariants checked: 31 (all of Part 2 R1–R14 and Part 3 M1–M17)
- Confirmed violations: 15
- Test gaps: 14

## Problems

### csharp-01 — Contract verification compares the embedded IDL to itself and never contacts the server
- Severity: high
- Invariant: M15
- Current behavior: In the runtime client, `clientIDL` defaults to `Contract.GetIDLParsed()` (`pkg/runtime/runtimes/csharp/PulseRPC/Client.cs:316-319`) and `serverIDL` is the very same `Contract.GetIDLParsed()` (`Client.cs:325`), populated once from the bootstrap `pulserpc-idl` response (`Client.cs:244`). `DiffEngine.DiffIDL(clientIDL, serverIDL)` (`Client.cs:330`) therefore diffs an object against itself unless `SetLocalIDL` was called; both checksums come from the same document (`Client.cs:346-347`). In generated clients it is worse: `clientIDL` is the embedded IDL (`pkg/generator/csharp_client_server.go:1539`), and `idlObj` is the *same* embedded IDL (`csharp_client_server.go:1541`) used as the "server" side at `csharp_client_server.go:1542`. `VerifyCompatibility` issues no transport request at all.
- Expected behavior: Verification must compare a local contract against the *running server's* contract (`openspec/specs/contract-compatibility-verification/spec.md:117-118`; `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:540-544`); catalog M15 (`docs/audit/invariants.md:89`).
- Test status: `pkg/runtime/runtimes/csharp/tests/DiffEngineTests.cs:11-233` and `AuditorTests.cs:8-188` test the diff engine and auditors in isolation; no test calls `Client.VerifyCompatibility` or the generated client's method, so self-comparison is invisible.
- Suggested fix: Fetch the server IDL via a fresh `pulserpc-idl` call inside `VerifyCompatibility` and diff it against the local (embedded/`SetLocalIDL`) IDL; make `ServerChecksum` come from that fetched document.
- Acceptance criteria:
  - [ ] With a divergent `SetLocalIDL`, `VerifyCompatibility()` returns `Compatible=false` and at least one `Severity.Error` delta.
  - [ ] Changing the server IDL after bootstrap is detected by a second `VerifyCompatibility()` call.
  - [ ] The generated client's `VerifyCompatibility()` issues a `pulserpc-idl` request over its transport.

### csharp-02 — Zero-parameter requests skip IDL validation and invoke handlers with default values
- Severity: high
- Invariant: M9
- Current behavior: `pkg/runtime/runtimes/csharp/PulseRPC/Server.cs:117` only validates when `positionalParams.Count > 0`. A request with no `params` for a method that declares required parameters bypasses `Contract.ValidateRequest` (`Server.cs:121`); `InvokeHandler` then fills each missing argument with `parameters[i].DefaultValue` (`Server.cs:311-340`, default at `:339`) and returns a success response. `Contract.ValidateRequest` itself already enforces exact count (`Contract.cs:222-226`).
- Expected behavior: catalog M9 (`docs/audit/invariants.md:83`); guide requires validating requests (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:419-423`).
- Test status: none — there is no `Server` unit test file. `tests/integration/test_http_api.sh:211-212` covers "missing one of two" params but never "missing all" params.
- Suggested fix: Remove the `positionalParams.Count > 0` guard so count-0 requests are validated; keep validation enabled for genuinely zero-argument methods (it will pass).
- Acceptance criteria:
  - [ ] A request with no `params` to a two-required-param method returns `-32602`.
  - [ ] A request with no `params` to a zero-param method still returns a normal result.

### csharp-03 — Notifications receive responses (id:null)
- Severity: medium
- Invariant: M4, R11
- Current behavior: Runtime `Server.cs:62-66` returns `SuccessResponse` for `pulserpc-idl` unconditionally; `isNotification` is not computed until `Server.cs:69-70`. `ErrorResponse` always adds an `"id"` key even when null (`Server.cs:199-208`), and error paths return it for requests without an id (`Server.cs:51,58,75,81,96,125,144,150,154,158,170`). Generated server: `pulserpc-idl` notifications are handled correctly (`pkg/generator/csharp_client_server.go:1029`), but `ErrorResponse` always emits `"id"` (`csharp_client_server.go:1067-1076`, id at `:1075`) for failed notifications (`:1049,1059,1136,1238,1291,1296,1392`). Success responses are suppressed (`Server.cs:176-179`; generator `:1397`).
- Expected behavior: "Notifications: Requests without `id` field (no response sent)" (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:814`); catalog R11/M4 (`docs/audit/invariants.md:66,78`). This corrects the catalog's claim that C# suppresses correctly.
- Test status: none; `tests/integration/README.md:187-188` still lists notification testing as future work.
- Suggested fix: After parsing `id`, return `null` immediately for notifications on every error path (including `pulserpc-idl`), and omit the `id` member when null.
- Acceptance criteria:
  - [ ] A notification with an unknown method produces no HTTP body (204) in both runtime and generated server.
  - [ ] A `pulserpc-idl` request without `id` produces no response in the runtime `Server.Call`.

### csharp-04 — Generated clients never validate responses; runtime guards skip null results
- Severity: medium
- Invariant: M14, R10
- Current behavior: Generated client methods only call `ITransport.CallAsync` and deserialize the result (`pkg/generator/csharp_client_server.go:1641-1674`) — there is no `Contract.ValidateResponse` call. The runtime client validates only under `result != null` (`pkg/runtime/runtimes/csharp/PulseRPC/Client.cs:200`), and the runtime server under the same guard (`Server.cs:162`), so a null value for a required (non-optional) return type is never rejected. The generated server does validate nulls correctly (`csharp_client_server.go:1386`), so the two halves disagree.
- Expected behavior: "Response validation: Validate response before returning (using Contract)" (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:543-544`); catalog M14 (`docs/audit/invariants.md:88`).
- Test status: none — no `Client` or `Server` test file exists.
- Suggested fix: Call `Contract.ValidateResponse` (or route through the runtime `Client`) in generated client methods, and remove `result != null` guards so `Validation.ValidateType` enforces non-optional nulls (`Validation.cs:208-217`).
- Acceptance criteria:
  - [ ] A handler returning null for a required return type yields an error response, not `"result": null`.
  - [ ] A generated client receiving a wrong-typed result throws a validation error.

### csharp-05 — Object-form params are coerced to an empty list (generated) or silently dropped (runtime)
- Severity: medium
- Invariant: M8
- Current behavior: Generated server: `var paramsList = paramsObj as System.Collections.IList ?? new List<object>();` (`pkg/generator/csharp_client_server.go:1140`) turns named/object params into an empty list, which then fails the count check at `csharp_client_server.go:1143-1146` with `-32602`. Runtime server only handles `IList` or null (`Server.cs:102-114`) and silently discards any other shape, after which validation is skipped (see csharp-02) and defaults are used.
- Expected behavior: JSON-RPC 2.0 allows structured (by-name) params; guide request format (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:811`); catalog M8 (`docs/audit/invariants.md:82`).
- Test status: none; `tests/integration/test_http_api.sh:162-188` only sends arrays or `null` params.
- Suggested fix: Normalize `IDictionary<string,object?>` params into the positional list using the IDL parameter names, rejecting unknown names; share the same normalization in runtime and generated servers.
- Acceptance criteria:
  - [ ] `A.add` with `params: {"a":5,"b":3}` returns the same result as `params: [5,3]`.
  - [ ] Runtime and generated servers return identical errors for unknown param names.

### csharp-06 — Generated server accepts an empty method string and returns the wrong error code
- Severity: low
- Invariant: M2
- Current behavior: Generated server checks only `method == null` (`pkg/generator/csharp_client_server.go:1010-1015`); an empty string reaches `method.Split('.', 2)` (`csharp_client_server.go:1045`) and yields `-32601 "Invalid method format"` (`:1049`). The runtime server uses `string.IsNullOrEmpty(method)` and returns `-32600` (`pkg/runtime/runtimes/csharp/PulseRPC/Server.cs:56-60`).
- Expected behavior: Method must be a non-empty string → `-32600` (catalog M2, `docs/audit/invariants.md:76`).
- Test status: none; `tests/integration/test_http_api.sh:222-232` checks a missing `jsonrpc` field but not an empty method.
- Suggested fix: Use `string.IsNullOrEmpty(method)` in the generated server for the `-32600` branch.
- Acceptance criteria:
  - [ ] `{"jsonrpc":"2.0","method":"","id":1}` returns `-32600` in both runtime and generated servers.

### csharp-07 — Qualified struct type names do not resolve
- Severity: medium
- Invariant: R6
- Current behavior: `Types.FindStruct` is exact-match only (`pkg/runtime/runtimes/csharp/PulseRPC/Types.cs:15-18`) and `Validation.ValidateType` uses it directly (`Validation.cs:269`, error at `:296`), so a `userDefined` value of `inc.Address` fails even when `Address` is in `Structs`. `Types.FindEnum` does have a base-name fallback (`Types.cs:23-38`), so enums and structs behave differently.
- Expected behavior: "Qualified names: Handle qualified type names (e.g., `inc.Response`)" (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:859`); catalog R6 (`docs/audit/invariants.md:61`).
- Test status: `pkg/runtime/runtimes/csharp/tests/TypesTests.cs:9-42` and `ValidationTests.cs:142-308` use only unqualified names; no namespace/qualified-name test exists in any runtime.
- Suggested fix: Give `FindStruct` the same qualified-then-base-name lookup as `FindEnum`, with a deterministic rule when two namespaces define the same base name.
- Acceptance criteria:
  - [ ] A struct field typed `ns.User` validates against struct `User`.
  - [ ] Ambiguous base names resolve deterministically (or are reported as an error).

### csharp-08 — `VerifyOnBootstrap()` in generated clients is inert
- Severity: medium
- Invariant: R13
- Current behavior: The generated client declares `_verifyOnBootstrap` (`pkg/generator/csharp_client_server.go:1505`) and `VerifyOnBootstrap()` sets it (`:1519-1523`), but no code ever reads the field; the constructor only assigns `_transport` (`:1508-1511`). The runtime client does honor the option (`pkg/runtime/runtimes/csharp/PulseRPC/Client.cs:127-130`).
- Expected behavior: Catalog R13 (`docs/audit/invariants.md:68`) and the compatibility-verification spec's bootstrap scenario (`openspec/specs/contract-compatibility-verification/spec.md:113-118`).
- Test status: There is no end-to-end test that constructs a client with `VerifyOnBootstrap()` and a `FailFastAuditor`; `AuditorTests.cs` only invokes auditors directly.
- Suggested fix: Invoke `VerifyCompatibility()` from the generated client's bootstrap path (or remove the API), and add a test that constructs a client against a deliberately divergent server IDL.
- Acceptance criteria:
  - [ ] A generated client configured with `VerifyOnBootstrap()` + `FailFastAuditor` fails when the server IDL differs.
  - [ ] A test calls `Client.VerifyCompatibility()` through a real `Contract` rather than synthetic deltas.

### csharp-09 — Two parallel transport hierarchies with inconsistent request IDs; client-generated IDs are discarded
- Severity: low
- Invariant: R9
- Current behavior: `ITransport.CallAsync(string, object[])` (`pkg/runtime/runtimes/csharp/PulseRPC/Transport.cs:12-18`) and abstract `Transport.Request(string, object?)` (`Transport.cs:23-29`) are unrelated; the runtime client uses the latter (`Client.cs:97`) while generated clients use the former (`pkg/generator/csharp_client_server.go:1503`, `:1641`). IDs differ: GUID string in `HttpTransport` (`Transport.cs:64`), hard-coded `1` in `InProcTransport` (`Transport.cs:143`), and an incrementing int in the client (`Client.cs:100,173-174`). `Client.Call` builds a request with that id (`Client.cs:176-181`) but never sends it — only `method` and `@params` reach the transport (`Client.cs:188`).
- Expected behavior: Catalog R9 (`docs/audit/invariants.md:64`); a single transport contract with caller-controlled IDs enables response id matching (guide `:811-815`).
- Test status: none — no transport tests exist.
- Suggested fix: Collapse onto one transport interface; let the client own ID generation and pass the id to the transport; use one ID type across all transports.
- Acceptance criteria:
  - [ ] Runtime and generated clients use the same transport abstraction.
  - [ ] The id sent in the request is the id checked in the response.

### csharp-10 — Generated server coerces integer enum values to enum names before validation
- Severity: medium
- Invariant: M10, M11
- Current behavior: For enum-typed params, the generated server maps an integer to `enumValues[int]`'s name and then validates that name (`pkg/generator/csharp_client_server.go:1200-1224`; JSON-element variant `:1173-1197`); `ConvertEnumIntsToStrings` does the same for struct fields (`:964-977`). This makes integer wire values pass `Validation.ValidateEnum`, which otherwise requires a string (`pkg/runtime/runtimes/csharp/PulseRPC/Validation.cs:121-124`).
- Expected behavior: Enum values are string names on the wire (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:135,229-233`); catalog M11 (`docs/audit/invariants.md:85`).
- Test status: `tests/integration/test_http_api.sh:219-220` exercises invalid *string* enum values only; no integer-enum test exists.
- Suggested fix: Delete the integer→name coercion and feed the raw value to `Validation.ValidateType`, letting non-string enum inputs fail with `-32602`.
- Acceptance criteria:
  - [ ] An enum parameter supplied as `0` returns `-32602`.
  - [ ] Only declared enum names are accepted for an enum field or parameter.

### csharp-11 — Divergent scalar acceptance sets: float rejects `long`, int accepts whole doubles
- Severity: low
- Invariant: R2
- Current behavior: `Validation.ValidateFloat` accepts only `float`, `double`, or `int` and rejects `long` (`pkg/runtime/runtimes/csharp/PulseRPC/Validation.cs:48-54`); `ValidateInt` accepts `int`, whole `double`, and whole `long` (`:27-42`). The generated JSON converter returns `long` for large integers (`pkg/generator/csharp_client_server.go:875-879`), so large integral values fail float validation but pass int validation. No `NaN`/`Infinity` rejection exists (`:50`).
- Expected behavior: "Scalar acceptance sets should be identical" (catalog R2, `docs/audit/invariants.md:57`).
- Test status: `pkg/runtime/runtimes/csharp/tests/ValidationTests.cs:24-52` covers only small ints/doubles/strings/null; no `long`, `NaN`, or `Infinity` cases.
- Suggested fix: Harmonize the acceptance matrix across runtimes (at minimum accept `long` for float) and encode it as a cross-runtime test matrix.
- Acceptance criteria:
  - [ ] `ValidateFloat(5L)` is accepted (or its rejection is documented and identical in every runtime).
  - [ ] A test covers `int`, `long`, `double`, `float`, `bool`, `NaN`, and `Infinity` for all four scalars.

### csharp-12 — Client does not validate the response envelope (id match, result XOR error)
- Severity: medium
- Invariant: M5
- Current behavior: `Client.Call` checks `"error"` first and throws, so a response containing both `error` and `result` silently ignores `result` (`pkg/runtime/runtimes/csharp/PulseRPC/Client.cs:190-198`); it never compares the response `id` to the request id (and the generated id is discarded, see csharp-09), and never checks `jsonrpc` or result/error exclusivity. Generated clients only test for the presence of `"result"` (`pkg/generator/csharp_client_server.go:1642-1648`).
- Expected behavior: guide response format and compliance requirement (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:810-815`); catalog M5 (`docs/audit/invariants.md:79`).
- Test status: none.
- Suggested fix: Validate `jsonrpc=="2.0"`, id equality, and exactly one of `result`/`error` before deserializing or throwing.
- Acceptance criteria:
  - [ ] A response with a mismatched id throws.
  - [ ] A response containing both `error` and `result` is rejected instead of silently returning the error.

### csharp-13 — Validation error `data` is a plain string, not the documented `{field, message}` object
- Severity: low
- Invariant: M13
- Current behavior: `Validation` throws `ArgumentException` with message strings (`pkg/runtime/runtimes/csharp/PulseRPC/Validation.cs:161,171,190,248,290`); the runtime server passes `ex.Message` directly as `data` (`Server.cs:125,154,170-171`) and the generated server does the same (`pkg/generator/csharp_client_server.go:1238,1392`).
- Expected behavior: `docs/idl-guide/validation.md:240-243` documents `data` as `{"field": ..., "message": ...}`; catalog M13 (`docs/audit/invariants.md:87`).
- Test status: none.
- Suggested fix: Introduce a structured validation-error type carrying `field`/`message` and emit it as `data` for field- and parameter-level failures.
- Acceptance criteria:
  - [ ] A missing required field returns `data.field` and `data.message`.
  - [ ] A parameter type error returns structured data with the parameter name.

### csharp-14 — Integration tests record wrong error codes as WARN/PASS and omit batch/notifications; no C# Server/Client unit tests
- Severity: medium
- Invariant: M17, M1, M7
- Current behavior: `tests/integration/test_http_api.sh:129-141` prints a WARN on a wrong error code but still executes `TESTS_PASSED++` (`:140-141`); the missing-`jsonrpc` check only requires the presence of `"error"` (`:222-232`), with no `-32600` assertion. No batch or notification cases exist (`tests/integration/README.md:187-188` lists them as TODO), and the C# test project contains only `ValidationTests.cs`, `RPCTests.cs`, `TypesTests.cs`, `AuditorTests.cs`, `DiffEngineTests.cs`, `SeverityTests.cs` — no tests for `Server.Call`, `Client.Call`, `Contract`, or transports.
- Expected behavior: catalog M17/M1/M7 (`docs/audit/invariants.md:75,89,91`); guide recommends batch and notification integration tests (`:853-854`).
- Test status: the test suite itself is the gap; `make test-runtime-csharp` (`Makefile:126-128`) runs `dotnet test` on exactly those six files.
- Suggested fix: Turn wrong-code warnings into `TESTS_FAILED`, assert standard codes for malformed requests, add batch/notification cases to the C# server run, and add C# unit tests for `Server.Call` and `Client.Call` with a mock transport.
- Acceptance criteria:
  - [ ] A deliberately wrong error code makes the integration script exit non-zero.
  - [ ] Batch and notification requests are exercised against the C# server.
  - [ ] `Server.Call`/`Client.Call` unit tests exist and run under `make test-runtime-csharp`.

### csharp-15 — Field-removal severity contradicts the spec's own scenario
- Severity: low
- Invariant: M16
- Current behavior: `DiffEngine.ClassifySeverity` returns `Info` for `Field`/`Removed`/`ClientHasMore` (`pkg/runtime/runtimes/csharp/PulseRPC/DiffEngine.cs:708-712`), and tests assert that Info (`pkg/runtime/runtimes/csharp/tests/DiffEngineTests.cs:100-129`, `SeverityTests.cs:28-32`).
- Expected behavior: The spec scenario requires `Compatible=false` with an `Error` delta when a required field is removed (`openspec/specs/contract-compatibility-verification/spec.md:117-118`), while its own severity table says Info (`:25`); catalog M16 (`docs/audit/invariants.md:90`).
- Test status: tests exist but encode the Info interpretation, so they cannot catch the contradiction.
- Suggested fix: Reconcile the spec table and scenario, then adjust `ClassifySeverity` and the tests to the agreed behavior.
- Acceptance criteria:
  - [ ] The spec table and scenario agree on field-removal severity.
  - [ ] C# classifies a removed required field per the reconciled spec.

## Verified-correct invariants
- R1 — `Validation.ValidateType`/`ValidateStruct` and `Types.FindStruct`/`FindEnum`/`GetStructFields` all exist (`Validation.cs:135,200`; `Types.cs:15,23,43`).
- R3 — C# is fail-fast: thrown `ArgumentException`s are wrapped with context (`Validation.cs:81-90,103-113,188-191`).
- R4 — Required/optional/null rules enforced and tested (`Validation.cs:151-193`; `tests/ValidationTests.cs:181-252`).
- R5 — Parent fields resolved first, child overrides (`Types.cs:53-85`; `tests/TypesTests.cs:125-178`).
- R7 — `RPCError` carries code/message/data (`RPCError.cs:13-34`; `tests/RPCTests.cs:9-35`).
- R10 (partial) — Runtime and generated servers do validate responses (`Server.cs:161-173`; `csharp_client_server.go:1299-1393`); gaps are recorded in csharp-04.
- R12 — `meta` is parsed only in the legacy Barrister-list branch (`Contract.cs:163-172`); the modern embedded IDL has no `meta` key (`pkg/parser/idl.go:8-15`), so there is no practical C# gap.
- M3 — `Interface.method` format produces `-32601` (`Server.cs:73-76`; `csharp_client_server.go:1045-1050`; `tests/integration/test_http_api.sh:235-237`).
- M6 — Parse error `-32700` is emitted by the generated HTTP adapter (`csharp_client_server.go:812-816`).
- M7 (feature) — Batch requests are handled by the generated server (`csharp_client_server.go:818-836`); test gap recorded in csharp-14.
- M10/M11 (partial) — Per-parameter and enum-membership validation are present (`Contract.cs:228-250`; `csharp_client_server.go:1149-1239`; `Validation.cs:119-130`); divergences recorded in csharp-10.
- M12 — Unknown/extra fields are intentionally ignored (`Validation.cs:151-194` iterates known fields only).
