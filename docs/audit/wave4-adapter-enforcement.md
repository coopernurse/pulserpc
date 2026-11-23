# Wave 4 — Generated HTTP Adapter Enforcement of Body & Batch Caps

Scope: where generated HTTP adapters in `pkg/generator/` enforce inbound request-body
and batch-size caps (left UNVERIFIED by the S3 reports). A **constant** is a declared
limit; an **enforcing check** is the comparison/read guard that acts on it, with call
site named. Line numbers are in the *generator template* (`pkg/generator/<lang>_client_server.go`),
which emits the literal text into the generated adapter. Caps implemented only in a
runtime transport already surveyed are flagged as such, not reported as new.

| runtime | inbound body cap (constant + actual check, file:line) | batch cap (constant + actual check, file:line) | generated adapter file | status |
|---|---|---|---|---|
| Go | CONST `maxRequestBodySize = 1 << 20` (go_client_server.go:955). CHECK `r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)` (go_client_server.go:969), wrapped `io.ReadAll` (970), `*http.MaxBytesError` branch -> `-32600 "Request body too large"` (972-975). Call site: generated `(*PulseRPCServer).handleRequest` (template 960; invoked by ServeHTTP at 944-945). Also CONST `maxBatchResponseSize = 10 MiB` (957) with CHECK `len(payload) > maxBatchResponseSize` (1022) for outbound batch payload. | CONST `maxBatchSize = 100` (go_client_server.go:956). CHECK `if len(requests) > maxBatchSize` (go_client_server.go:993) -> `-32600 "Batch exceeds maximum size"` (994). Call site: `handleRequest`, batch branch (988-996). | `server.go` (go_client_server.go:258) | INSPECTED — both caps enforced in generated adapter |
| Java | CONST is NOT in the template: `HttpLimits.MAX_REQUEST_BODY` (runtime transport `HttpLimits`, already surveyed). Adapter CHECK `HttpLimits.checkDeclaredLength(declaredLength, HttpLimits.MAX_REQUEST_BODY, "Request body")` (java_client_server.go:1054) and `HttpLimits.readBounded(requestBodyStream, HttpLimits.MAX_REQUEST_BODY, "Request body")` (1055), catch `BodyTooLargeException` (1056) -> `sendError(..., 200, -32600, "Request body too large")` (1063). Call site: generated `handleRequest(HttpExchange)` (1038). Streaming cap implementation lives in runtime transport `HttpLimits` (already surveyed), not re-reported as new. | CONST `HttpLimits.MAX_BATCH_SIZE` (runtime transport, already surveyed). CHECK `if (batch.size() > HttpLimits.MAX_BATCH_SIZE)` (java_client_server.go:1104) -> `errorEnvelope(-32600, "Batch exceeds maximum size", ...)` (1105). Call site: generated `handleRequestObject(List)` (1097). | `Server.java` (java_client_server.go:208) | INSPECTED — checks in adapter, constants/bounded read in surveyed runtime `HttpLimits` |
| C# | CONST declared in adapter: `private const int MaxRequestBodyBytes = 1024 * 1024` (csharp_client_server.go:797). CHECK `ContentLength.Value > MaxRequestBodyBytes` (856) plus `bodyBytes = await HttpTransport.ReadLimitedAsync(context.Request.Body, MaxRequestBodyBytes)` (866) and null guard (874-877) -> `-32600 "Request body too large"` (858/876). Call site: generated `HandleRequest(HttpContext)` (846). Streaming bounded-read implementation is runtime transport `HttpTransport.ReadLimitedAsync` (already surveyed), not re-reported as new. | CONST declared in adapter: `private const int MaxBatchSize = 100` (csharp_client_server.go:798). CHECK `if (batchLength > MaxBatchSize)` (csharp_client_server.go:902) -> `-32600 "Batch exceeds maximum size"` (905). Call site: `HandleRequest` batch branch (892-907). | `Server.cs` (csharp_client_server.go:169) | INSPECTED — both caps enforced in generated adapter (streaming read delegated to surveyed runtime) |
| Python | CONST declared in generated handler: `MAX_BODY_SIZE = 1024 * 1024` (python_client_server.go:1030). CHECK `if content_length > self.MAX_BODY_SIZE:` (python_client_server.go:1040) in `do_POST` (1031) -> `-32600 "Invalid Request" / "Request body too large"` (1041) before `self.rfile.read(content_length)` (1043). NOTE: this handler is the generated `TestRPCHandler` (1027); the production cap reportedly lives in runtime `pulserpc.server.DEFAULT_MAX_BODY_SIZE` (per python_client_server_test.go:1882) — NOT inspected here. | NO batch-size constant or check in the generated handler; `req = json.loads(...)` (1051) then `rpc_server.call(req)` (1056) delegates batch handling to runtime. No `MAX_BATCH_SIZE` in python_client_server.go. | generated `TestRPCHandler` (python_client_server.go:1027); exact emitted filename UNVERIFIED | PARTIAL — body cap inspected in generated test handler; batch cap and production-server cap UNVERIFIED |
| Python2 | — | — | — | UNVERIFIED — not inspected in this pass |
| ts-node | CONST `MAX_BODY_SIZE = 1024 * 1024` (ts_client_server.go:2173). CHECK `if (size > MAX_BODY_SIZE)` inside `req.on('data')` (ts_client_server.go:2190) sets `oversized=true` (2193) and emits `-32600 'Request body too large'` (2196); `req.on('end')` short-circuits if oversized (2206). Call site: generated `TestRPCHandler.handle` (2180). | CONST `MAX_BATCH_SIZE = 100` (ts_client_server.go:2174). CHECK `else if (data.length > MAX_BATCH_SIZE)` (ts_client_server.go:2237) -> `-32600 'Batch exceeds maximum size'` (2239) before `Promise.all(data.map(...))` dispatch (2241). Call site: `req.on('end')` handler (2205). | emitted by shared template `ts_client_server.go`; exact emitted filename UNVERIFIED | INSPECTED (shared template) — both caps enforced in generated adapter |
| ts-cjs | Same shared template `ts_client_server.go` as ts-node: CONST body (2173) / batch (2174); CHECK body (2190) / batch (2237). Module-format-specific filename not inspected. | Same shared template (2174 constant, 2237 check). | emitted by shared template `ts_client_server.go`; exact emitted filename UNVERIFIED | INSPECTED (shared template) — caps enforced; CJS-specific output path UNVERIFIED |

## Findings

Runtimes that **actually enforce both caps at the generated adapter layer**:
- **Go** — body via `http.MaxBytesReader` (go_client_server.go:969) and batch via `len(requests) > maxBatchSize` (993); call site `handleRequest`.
- **C#** — body via `HttpTransport.ReadLimitedAsync` (csharp_client_server.go:866) plus content-length precheck (856) and batch via `batchLength > MaxBatchSize` (902); call site `HandleRequest`. Streaming cap itself lives in the already-surveyed runtime transport `HttpTransport`.
- **TypeScript (ts-node and ts-cjs, shared template)** — body via `size > MAX_BODY_SIZE` (ts_client_server.go:2190) and batch via `data.length > MAX_BATCH_SIZE` (2237); call site `TestRPCHandler.handle`.

Runtimes enforcing caps only partly at the adapter layer (constant/check split with runtime transport):
- **Java** — the adapter performs the comparisons and dispatches the error (java_client_server.go:1054, 1055, 1063, 1104), but the constants `HttpLimits.MAX_REQUEST_BODY` / `HttpLimits.MAX_BATCH_SIZE` and the bounded-read implementation live in the already-surveyed runtime transport `HttpLimits`. No new runtime-side cap found.

Runtimes **not confirmed** to enforce caps at the adapter layer:
- **Python** — body cap present only in generated `TestRPCHandler` (python_client_server.go:1030/1040); no batch cap in the generated adapter (delegated to `rpc_server.call`, 1056). Production-server cap location UNVERIFIED.
- **Python2** — not inspected in this pass; status UNVERIFIED.

No cap was found that exists *only* in a runtime transport without an adapter-side call site, other than the constants/bounded-read helpers in the already-surveyed Java `HttpLimits` and C# `HttpTransport`.

## Residual

- Python2 adapter template: not inspected — body/batch caps UNVERIFIED.
- Python production server (`pulserpc.server.DEFAULT_MAX_BODY_SIZE`, hinted at python_client_server_test.go:1882): not inspected; only the generated test handler was verified.
- Python generated handler has no batch-size cap; whether runtime `rpc_server.call` enforces one is UNVERIFIED.
- Exact emitted filenames for the Python and TypeScript (`ts-node`/`ts-cjs`) generated adapters were not confirmed; the cap-bearing code was located in the generator templates.
- Java `HttpLimits` and C# `HttpTransport` constant values / bounded-read internals were not re-read here (already surveyed in S3); only their call sites in the generated adapters were verified.
