# Wave 4 — Input-Hardening Bounds in JSON-RPC Envelope Decoders

Scope: enumerate input-hardening bounds (nesting-depth, duplicate keys, numeric-ID >2^53, batch-size cap, response-size cap) in the JSON-RPC envelope decoders of the four runtimes: Python, Python2, ts-node, ts-cjs. Report only; no source changes.

Runtime enumerations are evidence-based from files actually inspected. Unchecked claims are marked UNVERIFIED.

| runtime | nesting-depth bound | duplicate-key behavior | numeric-ID bound (>2^53) | batch-size cap | response-size cap | exact file:line evidence | status (CONFIRMED / UNVERIFIED) |
| --- | --- | --- | --- | --- | --- | --- | --- |
