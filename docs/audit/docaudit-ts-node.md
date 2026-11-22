# Doc audit: ts-node runtime (documentation vs implementation)

## Scope & methodology

Audited the user-facing TypeScript documentation
(`docs/languages/typescript/reference.md`, `docs/languages/typescript/quickstart.md`,
`docs/RUNTIME_IMPLEMENTATION_GUIDE.md` TS-relevant sections, `docs/idl-guide/*`)
against the ts-node runtime implementation
(`pkg/runtime/runtimes/ts-node/pulserpc/*`) and the TypeScript sections of
`pkg/generator/ts_client_server.go`. Focus areas: exports (R1), scalar acceptance of
`Infinity`/`NaN`, the `-32000` transport code, notification handling, response
validation, `meta` items, void returns, envelope validation, params shape, exact
param count, and structured validation data.

Methodology: read every runtime source file and the generated-adapter sections of
the generator, then compared each documented claim in the focus areas to the
current code. Claims were confirmed against source (and, where the existing
runtime tests already cover them, against those tests). No source code was
modified; only this report was written. Bias applied throughout: documentation
should change to match the implementation unless the implementation is clearly
wrong, in which case the code defect is called out separately.

## Summary (counts by severity)

- Critical: 0
- High: 0
- Medium: 2
- Low: 3
- Confirmed doc/code mismatches: 5
- Test gaps: 5
- Verified as holding: 11 focus areas

## Confirmed issues

### doc-ts-node-01 — Guide says generated TypeScript (and Python) servers do not support batches

- Severity: medium
- Location: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:907-910`
- Doc claim: "Batch requests (an array of requests) are optional for generated
  adapters. The generated Go, C# and Java servers support them; generated
  TypeScript and Python servers do not."
- Code reality: the generated TypeScript test adapter (`pkg/generator/ts_client_server.go`,
  `generateTestServerTs`) implements bounded batches with a 100-entry cap, empty-batch
  handling (`-32600 "Invalid Request - empty batch"`), per-entry `-32600` for
  non-object entries, notification dropping, and HTTP 204 for all-notification
  batches. The Python generated adapter is likewise batch-aware
  (`pkg/runtime/runtimes/python/pulserpc/server.py`, `Server.call`/`_call_batch`,
  `DEFAULT_MAX_BATCH_SIZE = 100`).
- Recommendation: update the guide to list TypeScript and Python among the
  batch-supporting generated servers. (Docs change.)

### doc-ts-node-02 — Guide's Contract class omits `meta` parsing that the code implements

- Severity: low
- Location: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` §"Contract Class" (~lines 236-260)
- Doc claim: the `Contract` class exposes `interfaces`, `structs`, `enums` only;
  no `meta` field is documented.
- Code reality: `pkg/runtime/runtimes/ts-node/pulserpc/contract.ts` parses `meta`
  in both the dict form (`idlParsed.meta` as object or array of
  `{type:"meta", ...}` items) and the Barrister list form (`itemType === 'meta'`),
  stores it in `Contract.meta`, and guards against prototype pollution via
  `UNSAFE_META_KEYS` (`__proto__`, `constructor`, `prototype`)
  (`contract.ts:21,128-141,154-229`). Covered by `test_contract.ts` (meta cases).
- Recommendation: document `Contract.meta` and the supported `meta` item shapes in
  the guide and/or the TypeScript reference. (Docs change.)

### doc-ts-node-03 — Transport 204 handling disagrees with the documented `id: null` notification rule

- Severity: medium
- Location: `docs/languages/typescript/reference.md:488` ("A request with `id: null`
  (or no `id` member) is a notification"); `pkg/runtime/runtimes/ts-node/pulserpc/transport.ts:210-220`
- Doc claim: a request with `id: null` or no `id` member is a notification.
- Code reality: the server treats both as notifications and answers with HTTP 204
  (`server.ts:58-67`), but `HttpTransport.request` only converts a 204 into a
  neutral envelope when the request has no `id` property
  (`!Object.prototype.hasOwnProperty.call(req, "id")`). A request carrying
  `id: null` therefore surfaces as `RPCError "Empty response body"`. Client
  notifications avoid this by omitting `id` entirely (`client.ts:289-296`).
- Recommendation: this is a code defect rather than a doc error — either handle
  `id: null` in the 204 branch as well, or amend the docs to state that clients
  omit `id` for notifications and that an explicit `id: null` request is not
  supported over HTTP. Preferred fix is the code change (handle `id: null`).
  (Code should change.)

### doc-ts-node-04 — Reference lists only two of the runtime's exported constants

- Severity: low
- Location: `docs/languages/typescript/reference.md:523`
- Doc claim: "Constants are exported by the runtime: `HTTP_TIMEOUT_MS` and
  `MAX_RESPONSE_BYTES`."
- Code reality: `pkg/runtime/runtimes/ts-node/pulserpc/index.ts` additionally
  exports `RPCErrorCode`, `normalizeBaseUrl`, `Transport`/`HttpTransport`/
  `InProcTransport`, `Contract`, `Client`, `Server`, the full `types.js` and
  `validation.js` surfaces (`validateType`, `findStruct`, `findEnum`,
  `getStructFields`, `extractChecksum`, all scalar/aggregate validators), `diffIDL`,
  `classifySeverity`, and the auditors. The statement is not false, but it
  under-documents the public surface.
- Recommendation: expand the exports section (or add an API reference) so the
  documented surface matches `index.ts`. (Docs change.)

### doc-ts-node-05 — D1 wording can be read as rejecting `NaN`/`Infinity` for `float`

- Severity: low
- Location: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` D1 (~line 55);
  `pkg/runtime/runtimes/ts-node/pulserpc/validation.ts:53-58`
- Doc claim: D1 states that fractional numbers, `NaN`/`Infinity`, booleans and
  out-of-range values are invalid "in every runtime" (in the context of `int`).
- Code reality: `validateInt` rejects `NaN`/`Infinity` and fractions, but
  `validateFloat` accepts any `number`, including `NaN` and `Infinity`
  (`validation.ts:53-58`), matching the other TypeScript runtime and the
  IEEE-754-double statement (D26/R26). Python's `validate_float` likewise
  accepts them.
- Recommendation: clarify in D1 that the rejection applies to `int` only, and
  that `float` follows IEEE-754 (so `NaN`/`Infinity` pass runtime validation
  even though they are not representable in standard JSON). (Docs change.)

## Test gaps

- `pulserpc/tests/test_transport.ts` covers the 204 notification path only for a
  request with no `id` property; there is no assertion for a 204 returned to a
  request carrying an `id` (expected to surface as an empty-body error), nor for
  `id: null`.
- `pulserpc/tests/test_client.ts` does not exercise `Client.ready()` or
  `Client.notify()` directly.
- `pulserpc/tests/test_types.ts` does not assert `getStructFields()` returns `[]`
  for an unknown struct name, nor `extractChecksum` for a non-object IDL root.
- `pulserpc/tests/test_server.ts` does not assert that the `pulserpc-idl` method
  is suppressed for a notification (`call()` returns `null`), nor that a `null` id
  on a non-notification error path is echoed as `null`.
- No test covers `HttpTransport` handling of a 204 response to a request with
  `id: null` (the doc-ts-node-03 scenario).

## Verified as holding

- **Exports (R1)** — `pulserpc/index.ts` re-exports `RPCError`/`RPCErrorCode`,
  `Contract` (and its types), `Client`/`ClientOptions`, `Server`,
  `Transport`/`HttpTransport`/`InProcTransport`, `HTTP_TIMEOUT_MS`,
  `MAX_RESPONSE_BYTES`, `normalizeBaseUrl`, `JsonRpcRequest`/`JsonRpcResponse`,
  the full `types.js` and `validation.js` surfaces, `diffIDL`/`classifySeverity`,
  and the three auditors.
- **Scalar acceptance (`Infinity`/`NaN`)** — `validateInt` rejects non-numbers,
  `NaN`, non-finite values, fractions and out-of-range values; `validateFloat`
  accepts any `number` (`validation.ts:34-58`).
- **`-32000` transport code** — runtime uses standard `-32603`
  (`RPCErrorCode.InternalError`); no `-32000` remains in source
  (`rpc.ts`, `transport.ts`).
- **Notification handling** — `Server.call` returns `null` for a missing or
  `null` id on both success and error paths; `id: 0` is echoed
  (`server.ts:53-67`).
- **Response validation** — `Server.callImpl` invokes `contract.validateResponse`
  when `validateResponses` is true and converts failures to `-32603 "Internal
  error"` (`server.ts:217-224`); the client validates whenever `result !==
  undefined` (`client.ts:357`).
- **`meta` items** — parsed in both dict and Barrister forms with prototype-
  pollution guards (`contract.ts:154-229`).
- **Void returns** — `validateResponse` requires `null` (not `undefined`) when
  there is no return type (`contract.ts:356-366`).
- **Envelope validation** — the client checks `jsonrpc === "2.0"`, id match,
  result/error exclusivity and presence, and preserves a legitimate code `0`
  (`client.ts:322-349`).
- **Params shape** — arrays and objects accepted; non-object/non-array rejected
  with `-32602`; object form mapped by IDL parameter name with unknown keys
  rejected (`server.ts:155-190`).
- **Exact param count** — `checkParamCount` accepts any count between the
  required count and the full signature (`contract.ts:128-141`).
- **Structured data** — `validateRequest` throws `RPCError` with `-32602` and a
  `ValidationResult` (`valid`/`error`/`invalidFields`) as `data`; response
  validation failures carry the same shape under `-32603`
  (`contract.ts:273-338`, `validation.ts`).

## Unconfirmed hypotheses

- `HttpTransport` 204 handling for `id: null` (doc-ts-node-03): either the code
  or the docs must move; the preferred resolution is a code change.
- `pulserpc/client.ts:279-296` request validation for named params ignores
  unknown keys (the server rejects them), so a client with validation enabled
  can construct a request the server will reject. Severity low; not covered by
  tests.
- `validateFloat` accepts `NaN`/`Infinity`, which cannot be produced by
  `JSON.parse`. Whether the runtime should reject them (as some other runtimes
  do not) is not settled by the docs; current code matches the other TypeScript
  runtime and Python.

## Notes

- The user-facing TypeScript docs (`reference.md`, `quickstart.md`) are largely
  consistent with the current ts-node implementation for the focus areas; the
  main documentation defect is the stale batch-support statement in the
  implementation guide (doc-ts-node-01). The `meta` and exports gaps are
  documentation-coverage issues rather than contradictions.
- Older audit documents under `docs/audit/` (e.g. `invariants.md`,
  `invariants-additional.md`, `problems-ts-node.md`) still describe pre-remediation
  ts-node behavior (nonstandard `-32000`, inert `validateResponses`,
  notifications returning `id: null`, missing `meta` parsing). Those are
  historical audit artifacts, not the user-facing docs in scope here, and were not
  modified.
- No source files were modified by this audit; only this document was written.
EOF