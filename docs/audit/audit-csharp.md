# Audit: C# runtime

## Scope & methodology

Audited the C# runtime library (`pkg/runtime/runtimes/csharp/PulseRPC/`), the generated
C# server/client (`pkg/generator/csharp_client_server.go`), and the C# Makefile/test
wiring, against the invariants in `docs/audit/invariants.md`,
`docs/audit/invariants-additional.md`, and `docs/RUNTIME_IMPLEMENTATION_GUIDE.md`
(D1-D16). The stale baseline `docs/audit/problems-csharp.md` was re-verified
line-by-line against the current tree; most of its findings have since been fixed
(runtime M9/M8/M4/R15/M15/R6/R10/R2 limits and generated-server D5/R22/R24/M18-M28).

Method: read every runtime source file and the generated-server/client sections of the
generator; ran `dotnet test tests/PulseRPC.Tests.csproj` (119 tests, all passing) and
inspected the generator audit tests (`pkg/generator/csharp_audit_test.go`,
`csharp_invariants_test.go`, `csharp_namespace_paths_test.go`). Only findings with
concrete file:line evidence are reported below; suspicions that could not be
confirmed are listed separately.

No source code was modified by this audit; only this document was written.

## Summary (counts by severity)

- Critical: 0
- High: 0
- Medium: 4
- Low: 5
- Total confirmed: 9

## Confirmed issues

### csharp-01 — Generated client neither validates responses nor checks the response envelope
- Severity: medium
- Invariant: M5, M14 (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:895-900`)
- Evidence: `pkg/generator/csharp_client_server.go:1863` (`var response = await _transport.CallAsync(method, parameters);`),
  `:1866-1881` (error handling only), `:1882-1888` (result extraction without envelope checks).
  The generated method never calls `Client.ValidateEnvelope` and there is no `Contract`
  instance in the generated client, so a wrong-typed or malformed result is deserialized
  unchecked.
- Current behavior: a generated client accepts any response that contains `"result"` and
  deserializes it, ignoring `jsonrpc`, id mismatch, and result/error exclusivity.
- Expected: guide response-envelope rules (`jsonrpc=="2.0"`, id match, exactly one of
  `result`/`error`); response validation against the IDL return type.
- Suggested fix: route generated client calls through the runtime `Client` (which already
  validates the envelope and supports `validateResponse`), or add an inline `ValidateEnvelope`
  call plus `Contract.ValidateResponse` using `IdlHolder`.
- Test status: no generated-client unit test covers this; runtime `ClientTests.cs` covers the
  runtime path only.
- Acceptance criteria:
  - [ ] A generated client receiving a response with a mismatched id throws.
  - [ ] A generated client receiving a wrong-typed result throws a validation error.

### csharp-02 — Generated client `VerifyOnBootstrap()` is inert
- Severity: medium
- Invariant: R13, D15
- Evidence: `pkg/generator/csharp_client_server.go:1676-1677` (`private bool _verifyOnBootstrap;`),
  `:1692-1696` (`VerifyOnBootstrap()` sets it), `:1681-1684` (constructor only assigns
  `_transport` and never invokes `VerifyCompatibility()`). Contrast runtime
  `pkg/runtime/runtimes/csharp/PulseRPC/Client.cs:126-131`.
- Current behavior: configuring a generated client with `VerifyOnBootstrap()` + `FailFastAuditor`
  has no effect; no verification request is issued.
- Expected: compatibility verification runs during construction when requested
  (`openspec/specs/contract-compatibility-verification/spec.md:113-118`).
- Suggested fix: invoke `VerifyCompatibility()` from the constructor when `_verifyOnBootstrap`
  is set, mirroring the runtime client.
- Test status: no test constructs a generated client with `VerifyOnBootstrap()`.
- Acceptance criteria:
  - [ ] A generated client configured with `VerifyOnBootstrap()` + `FailFastAuditor` fails when
    the server IDL differs.
  - [ ] A generated client without the option behaves as before.

### csharp-03 — Generated client caches `_serverIDL`, so a changed server contract is not detected
- Severity: medium
- Invariant: D15, M15
- Evidence: `pkg/generator/csharp_client_server.go:1703` (`private object? _serverIDL;`),
  `:1724` (`if (_serverIDL == null)`), `:1763` (`var idlObj = _serverIDL;`). The fetch runs
  only on the first `VerifyCompatibility()` call; the runtime client
  (`pkg/runtime/runtimes/csharp/PulseRPC/Client.cs:319-348`) fetches on every call.
- Current behavior: after the first verification, `_serverIDL` is populated and a subsequent
  `VerifyCompatibility()` diffs against the stale document, so a server that changed its IDL
  after bootstrap is reported compatible.
- Expected: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:540-544` — verification fetches the running
  server's contract each time.
- Suggested fix: do not cache `_serverIDL`; always fetch via `pulserpc-idl` (or invalidate the
  cache on every call), matching the runtime client.
- Test status: `ClientTests.cs` covers the runtime path (`VerifyCompatibility_ServerChangedAfterBootstrap_IsDetectedOnSecondCall`);
  no generated-client equivalent.
- Acceptance criteria:
  - [ ] A second `VerifyCompatibility()` after the server IDL changes returns `Compatible=false`
    with an `Error` delta.

### csharp-04 — Generated server coerces integer enum values to enum names before validation
- Severity: medium
- Invariant: M10, M11 (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:135,229-233`)
- Evidence: `pkg/generator/csharp_client_server.go:1332-1399` (request parameter path) and
  `:1487-1491`, `:1522` (response path) map an integer to `enumValues[int]`'s name before
  calling `Validation.ValidateType`, which otherwise requires a string
  (`pkg/runtime/runtimes/csharp/PulseRPC/Validation.cs:142-145`).
- Current behavior: an enum parameter or field supplied as `0` is accepted and rewritten to the
  first declared enum name, bypassing wire-type validation.
- Expected: enums are string names on the wire; non-string enum inputs fail with `-32602`.
- Suggested fix: delete the integer-to-name coercion and feed the raw value to
  `Validation.ValidateType`.
- Test status: no test sends an integer enum value; `tests/integration/test_http_api.sh:219-220`
  exercises invalid string enums only.
- Acceptance criteria:
  - [ ] An enum parameter supplied as `0` returns `-32602`.
  - [ ] Only declared enum names are accepted for an enum field or parameter.

### csharp-05 — Empty method string is not rejected as `-32600` in the generated server
- Severity: low
- Invariant: M2
- Evidence: `pkg/generator/csharp_client_server.go:1123-1128` checks only `method == null`
  (`ExtractStringValue` returns `null` only for non-string/empty-of-JSON-string inputs); an
  empty string reaches `method.Split('.', 2)` at `:1045`-equivalent and yields `-32601`.
  Runtime server uses `string.IsNullOrEmpty(method)` and returns `-32600`
  (`pkg/runtime/runtimes/csharp/PulseRPC/Server.cs:91`).
- Current behavior: `{"jsonrpc":"2.0","method":"","id":1}` returns `-32601` from a generated
  server but `-32600` from the runtime server.
- Expected: method must be a non-empty string -> `-32600` (catalog M2).
- Suggested fix: use `string.IsNullOrEmpty(method)` in the generated server's method check.
- Test status: none; integration script checks a missing `jsonrpc` field but not an empty method.
- Acceptance criteria:
  - [ ] `{"jsonrpc":"2.0","method":"","id":1}` returns `-32600` in both runtime and generated
    servers.

### csharp-06 — Shared generated files declare `namespace PulseRPC` even when `-package` is set
- Severity: low
- Invariant: C8 (generated layout), package-output layout
- Evidence: `pkg/generator/csharp_client_server.go:659` (`generateContractCs`), `:725`
  (`generateServerCs`), `:1639` (`generateClientCs`) all emit a hard-coded
  `namespace PulseRPC`, while per-namespace files use the qualified package prefix
  (`:249`, `:2540`-`:2581`) and the runtime is rewritten to `<package>.PulseRPC`
  (`:97-101`). These three files are also always written to the output root
  (`:108`, `:169`, `:177`).
- Current behavior: with `-package MyApp.Lib.Rpc`, the runtime lives under
  `MyApp.Lib.Rpc.PulseRPC` but the shared `Contract`/`Server`/`Client` types remain in the
  global `PulseRPC` namespace, so the two halves of a generated project do not share one
  package root.
- Expected: either the shared files move under the package namespace or are documented as a
  global compatibility shim.
- Suggested fix: make the shared files' namespace follow `paths.GetNamespaceImportPrefix()`
  (or `GetRuntimeImport()`'s parent) when a package base is set, and emit them under the
  package directory.
- Test status: `TestCSharpMultiFileEndToEnd` asserts the current (global-namespace) layout;
  no test asserts a single package-root namespace.
- Acceptance criteria:
  - [ ] With `-package MyApp.Lib.Rpc`, shared `Contract.cs`/`Server.cs`/`Client.cs` declare
    `namespace MyApp.Lib.Rpc` (or a documented equivalent) and live under that path.
  - [ ] Existing no-package single-namespace output is unchanged.

### csharp-07 — Runtime `Types.FindEnum` does not implement D16's sorted fallback
- Severity: low
- Invariant: D16, R6
- Evidence: `pkg/runtime/runtimes/csharp/PulseRPC/Types.cs:52-67` (`FindEnum`) tries the exact
  key then the unqualified base name, but never the "first key in sorted order that shares
  the base name" fallback. `FindStruct` (`:15-47`) does implement that fallback for qualified
  references. Guide: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:918-922`.
- Current behavior: a qualified enum reference such as `inc.Color` resolves only if `Color` or
  `inc.Color` is registered exactly; an ambiguous base name does not resolve deterministically
  the way structs do.
- Expected: exact key, then exact base-name key, then first sorted key sharing the base name,
  for both structs and enums.
- Suggested fix: mirror `FindStruct`'s qualified-then-base-then-sorted lookup in `FindEnum`.
- Test status: `tests/TypesTests.cs:9-42` and `tests/ValidationTests.cs:142-308` use only
  unqualified names; no namespace/qualified-name test exists.
- Acceptance criteria:
  - [ ] A field typed `ns.Color` validates against enum `Color`.
  - [ ] Two enums sharing a base name resolve to the first in sorted order.

### csharp-08 — Validation error `data` is a plain string, not the documented `{field, message}` object
- Severity: low
- Invariant: M13 (`docs/idl-guide/validation.md:239-243`)
- Evidence: `pkg/runtime/runtimes/csharp/PulseRPC/Validation.cs:161,171,182,192,248,290`
  throw `ArgumentException` with message strings; `pkg/runtime/runtimes/csharp/PulseRPC/Server.cs:125,154,170-171`
  pass `ex.Message` straight into `data`; the generated server does the same
  (`pkg/generator/csharp_client_server.go:1407`). No C# code emits a structured object.
  (All runtimes share this gap; not C#-specific in cause.)
- Current behavior: a missing required field yields `"data": "Missing required field 'email'..."`
  rather than `{"field":..., "message":...}`.
- Expected: documented `data` shape `{"field": "email", "message": "Required field 'email' is
  missing"}`.
- Suggested fix: introduce a structured validation-error type carrying `field`/`message` and emit
  it as `data` for field- and parameter-level failures.
- Test status: none; `ValidationTests.cs` asserts only `Message` text.
- Acceptance criteria:
  - [ ] A missing required field returns `data.field` and `data.message`.
  - [ ] A parameter type error returns structured data with the parameter name.

### csharp-09 — Field-removal severity contradicts the spec's own scenario
- Severity: low
- Invariant: M16 (`openspec/specs/contract-compatibility-verification/spec.md:115-118`)
- Evidence: `pkg/runtime/runtimes/csharp/PulseRPC/DiffEngine.cs:744-745` returns `Severity.Info`
  for `(Field, Removed, ClientHasMore)`; `tests/DiffEngineTests.cs:100-129` and
  `tests/SeverityTests.cs:28-32` assert `Info`. The spec scenario requires `Compatible=false`
  with an `Error` delta when a required field is removed.
- Current behavior: a client that has a field the server removed is still reported compatible.
- Expected: either the spec or the implementation must change; if the spec wins,
  `ClassifySeverity` must return `Error` and the tests updated.
- Suggested fix: decide the intended severity once (all runtimes currently agree on `Info`),
  then either amend the spec or change `ClassifySeverity` + `SeverityTest`/`DiffEngineTest`.
- Test status: tests exist but encode the `Info` interpretation, so they cannot catch the
  contradiction.
- Acceptance criteria:
  - [ ] The spec table and scenario agree on field-removal severity.
  - [ ] C# classifies a removed required field per the reconciled spec.

## Test gaps

- No C# unit tests for the generated client's envelope handling or response validation
  (csharp-01); runtime `ClientTests.cs` covers only the runtime path.
- `ValidationTests.cs` does not cover `long`, `NaN`, `Infinity`, or `bool` for all four scalars
  (R2), and does not assert rejection of whole-number overflow.
- `TypesTests.cs`/`ValidationTests.cs` have no qualified-name or ambiguous-base-name cases for
  structs or enums (R6/D16, csharp-07).
- No test sends an integer enum value, so the coercion in csharp-04 is not exercised.
- No test checks the shape of validation `data` (csharp-08).
- `tests/integration/test_http_api.sh` still treats an unexpected error code as a warning in
  several paths; the C# server run has no batch or notification cases, and no assertion that a
  wrong code fails the script (csharp-14 from the stale baseline).
- No generated-client test covers `VerifyOnBootstrap()` (csharp-02) or a server contract change
  (csharp-03).
- No test covers `Server.Call` returning `null` for a malformed notification's every error
  path beyond the cases in `ServerTests.cs`.

## Verified as holding

- R1, R2 (runtime acceptance sets now include `long` for float and reject non-finite values),
  R3, R4, R5, R7, R10 (runtime and generated server validate responses, including null
  required returns), R11 (runtime and generated server suppress notifications on every path),
  R12, R13 (runtime client), R14.
- R15 (runtime default sink writes only type name; generated server logs server-side and
  returns fixed `"Internal error"` with no `data`), R16, R17, R18, R19, R20, R22, R23, R24,
  R25, R26, R27 (enums on the wire are strings, covered by
  `InvariantClientTests.TransportJsonOptions_SerializeEnumsAsStrings`), R29, R30, R31, R32.
- M1, M3, M4, M5 (runtime client), M6, M7, M8, M9, M10 (runtime), M11 (runtime), M12, M18,
  M19, M20, M21, M22, M23, M24, M25 (runtime), M26, M27, M28.
- C1, C2, C4, C5, C8, C10, C11, C12, C13.
- D1, D2, D3, D4, D5, D6, D7, D8, D9, D12, D13, D15 (runtime client and generated client
  fetch `pulserpc-idl`), and D16 for structs (enum fallback gap is csharp-07).

## Unconfirmed hypotheses

- `Client.Notify` (`Client.cs:207-216`) calls `Call`, which always sends a request id, so it
  never produces a true JSON-RPC notification; it relies on swallowing the transport's
  "empty response" exception for in-process transports and ignores the server's response for
  HTTP transports. Whether callers depend on side effects vs. fire-and-forget semantics is
  unclear.
- Runtime `Contract`/`Types` dictionaries are built once at construction and then read-only;
  they are not guarded against concurrent mutation, but no in-tree code mutates them after
  construction, so the practical risk is unverified.
- The generated client uses transport-generated ids and never compares the response id to the
  request id (related to csharp-01); impact on HTTP transports is untested.
- Whether the shared files' global `PulseRPC` namespace with `-package` (csharp-06) is an
  intentional compatibility shim or an oversight could not be confirmed from the docs; the
  multi-namespace spec and the runtime rewrite disagree.

## Notes

- `docs/audit/problems-csharp.md` is a stale baseline: its csharp-01 (M15), csharp-02 (M9),
  csharp-03 (notifications), csharp-05 (object params), csharp-09 (transport), csharp-11
  (scalar acceptance), and the runtime half of csharp-12 (envelope) findings are resolved in
  the current tree; csharp-06, csharp-07, csharp-08, csharp-10, csharp-13, csharp-14, and
  csharp-15 are re-reported above (some narrowed to their remaining, unaddressed parts).
- `-package` namespace bug status: **still present**. The shared `Contract.cs`/`Server.cs`/
  `Client.cs` always declare `namespace PulseRPC` and are emitted at the output root even when
  `-package` is set (`csharp_client_server.go:659,725,1639`), while the runtime and
  per-namespace files live under the package prefix. Reported as csharp-06.
- `dotnet test tests/PulseRPC.Tests.csproj`: 119 passed, 0 failed (run during this audit).