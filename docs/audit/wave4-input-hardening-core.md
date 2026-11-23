# Wave 4 — Input-hardening bounds in JSON-RPC envelope decoders (Go / Java / C# core runtimes)

Scope: enumerate input-hardening bounds in the JSON-RPC envelope decoders of the Go, Java and C# runtimes.
Method: static read of at most 2 files per runtime (transport / server / envelope decoder). No tests were run.
A bound is CONFIRMED only when an actual runtime check is observed; a constant that exists without an observed
enforcing check is called out explicitly in the evidence.

| runtime | nesting-depth bound | duplicate-key behavior | numeric-ID bound (>2^53) | batch-size cap | response-size cap | exact file:line evidence | status |
|---|---|---|---|---|---|---|---|
