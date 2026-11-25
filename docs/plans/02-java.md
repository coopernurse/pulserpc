# Java Plan

Worktree: `pulserpc-fix-java` · Branch: `fix-invariants-java`
Owns: `pkg/generator/java_client_server.go`, `pkg/generator/java_*_test.go`,
`pkg/runtime/runtimes/java/**`, `docs/languages/java/**`.

Read [README.md](README.md) for D1–D16. Do not edit shared harness scripts.

## P0 — fix

### C1 — deterministic generation
- Sort imports before emitting (`for imp := range imports` loops at
  `java_client_server.go:376,463,932,1582,1749,1833`).
- Add a test that generates the same IDL twice and byte-compares.

### C10 — nullable optional built-ins
- Add an optional-aware type mapper: optional `int` → `Integer`, `bool` →
  `Boolean`, `float` → `Double`, `string` stays `String`.
- Use it for generated struct fields, method params/returns, and client/server
  signatures. Keep required fields primitive.
- Add a generation test with optional `int/bool/float/string` and assert the
  types; compile the output.

### R15 — no internal detail leakage
- Replace `"Internal error: " + e.getMessage()`,
  `"Failed to load IDL: " + e.getMessage()`, and
  `"Internal error: response validation failed: " + ex.getMessage()` with the
  fixed `"Internal error"` message and no `data`. Log details server-side
  (`System.err` with a redacted message).
- Keep `-32602` validation messages (they are user-facing) and declared
  `RPCError` messages.
- Add a test with a throwing handler; assert no exception text/stack reaches
  the wire.

### R16 / R18 — generated server limits
- Cap the request body at 1 MiB before `readAllBytes` (check
  `Content-Length` and/or use a bounded read) and return `-32600 "Request body
  too large"`.
- Cap batches at 100 and return `-32600 "Batch exceeds maximum size"`.
- Add generator/e2e tests.

### R19 / R17 — client timeout and response cap
- Add `.timeout(Duration.ofSeconds(30))` to the generated `HttpRequest`.
- Cap response bodies at 10 MiB: check `Content-Length` and the read length;
  reject with a clear error.
- Add tests.

### R20 — atomic request ids
- `Client.java`: replace `requestID++` with `AtomicInteger.incrementAndGet()`.
- Add a concurrent test.

### R22 — non-POST shape
- Return HTTP **405** with a JSON-RPC `-32600` body (`id:null`) for non-POST.
- Add a test.

### R24 / R25 — id type and int range
- Server: reject boolean/object/array ids with `-32600`.
- `Validation.validateInt`: reject values outside signed 64-bit. Update tests.

### R29 / R30 — headers and trailing slash
- Add a `HTTPTransport(String baseUrl, JsonParser parser, Map<String,String>
  headers)` overload (keep the two-arg constructor) and apply headers.
- Strip a trailing `/` from the base URL in the transport and generated client.
- Add tests.

### M19 / M20 — batch and params
- Non-object batch entries: ensure a per-entry `-32600` (Java already checks
  `item instanceof Map`; verify and test).
- Runtime `Server`: stop silently dropping object-form params; either normalize
  them to positional using the IDL order or return an explicit `-32602`.
  Add a test for the chosen behavior.

## P1 — verify and test

- C4/C5: `IDL_JSON` carries `rootNamespace`/`checksum`; `pulserpc-idl` serves
  an object.
- C11/R27: enums serialize as string names (Jackson/Gson).
- R26: float is double.
- R31: `verifyCompatibility` fetches `pulserpc-idl` (already implemented);
  add an end-to-end/stub test with a divergent IDL.
- M18/M21–M28: empty batch, `id:null`, `error.data` nil, null result, numeric
  ids, error shape, `-32700`, `jsonrpc`, batch notifications.
- C6/C7 are TypeScript-only; do not touch.
- Add a `docs/languages/java/` note for D1/D3/D4/D5/D7.

## Test commands

```powershell
$env:PATH = [Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [Environment]::GetEnvironmentVariable("PATH","User")
cd pkg/runtime/runtimes/java
mvn -q test
```

Generated e2e: build the `pulserpc` binary (stub `pkg/webui/dist` if needed),
generate `examples/conform.pulse` with `-generate-test-files -package
com.example.rpc`, `mvn -q clean test-compile`, start `TestServer`, run
`TestClient`; repeat with `-json-lib gson`. Remove any stub before committing.

## Acceptance

- Maven suite passes; generated Java compiles and passes e2e with Jackson and
  Gson.
- Every P0 behavior has a test that fails before the change.
- No edits outside the Java generator/runtime/docs.
