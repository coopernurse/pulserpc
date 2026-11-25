# C# Plan

Worktree: `pulserpc-fix-csharp` · Branch: `fix-invariants-csharp`
Owns: `pkg/generator/csharp_client_server.go`,
`pkg/generator/csharp_*_test.go`, `pkg/runtime/runtimes/csharp/**`,
`docs/languages/csharp/**`.

Read [README.md](README.md) for D1–D16. Do not edit shared harness scripts.

## P0 — fix

### C1 — deterministic generation
- Sort `namespaceMap`/import iteration where it affects emitted output
  (`csharp_client_server.go:611,673,1465` and related).
- Add a test that generates twice and byte-compares.

### C10 — nullable optional built-ins
- `mapTypeToCsType` (`csharp_client_server.go:368-371`) must return `int?`,
  `bool?`, `double?`, `string?` for optional fields. The current condition
  omits `int` and `string`.
- Ensure the generated file has `#nullable enable` (or an equivalent) so
  `string?` is meaningful; update setters/getters and client/server signatures.
- Add a generation test with optional `int/bool/float/string` and compile it.

### R15 — no internal detail leakage
- Replace `$"Exception: {e.Message}\nStackTrace: {e.StackTrace}"` and
  `$"Failed to deserialize IDL JSON: {e.Message}"` with the fixed
  `"Internal error"` message and no `data`; log details via the logger.
- Keep `-32602` validation messages.
- Add a test with a throwing handler; assert no message/stack reaches the wire.

### R16 / R18 — generated server limits
- Cap request bodies at 1 MiB before reading; return `-32600 "Request body too
  large"`.
- Cap batches at 100; return `-32600 "Batch exceeds maximum size"`.
- Add generator/e2e tests.

### R19 / R17 — client timeout and response cap
- Set `HttpClient.Timeout = TimeSpan.FromSeconds(30)`.
- Cap response bodies at 10 MiB; reject with a clear error.
- Add tests.

### R22 — non-POST shape
- The generated server already sets 405; add a JSON-RPC `-32600` body with
  `id:null` (D3). Add a test.

### R24 / R25 — id type and int range
- Server: reject boolean/object/array ids with `-32600`.
- `Validation.ValidateInt`: reject values outside signed 64-bit. Update tests.

### M19 / M20 — batches and params
- Non-object batch entries: already checked; add a test.
- Object-form params: runtime server should normalize or explicitly reject
  (`-32602`) instead of silently dropping. Add a test for the chosen behavior.

## P1 — verify and test

- C4/C5: `IdlHolder.GetIDLJson()` carries `rootNamespace`/`checksum`;
  `pulserpc-idl` serves an object.
- C11/R27: enums serialize as strings (`JsonStringEnumConverter`); add a test.
- R20: request ids are GUIDs (unique); add a concurrency test.
- R26: float is `double`.
- R29/R30: custom headers work; trailing slash is trimmed (already); add tests.
- R31: `VerifyCompatibility` fetches `pulserpc-idl` (already); add a test with
  a divergent IDL.
- M18/M21–M28: empty batch, `id:null`, `error.data` nil, null result, numeric
  ids, error shape, `-32700`, `jsonrpc`, batch notifications.
- Add `docs/languages/csharp/` notes for D1/D3/D4/D5/D7.

## Test commands

```powershell
$env:PATH = [Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [Environment]::GetEnvironmentVariable("PATH","User")
cd pkg/runtime/runtimes/csharp
dotnet test tests/PulseRPC.Tests.csproj --nologo
```

Generated e2e: build the `pulserpc` binary (stub `pkg/webui/dist` if needed),
generate `examples/conform.pulse` with `-generate-test-files`, `dotnet build`
the test server/client projects, start the server, run the client; remove the
stub.

## Acceptance

- 100+ C# unit tests pass; generated C# compiles and passes e2e.
- Optional built-ins are nullable and the generated code compiles.
- Every P0 behavior has a test that fails before the change.
- No edits outside the C# generator/runtime/docs.
