# Wave 4 — Focused Security Review (report-only)

## Method and honesty note

This wave is **report-only**: no code changes were made. Findings below are split into
**CONFIRMED** (I read the exact file/line in this session) and **UNVERIFIED** (I did not
directly confirm it; stated as a hypothesis only).

What I actually inspected this session: `pkg/playground/manager.go`,
`pkg/playground/files.go`, `pkg/playground/manager_test.go` (test bodies for
`TestGetFile`/`TestGetFileRejectsTraversal`), `pkg/webui/server.go:290-343`,
`tests/integration/test_internal_errors.sh` (full file),
`tests/integration/test_http_api.sh:995-1031`, `tests/integration/test_all_runtimes.sh:85-184`,
`scripts/test-servers.sh:20-89`, and git history for `pkg/playground/files.go`.
I also ran one broad grep over `pkg/runtime/runtimes` for exception/error-detail patterns;
that grep is **not** a careful inspection of each runtime's dispatch code.

What I did **not** do: I did not execute any Go test, I did not run any of the seven live
runtime probes (Docker is unavailable locally), I did not read each of the seven runtime
server implementations end-to-end, and I did not open `.github/workflows/ci.yml`. Any
claim depending on those is marked UNVERIFIED.

## Findings table

| ID | Severity | File:line | Hypothesis | Status + confirmation method | Fix / recommendation |
|----|----------|-----------|------------|------------------------------|----------------------|
| S1-a | Low (mitigation present) | `pkg/playground/files.go:18-43`, `:46-64`; `pkg/webui/server.go:290-343` | Playground `/api/playground/files/<session>/<path>` allows `../`, encoded, or absolute path to read files outside the session dir. | **CONFIRMED MITIGATED (code read).** `resolveSessionFile` rejects empty/absolute/volume/backslash-normalized paths, cleans with `filepath.Clean`, and `filepath.Rel`-checks containment; `GetFile` calls it before `os.ReadFile`. The web handler passes the URL suffix to `GetFile`. Git history shows `e7ce7ce fix(playground): contain session file paths`. | Keep. Add an end-to-end HTTP handler test (see S1-c). |
| S1-b | Low (defense-in-depth) | `pkg/playground/files.go:18-43` (no `EvalSymlinks`); `:58` (`os.ReadFile`) | A symlink inside a session dir pointing outside is followed when reading. | **PARTIALLY CONFIRMED / EXPLOITABILITY UNVERIFIED.** I confirmed the resolver is *lexical only* (no `filepath.EvalSymlinks`). I did **not** confirm any generator writes symlinks into the session dir, so there is no demonstrated attacker path to create one. | Optionally resolve the final path with `EvalSymlinks` and re-check containment before read; document that symlink creation requires pre-existing FS write access. |
| S1-c | Low (unverified) | `pkg/webui/server.go:299` (`r.URL.Path`) | URL-encoded `%2e%2e%2f` bypasses containment. | **UNVERIFIED (reasoned, not tested).** Handler uses `r.URL.Path`, which Go's `net/http` decodes before handler entry, so `%2e%2e` becomes `..` and reaches the existing resolver. I did not run a probe to confirm. | Add one HTTP-level test using an encoded traversal suffix through `handlePlaygroundFiles`. |
| S1-d | Low | `pkg/webui/server.go:332` | Internal read errors echo detail to the client. | **CONFIRMED (code read).** `fmt.Sprintf("Failed to read file: %v", err)` is returned with HTTP 404; the wrapped error includes the caller-supplied filename. This is an echo of user input plus generic OS read errors, not a secret, but it is unnecessary detail. | Replace with a constant message (e.g. "File not found") and log internally. |
| S6-a | N/A (gate present) | `tests/integration/test_internal_errors.sh:222-245`; `tests/integration/test_all_runtimes.sh:89,145`; `scripts/test-servers.sh:28-36` | Internal errors leak exception/stack/sentinel instead of `-32603`. | **PROBE CONFIRMED PRESENT AND WIRED (code read), RUNTIME BEHAVIOR UNVERIFIED.** The script asserts HTTP 200, `jsonrpc=2.0`, id echo, `-32603`, exactly `"Internal error"`, absent `error.data`, and absence of `PULSERPC_SECRET_TOKEN_7F3A`. The harness runs it for all seven runtimes (`python ts csharp java go python2 ts-cjs`). I did **not** run it, so I cannot claim all seven currently pass. | Re-run in CI (harness already does); if any runtime fails, fix that runtime's generic error path. |
| S6-b | Low (unverified) | `pkg/runtime/runtimes/csharp/PulseRPC/Server.cs:157` (grep hit only) | Exception text escapes into a wire response. | **UNVERIFIED.** A broad grep showed `Fail(-32602, "Invalid params", ex.Message)`. This is a `-32602` validation path, which the error-plans intentionally keep verbose; I did not read the full dispatch path or the other six runtimes. | Inspect all seven server dispatch paths for `-32603` responses carrying `ex.Message`/`StackTrace`/`str(exc)`; confirm fixed message + no `data`. |
| S3 | Medium (unverified) | JSON-RPC envelope decoders in all 7 runtimes (not inspected) | Unbounded nesting, duplicate keys, numeric IDs > 2^53, non-string methods, huge batches, no response cap. | **UNVERIFIED — NOT INSPECTED THIS PASS.** I only saw response-size-limit references in a grep (`ts-node`/`ts-cjs` `transport.ts`, `csharp/PulseRPC/Transport.cs` `MaxResponseBytes`); I did not read the decoders, so I cannot state which are bounded. | Survey each runtime's parse loop for depth/batch/ID caps and response caps; add focused tests where missing. |
| S4 | Medium (unverified by me) | `.github/workflows/ci.yml` (not opened this session) | No SAST / dependency-audit step in CI; `make lint` is not a CI job. | **UNVERIFIED BY DIRECT INSPECTION.** The mission brief asserts this, but I did not open `ci.yml`, so I am not presenting it as confirmed. A reviewer should confirm with one read. | If confirmed, add low-flake steps: `govulncheck` for Go modules, `npm audit --omit=dev` for the web UI, and a Dependabot config. Do not gate on advisory freshness to avoid flakes. |

## What I ruled out

- **Basic S1 traversal (`../`, `..\`, `a/../../`, absolute and slash-normalized absolute paths):**
  ruled out at the library layer by direct code inspection of `resolveSessionFile`
  (`files.go:18-43`) and by the existing cases in `TestGetFileRejectsTraversal`
  (`manager_test.go:493-505`). **Caveat:** the tests were not executed in this pass.
- **S6 gate absent from the harness:** ruled out by reading `test_all_runtimes.sh:89,145`
  and `test_internal_errors.sh:222-245` — the hard-gate probe exists and is invoked for
  all seven runtimes. Whether all seven *pass* remains unverified here.
- **No code was changed**, so no observable behavior was modified in this wave.

## Prioritized remediation list

1. **Confirm S6 live** — run the existing `test_internal_errors.sh` across all seven runtimes
   in CI; this is the highest-value verification and requires no code change.
2. **Confirm S4** with a single read of `ci.yml`; if the gap is real, add `govulncheck` and
   `npm audit --omit=dev` as non-blocking or carefully pinned steps plus Dependabot config.
3. **Complete S1** with two small tests: an HTTP-level encoded-traversal probe (S1-c) and a
   symlink-escape probe against `resolveSessionFile` (S1-b); harden with `EvalSymlinks` only
   if a realistic symlink-creation path is demonstrated. Change `server.go:332` to a constant
   error message (S1-d).
4. **Survey S3** envelope decoding across runtimes and add bounded-depth/batch/ID tests where
   caps are missing; do not redesign transports.

## Deliberately skipped

- All live runtime/HTTP probes (Docker unavailable locally).
- Any code fix, config change, or new dependency (report-only wave).
- Careful per-runtime server dispatch reading for S6/S3 (only a broad grep was performed).

## Orchestrator follow-up verification (read-only)

- **S4 CONFIRMED:** `.github/workflows/ci.yml` runs gofmt-on-changed-files,
  `go vet`, `make test`, the integration harness, cross-generator checks, and
  seven runtime unit jobs. There is no SAST, dependency-audit, or coverage step;
  `make lint` (golangci-lint) is not a CI job.
- **S6 live behavior:** `test_internal_errors.sh` is wired for all seven runtimes
  in `test_all_runtimes.sh`, and the final push's CI Conformance run
  (`45087679745`, commit `d57f9b2`) was green. The hard gate therefore passes
  end-to-end in CI even though it could not be run locally here.
- **S1-d CONFIRMED:** `pkg/webui/server.go:332` returns
  `fmt.Sprintf("Failed to read file: %v", err)` with HTTP 404, echoing the
  caller-supplied filename and raw OS error.
- **S1 mitigation CONFIRMED:** `pkg/playground/files.go:18-43` rejects absolute,
  volume-qualified, backslash-normalized, and `..`-escaping paths before
  `os.ReadFile`. Symlink escape remains possible in principle but no generator
  path that creates symlinks in a session dir was identified.
