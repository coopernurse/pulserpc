# Wave 6 Audit: TypeScript Imported-Interface Identifiers

Status: **COMPLETE** — repro confirmed, fix applied, regression test added and passing for both module styles.

## Scope

Imported interfaces declared in another `.pulse` file carry a qualified IDL
name (e.g. `common.CommonOps`). The TypeScript generator interpolated that
qualified name into TS identifiers, producing invalid syntax such as
`export class common.CommonOpsClient {`. Go/C#/Java/Python were already fixed
to use base-name identifiers (`CommonOps`) while keeping the qualified name in
wire strings (`common.CommonOps.Ping`). This audit covers the TS generator
(`pkg/generator/ts_client_server.go`) for `esm-node` and `cjs` output.

## Confirmed repro

Generating committed fixture `examples/cross-ns/app.pulse` (namespace `app`
imports `common.pulse`; `interface CommonOps` declared in namespace `common`)
with `-plugin ts-client-server`:

- `client.ts`: `export class common.CommonOpsClient {` (invalid TS)
- `server.ts`: `export abstract class common.CommonOps {` (invalid TS)
- method strings were correctly qualified: `method: "common.CommonOps.Ping"`

Source: `docs/audit/wave6-idl-fixtures.md:87-90`.

## Findings

All file:line references are post-fix locations in
`pkg/generator/ts_client_server.go` unless noted.

| # | Candidate | file:line (post-fix) | Status | Severity | Fix |
| --- | --- | --- | --- | --- | --- |
| 1 | Qualified interface name in abstract server class declaration (`export abstract class common.CommonOps`) — flat and namespace paths | `ts_client_server.go:1526`, `:1578` | CONFIRMED (fixture repro + generated file inspection) | High — generated TS does not parse | `tsInterfaceIdent(iface)` (`GetBaseName`) at the declaration site |
| 2 | Qualified interface name in static client class name (`common.CommonOpsClient`) | `ts_client_server.go:2094` (site), helper `:1482` | CONFIRMED (generated `common/client.ts:15` before fix) | High — invalid TS class name | `tsInterfaceIdent(iface) + "Client"` |
| 3 | Qualified name in generated `test_server.ts` imports (`import { common.CommonOps }`) | `ts_client_server.go:2154`, `:2157`, `:2160`, `:2163` | CONFIRMED (same ESM/CJS import sites) | High — invalid TS import specifier | `tsInterfaceIdent(iface)`; CJS transform then emits valid `const { CommonOps } = require(...)` |
| 4 | Qualified name in test implementation class / extends clause (`class common.CommonOpsImpl extends common.CommonOps`) | `ts_client_server.go:2317` | CONFIRMED | High — invalid TS class declaration | `tsInterfaceIdent(iface)` at both sites |
| 5 | Qualified name in `new <iface>Impl()` constructor call inside `rpcServer.addHandler` | `ts_client_server.go:2182` | CONFIRMED | High — invalid TS expression | `tsInterfaceIdent(iface)` for the class; the handler key stays `iface.Name` because the runtime `Contract` is keyed by the qualified IDL name (wire/registration key) |
| 6 | Qualified struct IDL name interpolated into a TS type reference (`types.common.Shared`) in the imported interface's own namespace files | `ts_client_server.go:1439` (namespace resolver), `:1667` (flat resolver) | CONFIRMED — `tsc --noEmit` on generated fixture produced `TS2694: Namespace '.../common/types' has no exported member 'common'` at `common/server.ts:13` and `common/client.ts:50,79`. `types.ts` declares `Shared`, not `common.Shared`. | High — generated ESM project does not typecheck | Use `GetBaseName(key)` in the struct base-name fallback loop (the enum loop at `:1447` already did this) |
| 7 | Qualified name in the dynamic client proxy member access (`client.<qualified>.<method>()`) | `ts_client_server.go:2656`, `:2658`, `:2669` | CONFIRMED — the runtime proxy is keyed by the IDL name (`pkg/runtime/runtimes/ts-node/pulserpc/client.ts:176`), so a dotted chain both mis-resolves the key and interpolates a qualified name into property identifiers | Medium — generated `test_client.ts` fails at runtime for imported interfaces | Emit bracket notation with the qualified key: `client["common.CommonOps"].Ping()`; key stays qualified, no dotted identifier |
| 8 | Qualified name in JSON-RPC method strings | `ts_client_server.go:2032` | CONFIRMED correct — `method: "common.CommonOps.Ping"` present in generated output | N/A — must not change | Kept `iface.Name` |
| 9 | Qualified name in doc comments, IDL metadata, test-name comments, and `"B"` special-case keys | `ts_client_server.go:2032`, `:2318`, `:2345`, `:2645`, `:2667` (and `idl.json`) | CONFIRMED intentional | N/A | Kept `iface.Name` |
| 10 | Regression coverage for TS imported-interface identifiers | `pkg/generator/ts_import_identifiers_regression_test.go` | CONFIRMED added | Medium — no guard against regression | New `TestCrossNamespaceFixtureTypeScriptIdentifiers` generates from the committed fixture for `esm-node` and `cjs`, asserts base-name identifiers present, raw dotted identifiers absent, qualified wire/proxy strings present, and typechecks output |

## Verification (executed in worktree, not claimed from inspection)

- `gofmt -l pkg/generator/ts_client_server.go pkg/generator/ts_import_identifiers_regression_test.go` — no output.
- `go vet ./pkg/generator` — no output (exit 0).
- `go test ./pkg/generator -run TestCrossNamespaceFixtureTypeScriptIdentifiers -count=1 -v` — **PASS** for both `esm-node` and `cjs` subtests. Go test binaries executed natively; the Application Control block from the earlier session did not reproduce for these commands.
- Generated-project typecheck with global TypeScript 7.0.2 (`tsc` on PATH):
  - `esm-node` — **PASS**, zero diagnostics using a version-neutral override tsconfig (`module/moduleResolution: NodeNext`, global `@types/node`).
  - `cjs` — **PASS** with only the pre-existing, tolerated CJS-transform diagnostic codes TS2749/TS2503, matching the repo's own `tests/integration/test_generator_ts.sh:159` filter. The generated TS5-era `tsconfig.json` (Node10, `ignoreDeprecations: "5.0"`) is rejected outright by TS 7, so the test writes a version-neutral override config (`module/moduleResolution: Node16`, `moduleDetection: force`) for the check.
- `go test ./pkg/generator -count=1` — FAIL only in `TestCrossNamespaceFixtureJavaAndCSharp` (Java `String method = "common.CommonOps.*"` assertions at `cross_namespace_fixture_test.go:123`; C# `dotnet build` of generated projects at `:172`). These are the pre-existing Java/C# wave-6 defects documented in `docs/audit/wave6-idl-fixtures.md:76-84,151-157`; no TS test fails. Out of scope for this fix.

## Notes

- `iface.Name` is deliberately retained for JSON-RPC method strings, IDL
  metadata, doc comments, test-name strings, the `"B"` special-case key, the
  `rpcServer.addHandler` registration key, and the dynamic proxy property key
  (now emitted as a quoted string via bracket notation).
- No file names in the TS generator are derived from interface names; the
  output files are fixed as `client.ts`, `server.ts`, `types.ts`, etc.
