# Wave 4 — Input-Hardening Bounds in JSON-RPC Envelope Decoders

Scope: enumerate input-hardening bounds (nesting-depth, duplicate keys, numeric-ID >2^53, batch-size cap, response-size cap) in the JSON-RPC envelope decoders of the four runtimes: Python, Python2, ts-node, ts-cjs. Report only; no source changes.

Runtime enumerations are evidence-based from files actually inspected. Unchecked claims are marked UNVERIFIED.

| runtime | nesting-depth bound | duplicate-key behavior | numeric-ID bound (>2^53) | batch-size cap | response-size cap | exact file:line evidence | status (CONFIRMED / UNVERIFIED) |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Python | none in inspected files (server takes decoded dict; client json.loads at transport.py:102, no explicit depth cap) | none explicit; stdlib json.loads last-wins default (not in these files) | NONE - any finite int/float accepted, no >2^53 magnitude check (server.py:35-38) | 100 (DEFAULT_MAX_BATCH_SIZE server.py:23, enforced :148) | 10 MiB client-side read cap (transport.py:12, enforced :88-93) | pkg/runtime/runtimes/python/pulserpc/server.py:23,26-39,148,164-182 ; transport.py:12,88-102 | CONFIRMED (batch/response/id-type); UNVERIFIED (depth, duplicate-key) |
