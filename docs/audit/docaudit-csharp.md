# Doc Audit: C# runtime

## Scope & methodology

Audited the C# documentation (`docs/languages/csharp/reference.md`,
`docs/languages/csharp/quickstart.md`, the C# sections of
`docs/RUNTIME_IMPLEMENTATION_GUIDE.md`, and `docs/idl-guide/*`) against the C#
runtime implementation (`pkg/runtime/runtimes/csharp/PulseRPC/*`) and the
generated C# server/client (`pkg/generator/csharp_client_server.go`).

Method: every documentation claim touching the focus areas (validation data
shape, severity, notification handling, response validation, id types,
batch/limits, scalar acceptance, void returns, meta, contract verification,
namespace/package layout, transport) was traced to concrete source. Where the
documentation and the code disagree, the bias is to change the documentation
unless the code is clearly wrong, in which case the code fix is called out.

No source code was modified by this audit; only this document was written.

## Summary (counts by severity)

- Critical: 0
- High: 0
- Medium: 6
- Low: 8
- Total confirmed: 14

## Confirmed issues

### doc-csharp-01 — Reference examples use PascalCase method names, but generated code uses IDL casing
- Severity: medium
- Evidence: `docs/languages/csharp/reference.md` (Server Implementation, Client Usage,
  Async/Await, Validation sections) shows `ListProducts()`, `GetProduct()`,
  `CreateOrder()`, `AddToCart()`; `pkg/generator/csharp_client_server.go:755-760`
  (`writeInterfaceStubCs`) emits `method.Name` verbatim, and the quickstart include
  (`docs/_includes/quickstart/csharp-client.md`) uses `listProductsAsync()`,
  `addToCartAsync()`, `createOrderAsync()`.
- Current behavior: copy-pasting the reference examples does not compile against
  generated interfaces/clients.
- Expected: examples must use the exact IDL method names (e.g. `listProducts`,
  `getProduct`, `createOrder`, `addToCart`).
- Recommended change: documentation.
- Acceptance criteria:
  - [ ] All C# reference examples use the camelCase names emitted by the generator.
  - [ ] A reader can paste an example verbatim into a generated project.

### doc-csharp-02 — Reference uses `RPCException`, but the runtime exposes `RPCError`
- Severity: low
- Evidence: `docs/languages/csharp/reference.md` Validation section
  (`throw RPCException (-32602)`) and Async/Await catch (`catch (RPCException ex)`);
  `pkg/runtime/runtimes/csharp/PulseRPC/RPCError.cs` defines `RPCError`, and the
  generated server/client throw/parse `RPCError`. The Error Handling section
  already uses `RPCError`.
- Current behavior: the two exception names are used interchangeably in the
  reference.
- Expected: only `RPCError` is a real type.
- Recommended change: documentation.
- Acceptance criteria:
  - [ ] Every reference example uses `RPCError`.

### doc-csharp-03 — `idl-guide/validation.md` says C# validation `data` is a plain string
- Severity: medium
- Evidence: `docs/idl-guide/validation.md:243-246` states "`data` is a plain
  string in the Go, Java and C# runtimes"; `pkg/runtime/runtimes/csharp/PulseRPC/Contract.cs`
  (`ValidateRequest`/`ValidateResponse`) and the generated server throw `RPCError`
  with `data = ValidationResult.ToDictionary()`, i.e.
  `{"valid":false,"error":"...","invalid_fields":[...]}`. `ValidationResult.cs`
  defines exactly that shape.
- Current behavior: a C# validation failure returns structured data, not a
  plain string.
- Expected: the guide describes the C# (and now structured) shape, consistent
  with the manual-validation section.
- Recommended change: documentation.
- Acceptance criteria:
  - [ ] The validation guide no longer lists C# under "plain string".
  - [ ] The documented `data` shape matches `ValidationResult.ToDictionary()`.

### doc-csharp-04 — Generated client `VerifyOnBootstrap()` is documented but inert
- Severity: medium
- Evidence: `docs/languages/csharp/reference.md` Contract Compatibility section
  shows `new CatalogServiceClient(transport).WithAuditor(...).VerifyOnBootstrap()`;
  `pkg/generator/csharp_client_server.go` (client constructor) only assigns
  `_transport` and never calls `VerifyCompatibility()`, while the runtime
  `Client` (`pkg/runtime/runtimes/csharp/PulseRPC/Client.cs`) does invoke it.
- Current behavior: configuring a generated client with `VerifyOnBootstrap()` has
  no effect.
- Expected: either the generated client invokes verification in the constructor
  (matching the runtime client and the docs) or the docs stop promising it.
- Recommended change: code (the feature is intended and the runtime implements it).
- Acceptance criteria:
  - [ ] A generated client configured with `VerifyOnBootstrap()` + `FailFastAuditor`
    fails when the server IDL differs.
  - [ ] A generated client without the option behaves as before.

### doc-csharp-05 — Generated client does not check the response `id`
- Severity: medium
- Evidence: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:810-815` requires clients to
  validate the response envelope including id match; the runtime `Client`
  (`ValidateEnvelope`) compares ids, but the generated client
  (`pkg/generator/csharp_client_server.go`, `writeClientMethodImplCs`) only checks
  that an `id` member is present and never compares it to the request id.
- Current behavior: a generated client accepts a response whose id differs from
  the request.
- Expected: the generated client should match the runtime client and the guide.
- Recommended change: code (generated client must implement the documented envelope
  rules).
- Acceptance criteria:
  - [ ] A generated client receiving a response with a mismatched id throws.

### doc-csharp-06 — Generated server coerces integer enum values to names
- Severity: medium
- Evidence: `docs/languages/csharp/reference.md` states "Enums are wire strings"
  and `Validation.ValidateEnum` requires a string; but
  `pkg/generator/csharp_client_server.go` (param and response paths) rewrites an
  integer enum value to `enumValues[int]`'s name before validation.
- Current behavior: an enum parameter supplied as `0` is accepted and rewritten
  to the first declared name, bypassing wire-type validation.
- Expected: only declared enum names are accepted on the wire.
- Recommended change: code (the code is clearly wrong relative to the documented
  and invariant-defined behavior).
- Acceptance criteria:
  - [ ] An enum parameter supplied as `0` returns `-32602`.
  - [ ] Only declared enum names are accepted.

### doc-csharp-07 — Shared generated files stay in the global `PulseRPC` namespace with `-package`
- Severity: low
- Evidence: `docs/languages/csharp/quickstart.md` multi-namespace section shows
  `Contract.cs`/`Server.cs`/`Client.cs` at the output root with runtime imports
  under the package prefix; `pkg/generator/csharp_client_server.go:659,725,1639`
  emit a hard-coded `namespace PulseRPC` for those three files even when
  `-package` is set, while per-namespace files and the runtime move under the
  package prefix.
- Current behavior: with `-package MyApp.Lib.Rpc` the runtime lives under
  `MyApp.Lib.Rpc.PulseRPC` but the shared types remain in the global `PulseRPC`
  namespace.
- Expected: the documentation should state this compatibility shim explicitly, or
  the generator should move the shared files under the package root.
- Recommended change: documentation (clarify the actual layout) unless the package
  root behavior is desired, in which case code.
- Acceptance criteria:
  - [ ] The quickstart documents the namespace of `Contract.cs`/`Server.cs`/
    `Client.cs` for both the no-package and `-package` cases.

### doc-csharp-08 — `VerificationResult` and `ContractDelta` docs are incomplete
- Severity: low
- Evidence: `docs/languages/csharp/reference.md` lists `VerificationResult`
  properties (Compatible, ServerChecksum, ClientChecksum, Deltas, Timestamp) and
  `ContractDelta` properties (EntityType, EntityName, ChangeType, Direction,
  Severity, Description); `pkg/runtime/runtimes/csharp/PulseRPC/VerificationResult.cs`
  also has `Error`, and `ContractDelta.cs` also has `MemberName`.
- Current behavior: readers cannot discover the `Error` or `MemberName` fields.
- Expected: the tables include every public field.
- Recommended change: documentation.
- Acceptance criteria:
  - [ ] `Error` is documented on `VerificationResult`.
  - [ ] `MemberName` is documented on `ContractDelta`.

### doc-csharp-09 — `errors.md` shows `"data": null` for JSON-RPC errors
- Severity: low
- Evidence: `docs/idl-guide/errors.md` error-response example includes
  `"data": null`; `pkg/runtime/runtimes/csharp/PulseRPC/Server.cs`
  (`ErrorResponse`) and the generated server omit `data` entirely when it is null.
- Current behavior: the example does not match the emitted envelope.
- Expected: either omit `data` or state that it is present only when supplied.
- Recommended change: documentation.
- Acceptance criteria:
  - [ ] The error-response example matches the runtime envelope.

### doc-csharp-10 — Misplaced `id` sentence under "Enums are wire strings"
- Severity: low
- Evidence: `docs/languages/csharp/reference.md` Wire Compatibility section ends
  the enum bullet with "`id` must be a string, number, or null."; `id` validity
  is a JSON-RPC request property (D2), not an enum property.
- Current behavior: the sentence is attached to the wrong bullet.
- Expected: move it to a dedicated `id` bullet.
- Recommended change: documentation.
- Acceptance criteria:
  - [ ] The `id` rule appears under its own bullet.

### doc-csharp-11 — Reference claims `Serialization` uses Newtonsoft.Json
- Severity: low
- Evidence: `docs/languages/csharp/reference.md` Serialization section says
  "PulseRPC C# uses System.Text.Json or Newtonsoft.Json"; the runtime and
  generated code use only `System.Text.Json`.
- Current behavior: readers may expect a Newtonsoft option that does not exist.
- Expected: document System.Text.Json only.
- Recommended change: documentation.
- Acceptance criteria:
  - [ ] The Serialization section names only System.Text.Json.

### doc-csharp-12 — `idl-guide/types.md` omits C# from the int width table
- Severity: low
- Evidence: `docs/idl-guide/types.md` table lists `int` as 64-bit for Go/Java/
  Python/TypeScript; `docs/languages/csharp/reference.md` and D1 document C# `int`
  as signed 32-bit.
- Current behavior: the general type table gives no C#-specific width.
- Expected: add a C# column or footnote stating the 32-bit rule.
- Recommended change: documentation.
- Acceptance criteria:
  - [ ] The types table or a footnote states that C# `int` is signed 32-bit.

### doc-csharp-13 — Generated client response validation is not documented as unsupported
- Severity: medium
- Evidence: `docs/languages/csharp/reference.md` Validation section says "PulseRPC
  automatically validates: Required fields are present, Types match IDL
  definition, Enum values are valid"; the runtime `Client` validates responses
  when `_validateResponse` is set, but the generated client
  (`pkg/generator/csharp_client_server.go`, `writeClientMethodImplCs`) only
  deserializes the result and never calls `Contract.ValidateResponse`.
- Current behavior: the documentation implies response validation for all
  clients, but generated clients do not perform it.
- Expected: either the generated client validates responses (matching the
  runtime client and the docs) or the docs state that only the runtime client
  does.
- Recommended change: code (the runtime already implements it; the generated
  client should match).
- Acceptance criteria:
  - [ ] A generated client receiving a wrong-typed result throws a validation
    error, or the docs state the limitation.

### doc-csharp-14 — `errors.md` says codes can be any integer
- Severity: low
- Evidence: `docs/idl-guide/syntax.md` says "Error codes can be any integer";
  the parser requires non-negative codes (`pkg/parser/validator.go`).
- Current behavior: negative application error codes are documented but rejected.
- Expected: document the non-negative rule.
- Recommended change: documentation.
- Acceptance criteria:
  - [ ] The syntax guide states that error codes must be non-negative.

## Test gaps

- No C# documentation example is compiled in CI, so casing mismatches
  (doc-csharp-01) are not caught.
- No doc or test exercises a generated client with `VerifyOnBootstrap()`
  (doc-csharp-04) or a mismatched response id (doc-csharp-05).
- No test asserts the structured shape of a C# validation `data` payload
  (doc-csharp-03).

## Verified as holding

- Type mappings in `reference.md` (int 32-bit, float -> double, arrays/maps,
  nullable optionals) match the runtime and generated code.
- Wire limits in `reference.md` (1 MiB request, 10 MiB response, 100-entry
  batch, empty batch -> single `-32600`) match the generated server.
- Internal-error redaction in `reference.md` matches the generated server
  (`ILogger<PulseRPCServer>`) and runtime `Server` (`InternalErrorLogger`).
- Transport defaults in `reference.md` (30 s timeout, custom headers, trailing
  `/` trim, GUID request ids) match `HttpTransport`.
- Notification handling described in the runtime guide matches the generated
  server (no response, including errors, for notifications).
- Batch support in the generated C# server matches the runtime guide.
- Enum wire representation as strings matches `JsonStringEnumConverter` usage.
- Severity table in `reference.md` matches `Severity` and `ClassifySeverity`.
- `D16` type-name resolution for structs matches `Types.ResolveQualifiedName`.

## Unconfirmed hypotheses

- Whether the shared files' global `PulseRPC` namespace (doc-csharp-07) is an
  intentional compatibility shim or an oversight could not be confirmed from
  the docs.
- Whether the documentation's "automatic validation" wording is intended to
  cover only request validation or also response validation (doc-csharp-13).
- Whether the empty-batch message text is part of the documented contract or an
  implementation detail.

## Notes

- This audit compares documentation to the C# implementation only. Code bugs
  marked "Recommended change: code" are listed here so the documentation and
  implementation can be reconciled; the documentation should not be changed to
  match those bugs.
- `docs/audit/audit-csharp.md` covers the separate code-vs-invariant audit and
  remains the reference for runtime defects.
