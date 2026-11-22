# Audit: ts-cjs runtime

## Scope & methodology

Audited the CommonJS TypeScript runtime library (`pkg/runtime/runtimes/ts-cjs/pulserpc/`:
`client.ts`, `server.ts`, `transport.ts`, `validation.ts`, `contract.ts`, `diff.ts`,
`rpc.ts`, `types.ts`, `index.ts`), the shared TypeScript generator
`pkg/generator/ts_client_server.go` (generated HTTP adapter, CJS transform, test
server), and the runtime/test wiring (`Makefile`, `package.json`, `tsconfig.json`,
`tests/integration/test_generator_ts.sh`, `tests/integration/test_http_api.sh`)
against `docs/audit/invariants.md`, `docs/audit/invariants-additional.md`, and
`docs/RUNTIME_IMPLEMENTATION_GUIDE.md` (D1-D16). The stale baseline
`docs/audit/problems-ts-cjs.md` (ts-cjs-01..ts-cjs-18) was re-verified
line-by-line against the current tree.

Methodology: read every runtime source file and the generated-adapter sections of
the generator; ran the full runtime suite (`tsx tests/*.js` / `tests/*.ts`,
14 files, all passing) and `npx tsc --noEmit -p tsconfig.json` (clean); compared
the ts-cjs tree against the ts-node tree for intentional divergence. No source
code was modified by this audit; only this document was written.

## Summary (counts by severity)

- Critical: 0
- High: 0
- Medium: 0
- Low: 0
- Total confirmed: 0
- Test gaps: 6 (see below)
- Invariants verified as holding: R1-R15, R17-R32 (ts-cjs-relevant), M1-M28,
  C1-C7, C9, C11-C13, S1-S8, D1-D16, G20, G34, G35 (see evidence below)

## Confirmed issues

None. Every baseline finding in `docs/audit/problems-ts-cjs.md` is resolved in
the current tree:

- ts-cjs-01 (Infinity in `validateInt`) — `validation.ts:34-51` now rejects
  NaN/Infinity; covered by `test_validation.js:55-70`.
- ts-cjs-02 (`validateResponses` inert) — `server.ts:192-200` calls
  `contract.validateResponse`; covered by `test_server.js:79-107`.
- ts-cjs-03 (notifications return `id:null`) — `server.ts:55-64` returns
  `null`; covered by `test_server.js:109-150`.
- ts-cjs-04 (removed required field classified as `Error`) — obsolete: the
  amended spec scenario (`openspec/specs/contract-compatibility-verification/spec.md:120-124`)
  and the implementation both classify a removed field as `Info` with
  `compatible=true`; `diff.ts:282-291` and `classifySeverity` (`diff.ts:481-483`)
  agree, and `test_diff.js` asserts `Info`.
- ts-cjs-05 (null results bypass response validation) — `client.ts:297-309`
  validates whenever `result !== undefined`; covered by
  `test_client_validation.js:42-56`.
- ts-cjs-06 (trailing optional params rejected) — `contract.ts:240-253` uses
  `requiredCount..paramDefs.length`; covered by `test_contract.js:180-195`.
- ts-cjs-07 (response envelope not validated) — `client.ts:267-295` validates
  version, id, and result/error exclusivity, and uses `??` for the code fallback;
  covered by `test_client_validation.js:70-105`.
- ts-cjs-08 (`meta` not parsed) — `contract.ts:122-206` parses both IDL
  shapes; covered by `test_contract.js:70-95`.
- ts-cjs-09 (no diff test) — `pulserpc/tests/test_diff.js` exists and is listed
  in both Makefile targets (`Makefile:31,42`).
- ts-cjs-10 (`make clean` deletes `.js` sources) — `Makefile:60-62` removes
  only `*.js.map` and `node_modules`; covered by `test_build_scripts.js`.
- ts-cjs-11 (tsc errors swallowed) — `tests/integration/test_generator_ts.sh:111-133`
  captures diagnostics and fails on any code other than the documented
  TS2749/TS2503 type-only allowlist; covered by `test_build_scripts.js`.
- ts-cjs-12 (HTTP API warns on wrong codes) — `tests/integration/test_http_api.sh`
  now compares `error.code` and fails the run when it differs
  (`test_validation_error`, `:127-142`).
- ts-cjs-13 (nonstandard `-32000`) — `transport.ts` uses `-32603` throughout
  (`:198,204,211,221,227,231`).
- ts-cjs-14 (batch unsupported in generated adapter) —
  `ts_client_server.go:2209-2232` implements bounded batches with notification
  dropping and empty-batch handling.
- ts-cjs-15 (index does not re-export scalar validators) — `index.ts:26-38`
  re-exports all validators; covered by `test_index.js`.
- ts-cjs-16 (`Transport` concrete) — `transport.ts:146-152` is `abstract class`
  with `abstract request`; covered by `test_abstract_transport.js`.
- ts-cjs-17 (raw severity string comparisons) — `contract.ts`, `client.ts`,
  `diff.ts` use `Severity.*`; covered by `test_build_scripts.js`.
- ts-cjs-18 (`LoggingAuditor` untested) — covered by
  `test_contract.js:testLoggingAuditorDispatchesBySeverity`.

## Test gaps

1. **Qualified type-name resolution is untested** (R6/D16): `types.ts:92-98`
   resolves exact key then base name, but no test exercises a namespace-qualified
   reference such as `inc.Response` against a multi-namespace IDL. Shared gap with
   ts-node.
2. **Non-finite `float` inputs are untested** (R2): `validateFloat`
   (`validation.ts:53-58`) accepts `NaN` and `Infinity` and no test pins either
   acceptance or rejection. The guide defines `float` only as IEEE-754 double, so
   the intended behavior is unsettled (recorded as a hypothesis below).
3. **Envelope-mismatch exception type is untested** (M5): `client.ts:267-284`
   throws a plain `Error` for a bad envelope while ts-node throws
   `RPCError(-32600)`. No test asserts the thrown constructor, so the documented
   divergence (`reviews-final.md`) is invisible to the suite.
4. **`RPCErrorCode` constants are absent and untested** (R8): `rpc.ts` defines
   only `RPCError`, whereas ts-node defines `RPCErrorCode`
   (`ts-node/pulserpc/rpc.ts:9-16`) and uses it in its transport. No test covers
   their presence or absence.
5. **Integration harness type-only diagnostic allowlist** (C13): `tests/integration/test_generator_ts.sh:111-133`
   still permits TS2749/TS2503 diagnostics. A genuine generated-code defect that
   happens to surface as one of those codes would not fail the cjs style.
6. **Response-validation guard edge (`undefined` vs `null`)** (M14): the client
   guard is `result !== undefined` (`client.ts:303`), and the tests cover the
   `null` case (`test_client_validation.js:42-56`) but not an absent/`undefined`
   result produced by a custom transport.

## Verified as holding (with evidence)

- **R1** — `validateType` (`validation.ts:162`), `findStruct`/`findEnum`/`getStructFields`
  (`types.ts:140-175`), `extractChecksum` (`types.ts:177-182`) are exported, along
  with all scalar validators (`index.ts:26-38`); `test_index.js` imports them all
  from the package root.
- **R2** — `validateInt` enforces type, finiteness, integrality, and signed 32-bit
  range (`validation.ts:34-51`, `INT32_MIN`/`INT32_MAX` at `:31-32`);
  `validateBool`/`validateString` are strict (`validation.ts:19-24,60-65`);
  `test_validation.js:36-70`.
- **R3** — validation accumulates `ValidationError[]` with paths
  (`validation.ts:75-81,118-159`); multi-error assertion at
  `test_validation.js:240-262`.
- **R4** — missing required / null required / undefined required are errors, missing
  optional is OK (`validation.ts:139-156`); `test_validation.js:185-286`.
- **R5** — parent fields resolved first, child overrides by name
  (`types.ts:156-175`); `test_types.js:48-82`, `test_shared.ts:76-143`.
- **R6** — exact key then base-name key via a cached, sorted base-name index
  (`types.ts:108-138`); no dedicated namespace test (gap above).
- **R7** — `RPCError(code, message, data?)` (`rpc.ts:5-18`); `test_rpc.js`.
- **R8** — standard literal codes `-32600..-32603` in `server.ts` and `transport.ts`;
  no nonstandard `-32000` remains.
- **R9** — abstract `Transport` with `HttpTransport`/`InProcTransport`
  (`transport.ts:146-267`); custom headers, trailing-slash normalization
  (`transport.ts:136-141,166-173`); 30 s timeout (`transport.ts:19,185-186`).
- **R10** — response validation is invoked when enabled, including `null` results
  (`server.ts:192-200`); `test_server.js:79-107`.
- **R11 / M4 / M21** — notifications produce no response on every path, including
  throwing handlers (`server.ts:55-64`); `test_server.js:109-150`.
- **R12** — `meta` parsed in both IDL shapes, with prototype-pollution guards
  (`contract.ts:122-206`); `test_contract.js:70-136`.
- **R13** — `diffIDL`, auditors, and `test_diff.js` are wired into `make test`
  (`Makefile:31,42`).
- **R14** — void returns accept both `null` and `undefined`
  (`contract.ts:308-317`).
- **R15 / D4 / S6** — unhandled handler exceptions are logged server-side and
  returned as `-32603 "Internal error"` with no `data` (`server.ts:207-214`);
  `test_server.js` and `tests/integration/test_http_api.sh` redaction probe.
- **R17** — response body capped at 10 MiB via streaming read
  (`transport.ts:20-21,73-130`).
- **R19 / D6** — 30 s request timeout with `AbortController`
  (`transport.ts:19,185-186`).
- **R21** — base-name index cached per types map via `WeakMap`
  (`types.ts:108-126`).
- **R22 / D3 / S8** — generated adapter rejects non-POST with HTTP 405 and
  `-32600` (`ts_client_server.go:2262-2267`).
- **R23** — notifications yield 204 / no body in the generated adapter
  (`ts_client_server.go:2230-2232`).
- **R24 / D2** — `id` must be string/number/null; booleans/objects/arrays
  rejected with `-32600` (`server.ts:77-90`); `test_server.js`.
- **R25** — `int` is signed 32-bit on the wire (`validation.ts:31-51`).
- **R26** — `float` is a TS `number` (IEEE-754 double) (`validation.ts:53-58`).
- **R27** — enums are wire strings in generated code.
- **R28 / S5** — untrusted `meta` keys cannot pollute prototypes
  (`contract.ts:20-206`); `test_contract.js:103-136`.
- **R29 / R30 / D13** — custom headers and single trailing-slash normalization
  (`transport.ts:136-141,166-173`).
- **R31** — client bootstraps from `pulserpc-idl` and `verifyCompatibility`
  diffs the fetched server IDL against the local one
  (`client.ts:120-147,185-217`).
- **M1** — `jsonrpc !== "2.0"` → `-32600` (`server.ts:92-99`).
- **M2** — missing/empty/non-string `method` → `-32600` (`server.ts:101-109`).
- **M3** — `Interface.method` parse and lookup → `-32601` (`server.ts:119-135`).
- **M5** — response-envelope validation (version, id match, result XOR error,
  error shape) in `client.ts:267-295`.
- **M6** — generated adapter emits `-32700` on JSON parse failure
  (`ts_client_server.go:2137-2144`).
- **M7** — batch requests supported in the generated adapter
  (`ts_client_server.go:2209-2232`).
- **M8** — array/object params accepted, other shapes rejected
  (`server.ts:143-172`).
- **M9** — exact parameter count with trailing-optionals
  (`contract.ts:240-253`).
- **M10** — per-param IDL validation wired (`contract.ts:255-295`,
  `server.ts:174-187`).
- **M11** — enum membership and recursive array/map/struct validation
  (`validation.ts:83-217`).
- **M12** — unknown/extra fields intentionally ignored
  (`validation.ts:133-157`).
- **M13** — validation `data` is the structured `ValidationResult` shape, as
  documented for TS (`contract.ts:96-109`).
- **M14** — response validation runs for `null` results
  (`client.ts:297-309`, `server.ts:192-200`).
- **M15** — client diffs local IDL against the server IDL, not against itself
  (`client.ts:185-217`).
- **M16** — removed field classified as `Info` per the amended spec
  (`diff.ts:282-291`, `classifySeverity:481-483`).
- **M18-M28** — empty batch → single `-32600`; non-object entries → per-entry
  `-32600`; notifications dropped from batches; `id:null` is a notification;
  object-form params handled explicitly; extra fields accepted
  (`ts_client_server.go:2209-2232`, `server.ts`).
- **C1 / D12** — byte-deterministic generation (generator audit tests).
- **C2 / D8** — uniform IDL JSON schema including `rootNamespace`
  (`ts_client_server.go` embeds the Go-produced IDL).
- **C4 / C5** — 43-character checksum served as an object
  (`tests/integration/test_http_api.sh` probe).
- **C6** — TS config/package.json generation refuses to overwrite
  (`ts_client_server.go:799-805,883-889`).
- **C7** — module-style precedence (`ts_client_server.go:62-74,231-240`).
- **C11** — enums serialize as declared string names.
- **C12** — flag defaults (`ts_client_server.go`).
- **C13** — generated code compiles under the selected module system (with the
  documented TS2749/TS2503 type-only allowlist).
- **S1** — playground file access contained (Go side; not ts-cjs-specific).
- **S2-S4** — paths/separators correct (generator).
- **S7** — body/response/batch limits enforced (generated adapter + transport).
- **G20 / G34 / G35** — 43-char checksum, excludes comments/error text,
  includes parameter position (parser side; consumed by the runtime).

## Unconfirmed hypotheses

- **`validateFloat` accepts non-finite doubles** (`validation.ts:53-58` has no
  finiteness check). The catalog lists TS `float` accepting NaN/Infinity as a
  divergence, but the guide defines `float` only as IEEE-754 double and other
  runtimes (Go, Java, C#, Python) also accept non-finite doubles, so whether the
  ts-cjs runtime should reject them is unsettled. No test pins the behavior.
- **`Transport.request` uses numeric literals** instead of `RPCErrorCode`
  constants (see divergence above); whether convergence with ts-node is required
  is a product decision, not a confirmed invariant violation.
- **Envelope-mismatch exception type differs from ts-node** (plain `Error` vs
  `RPCError`); both satisfy M5's "makes `client.call` throw" acceptance
  criterion, and no invariant fixes the exception class.
- **`requestId` is not atomic** (`client.ts:67,247`), but JavaScript is
  single-threaded per event loop, so no race is observable; whether a worker-based
  caller needs atomicity is unconfirmed.
- **Generated production TS servers do not expose an HTTP adapter** (only the
  test server does, `ts_client_server.go:2144`); whether first-time users can
  assemble a production server without the guide is a documentation question, not
  a confirmed defect.

## Notes

- `docs/audit/problems-ts-cjs.md` is a stale baseline: all of its ts-cjs-01..18
  items are either resolved in the current tree or superseded by the amended spec
  (ts-cjs-04). This document supersedes it for the ts-cjs runtime.
- The ts-cjs runtime is now largely at parity with ts-node: the previously
  divergent Infinity handling, notification suppression, response validation,
  `meta` parsing, optional-parameter counting, severity comparisons, and index
  exports are all aligned. Remaining differences are cosmetic (exception class on
  envelope failure, `RPCErrorCode` constants, type-only diagnostic allowlist) and
  are recorded above as test gaps / hypotheses rather than confirmed defects.
- `npx tsc --noEmit -p tsconfig.json` is clean, and all 14 runtime test files
  pass under `tsx` on the audited tree.
- No source code was modified by this audit; only this document was written.