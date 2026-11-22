# Doc vs Implementation Audit: ts-cjs runtime

## Summary

Scope: the user-facing TypeScript documentation (`docs/languages/typescript/reference.md`,
`docs/languages/typescript/quickstart.md`, the quickstart includes it renders,
`docs/RUNTIME_IMPLEMENTATION_GUIDE.md` TypeScript sections, `docs/idl-guide/*`) and the
approved `openspec/specs/ts-contract-verification/spec.md`, compared against the
`ts-cjs` runtime (`pkg/runtime/runtimes/ts-cjs/pulserpc/*.ts`), the `ts-node` runtime
(for divergence checks), and the shared TypeScript generator
(`pkg/generator/ts_client_server.go`).

The `ts-cjs` runtime itself is in good shape: the integer range, `NaN`/`Infinity`
handling, notifications on the server, response validation on the server, envelope
validation in the runtime `Client`, `meta` parsing, structured validation data, and
the wire limits all match the current code. Almost all deltas are **stale
documentation**: the generated-code sections of `reference.md` and parts of
`quickstart.md` describe an older generator API (struct classes, `HTTPTransport`,
`PulseRPCServer`) that no longer exists.

Counts:

- **26 deltas**: 8 High, 9 Medium, 9 Low
- **0 Critical**
- **4 cases where the code may be wrong** (see the last section); three align with
  deltas tscjs-doc-13, -23, and -24, and the fourth is a ts-node parity gap related
  to tscjs-doc-08
- **Verified accurate**: the wire-limit/batch/notification/int/envelope sections of
  `reference.md`, the `ValidationResult` shape in `reference.md`, the diff/auditor
  requirements in the spec, and the IDL-guide optional-field/array/map validation
  rules

Severity key: High = copy-pasted examples fail or a core API is misdescribed;
Medium = a statement or example is wrong but easy to work around; Low =
under-documentation or a narrow edge case.

---

## Deltas

### tscjs-doc-01 — Structs are documented as classes; the generator emits interfaces

- **Severity:** High
- **Doc claim:** `docs/languages/typescript/reference.md:19` maps `Struct` to
  `Class` / `new Product({...})`; `:83-103` says "Each struct in your IDL becomes a
  TypeScript class" and shows `new checkout.Product({...})`, `new checkout.Cart(...)`;
  `:111-118` repeats the constructor pattern; `:393-413` builds nested structs with
  `new checkout.Order` / `new checkout.Cart` / `new checkout.Address`.
- **Code reality:** `pkg/generator/ts_client_server.go:1728-1754` emits
  `export interface <Name> { ... }` for every struct. Optional fields are emitted as
  `field?: T` (`:1747-1752`). There is no generated constructor or class, so every
  `new checkout.Product(...)` example throws at runtime and fails to type-check.
- **Recommendation:** Change doc.
- **Rationale:** The generated `types.ts` is an interface-only module. The examples
  should use object literals (`const product: checkout.Product = { ... }`) and the
  type-mapping table should say `interface`. This is the single largest source of
  broken copy-paste examples in the TypeScript docs.

### tscjs-doc-02 — The server API described for TypeScript does not exist

- **Severity:** High
- **Doc claim:** `docs/languages/typescript/reference.md:178` imports
  `{ PulseRPCServer, CatalogService } from './server'`; `:202-204` constructs
  `new PulseRPCServer(8080)`, calls `server.registerCatalogService(...)` and
  `server.start()`; `:438-443` repeats `new PulseRPCServer(8080)` for Express.
- **Code reality:** the generated `server.ts` contains only abstract service classes
  (`pkg/generator/ts_client_server.go:1470-1551`, `:1510`); it has no HTTP server, no
  `PulseRPCServer`, and no `register*`/`start()` methods. The runtime exports a
  transport-independent `Server` constructed with
  `new Server({ contract, validateRequests, validateResponses })`, registered with
  `addHandler(ifaceName, handler)`, and invoked with `await server.call(req, ctx)`
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/server.ts:34-64`). `PulseRPCServer` exists
  only in the Go and C# generators, not the TypeScript one.
- **Recommendation:** Change doc.
- **Rationale:** The documented lifecycle is from another language's runtime. Rewrite
  the server/Express sections around `Contract` + `Server` + a user-supplied HTTP
  adapter, using the shape already present in
  `docs/_includes/quickstart/ts-server.md`.

### tscjs-doc-03 — Generated abstract methods require `ctx` and return `Promise`

- **Severity:** High
- **Doc claim:** `docs/languages/typescript/reference.md:187-198` shows
  `listProducts(): checkout.Product[]` and
  `getProduct(productId: string): checkout.Product | null` with no context parameter
  and synchronous return values; `:235-247` shows an `async createOrder(...)` that
  omits `ctx`.
- **Code reality:** `pkg/generator/ts_client_server.go:1528-1548` (and the namespaced
  variant at `:1580-1600`) emits
  `abstract listProducts(ctx: Record<string, any>): Promise<types.Product[]>`, i.e.
  the context object is always the first parameter and non-void returns are wrapped
  in `Promise<>`. The server invokes handlers as `func(ctx, ...positional)`
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/server.ts:189-190`).
- **Recommendation:** Change doc.
- **Rationale:** A subclass written exactly as documented fails with a return-type
  incompatibility against the abstract base class (and a parameter-count mismatch if
  `ctx` is omitted).

### tscjs-doc-04 — Generated client methods are asynchronous

- **Severity:** Medium
- **Doc claim:** `docs/languages/typescript/reference.md:218` assigns
  `const products: checkout.Product[] = catalog.listProducts();`; `:224` does the
  same for `catalog.getProduct('prod001')`; `:369` and `:378-380` repeat synchronous
  signatures.
- **Code reality:** `pkg/generator/ts_client_server.go:1986-2010` emits every
  generated client method as `async ... : Promise<T>` (or `Promise<void>`), and the
  method body awaits `this.transport.request(...)` at `:2029`. Callers must `await`
  (or chain `.then`).
- **Recommendation:** Change doc.
- **Rationale:** The quickstart client include already uses `await`
  (`docs/_includes/quickstart/ts-client.md:10,16,23`); only `reference.md` is stale.

### tscjs-doc-05 — Transport class is named `HttpTransport`, not `HTTPTransport`

- **Severity:** Medium
- **Doc claim:** `docs/languages/typescript/reference.md:210` imports
  `{ HTTPTransport } from './client'`; `:214` constructs
  `new HTTPTransport('http://localhost:8080')`. The quickstart client include does the
  same at `docs/_includes/quickstart/ts-client.md:2,4`.
- **Code reality:** the runtime exports `HttpTransport`
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/transport.ts:157`,
  `pkg/runtime/runtimes/ts-cjs/pulserpc/index.ts:20`), and the generated `client.ts`
  imports/re-exports `HttpTransport`
  (`pkg/generator/ts_client_server.go:1958-1962`).
- **Recommendation:** Change doc.
- **Rationale:** `HTTPTransport` is the Go/Java naming; in TypeScript it is a
  `ReferenceError` at runtime. The guide itself uses `HttpTransport` correctly at
  `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:175`.

### tscjs-doc-06 — `RPCException` is not a TypeScript symbol

- **Severity:** Medium
- **Doc claim:** `docs/languages/typescript/reference.md:242` throws
  `new RPCException(1002, ...)`; `:257` says validation "will throw RPCException
  (-32602)"; `:389` advises catching `RPCException`.
- **Code reality:** the only runtime error class is `RPCError`
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/rpc.ts:5-17`), exported from the package root
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/index.ts:7`). The generated code and the
  quickstart use `RPCError` (`docs/_includes/quickstart/ts-server.md:5,56,88,92`).
- **Recommendation:** Change doc.
- **Rationale:** `reference.md` already uses `RPCError` in its own error-handling
  section (`:152-162`), so this is an internal inconsistency as well as a nonexistent
  symbol. `RPCException` is the Python naming.

### tscjs-doc-07 — Client-side validation is opt-in, not automatic

- **Severity:** High
- **Doc claim:** `docs/languages/typescript/reference.md:254-269` says "PulseRPC
  automatically validates requests and responses against your IDL" and shows a client
  call that "will throw ... if validation fails". `docs/idl-guide/validation.md:12-16`
  says validation happens client-side before sending and server-side before/after
  processing.
- **Code reality:** the runtime `Client` constructor/`Client.create` defaults both
  flags to `false` (`pkg/runtime/runtimes/ts-cjs/pulserpc/client.ts:74-101`;
  request validation at `:228-245`, response validation at `:303-309`). Server-side
  validation defaults to `true`
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/server.ts:43-44`). The generated `<Iface>Client`
  stubs perform no validation at all: they build a request, call the transport, and
  return `_resp.result` (`pkg/generator/ts_client_server.go:2012-2048`).
- **Recommendation:** Change doc.
- **Rationale:** Users who rely on the documented "automatic" client validation get no
  protection unless they call `Client.create(transport, true, true, ...)`, and the
  generated clients never validate. State the defaults explicitly and recommend
  server-side validation as the trust boundary.

### tscjs-doc-08 — Local client validation failures are plain `Error`, not coded RPC errors

- **Severity:** Medium
- **Doc claim:** `docs/languages/typescript/reference.md:257` says validation throws
  `RPCException (-32602)`. The runtime error-handling section (`:150-171`) presents
  `RPCError` as the error type for JSON-RPC failures.
- **Code reality:** client request validation wraps failures in
  `new Error("Request validation failed: ...")`
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/client.ts:235,242`); client response
  validation does the same (`:307`). Envelope failures also throw plain `Error`
  (`:267-284`). Only transport/server errors use `RPCError`
  (`:290-295`, `transport.ts:198-227`). `ts-node` throws
  `RPCError(-32600, ...)` for envelope failures
  (`pkg/runtime/runtimes/ts-node/pulserpc/client.ts:322-349`).
- **Recommendation:** Change doc.
- **Rationale:** If the docs intend callers to branch on `err.code`, the current
  client-side errors cannot be used that way. Either document "local validation
  failures are plain `Error`" or align the client with `ts-node` by throwing
  `RPCError(-32602/-32600)`. Given the bias that the implementation is correct,
  documenting the current contract is the lower-risk fix.

### tscjs-doc-09 — IDL `int` is documented as 64-bit in the IDL guide

- **Severity:** Medium
- **Doc claim:** `docs/idl-guide/types.md:15` says `int` is "64-bit integers" (with Go
  `int64`, Java `Long`, Python `int`).
- **Code reality:** request/response validation enforces signed 32-bit:
  `INT32_MIN = -2147483648`, `INT32_MAX = 2147483647`, with explicit `NaN`,
  non-finite, fractional and range rejection
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/validation.ts:31-51`). The canonical
  invariant is D1 in `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:54-58`, and
  `docs/languages/typescript/reference.md:13,474-479` already says signed 32-bit.
- **Recommendation:** Change doc.
- **Rationale:** `types.md` is the IDL-level type table and currently contradicts both
  the runtime and the implementation guide. It should say "signed 32-bit on the wire;
  native types may be wider" rather than "64-bit".

### tscjs-doc-10 — Generated file list and import paths are stale

- **Severity:** Medium
- **Doc claim:** `docs/languages/typescript/quickstart.md:56-60` says generation
  creates `checkout.ts` (types), `server.ts` (server framework), `client.ts` (HTTP
  client framework). `reference.md:86,131,179,211-212` import types from
  `'./checkout'`. `quickstart.md:88` adds a
  `"pulserpc-ts-runtime": "file:./pulserpc"` dependency.
- **Code reality:** the generator writes `types.ts`, `server.ts`, and `client.ts`
  (`pkg/generator/ts_client_server.go:1148-1169`); no `<namespace>.ts` file is
  emitted. `server.ts` contains abstract classes only (`:1470-1551`); `client.ts`
  contains typed client stubs, not an HTTP framework (`:1905-1982`). Imports should
  use `./types`, `./server`, and `./client`. The generated `pulserpc/` directory for
  the CJS style has no `package.json` (unlike the ESM runtime's
  `pkg/runtime/runtimes/ts-node/pulserpc/package.json`), so the
  `file:./pulserpc` dependency is both unused by the generated relative imports and
  not a valid package directory.
- **Recommendation:** Change doc.
- **Rationale:** Copy-pasting the documented imports fails to resolve. Update the file
  list, the `checkout` import alias to `./types`, and the client import to `./client`;
  remove the `pulserpc-ts-runtime` dependency line (or generate a package.json for the
  CJS runtime copy).

### tscjs-doc-11 — The IDL is not embedded in `server.ts`

- **Severity:** High
- **Doc claim:** `docs/languages/typescript/quickstart.md:68` says "The IDL is
  embedded directly in `server.ts` for the `pulserpc-idl` RPC method."
  `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:738-749` says the IDL JSON is embedded in the
  server file as a language-appropriate string constant, and specifically that
  TypeScript uses a template literal (`:747`).
- **Code reality:** the TypeScript generator writes `idl.json` to the output
  directory (`pkg/generator/ts_client_server.go:1068-1091`, `:1269-1288`). The
  generated test server reads it from disk (`:2112`, `:2141`), and the runtime
  `Server` returns the parsed `Contract` document for `pulserpc-idl`
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/server.ts:111-116`) rather than an embedded
  string. The runtime `Client` also discovers a local `idl.json` by walking up from
  `__dirname` (`client.ts:149-179`).
- **Recommendation:** Change doc.
- **Rationale:** The file-based IDL is an intentional generator design (the
  generator comment at `:1269-1273` documents the placement rules) and is also what
  the contract-verification spec expects. The docs should say the server loads
  `idl.json` into a `Contract`, and the `pulserpc-idl` result is the parsed IDL
  object. If embedding is a hard cross-language requirement, this becomes a code
  change instead.

### tscjs-doc-12 — The guide says generated TypeScript servers do not support batches

- **Severity:** Medium
- **Doc claim:** `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:906-910` says "generated
  TypeScript and Python servers do not" support batch requests. The checklist repeats
  this at `:1313` ("optional; Go, C# and Java generated servers do").
- **Code reality:** the generated TypeScript test adapter implements bounded batches:
  arrays are dispatched with `Promise.all`, notifications are filtered out, an empty
  batch returns one `-32600`, more than 100 entries returns
  `-32600 "Batch exceeds maximum size"`, and an all-notification batch returns HTTP
  204 (`pkg/generator/ts_client_server.go:2209-2231`, limits at `:2155-2156`). This
  matches the user-facing description in
  `docs/languages/typescript/reference.md:527-529` and
  `docs/languages/typescript/quickstart.md:62`.
- **Recommendation:** Change doc.
- **Rationale:** The guide contradicts the current generator and the TypeScript
  reference. Note that the core `Server.call` still handles a single request; batch
  handling lives in the generated HTTP adapter.

### tscjs-doc-13 — `id: null` notifications work on the server but not through `HttpTransport`

- **Severity:** Low
- **Doc claim:** `docs/languages/typescript/reference.md:486-489` says a request with
  `id: null` (or no `id`) is a notification: the handler runs and no response is
  written. D2 in the guide repeats this (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:60-62`).
- **Code reality:** `Server.call` treats both `id === undefined` and `id === null` as
  notifications (`pkg/runtime/runtimes/ts-cjs/pulserpc/server.ts:55-64`), so the
  server side matches. However, `HttpTransport.request` only converts an empty 204
  response into a neutral envelope when the request has no `id` property at all
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/transport.ts:211-221`); a caller that sends
  `id: null` gets `RPCError(-32603, "Empty response body")`. `Client.notify` avoids
  this by omitting `id` entirely (`client.ts:247-259,314-316`).
- **Recommendation:** Either; doc clarification is sufficient if `id: null` over HTTP
  is unsupported.
- **Rationale:** The public `Client.notify` path is unaffected, so this is a narrow
  edge case for direct `Transport` users. Preferred code fix: treat
  `req.id === null` like an absent id in the 204 branch. Acceptable doc fix: state
  that clients must omit `id` for notifications.

### tscjs-doc-14 — The guide requires strict error-object validation; the client normalizes instead

- **Severity:** Low
- **Doc claim:** `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:895-899` says clients must
  validate that an `error` member is an object with a numeric `code` and a string
  `message`, and that exactly one of `result`/`error` is present.
- **Code reality:** the runtime `Client` does validate `jsonrpc`, `id`, and
  result/error exclusivity/presence (`client.ts:267-284`), but a malformed `error`
  member is not rejected as an invalid envelope. A non-object, a missing/non-numeric
  `code`, or a non-string `message` is converted to
  `RPCError(-32603, "Unknown error")` (`client.ts:286-295`), and `error: null` is
  correctly treated as an error (not a success). `ts-node` behaves the same way.
- **Recommendation:** Change doc.
- **Rationale:** The client is fail-closed (it still throws), just not with `-32600`.
  Document the normalization rule; otherwise readers will expect a malformed error
  member to be rejected as an invalid response.

### tscjs-doc-15 — `ValidationResult` fields are `undefined` when valid, not `null`

- **Severity:** Low
- **Doc claim:** `docs/idl-guide/validation.md:154-155` types `error` and
  `invalidFields` as `string | null` / `string[] | null` and says they are null when
  the value is valid.
- **Code reality:** `buildValidationResult` returns `{ valid: true }` with neither
  property set when there are no errors, and only assigns `error`/`invalidFields`
  on failure (`pkg/runtime/runtimes/ts-cjs/pulserpc/contract.ts:96-109`;
  `types.ts:189-193`). `docs/languages/typescript/reference.md:301-302` already
  documents `undefined`.
- **Recommendation:** Change doc.
- **Rationale:** `validation.md` is a language-neutral page with TypeScript examples;
  it should either say "absent/`undefined` (`null` in languages that use null)" or
  split the TypeScript row. The two docs currently disagree.

### tscjs-doc-16 — `Contract.meta` is implemented but undocumented

- **Severity:** Low
- **Doc claim:** `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:234-266` describes the
  `Contract` class as exposing only `interfaces`, `structs`, and `enums`; the IDL
  schema D8 (`:86-88`) lists only `rootNamespace`, `checksum`, `interfaces`,
  `structs`, `enums`, `errors`.
- **Code reality:** `Contract` has a public `meta: Record<string, any>` and parses
  both the dict form (`idlParsed.meta`) and the Barrister list form
  (`{type: "meta", ...}`), rejecting `__proto__`, `constructor`, and `prototype`
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/contract.ts:20,117-160,162-206`).
- **Recommendation:** Change doc.
- **Rationale:** The feature exists and is security-hardened (covered by runtime
  tests), but the current generator's `MarshalIDLJSON` emits no `meta` key
  (`pkg/generator/idl_json.go:38-45`), so it is only populated by legacy/Barrister
  IDL documents. Document it as legacy compatibility, not as a generator feature.

### tscjs-doc-17 — TypeScript does not generate per-error classes or type guards

- **Severity:** Medium
- **Doc claim:** `docs/idl-guide/errors.md:239` says TypeScript generates "Error
  classes and type guards" from `errors` declarations.
- **Code reality:** the TypeScript generator emits enums, struct interfaces, abstract
  service classes, and client stubs, but no error classes or type guards
  (`pkg/generator/ts_client_server.go:1694,1728,1510,2061`; a search for error-code
  generation returns only test-harness code). Application errors are raised with the
  generic `RPCError(code, message, data)` (`pkg/runtime/runtimes/ts-cjs/pulserpc/rpc.ts:5-17`),
  which is also what the TypeScript reference documents
  (`docs/languages/typescript/reference.md:150-171`).
- **Recommendation:** Change doc.
- **Rationale:** The table row is the only place claiming generated error classes for
  TypeScript. Change it to "`RPCError` with declared numeric codes" unless per-error
  classes are actually implemented.

### tscjs-doc-18 — The error example shows `data: null`; the server omits `data`

- **Severity:** Low
- **Doc claim:** `docs/idl-guide/errors.md:95-105` shows an error response containing
  `"data": null`.
- **Code reality:** `Server.errorResponse` only sets `error.data` when `data !==
  undefined` (`pkg/runtime/runtimes/ts-cjs/pulserpc/server.ts:217-229`). This matches
  the documented M22 rule in the guide (`docs/RUNTIME_IMPLEMENTATION_GUIDE.md:16`) and
  the TypeScript reference (`docs/languages/typescript/reference.md:512`: "`error.data`
  is omitted entirely when there is no data").
- **Recommendation:** Change doc.
- **Rationale:** The example teaches clients to expect a key that the runtime never
  sends. Remove `"data": null` or show a case where the handler actually supplies
  data.

### tscjs-doc-19 — "Arrays/maps are optional if their element type is optional" is not an IDL rule

- **Severity:** Low
- **Doc claim:** `docs/idl-guide/types.md:69` says arrays/maps are optional if their
  element type is optional.
- **Code reality:** the parser has no optional marker on a type at all: `parser.Type`
  only carries `BuiltIn`, `Array`, `MapValue`, and `UserDefined`
  (`pkg/parser/idl.go:88-102`); optionality is a property of the field
  (`pkg/parser/parser.go:133-138`). Validation applies `isOptional` from the field
  definition (`pkg/runtime/runtimes/ts-cjs/pulserpc/validation.ts:136-154`) and
  array/map elements are always validated as non-optional (`:190-199`).
- **Recommendation:** Change doc.
- **Rationale:** The sentence describes a syntax that cannot occur. Replace it with
  "a field whose type is an array or map is optional only when the field itself is
  marked `[optional]`; element types are never optional."

### tscjs-doc-20 — The documented runtime export surface is incomplete

- **Severity:** Low
- **Doc claim:** `docs/languages/typescript/reference.md:523-525` says "Constants are
  exported by the runtime: `HTTP_TIMEOUT_MS` and `MAX_RESPONSE_BYTES`."
- **Code reality:** `pkg/runtime/runtimes/ts-cjs/pulserpc/index.ts` additionally
  exports `INT32_MIN`/`INT32_MAX` (`:36-37`), `normalizeBaseUrl` (`:24`), the full
  scalar/aggregate validator set (`:26-38`), the type helpers
  (`findStruct`/`findEnum`/`getStructFields`/`extractChecksum`, `:40-61`),
  `diffIDL`/`classifySeverity` (`:39`), `Server`/`Client`/`Transport`/`HttpTransport`/
  `InProcTransport`, and the auditors. Notably, unlike `ts-node`
  (`pkg/runtime/runtimes/ts-node/pulserpc/index.ts:1-17`), the `ts-cjs` root does not
  export `RPCErrorCode`.
- **Recommendation:** Change doc.
- **Rationale:** The sentence is not false, but it under-describes the public surface
  and hides the only exported int-range constants. Add an API/exports table. If
  package-root parity with `ts-node` is intended, also export `RPCErrorCode` (code
  change) or document the difference.

### tscjs-doc-21 — The spec's `new Client(transport, options)` API does not exist

- **Severity:** High
- **Doc claim:** `openspec/specs/ts-contract-verification/spec.md:116-128` says the
  client constructor accepts an options object and its scenarios use
  `new Client(transport, { auditor: ... })` and
  `new Client(transport, { verifyOnBootstrap: true })`. The scenario at `:123-124`
  says verification results are passed to the auditor "after each call".
- **Code reality:** the constructor is private
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/client.ts:74-88`). The only public
  construction path is
  `static async create(transport, validateRequest = false, validateResponse = false, options?: ClientOptions)`
  (`:90-101`), so options are the **fourth positional argument**:
  `await Client.create(transport, false, false, { auditor, verifyOnBootstrap })`.
  Auditors are invoked by `verifyCompatibility` (`:212-214`), not by ordinary RPC
  calls.
- **Recommendation:** Change spec.
- **Rationale:** The spec describes a constructor/options shape that no TypeScript
  runtime implements (ts-node is also a private constructor with the same static
  factory). The spec should use `Client.create` and state that the auditor runs only
  during compatibility verification.

### tscjs-doc-22 — The spec names `IContractAuditor`; the runtime exports `ContractAuditor`

- **Severity:** Low
- **Doc claim:** `openspec/specs/ts-contract-verification/spec.md:50-61` requires an
  `IContractAuditor` interface with `audit(result: VerificationResult): void` and
  `name(): string`.
- **Code reality:** the runtime exports `ContractAuditor` with
  `audit(result: { compatible: boolean; deltas: any[] }): void` and `name(): string`
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/contract.ts:22-25`; re-exported at
  `index.ts:14`). `ClientOptions.auditor` is typed `any` rather than the interface
  (`client.ts:47-50`). ts-node uses the same `ContractAuditor` name.
- **Recommendation:** Change spec.
- **Rationale:** The behavioral requirements (NoOp/Logging/FailFast) are met; only the
  name and parameter type differ. Renaming the spec is lower risk than changing a
  public type. If compatibility matters, export
  `type IContractAuditor = ContractAuditor` from `contract.ts` instead (code change).

### tscjs-doc-23 — The spec says missing `idl.json` falls back to server-only comparison

- **Severity:** Medium
- **Doc claim:** `openspec/specs/ts-contract-verification/spec.md:78-86` says if
  `idl.json` is not found, "verification uses server IDL only (no local comparison)".
- **Code reality:** `verifyCompatibility` throws
  `new Error("No local IDL available - call setLocalIDL() first")` when no local IDL
  was loaded (`pkg/runtime/runtimes/ts-cjs/pulserpc/client.ts:185-195`). A diff
  cannot be computed against only one side, so there is no "server-only" result.
  (`_findIDLJson` is best-effort only for a missing file, not for malformed JSON; a
  malformed `idl.json` throws during `Client.create` at `:149-179,97`.)
- **Recommendation:** Change spec.
- **Rationale:** The code's behavior is the only coherent one for a structural diff.
  Document that verification requires a local IDL (from `idl.json` or
  `setLocalIDL`), and optionally that a malformed local file is a hard error.

### tscjs-doc-24 — Generated client stubs skip the envelope validation the guide requires

- **Severity:** High
- **Doc claim:** `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:888-900` states that clients
  must validate the JSON-RPC envelope: `jsonrpc` is `"2.0"`, the `id` matches,
  exactly one of `result`/`error` is present, and an `error` member is an object with
  a numeric `code` and string `message`. `docs/languages/typescript/reference.md:207-228`
  presents the generated `<Iface>Client` as the client API.
- **Code reality:** generated client methods do not perform any of those checks. They
  send the request, then do `if (_resp.error) { throw ... }` and
  `return _resp.result as T` (`pkg/generator/ts_client_server.go:2029-2048`). They
  therefore accept a mismatched `id`, ignore `jsonrpc`, and treat `error: null` as a
  successful `undefined` result. The runtime `Client` does validate the envelope
  (`pkg/runtime/runtimes/ts-cjs/pulserpc/client.ts:267-295`), but the documented
  generated clients are not built on it.
- **Recommendation:** Change code (preferred); if the thin-stub behavior is
  intentional, change doc to state that generated clients do not validate and that
  callers needing validation must use the runtime `Client`.
- **Rationale:** This is the one place where the code clearly violates a normative
  requirement in the guide, and it can silently accept a malformed response. The
  `error: null` case is a concrete correctness bug. This is a deliberate disagreement
  with the "code is probably right" bias; see the final section.

### tscjs-doc-25 — The quickstart server sample cannot compile or work as written

- **Severity:** High
- **Doc claim:** `docs/languages/typescript/quickstart.md:74` includes
  `docs/_includes/quickstart/ts-server.md`, whose handlers are synchronous
  (`:31-37,40-81,84-113`) and whose adapter does
  `const response = this.rpcServer.call(data);` (`:135`) before checking
  `response === null` and serializing it.
- **Code reality:** generated abstract methods require `Promise<T>` returns and a
  leading `ctx` parameter (`pkg/generator/ts_client_server.go:1528-1548`), so the
  synchronous overrides fail type-checking. `Server.call` is `async` and returns
  `Promise<JsonRpcResponse | null>` (`pkg/runtime/runtimes/ts-cjs/pulserpc/server.ts:55-64`),
  so `response` is a Promise: `response === null` is false and
  `JSON.stringify(response)` produces `{}`. The sample must `await` the call and
  return promises from the handlers.
- **Recommendation:** Change doc.
- **Rationale:** This is the primary end-to-end quickstart path; as written it
  produces a 200 response with an empty `{}` body and does not compile under the
  generated abstract classes.

### tscjs-doc-26 — The quickstart client passes `null` where the generated type only allows `undefined`

- **Severity:** Low
- **Doc claim:** `docs/_includes/quickstart/ts-client.md:17` calls
  `cart.addToCart({ cartId: null, ... })`. `docs/languages/typescript/reference.md:533`
  says optional values are `T | undefined` in generated types but may be `null` on
  the wire.
- **Code reality:** the generator emits optional struct fields as `field?: T`
  (`pkg/generator/ts_client_server.go:1747-1752`), which TypeScript types as
  `T | undefined`, not `T | null`. Runtime validation does accept `null` for an
  optional field (`pkg/runtime/runtimes/ts-cjs/pulserpc/validation.ts:139-156`), so
  the sample works at runtime when strict null checks are off (the quickstart
  tsconfig sets `strict: false` at `quickstart.md:109`), but it fails under
  `strict: true`, which the reference recommends (`reference.md:385`).
- **Recommendation:** Change doc.
- **Rationale:** Prefer omitting `cartId` (or passing `undefined`) in the sample. If
  `null` is meant to be a first-class value, the generator should emit `T | null` for
  optional fields instead (code change), but that is a broader type-contract change.

---

## Docs that are accurate

- **Wire limits and constants** — `reference.md:514-525` matches
  `transport.ts:19-21,139-141,196-208`: 10 MiB response cap, 30 s timeout,
  `HTTP_TIMEOUT_MS`/`MAX_RESPONSE_BYTES`, one trailing-slash normalization, and the
  1 MiB request / 100-entry batch limits from the generated adapter
  (`pkg/generator/ts_client_server.go:2155-2156`).
- **Batch semantics** — `reference.md:527-529` matches
  `pkg/generator/ts_client_server.go:2209-2231`: empty batch → one `-32600`,
  non-object entries → per-entry `-32600`, all-notification batch → HTTP 204,
  over-limit → one `-32600`.
- **Notification handling on the server** — `reference.md:486-489` matches
  `server.ts:55-64`: both missing and `null` ids run the handler and produce no
  response; `id: 0` is echoed.
- **Internal errors** — `reference.md:500-512` matches `server.ts:189-214,217-229`:
  handler exceptions and response-validation failures become
  `-32603 "Internal error"` with no `data`; declared `RPCError` values keep their
  code/message/data; `error.data` is omitted when absent.
- **Runtime client envelope validation** — `RUNTIME_IMPLEMENTATION_GUIDE.md:888-900`
  matches `client.ts:267-295` for `jsonrpc`, id matching, and result/error
  exclusivity/presence (the only caveat is the malformed-error normalization in
  tscjs-doc-14).
- **`ValidationResult` / manual validation** — `reference.md:271-361` matches
  `contract.ts:211-231` and `validation.ts`; error paths such as `.age`,
  `.address.city`, and `.items[1].quantity` match `joinPath`/`arrayIndexPath`
  (`validation.ts:7-13`).
- **`ValidationResult` field types in the TypeScript reference** —
  `reference.md:296-311` correctly says `undefined` when valid, matching
  `contract.ts:96-109`.
- **Structured validation data** — `validation.md:245-247` matches
  `contract.ts:288-294,321-326`: request validation throws `RPCError(-32602)` with a
  `ValidationResult` as `data`; response validation does the same under `-32603`.
- **Optional-field null/omitted rules** — `types.md:55-68` matches
  `validation.ts:139-156` for field-level optionality.
- **Enums as string values** — `reference.md:126-148` matches generator output at
  `pkg/generator/ts_client_server.go:1694-1710` (string-valued `enum` members, no
  ordinals).
- **Spec diff/result/auditor behavior** — `spec.md:6-48,63-76` matches
  `diff.ts`/`types.ts`/`contract.ts`: directional severities, `ContractDelta` and
  `VerificationResult` shapes, and the NoOp/Logging/FailFast auditors.
- **`setLocalIDL` and bootstrap-time server IDL** — `spec.md:89-94,130-137` matches
  `client.ts:120-147,181-183` (the server IDL is fetched once at bootstrap and
  reused; only the missing-local-IDL case is wrong, tscjs-doc-23).

---

## Recommended doc edits (concrete)

1. **`docs/languages/typescript/reference.md`**
   - Type-mapping table (`:10-20`): change `Struct | Class | new Product(...)` to
     `Struct | interface | { productId: ... }`.
   - "Generated Classes" (`:81-124`): replace class/constructor examples with
     object literals typed as `checkout.Product` etc.
   - "Server Implementation" (`:173-205`) and "Using with Express" (`:434-451`):
     replace `PulseRPCServer`/`register*`/`start()` with
     `Contract` + `Server` + `addHandler` + `await server.call(req, ctx)` and a
     user-supplied HTTP adapter; add `ctx` to every handler and make returns
     promises.
   - "Client Usage" (`:207-228`) and "Type Safety" (`:363-381`): use
     `HttpTransport`, import types from `./types` and clients from `./client`, and
     `await` every generated client call.
   - Error handling/async/best practices (`:150-171,230-248,383-389`): replace
     `RPCException` with `RPCError`.
   - Validation (`:250-263`): state that server validation is on by default, runtime
     client validation is opt-in via `Client.create(transport, true, true, ...)`, and
     generated client stubs do not validate.
   - Exports (`:523-525`): add `INT32_MIN`/`INT32_MAX` and the rest of the
     package-root surface, or add a short API-reference table.

2. **`docs/languages/typescript/quickstart.md` and includes**
   - File list (`:56-60`): `checkout.ts` → `types.ts`; describe `server.ts` as
     abstract service classes and `client.ts` as typed client stubs.
   - Remove the "IDL is embedded directly in `server.ts`" sentence (`:68`); say the
     generator writes `idl.json`, the server loads it into `Contract`, and
     `pulserpc-idl` returns that parsed document.
   - Remove the `pulserpc-ts-runtime` `file:./pulserpc` dependency (`:88`) or generate
     a `package.json` for the CJS runtime copy.
   - `docs/_includes/quickstart/ts-server.md`: make handlers `async` and return
     `Promise<...>`, and `await this.rpcServer.call(data)`; consider adding the
     documented 1 MiB/100-batch/POST-only behavior or pointing at the generated test
     adapter.
   - `docs/_includes/quickstart/ts-client.md`: `HTTPTransport` → `HttpTransport`;
     omit `cartId` instead of passing `null` (`:17`).

3. **`docs/RUNTIME_IMPLEMENTATION_GUIDE.md`**
   - Batch statement (`:906-910`) and checklist (`:1313`): include TypeScript among
     the generated adapters that support bounded batches; clarify that the core
     `Server.call` is single-request.
   - Embedded IDL section (`:736-749`): note the TypeScript exception — the generator
     writes `idl.json` and the runtime serves the parsed contract; only use the
     template-literal rule if embedding is actually implemented.
   - Contract class (`:234-266`): add `meta` and describe it as legacy/Barrister
     compatibility, noting the current generator does not emit it.
   - Envelope validation (`:895-899`): document the client's normalization of a
     malformed `error` member to `-32603 "Unknown error"`, or tighten the code.

4. **`docs/idl-guide/types.md`**
   - `:15`: change `int` from "64-bit integers" to "signed 32-bit integers on the
     wire (native types may be wider)".
   - `:69`: replace the element-type-optional sentence with a field-level
     `[optional]` rule.

5. **`docs/idl-guide/validation.md`**
   - `:154-155`: change `null` to `undefined`/absent for TypeScript, or make the
     table language-specific.
   - Optionally clarify that the `-32602` message is
     `Function '<Iface>.<method>' invalid params` with a structured `data`
     (`ValidationResult`), not the generic Go string shown at `:239-240`.

6. **`docs/idl-guide/errors.md`**
   - `:102`: remove `"data": null` or show a handler-supplied `data` value.
   - `:239`: change the TypeScript row to "`RPCError` with declared numeric codes"
     unless per-error classes are implemented.

7. **`openspec/specs/ts-contract-verification/spec.md`**
   - `:116-128`: replace the constructor scenarios with
     `await Client.create(transport, false, false, { auditor, verifyOnBootstrap })`
     and state that the auditor is invoked by `verifyCompatibility`, not every RPC
     call.
   - `:50-61`: rename `IContractAuditor` to `ContractAuditor` (and narrow the
     `audit` parameter to `{ compatible, deltas }`), or add a type alias in code.
   - `:78-86`: state that verification requires a local IDL and throws when neither
     `idl.json` nor `setLocalIDL` provided one.

---

## Cases where code may be wrong (disagreements with the user's bias)

1. **Generated client stubs do not validate responses (tscjs-doc-24).**
   `pkg/generator/ts_client_server.go:2029-2048` only checks the truthiness of
   `_resp.error` and returns `_resp.result`. This violates the guide's client
   envelope requirement (`RUNTIME_IMPLEMENTATION_GUIDE.md:888-900`), accepts
   mismatched ids, and treats `error: null` as success. Recommended fix: generate
   clients that call the runtime `Client` (or replicate its envelope checks). This is
   the strongest code-side finding.

2. **`HttpTransport` cannot complete an explicit `id: null` notification
   (tscjs-doc-13).** `transport.ts:211-221` only accepts a 204 when `id` is absent.
   Fix: `if (response.status === 204 && (req.id === undefined || req.id === null))`
   or use `!hasOwnProperty || req.id === null`. Low user impact because
   `Client.notify` omits `id`, but it contradicts the documented notification rule.

3. **`ts-cjs` client envelope errors are plain `Error`, unlike `ts-node`'s
   `RPCError(-32600)` (related to tscjs-doc-08).** `client.ts:267-284` throws
   `Error("Response validation failed: ...")`; `ts-node/client.ts:322-337` throws
   `RPCError(-32600, ...)`. Aligning ts-cjs would give callers a stable `code` and
   match the guide's standard-error-code convention. Low-to-medium priority.

4. **`Client` construction can fail on a malformed local `idl.json` even when
   verification is not requested.** `Client.create` calls `_findIDLJson` before
   bootstrap (`client.ts:96-98`), and `_findIDLJson` propagates JSON parse errors
   (`:149-179`). The spec treats `idl.json` as an optional input for verification
   (`spec.md:78-86`). A catch around the parse (treat malformed/missing as "no local
   IDL") would make the documented optionality true. Low priority; either fix the
   code or document that a malformed `idl.json` is fatal.

Minor parity/typing notes that are not full deltas:

- `ClientOptions.auditor` is `any` in ts-cjs (`client.ts:47-50`) but
  `ContractAuditor` in ts-node (`ts-node/pulserpc/client.ts`), so ts-cjs loses the
  spec's auditor typing even though runtime behavior is fine.
- The ts-cjs package root omits `RPCErrorCode`, which ts-node exports
  (`ts-node/pulserpc/index.ts:1-17`). If package-root parity is intended, add the
  re-export; otherwise document that only the submodule (`./rpc`) exposes it.
- `Client.namedToPositional` drops unknown keys (`client.ts:335-338`) while the
  server rejects them (`server.ts:154-164`), so client validation can still build a
  request the server will reject. Not covered by current docs; a client-side
  unknown-key check would close the gap.

No source code or other documentation files were modified by this audit; only this
report was written.
