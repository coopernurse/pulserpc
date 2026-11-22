# TypeScript Plan — ts-node + ts-cjs

Worktree: `pulserpc-fix-typescript` · Branch: `fix-invariants-typescript`
Owns: `pkg/generator/ts_client_server.go`, `pkg/generator/ts_*_test.go`,
`pkg/runtime/runtimes/ts-node/**`, `pkg/runtime/runtimes/ts-cjs/**`,
`docs/languages/typescript/**`.

Read [README.md](README.md) for D1–D16. Both module styles share the generator
and many sources; keep them in sync.

## P0 — fix

### C1 — real determinism + sorted output
- Fix the vacuous determinism unit test in
  `pkg/generator/ts_client_server_test.go` (~1225–1250): it currently compares
  `collectedFiles[relPath]` to itself in the same iteration. Keep the first
  iteration's map and compare subsequent iterations to it.
- Sort `namespaceMap`/import iteration where it affects emitted bytes.
- Add a test that generates twice and byte-compares.

### R15 — no internal detail leakage
- `ts-node/pulserpc/server.ts` and `ts-cjs/pulserpc/server.ts`: replace
  `Handler exception: ${e.message}` / `Response validation failed: ...` with
  the fixed `"Internal error"` message and no `data`; log the exception
  server-side.
- Keep `-32602` validation messages and `RPCError` data.
- Add tests with a throwing handler; assert no exception text reaches the wire.

### R16 / R18 — generated test-server limits
- Cap request bodies at 1 MiB in the generated test-server adapter; return
  HTTP 413 or `-32600 "Request body too large"` (match the documented shape).
- Cap batches at 100; return `-32600 "Batch exceeds maximum size"`.
- Add generator tests.

### R19 / R17 — client timeout and response cap
- `HttpTransport` (both runtimes): 30 s timeout via `AbortController`.
- Cap response bodies at 10 MiB and reject with a clear error.
- Add tests.

### R22 — non-POST shape
- Generated test server already returns 405; ensure it includes a JSON-RPC
  `-32600` body with `id:null` (D3). Add a test.

### R24 / R25 — id type and int range
- Servers reject boolean/object/array ids with `-32600`.
- `validateInt` (both runtimes): reject integral values outside signed 32-bit.
  Update tests.

### M19 / M20 — batches and params
- Non-object batch entries: ensure a per-entry `-32600` (the generated adapter
  delegates to `Server.call`; verify and test).
- Object-form params: document and test the explicit behavior (accept or
  `-32602`), not a silent drop.

## P1 — verify and test

- C4/C5: embedded IDL JSON carries `rootNamespace`/`checksum`; the served
  `pulserpc-idl` result is an object.
- C6/C7: add integration tests for `-ts-gen-package-json`/`-ts-gen-tsconfig`
  overwrite refusal and `-ts-no-detect`/tsconfig/package.json precedence.
- R20: request ids are unique under `Promise.all` (JS is single-threaded; the
  counter must increment synchronously). Add a concurrency test.
- R28: hostile `meta` keys (`__proto__`, `constructor`, `prototype`) are
  rejected/ignored; add a wire-level test.
- R29/R30: custom headers work in `HttpTransport`; normalize a trailing `/` on
  the base URL.
- R31: runtime `verifyCompatibility` fetches the server IDL at bootstrap
  (already); add a test with a divergent IDL.
- C11/R26/R27: enums are wire strings, float is `number`/double; add tests.
- M18/M21–M28: empty batch, `id:null`, `error.data` nil, null result, numeric
  ids, error shape, `-32700`, `jsonrpc`, batch notifications.
- Add `docs/languages/typescript/` notes for D1/D3/D4/D5/D7.

## Test commands

```powershell
$env:PATH = [Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [Environment]::GetEnvironmentVariable("PATH","User")
cd pkg/runtime/runtimes/ts-node
npx tsx pulserpc/tests/test_types.ts   # and test_validation/contract/client/server/transport/rpc/diff/shared
cd ../ts-cjs
npx tsx pulserpc/tests/test_types.js   # and the other .js/.ts tests
```

Generated e2e: build the `pulserpc` binary, generate `examples/conform.pulse`
for `-ts-module cjs` and the ESM default with `-generate-test-files`, compile
CJS with `tsc`, run both test servers/clients; remove any stub.

## Acceptance

- Both TS runtime suites pass; generated CJS/ESM code compiles and runs e2e.
- The determinism test is real (fails if output changes).
- Every P0 behavior has a test that fails before the change.
- No edits outside the TS generator/runtime/docs.
