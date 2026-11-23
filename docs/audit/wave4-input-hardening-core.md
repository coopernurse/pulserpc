# Wave 4 — Input-hardening bounds in JSON-RPC envelope decoders (Go / Java / C# core runtimes)

Scope: enumerate input-hardening bounds in the JSON-RPC envelope decoders of the Go, Java and C# runtimes.
Method: static read of at most 2 files per runtime (transport / server / envelope decoder). No tests were run.
A bound is CONFIRMED only when an actual runtime check is observed; a constant that exists without an observed
enforcing check is called out explicitly in the evidence.

| runtime | nesting-depth bound | duplicate-key behavior | numeric-ID bound (>2^53) | batch-size cap | response-size cap | exact file:line evidence | status |
|---|---|---|---|---|---|---|---|
| Go | No app-level depth limit in inspected files. Client response decode uses `json.Unmarshal` (only the stdlib internal max-depth guard applies; no explicit check). | No duplicate-key check. Go `encoding/json` resolves duplicate object keys last-wins silently (transport.go:204). | No range/precision bound. `validRequestID` accepts any number type (server.go:224-236); `numericID` converts `json.Number` via `.Float64()` (lossy for >2^53) and `idsEqual` compares float64 (transport.go:43-59, 27-41). | None observed. `Server.Call` handles a single request map; no batch loop or cap in either file (server.go:48). | CONST+CHECK: `maxResponseSize = 10 * 1024 * 1024` (10 MiB) at transport.go:110; enforced by `readCappedBody` (io.LimitReader(limit+1) + `len(data) > limit` error) at transport.go:115-124; called on the client response read at transport.go:189. This is a CLIENT-side response cap, not a server output cap. | transport.go:110,115-124,189,204,43-59; server.go:48,224-236 | CONFIRMED (response-size cap only); nesting/duplicates/numeric-ID/batch UNVERIFIED |
