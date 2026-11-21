# ts-cjs — audit findings

## Summary
- Invariants checked: 31 (R1–R14, M1–M17)
- Confirmed violations: 13
- Test gaps: 5
- Scope: `pkg/runtime/runtimes/ts-cjs/`, `pkg/generator/ts_client_server.go` (generated output only), `tests/integration/`, runtime + root Makefiles. All file:line verified by reading source.

## Problems

### ts-cjs-01 — `validateInt` accepts ±Infinity
- Severity: high
- Invariant: R2 (also M11)
- Current behavior: `pkg/runtime/runtimes/ts-cjs/pulserpc/validation.ts:26-37` checks only typeof, `Number.isInteger`, and `value === Math.floor(value)`. Since `Infinity === Math.floor(Infinity)`, ±Infinity pass. ts-node explicitly rejects non-finite values at `ts-node/pulserpc/validation.ts:30-35`. `JSON.parse("1e999")` yields Infinity, so this is reachable from ordinary JSON input.
- Expected behavior: R2 requires identical scalar acceptance sets; all other runtimes reject non-finite ints. Add `Number.isNaN`/`Number.isFinite` guards as in `ts-node/pulserpc/validation.ts:30-35`.
- Test status: `ts-cjs/pulserpc/tests/test_validation.js:36-55` tests only 0/42/-100/5.0/3.14/5.1; `test_shared.ts:30-40` only 42/3.14. Neither asserts Infinity/NaN behavior, so the ts-node/ts-cjs split is invisible.
- Suggested fix: Port the two guards from ts-node into `validateInt` before the integer checks. Add Infinity, -Infinity, and NaN cases to both `test_validation.js` and the shared suite so parity is enforced.
- Acceptance criteria:
  - [ ] `validateInt(Infinity)`, `validateInt(-Infinity)` and `validateInt(NaN)` each return one error in ts-cjs.
  - [ ] `validateInt(1e999)` (post-`JSON.parse`) is rejected by a test in `test_validation.js` and `test_shared.ts`.

### ts-cjs-02 — `Server.validateResponses` is declared but never called
- Severity: high
- Invariant: R10
- Current behavior: `ts-cjs/pulserpc/server.ts:15,25,31` declare/default `validateResponses`, but `call()` at `server.ts:134-141` returns the handler result directly without calling `this.contract.validateResponse(...)`. `Contract.validateResponse` exists at `contract.ts:216-246` but is only reachable from the client path (`client.ts:278-284`). Generated test servers pass `validateResponses: true` (`pkg/generator/ts_client_server.go:2109`), so the flag is silently inert.
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:425` requires "Validate response against Contract (if enabled)"; `:441-459` shows the flag governing that call.
- Test status: none. `tests/test_client.js:57,89` construct `Server` with `validateRequests:false` and never assert response validation; no `test_server.js` exists.
- Suggested fix: After `await func(...)` and before building the success response, call `this.contract.validateResponse(ifaceName, funcName, result)` when the flag is true, mapping thrown `RPCError` to an error response (code -32603). Add a server-level test with a handler returning the wrong type.
- Acceptance criteria:
  - [ ] A Server with `validateResponses:true` and a handler returning a wrong-typed result yields a JSON-RPC error, not a result.
  - [ ] A Server with `validateResponses:false` still returns the wrong-typed result unchanged.

### ts-cjs-03 — Notifications get a success response with `id: null`
- Severity: high
- Invariant: R11 / M4
- Current behavior: `ts-cjs/pulserpc/server.ts:75` reads `reqId = req.id`; the success path at `server.ts:137-141` always returns `{jsonrpc, result, id: reqId ?? null}`, so a request with no `id` produces a response with `id:null`. `call()` is typed `Promise<JsonRpcResponse | null>` (`:38`) but never returns `null`. Error responses do suppress id (`errorResponse`, `:165-167`), so behavior is inconsistent. The generated HTTP adapter's 204 branch for null responses (`ts_client_server.go:2130-2132`) is therefore dead code for cjs.
- Expected behavior: `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:416` — "Returns JSON-RPC response dict or `None` for notifications"; `:814` — notifications produce no response. The ts-node server has the same defect (`ts-node/pulserpc/server.ts:94,168-172`).
- Test status: none. No server unit test; `test_client.js` never sends a request without `id`.
- Suggested fix: Detect the absence of `id` after version/method checks; still execute the handler but return `null` from `call()` for both success and error paths. Ensure generated adapters keep a 204/no-body path.
- Acceptance criteria:
  - [ ] `await server.call({jsonrpc:"2.0", method:"Svc.f", params:[]})` returns `null` for a valid handler invocation.
  - [ ] A notification whose handler throws also returns `null` (no `id:null` error response emitted).
  - [ ] Requests with `id:0` still echo `id:0`.

### ts-cjs-04 — Removing a required field is classified Info, so compatibility stays true
- Severity: medium
- Invariant: M16
- Current behavior: `ts-cjs/pulserpc/diff.ts:281-291` hardcodes `severity: Severity.Info` for every field removed from the server, without checking `optional`. `classifySeverity` agrees (`diff.ts:481-483` returns Info for Field/Removed/ClientHasMore). `Client.verifyCompatibility` only looks for `"Error"` (`client.ts:197-199`), so the incompatibility is reported as compatible.
- Expected behavior: `openspec/specs/contract-compatibility-verification/spec.md:117-118` — a removed required field yields `Compatible=false` and an Error-level delta. (Note the contradiction with the table at `:25`; the spec scenario is the authoritative acceptance case.)
- Test status: `test_shared.ts:198-207` covers only method removal; `ts-node/pulserpc/tests/test_diff.ts:87-116` asserts the wrong Info classification for a removed field. No ts-cjs test distinguishes required vs optional removed fields.
- Suggested fix: In `diffStructFields`, use `isFieldRequired(clientFields[name])` to choose Error for required removals and Info for optional removals; mirror in `classifySeverity` with an `extra` argument. Resolve the spec contradiction in the spec table as well.
- Acceptance criteria:
  - [ ] `diffIDL` on a struct where server removed a required field produces a delta with `severity === "Error"`.
  - [ ] `verifyCompatibility()` for that IDL returns `compatible === false` and `FailFastAuditor.audit` throws.
  - [ ] Removing an optional field still yields Info.

### ts-cjs-05 — Null/undefined results bypass client response validation
- Severity: medium
- Invariant: M14
- Current behavior: `ts-cjs/pulserpc/client.ts:278` guards response validation with `result !== undefined && result !== null`. `Contract.validateResponse` (`contract.ts:216-246`, via `validation.ts:156-166`) would reject null for a non-optional return, but the guard prevents the call. Void returns are separately handled at `contract.ts:228-235`.
- Expected behavior: M14 requires null results for required returns to be validated; only optional returns should accept null.
- Test status: `test_contract.js:140-176` calls `contract.validateResponse` directly and never exercises the `client.call` guard; `test_client.js` has no response-validation assertions.
- Suggested fix: Remove the `result !== null` part of the guard (keep `result !== undefined` only if needed to avoid validating absent results), letting `validateResponse` apply optionality rules. Add a client test with a required return whose handler returns null.
- Acceptance criteria:
  - [ ] With `validateResponse:true`, a required-return method returning `null` causes `client.call` to throw "Response validation failed".
  - [ ] An optional-return method returning `null` still succeeds.

### ts-cjs-06 — Trailing optional parameters cannot be omitted; optional-count logic is unreachable
- Severity: medium
- Invariant: M9
- Current behavior: `ts-cjs/pulserpc/contract.ts:184` throws `-32602` whenever `params.length !== paramDefs.length`, regardless of `paramDef.optional`. The server's `positionalToNamedParams` contains the correct trailing-optional range check at `server.ts:191-200`, but `validateRequest` is called first (`server.ts:100-107`) and rejects the short array before that code can run.
- Expected behavior: Optional trailing parameters may be omitted; `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:140,682,877` require optional fields to be handled, and the dead code at `server.ts:191-200` shows the intended rule.
- Test status: `test_contract.js:95-120` asserts that `[1]` throws for a 2-parameter method, enshrining the exact-count behavior; no test uses an optional parameter.
- Suggested fix: Change `contract.ts:184` to check `params.length < requiredCount || params.length > paramDefs.length` (as the server helper already does), validation of provided values remains at `:192-205`. Update or add a test for omitted optional params.
- Acceptance criteria:
  - [ ] `contract.validateRequest` accepts a short array when the omitted trailing params are optional.
  - [ ] Missing required params and extra params still raise `-32602`.

### ts-cjs-07 — No response-envelope validation; `error` silently wins and code 0 is remapped
- Severity: medium
- Invariant: M5
- Current behavior: `ts-cjs/pulserpc/client.ts:261-286` reads `response.error` first and throws (`:267-274`) without checking `response.jsonrpc`, the echoed `id`, or result/error exclusivity. `error.code || -32603` (`:270`) remaps a legitimate code `0` to `-32603`. The same pattern is in the transport fallback (`transport.ts:87-90`), which manufactures a response with no `jsonrpc` field.
- Expected behavior: M5 / `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:810-815` require response-envelope validation (jsonrpc, matching id, result XOR error).
- Test status: `test_client.js:70-109` only checks that an RPCError with code 42 propagates; no malformed-envelope test.
- Suggested fix: Validate `jsonrpc === "2.0"`, compare echoed id to the request id, reject responses carrying both `result` and `error`, and use `??` instead of `||` for the code fallback.
- Acceptance criteria:
  - [ ] A response with a mismatched id or wrong jsonrpc causes `client.call` to throw.
  - [ ] A response with both `result` and `error` is rejected rather than silently taking the error branch.
  - [ ] An error with `code: 0` propagates as code 0.

### ts-cjs-08 — IDL `meta` items are never parsed
- Severity: medium
- Invariant: R12
- Current behavior: `ts-cjs/pulserpc/contract.ts:120-143` iterates `interfaces`, `structs`, `enums` (and array `type` items) but has no `meta` branch; `grep` finds no `meta` handling anywhere in the ts-cjs runtime. Sibling runtimes do: `go/pulserpc/contract.go:112-113`, `java/.../Contract.java:115-119`, `python/pulserpc/contract.py:179-183`, `python2/pulserpc/contract.py:66-69`, `csharp/PulseRPC/Contract.cs:163-164`.
- Expected behavior: R12 — TS should parse `meta` items like the other five runtimes.
- Test status: none; no contract test asserts metadata is retained.
- Suggested fix: Add `meta` to both parsing branches in `Contract` (accumulate key/value pairs on the contract), export a `meta` map analogous to other runtimes, and add a test using a generated `idl.json` containing a `meta` item.
- Acceptance criteria:
  - [ ] `new Contract({meta:[{key:"k",value:"v"}], ...}).meta.k === "v"` (or the agreed accessor).
  - [ ] Both object-form and array-form IDL JSON parse `meta` without throwing.

### ts-cjs-09 — No dedicated diff test file; most diff scenarios exist only in ts-node
- Severity: medium
- Invariant: R13
- Current behavior: `ts-cjs/Makefile:23-29` runs seven files: `test_rpc.js`, `test_types.js`, `test_validation.js`, `test_contract.js`, `test_client.js`, `test_transport.js`, `test_shared.ts`. There is no `test_diff.js`, while `ts-node/Makefile:15` runs `test_diff.ts`. The shared suite has only three diff cases plus `classifySeverity` (`test_shared.ts:177-220`), missing ts-node's 14 scenarios (`ts-node/pulserpc/tests/test_diff.ts:26-247`: added optional/required field, removed field, made optional/required, struct removed, interface added, etc.).
- Expected behavior: R13 asks for contract-verification (diff) tests; parity with the sibling TS runtime is the natural baseline.
- Test status: partial as described; importantly the ts-cjs Makefile *does* run its tests (unlike some other runtimes' test exclusions), so this is a missing-coverage gap, not a "tests excluded" gap.
- Suggested fix: Add `pulserpc/tests/test_diff.js` porting ts-node's `test_diff.ts` (with CJS `require` paths), and list it in both `test` and `test-docker` targets (`ts-cjs/Makefile:23-29,36`).
- Acceptance criteria:
  - [ ] `make test` in `ts-cjs` executes a `test_diff.js` covering all 14 ts-node diff scenarios.
  - [ ] Both `test` and `test-docker` targets include the new file.

### ts-cjs-10 — `make clean` deletes the checked-in `.js` test sources
- Severity: medium
- Invariant: R13 (test-suite integrity)
- Current behavior: `ts-cjs/Makefile:54` runs `find . -type f -name "*.js" -not -path "./node_modules/*" -delete` under a "Clean build artifacts" target. All six test sources are plain `.js` files (`pulserpc/tests/test_rpc.js:1`, `test_validation.js:1`, `test_types.js:1`, `test_contract.js:1`, `test_client.js:1`, `test_transport.js:1`) and are not ignored by `.gitignore` (which only lists `node_modules/`, `dist/` paths). `make clean` therefore destroys test sources; a subsequent `make test` fails.
- Expected behavior: Clean should remove only generated artifacts (`.js.map`, `node_modules`), not hand-written sources.
- Test status: none; no CI check runs `make clean && make test` for this runtime.
- Suggested fix: Delete only `*.js.map` and `node_modules`, or restrict `*.js` deletion to compiled output directories/explicit generated file list.
- Acceptance criteria:
  - [ ] After `make clean`, all seven test files still exist.
  - [ ] `make clean && make test` passes.

### ts-cjs-11 — CJS integration test swallows TypeScript compilation errors
- Severity: medium
- Invariant: R13
- Current behavior: `tests/integration/test_generator_ts.sh:111` ignores `npm install` failure (`|| true`), `:113` runs `npx tsc --project tsconfig.json 2>&1 || true`, and `:115-118` only checks that `test_server.js` was emitted. Type errors in generated CJS code never fail the test; only a total emit failure is caught.
- Expected behavior: The harness header (`:4`) promises "generate → compile → start server → run client → assert RPC success"; a compile step whose errors are discarded does not enforce that.
- Test status: none; this is the only end-to-end CJS test (`STYLES=(... "cjs")` at `:27`, exercised via `ts-node/Makefile:37` / root `Makefile:150-152`).
- Suggested fix: Fail the style when `tsc` exits non-zero (remove `|| true`), or at minimum capture and assert on compiler diagnostics while allowing known type-only warnings explicitly.
- Acceptance criteria:
  - [ ] Injecting a type error into generated CJS output makes the cjs style fail.
  - [ ] A clean cjs generation still compiles and passes end-to-end.

### ts-cjs-12 — HTTP API test warns instead of failing on wrong error codes
- Severity: low
- Invariant: M17
- Current behavior: `tests/integration/test_http_api.sh:127-142` prints `⚠ WARN` for wrong/missing error codes and then falls through to `✓ PASS` (`:140-142`). This script is run for the cjs style by `test_generator_ts.sh:183-186`.
- Expected behavior: M17 — wrong JSON-RPC error codes should fail the test.
- Test status: none; codes are not asserted.
- Suggested fix: Return non-zero at `:128-137` when the observed code differs from `expected_error_code`, incrementing a failure counter instead of `TESTS_PASSED`.
- Acceptance criteria:
  - [ ] A server returning `-32000` where `-32602` is expected makes `test_http_api.sh` exit non-zero.
  - [ ] `test_generator_ts.sh` propagates that failure for the cjs style (it already does at `:196-199`).

### ts-cjs-13 — Nonstandard `-32000` error codes in HttpTransport
- Severity: low
- Invariant: R8
- Current behavior: `ts-cjs/pulserpc/transport.ts:55` (HTTP non-OK), `:60` (empty body), `:66` (invalid JSON) all throw `RPCError(-32000, ...)`. ts-node does the same (`ts-node/pulserpc/transport.ts:80,85,91`).
- Expected behavior: R8 / `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:815` require standard codes (-32700, -32600, -32601, -32602, -32603); -32000 is outside that set.
- Test status: `test_transport.js:30-47` asserts only URL pass-through, not error codes.
- Suggested fix: Use -32603 for transport/parse failures (or define named constants per R8) and assert the chosen code in `test_transport.js`.
- Acceptance criteria:
  - [ ] All `RPCError`s raised by `HttpTransport.request` use documented standard codes.
  - [ ] A test asserts the code for a non-OK response, empty body, and invalid JSON.

### ts-cjs-14 — Batch requests unsupported in generated HTTP adapter
- Severity: low
- Invariant: M7
- Current behavior: `pkg/generator/ts_client_server.go:2128-2129` does `const data = JSON.parse(body); const response = await this.rpcServer.call(data);` — a single object only; `grep` finds no `batch`/array handling anywhere in the generator. This adapter is what the cjs integration server uses (`test_generator_ts.sh:120-123`).
- Expected behavior: M7 / `docs/RUNTIME_IMPLEMENTATION_GUIDE.md:813` require "Batch requests: Array of requests".
- Test status: none; `tests/integration/test_http_api.sh` has no batch case.
- Suggested fix: In the generated handler, if `Array.isArray(data)`, map `this.rpcServer.call` over elements, drop `null` notification results, and return an array (or 204 when empty). Add an integration batch case.
- Acceptance criteria:
  - [ ] A POST with a JSON array of two requests returns a JSON array of two responses.
  - [ ] A batch containing only notifications returns no body (204).

### ts-cjs-15 — Package index does not re-export scalar validators
- Severity: low
- Invariant: R1
- Current behavior: `ts-cjs/pulserpc/index.ts:19` exports only `validateType` from `./validation`, whereas ts-node uses `export * from "./validation.js"` (`ts-node/pulserpc/index.ts:16`). Consumers importing `validateInt`, `validateString`, etc. from the package root fail (generated code itself imports submodules directly, e.g. `ts_client_server.go:1444,1880,2078`). **Correction to catalog R1:** `findStruct`, `findEnum`, `getStructFields`, and `extractChecksum` *are* exported at `index.ts:36-39`, so the "only validateType" claim is too broad — the gap is limited to the eight scalar/aggregate validators.
- Expected behavior: R1 API parity across runtimes; ts-node exposes all validators from its index.
- Test status: none — tests import directly from `../validation` (`test_validation.js:6-16`, `test_shared.ts:13-16`), so index exports are never checked.
- Suggested fix: Add `export { validateString, validateInt, validateFloat, validateBool, validateArray, validateMap, validateEnum, validateStruct } from "./validation";` to `index.ts`, and add an index-level import smoke test.
- Acceptance criteria:
  - [ ] All validators named above are importable from the package index.
  - [ ] A test imports and calls at least one validator via the index.

### ts-cjs-16 — `Transport` is concrete and fails only at runtime
- Severity: low
- Invariant: R9
- Current behavior: `ts-cjs/pulserpc/transport.ts:15-23` declares `export class Transport` with a `request()` that throws `"Transport.request must be implemented by subclass"`. ts-node declares `export abstract class Transport` with `abstract request(...)` (`ts-node/pulserpc/transport.ts:32-36`), so omission is a compile-time error there.
- Expected behavior: R9 parity with the sibling runtime; an abstract base should not be instantiable.
- Test status: none; no test asserts `new Transport()` is invalid.
- Suggested fix: Restore `abstract class` / `abstract request`, keeping the default `close()` implementation. Note the hand-converted CJS runtime must still compile under `tsconfig.json:4` module CommonJS (transformed by `ts_client_server.go:344-352`).
- Acceptance criteria:
  - [ ] TypeScript compilation rejects `new Transport()`.
  - [ ] `HttpTransport` and `InProcTransport` still compile and their tests pass.

### ts-cjs-17 — Auditors compare raw severity strings instead of the `Severity` constant
- Severity: low
- Invariant: M16 (severity handling)
- Current behavior: `ts-cjs/pulserpc/contract.ts:33,35,37` and `:55` compare `delta.severity === "Error" | "Warning" | "Info"`, and `client.ts:198` uses `=== "Error"`, even though `Severity` is defined at `types.ts:67-72`. ts-node uses the enum (`ts-node/pulserpc/contract.ts:33-38,53`).
- Expected behavior: Severity comparisons should use the single source of truth so value changes cannot silently desync auditors and compatibility logic.
- Test status: `test_contract.js:185-190` passes string literals in its fixture, so it does not catch drift; `test_shared.ts:211-220` only checks `classifySeverity` outputs.
- Suggested fix: Import `Severity` in `contract.ts`/`client.ts` and replace the literals; construct test fixtures with `Severity.Error`.
- Acceptance criteria:
  - [ ] No `severity === "..."` string comparisons remain in ts-cjs source.
  - [ ] Auditor and `verifyCompatibility` tests use `Severity.*` constants.

### ts-cjs-18 — `LoggingAuditor` is imported but never exercised
- Severity: low
- Invariant: R13
- Current behavior: `ts-cjs/pulserpc/tests/test_contract.js:2` imports `LoggingAuditor`, but `main()` at `:192-204` only calls `testNoOpAuditor` and `testFailFastAuditorThrowsOnIncompatible`. The implementation (`contract.ts:25-50`) has three severity branches and a success message that are untested.
- Expected behavior: All exported auditor behaviors should be asserted, especially the error/warning/info dispatch tied to the M16 severity contradiction.
- Test status: none for `LoggingAuditor`.
- Suggested fix: Add a test that stubs `console.error/warn/info` and asserts one call per severity using `Severity.*` values plus the zero-delta success message.
- Acceptance criteria:
  - [ ] A result with one delta of each severity triggers the corresponding console method exactly once.
  - [ ] A compatible zero-delta result logs the success message.

## Verified-correct invariants

- **R1 (partial)** — `index.ts:36-39` does export `findStruct`, `findEnum`, `getStructFields`, `extractChecksum`; only scalar validator re-exports are missing (see ts-cjs-15).
- **R3** — Validation accumulates `ValidationError[]` with paths (`validation.ts:61-66,117-145`); multi-error assertion at `test_validation.js:240-262`.
- **R4** — Missing required/optional/null/undefined handled at `validation.ts:125-141`; tests `test_validation.js:185-213,272-286`.
- **R5** — Parent fields resolved first, child overrides at `types.ts:100-127`; tests `test_types.js:48-82`, `test_shared.ts:76-112,131-143`.
- **R6** — Exact-name lookup only (`types.ts:92-98`), same as ts-node; no dedicated namespace test (gap, not a ts-cjs-specific divergence).
- **R7** — `RPCError(code, message, data)` at `rpc.ts:5-18`; tests `test_rpc.js:8-33`.
- **M1** — `jsonrpc !== "2.0"` → -32600 at `server.ts:48-55`.
- **M2** — method presence/type check at `server.ts:57-65`.
- **M3** — `Interface.method` parsing and -32601 at `server.ts:77-93`.
- **M6** — Generated adapter emits -32700 parse error (`ts_client_server.go:2137-2144`).
- **M8** — Non-object/array params rejected at `server.ts:114-120`.
- **M10** — Per-param IDL validation at `contract.ts:171-214`, invoked from `server.ts:100-107,122-132`; assertions `test_contract.js:69-138`.
- **M11** — Enum membership (`validation.ts:89-102`) and fractional-int rejection (`:33-36`); tests `test_validation.js:44-55,143-161` (except Infinity — see ts-cjs-01).
- **M12** — Unknown fields ignored intentionally (`validation.ts:119-143` iterates declared fields only).
- **M15** — `client.ts:190-197` diffs local IDL against server IDL, not IDL against itself.
- **R14** — Void return accepts both null and undefined (`contract.ts:228-235`).
- **Key-context correction (`??` vs `||`)** — ts-cjs uses `??` (`server.ts:41,50,60,71`, `transport.ts:89`) while ts-node uses `??` in the server too (`ts-node/pulserpc/server.ts:57,67,78,90`); the only `||` is ts-node's `transport.ts:115`. `??` is more correct (preserves `id:0`), so this is NOT a ts-cjs defect.
- **Key-context correction (shared tests)** — `ts-cjs/pulserpc/tests/test_shared.ts` is logic-identical to ts-node's; the two files differ only in import suffixes (`.js` vs none), verified by line-wise comparison.
