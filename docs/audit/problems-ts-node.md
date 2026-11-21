# ts-node — audit findings

## Summary
- Invariants checked: 31 (R1–R14, M1–M17)
- Confirmed violations: 12
- Test gaps: 3
- Scope: `pkg/runtime/runtimes/ts-node/` (source + tests), `pkg/generator/ts_client_server.go` (generated output only), `tests/integration/`, runtime + root Makefiles. All file:line references were read directly; the live behaviors in ts-node-01/02/04/06 were also reproduced with `npx tsx`.

## Problems

### ts-node-01 — `Server.validateResponses` is declared but never used
- Severity: high
- Invariant: R10
- Current behavior: `pulserpc/server.ts:15,31,36` declare/default `validateResponses` (default `true`), but `call()` at `pulserpc/server.ts:163-182` invokes the handler and returns `{jsonrpc:"2.0", result, id:reqId ?? null}` (`:168-172`) without ever calling `this.contract.validateResponse(...)`. `Contract.validateResponse` exists at `pulserpc/contract.ts:269-303` but is only reachable from the client path (`pulserpc/client.ts:338`). Generated test servers explicitly pass `validateResponses: true` (`pkg/generator/ts_client_server.go:2109`), so the flag is silently inert. Reproduced live: a handler returning `123` for a `string` return type was returned unchanged with `validateResponses:true`.
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:425` requires "Validate response against Contract (if enabled)" (constructor contract at `:406-410`).
- Test status: none. `Makefile:15,23` runs six files; none constructs a `Server`. `ts-cjs/pulserpc/tests/test_client.js` (sibling runtime) at least constructs one, but ts-node has no `test_server.ts`/`test_client.ts`.
- Suggested fix: After `await func(...)` and before building the success response, call `this.contract.validateResponse(ifaceName, funcName, result)` when `this.validateResponses` is true; map the thrown `RPCError` to an error response (code `-32603`). Add a server unit test with a wrong-typed handler result.
- Acceptance criteria:
  - [ ] `validateResponses:true` + wrong-typed handler result yields a JSON-RPC error response, not a result.
  - [ ] `validateResponses:false` still returns the wrong-typed result unchanged.

### ts-node-02 — Notifications receive a success response with `id: null`
- Severity: high
- Invariant: R11 / M4
- Current behavior: `pulserpc/server.ts:94-95` detects the missing `id` only by reading `reqId`; it never suppresses the response. The success path at `pulserpc/server.ts:168-172` always returns `id: reqId ?? null`, so an id-less request produces `{"jsonrpc":"2.0","result":…,"id":null}`. The error path (`errorResponse`, `:188-210`) omits `id` but still returns a response object, so a failing notification is also answered. The JSDoc at `:51` promises "null for notification". Reproduced live: `call({jsonrpc:"2.0",method:"Svc.f"})` returned `{"jsonrpc":"2.0","result":123,"id":null}`. The generated adapter's 204/no-body branch for null responses (`pkg/generator/ts_client_server.go:2130-2132`) is therefore dead code.
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:416` — "Returns JSON-RPC response dict or `None` for notifications"; `:814` — "Notifications: Requests without `id` field (no response sent)".
- Test status: none. No server unit test; `tests/integration/test_http_api.sh` sends `id:1` on every request (`:43,45,84,86`) and has no notification case (`:155-241`).
- Suggested fix: After the version/method checks, record whether `id` is present (note `id:0` is a valid id); still execute the handler, but return `null` from `call()` on both success and error paths when the request was a notification.
- Acceptance criteria:
  - [ ] `await server.call({jsonrpc:"2.0",method:"Svc.f",params:[]})` returns `null`.
  - [ ] A notification whose handler throws also returns `null` (no `id:null` error response).
  - [ ] Requests with `id:0` still echo `id:0`.

### ts-node-03 — No unit tests for `Contract`, `Server`, `Client`, or the auditors
- Severity: high
- Invariant: R13 (also M10, M14, R10, R11)
- Current behavior: `Makefile:15` (local) and `:23` (Docker) run exactly six files — `test_rpc.ts`, `test_types.ts`, `test_validation.ts`, `test_transport.ts`, `test_diff.ts`, `test_shared.ts`. A grep of `pulserpc/tests/` finds no reference to `validateRequest`, `validateResponse`, `Server(`, `Client.create`, or `Auditor`. Consequently `Contract.validate` (`pulserpc/contract.ts:210-214`), `validateRequest` (`:219-264`), `validateResponse` (`:269-303`), all three auditors (`:20-61`), `Server.call` (`pulserpc/server.ts:53-183`), and `Client.call`/`verifyCompatibility` (`pulserpc/client.ts:220-252,262-345`) are entirely untested. The sibling runtime has `ts-cjs/pulserpc/tests/test_contract.js` and `test_client.js`; ts-node has neither. All six current files pass, so CI is green despite ts-node-01/02/04/05/06/07.
- Expected behavior: R13 calls for contract-verification tests; `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:843-847` requires testing error handling and edge cases beyond pure validation helpers.
- Test status: none for the named classes.
- Suggested fix: Add `pulserpc/tests/test_contract.ts` (validate/validateRequest/validateResponse, NoOp/Logging/FailFast auditors) and `pulserpc/tests/test_server.ts` + `test_client.ts` (request validation, response validation, notifications, id echo), and list all three in both `Makefile:15` and `:23`.
- Acceptance criteria:
  - [ ] `make test` executes the three new files and fails if any of them is removed.
  - [ ] Contract request/response validation and all three auditors have at least one asserting test each.

### ts-node-04 — Removing a required field is classified `Info`, so compatibility stays true
- Severity: medium
- Invariant: M16
- Current behavior: `pulserpc/diff.ts:305-313` hardcodes `severity: Severity.Info` for every field present in the client but absent from the server, without checking `optional`. `classifySeverity` agrees (`pulserpc/diff.ts:517-519`: Field/Removed/ClientHasMore → Info). `Client.verifyCompatibility` only treats `"Error"` as breaking (`pulserpc/client.ts:233`), so `compatible` stays `true`. Reproduced live: removing a *required* field produced `severity: "Info"`.
- Expected behavior: `openspec/specs/ts-contract-verification/spec.md:104-106` and `openspec/specs/contract-compatibility-verification/spec.md:115-118` — a removed required field yields `compatible:false` and an Error-level delta. (Both specs' summary tables at `ts-contract-verification/spec.md:24` and `contract-compatibility-verification/spec.md:25` say "Field removed → Info"; that contradiction should be resolved in favour of the scenario, which is the acceptance case.)
- Test status: `pulserpc/tests/test_diff.ts:87-117` asserts `Severity.Info` for a removed field, but its field2 is `[optional]` (`:95`), so it enshrines the wrong classification for optional removals and never exercises a required removal. `test_shared.ts:198-207` covers only method removal.
- Suggested fix: In `diffStructFields`, use `isFieldRequired(clientFields[name])` (`pulserpc/diff.ts:369-371`) to emit `Severity.Error` for required removals and `Severity.Info` for optional ones; give `classifySeverity` an `extra` argument that carries the same information. Update the spec tables to match.
- Acceptance criteria:
  - [ ] `diffIDL` where the server removed a required field returns `severity === Severity.Error`.
  - [ ] `verifyCompatibility()` on that IDL returns `compatible === false` and `FailFastAuditor.audit` throws.
  - [ ] Removing an optional field still yields `Severity.Info`.

### ts-node-05 — `null`/`undefined` results bypass client response validation
- Severity: medium
- Invariant: M14
- Current behavior: `pulserpc/client.ts:336` guards response validation with `this.validateResponse && this.contract && result !== undefined && result !== null`. `Contract.validateResponse` would reject `null` for a non-optional return (`pulserpc/contract.ts:294-302`), but the guard prevents the call. There is no server-side counterpart (see ts-node-01), so a required-return handler yielding `null` is never validated anywhere.
- Expected behavior: M14 and `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:889` — "Validation: Validate return values match IDL definition"; only optional returns should accept null (`docs/idl-guide/syntax.md` optional semantics).
- Test status: none — no `test_client.ts` exists (see ts-node-03).
- Suggested fix: Remove the `result !== null` part of the guard at `pulserpc/client.ts:336`, letting `validateResponse` apply `returnOptional` rules; keep the `undefined` guard only if a missing `result` must be treated as absent.
- Acceptance criteria:
  - [ ] With `validateResponse:true`, a required-return method whose handler returns `null` makes `client.call` throw "Response validation failed".
  - [ ] An optional-return method returning `null` still succeeds.

### ts-node-06 — Trailing optional parameters cannot be omitted; the intended logic is dead code
- Severity: medium
- Invariant: M9
- Current behavior: `pulserpc/contract.ts:233-238` throws `-32602` whenever `params.length !== paramDefs.length`, ignoring `paramDef.optional`. `Server.call` calls `validateRequest` first (`pulserpc/server.ts:126-133` for positional params, `:150-161` for named), so the correct trailing-optional range check in `positionalToNamedParams` (`pulserpc/server.ts:242-252`) can never be reached. Reproduced live: a 2-parameter method with the second parameter optional rejected `[1]` with "expects 2 param(s). 1 given."
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:873-877` — optional fields must be handled correctly; the dead code at `server.ts:242-252` documents the intended `requiredCount <= length <= paramDefs.length` rule.
- Test status: none in ts-node. `tests/integration/test_http_api.sh:240-241` only asserts that too many params fail; there is no omitted-optional case.
- Suggested fix: Change `pulserpc/contract.ts:233` to accept `params.length` between the required count (`paramDefs.filter(p => !p.optional).length`) and `paramDefs.length`, then remove or reuse the now-redundant server-side helper at `server.ts:242-252`.
- Acceptance criteria:
  - [ ] `contract.validateRequest` accepts a short array when the omitted trailing params are optional.
  - [ ] Missing required params and extra params still raise `-32602`.

### ts-node-07 — No response-envelope validation; `error` silently wins and code `0` is remapped
- Severity: medium
- Invariant: M5
- Current behavior: `pulserpc/client.ts:315-344` awaits the transport response and immediately checks `response.error` (`:323-330`), throwing `new RPCError(error.code || -32603, …)` — the `||` remaps a legitimate code `0` to `-32603`. It never checks `response.jsonrpc`, the echoed `id`, or result/error exclusivity; a response carrying both `result` and `error` silently takes the error branch. `InProcTransport` manufactures a response with no `jsonrpc` field (`pulserpc/transport.ts:111-117`).
- Expected behavior: M5 / `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:810-815` — full JSON-RPC 2.0 compliance, response format at `:812`, standard error codes at `:815`.
- Test status: none — no client tests, and `pulserpc/tests/test_transport.ts` only checks URL pass-through (`:29-46`) and the HTTP-error wrapper (`:48-66`).
- Suggested fix: In `Client.call`, validate `response.jsonrpc === "2.0"`, compare `response.id` to `reqId`, reject responses with both `result` and `error`, and use `??` instead of `||` for the code fallback.
- Acceptance criteria:
  - [ ] A response with a mismatched `id` or wrong `jsonrpc` makes `client.call` throw.
  - [ ] A response with both `result` and `error` is rejected rather than silently taking the error branch.
  - [ ] An error with `code: 0` propagates as code `0`.

### ts-node-08 — IDL `meta` items are never parsed
- Severity: medium
- Invariant: R12
- Current behavior: `pulserpc/contract.ts:141-179` builds `interfaces`, `structs`, `enums` for both PulseRPC-dict and Barrister-array IDL forms; the array branch (`:166-177`) handles only `type === 'struct' | 'enum' | 'interface'`. There is no `meta` field on `Contract` and a grep for `meta` in the runtime source finds only doc comments. Sibling runtimes do parse it: `pkg/runtime/runtimes/python/pulserpc/contract.py:179-183`, `python2/pulserpc/contract.py:66-69` (and Go/Java/C# per catalog R12).
- Expected behavior: R12 — TS should parse `meta` items like the other five runtimes.
- Test status: `pulserpc/tests/test_shared.ts:238-246` constructs a Barrister-format IDL but includes no `meta` item, and no test asserts metadata retention.
- Suggested fix: Add an optional `meta` map to `Contract`, handle `itemType === 'meta'` in both constructor branches, and export an accessor; add a test with a Barrister `meta` item.
- Acceptance criteria:
  - [ ] `new Contract([{type:"meta", namespace:"x"}, …]).meta.namespace === "x"` (or the agreed accessor).
  - [ ] Both dict-form and array-form IDL JSON with meta items parse without throwing.

### ts-node-09 — HTTP API harness warns instead of failing on wrong error codes; no notification/batch cases
- Severity: low
- Invariant: M17
- Current behavior: `tests/integration/test_http_api.sh:127-142` prints `⚠ WARN` when the observed error code differs from `expected_error_code` and then falls through to `✓ PASS` / `TESTS_PASSED++` (`:140-142`). The missing-`jsonrpc` check at `:222-232` only requires the presence of `"error"`, with no `-32600` assertion. There are no batch or notification requests anywhere in the script (`:155-241`), and every request carries `id:1` (`:43,45,84,86`). This script is invoked for the TS styles by `tests/integration/test_generator_ts.sh:183-186`.
- Expected behavior: M17 — wrong JSON-RPC error codes should fail the suite; `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:853-854` lists notifications (and `:813` batches) as test cases.
- Test status: none that asserts codes or notification silence; see ts-node-02.
- Suggested fix: Return non-zero / increment `TESTS_FAILED` at `:128-137` on a code mismatch, assert `-32600` for the missing-`jsonrpc` case, and add notification + batch cases.
- Acceptance criteria:
  - [ ] A server returning `-32000` where `-32602` is expected makes `test_http_api.sh` exit non-zero.
  - [ ] `test_generator_ts.sh` propagates that failure for esm-node (it already does at `:196-199`).

### ts-node-10 — `HttpTransport` uses nonstandard `-32000` for transport failures
- Severity: low
- Invariant: R8
- Current behavior: `pulserpc/transport.ts:80` (non-OK HTTP), `:85` (empty body), `:91` (invalid JSON) all throw `RPCError(-32000, …)`. `pulserpc/tests/test_transport.ts:61` asserts that code, enshrining it. No named error-code constants exist anywhere in the runtime (codes are literals).
- Expected behavior: R8 / `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:815` — standard JSON-RPC codes `-32700, -32600, -32601, -32602, -32603`. (`-32000` is inside the JSON-RPC server-error range per `docs/idl-guide/syntax.md:65`, but is outside the documented set and unique to the TS transports.)
- Test status: `test_transport.ts:48-66` asserts `-32000`, so the divergence is actively locked in.
- Suggested fix: Use `-32603` for transport/parse failures (or introduce named constants) and update `test_transport.ts:61` to assert the chosen standard code.
- Acceptance criteria:
  - [ ] Every `RPCError` from `HttpTransport.request` uses a code from the documented standard set.
  - [ ] Tests assert the code for non-OK responses, empty bodies, and invalid JSON.

### ts-node-11 — Generated HTTP adapter has no batch-request handling
- Severity: low
- Invariant: M7
- Current behavior: `pkg/generator/ts_client_server.go:2128-2129` does `const data = JSON.parse(body); const response = await this.rpcServer.call(data);` — a single object only. No `batch`/array handling exists in the generator; the generated production `server.ts` is abstract-interface-only (`ts_client_server.go:1437-1438`), so this test-server adapter is the only HTTP binding exercised by `tests/integration/test_generator_ts.sh:120-129`.
- Expected behavior: M7 / `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:813` ("Batch requests: Array of requests") and the checklist at `:1212` ("Server handles batch requests").
- Test status: none; `tests/integration/test_http_api.sh` has no batch case (see ts-node-09).
- Suggested fix: In the generated handler, if `Array.isArray(data)`, map `rpcServer.call` over elements, drop `null` notification responses, and return an array (or 204 when empty).
- Acceptance criteria:
  - [ ] A POST with an array of two requests returns an array of two responses.
  - [ ] A batch containing only notifications returns no body (204).

### ts-node-12 — `verifyCompatibility` compares severity with a raw string instead of the `Severity` enum
- Severity: low
- Invariant: M16 (severity handling)
- Current behavior: `pulserpc/types.ts:26-30` defines the `Severity` enum and `contract.ts:9` imports it; the auditors use the constants (`pulserpc/contract.ts:33-38,53`). But `pulserpc/client.ts:233` uses `d.severity === "Error"`, and `client.ts:16` does not import `Severity`. Functionally equal today, but a value change would deserialise the compatibility decision from the auditors.
- Expected behavior: A single source of truth for severity across diff, client, and auditors (catalog's ts-node/ts-cjs distinction: typed enum in auditors).
- Test status: none — no test calls `verifyCompatibility`.
- Suggested fix: Import `Severity` from `./types.js` in `client.ts` and compare `d.severity === Severity.Error`; add a `verifyCompatibility` test using enum-valued deltas.
- Acceptance criteria:
  - [ ] No `severity === "..."` string comparisons remain in ts-node source.
  - [ ] A test asserts `verifyCompatibility()` flips `compatible` only for `Severity.Error` deltas.

### ts-node-13 — Void returns accept `undefined` in addition to `null`
- Severity: low
- Invariant: R14
- Current behavior: `pulserpc/contract.ts:284` treats a void return as valid when `result === null || result === undefined` (`result !== null && result !== undefined` throws). The catalog's other five runtimes accept only null/nil/None. In JSON-RPC over HTTP `undefined` cannot appear, but `InProcTransport` (`pulserpc/transport.ts:101-119`) can return it.
- Expected behavior: R14 — uniform void-return acceptance across runtimes (null only).
- Test status: none — no `validateResponse` test exists (see ts-node-03).
- Suggested fix: Decide the canonical value (recommend `null` only) and change `contract.ts:284` accordingly, or document the JS-specific `undefined` tolerance explicitly.
- Acceptance criteria:
  - [ ] `validateResponse("Svc","voidFn", null)` succeeds and `validateResponse("Svc","voidFn", undefined)` fails (if null-only is chosen).
  - [ ] The chosen behavior is asserted in `test_contract.ts`.

### ts-node-14 — No test asserts `int` rejection of NaN/Infinity (the ts-node/ts-cjs split)
- Severity: low
- Invariant: R2
- Current behavior: `pulserpc/validation.ts:30-35` explicitly rejects `NaN` and non-finite numbers for `int` (live-verified: one error each); ts-cjs accepts ±Infinity (`ts-cjs/pulserpc/validation.ts:33`, per `docs/audit/problems-ts-cjs.md:11-20`). `validateFloat` accepts NaN/Infinity at `pulserpc/validation.ts:45-50`, matching the other TS runtime. No test asserts any of this.
- Expected behavior: R2 — identical scalar acceptance sets across runtimes; the divergence must be visible to CI.
- Test status: `pulserpc/tests/test_validation.ts:37-56` covers only `0/42/-100/5.0/3.14/5.1`; `test_shared.ts:30-40,63-72` covers `42/3.14/null/undefined`. Neither mentions NaN, Infinity, or `JSON.parse("1e999")`.
- Suggested fix: Add NaN, Infinity, -Infinity, and `JSON.parse("1e999")` cases to both `test_validation.ts` and the shared suite so ts-cjs inherits the same assertion and its bug becomes a failing test.
- Acceptance criteria:
  - [ ] Each of NaN/±Infinity yields one error from `validateInt` in an asserting test.
  - [ ] The shared suite fails when run against the current ts-cjs `validation.js`.

### ts-node-15 — Validation error `data` shape does not match the documented `{field, message}`
- Severity: low
- Invariant: M13
- Current behavior: `pulserpc/contract.ts:120-133` builds `{valid, error, invalidFields}` via `buildValidationResult`, and that object becomes the `RPCError.data` on request failure (`:257-263`) and response failure (`:296-302`). `RPCError.data` is typed `any` (`pulserpc/rpc.ts:7,13`), so no structure is enforced.
- Expected behavior: `docs/idl-guide/validation.md:229-245` documents RPC validation errors with `data: {"field":"email","message":"Required field 'email' is missing"}`.
- Test status: none — no contract/server tests assert the `data` payload (see ts-node-03).
- Suggested fix: Either emit the documented `{field, message}` shape (one per error, or an array) from `buildValidationResult` callers, or amend `validation.md:229-245` to document `ValidationResult` as the actual TS payload and align all runtimes.
- Acceptance criteria:
  - [ ] A failing `validateRequest` produces an `RPCError.data` matching the documented shape (or the docs are updated to match the implementation).
  - [ ] A test asserts that shape.

## Verified-correct invariants

- **R1** — `pulserpc/index.ts:15-16` re-exports all of `types.js` and `validation.js` (findStruct/findEnum/getStructFields/extractChecksum and every scalar+aggregate validator); ts-node is the correct side of the catalog's divergence.
- **R2 (ts-node side)** — `validateInt` rejects NaN and ±Infinity (`validation.ts:30-35`, live-verified); ts-cjs is the divergent runtime. `validateFloat` accepts NaN/Infinity (`:45-50`) as in other TS runtimes. The gap is the missing test (ts-node-14).
- **R3** — Errors accumulate as `ValidationError[]` with paths (`validation.ts:67-72,123-151`); multi-error assertion at `test_validation.ts:246-269`.
- **R4** — Missing required (`:131-134`), missing optional (`:131-133`), null (`:137-140`) and undefined (`:141-144`) handling correct; tests `test_validation.ts:186-218,278-295`.
- **R5** — Parent fields first, child overrides (`types.ts:83-113`); tests `test_types.ts:50-86`, `test_shared.ts:76-112,131-143` pass.
- **R6** — Exact lookup (`types.ts:75-81`) is correct for generated IDL: the parser qualifies imported definitions *and* references identically (`parser.go:650,660,729`), so `inc.MathOp`/`inc.Response` resolve. Catalog's "suffix-scan" divergence does not produce a ts-node failure here.
- **R7** — `RPCError(code, message, data)` at `rpc.ts:5-18`; `test_rpc.ts:8-37` asserts code/message/data.
- **M1** — `jsonrpc !== "2.0"` → `-32600` (`server.ts:65-72`); integration `test_http_api.sh:222-232` (assertion only checks presence of `error`, see ts-node-09).
- **M2** — Method presence and non-empty string check (`server.ts:75-83`), so the Java-style NPE risk does not apply.
- **M3** — `Interface.method` parsed with `lastIndexOf(".")` and unknown interface/method → `-32601` (`server.ts:98-116`); `test_http_api.sh:235-237`.
- **M6** — Generated test-server adapter emits `-32700` parse errors (`pkg/generator/ts_client_server.go:2137-2144`).
- **M8** — Non-object/array `params` rejected with `-32602` (`server.ts:145-147`).
- **M10 (code)** — Per-parameter IDL validation exists (`contract.ts:242-255`) and is invoked from `server.ts:128,155`; only the test coverage is missing (ts-node-03).
- **M11** — Enum membership (`validation.ts:95-108`), nested/recursive validation (`test_validation.ts:323-361`), fractional-int rejection (`validation.ts:36-42`, `test_validation.ts:50-55`).
- **M12** — Extra/unknown fields are ignored intentionally (`validation.ts:125-149` iterates declared fields only).
- **M15** — `Client.verifyCompatibility` diffs the server IDL against a separate local IDL (`client.ts:225-237`, loaded at `:183-214` or via `setLocalIDL` at `:216-218`), not an IDL against itself.
- **R13 (diff)** — `test_diff.ts` exists and *does* run: listed in both targets at `Makefile:15,23`, with 14 asserting scenarios (`test_diff.ts:5-247`). The gap is Contract/Server/Client tests (ts-node-03), not diff tests.
- **Key-context checks** — ts-node uses typed `Severity` in auditors (`contract.ts:33-38,53`) except the one string comparison in ts-node-12; `test_diff.ts` present; `validateResponses` inert (ts-node-01); notifications always return `id:null` (ts-node-02); no `meta` parsing (ts-node-08); void returns accept both null and undefined (ts-node-13). All confirmed against source.
