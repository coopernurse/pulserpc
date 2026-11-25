# Invariant Remediation Plans

Six worktrees, one plan each:

| # | Plan | Worktree / branch | Owns |
|---|---|---|---|
| 0 | [00-core.md](00-core.md) | `pulserpc-fix-core` / `fix-invariants-core` | parser, checksum, playground, webui, shared harness, docs |
| 1 | [01-go.md](01-go.md) | `pulserpc-fix-go` / `fix-invariants-go` | `go_client_server.go`, Go runtime |
| 2 | [02-java.md](02-java.md) | `pulserpc-fix-java` / `fix-invariants-java` | `java_client_server.go`, Java runtime |
| 3 | [03-python.md](03-python.md) | `pulserpc-fix-python` / `fix-invariants-python` | `python_client_server.go`, Python 3 + Python 2 runtimes |
| 4 | [04-typescript.md](04-typescript.md) | `pulserpc-fix-typescript` / `fix-invariants-typescript` | `ts_client_server.go`, ts-node + ts-cjs runtimes |
| 5 | [05-csharp.md](05-csharp.md) | `pulserpc-fix-csharp` / `fix-invariants-csharp` | `csharp_client_server.go`, C# runtime |

**Why six:** Python 2 shares `pkg/generator/python_client_server.go` with
Python 3, and ts-node/ts-cjs share `pkg/generator/ts_client_server.go`.
Splitting either pair would create two worktrees editing the same file. Six
worktrees also matches the requested 5–6 target.

**Source of truth:** [../audit/invariants.md](../audit/invariants.md) (original,
now partly stale) and [../audit/invariants-additional.md](../audit/invariants-additional.md)
(new G/C/R/M/S invariants). Coverage status is in
[../audit/integration-coverage.md](../audit/integration-coverage.md).

---

## Canonical decisions (all plans must implement these identically)

- **D1 — IDL `int` is signed 64-bit.** Valid values are integral JSON numbers
  in `[-9223372036854775808, 9223372036854775807]`. Fractional, NaN/Inf, bool,
  and out-of-range values are invalid in every runtime. Generated native types
  are `int64` (Go), `long`/`Long` (Java), `long` (C#), `int` (Python), and
  `number` (TypeScript). JavaScript numbers are exact only through 2^53-1, so
  the TypeScript runtime validates the safe-integer range; the other runtimes
  support the full range. A `float64` input may represent `-2^63` exactly but
  not `9223372036854775807` (it rounds to `2^63`), so the accepted float bounds
  are `[-2^63, 2^63)`. This is a documented breaking change from signed 32-bit
  `int`.
- **D2 — `id` is string, number, or null.** Servers reject boolean/object/array
  ids with `-32600`. Numeric ids are compared by value.
- **D3 — Non-POST requests** return HTTP **405** with a JSON-RPC error body:
  `{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request - only POST allowed"},"id":null}`.
- **D4 — Internal errors** return `-32603` with the fixed message
  `"Internal error"` and **no `data`**. Exception/panic text, stack traces, and
  handler messages are logged server-side only.
- **D5 — Resource limits:** request body **1 MiB**, response body **10 MiB**,
  batch entries **100**. Generated adapters enforce these and return
  `-32600 "Request body too large"` / `"Batch exceeds maximum size"` as
  appropriate. Constants are documented.
- **D6 — HTTP client timeout:** 30 s request timeout (10 s connect where the
  language separates them).
- **D7 — Optional built-ins are nullable:** Java boxed (`Integer`, `Boolean`,
  `Double`), C# `int?`, `bool?`, `double?`, `string?`. Go represents absence by
  omission (`omitempty`, optional fields only); this is documented, not changed
  to pointers in this pass. Python/TS are dynamic.
- **D8 — IDL JSON schema (all generators):** top-level keys
  `rootNamespace`, `checksum`, `interfaces`, `structs`, `enums`, `errors`;
  declaration comments included; positions excluded.
- **D9 — Checksum:** include the parameter **position**; exclude comments and
  error-message text; 43-character unpadded base64url SHA-256.
- **D10 — Imports:** a trailing comment is allowed; de-duplicate by **resolved
  absolute path**; reject a root namespace that equals an imported namespace.
- **D11 — String literals have no escapes.** Document and test the restriction;
  do not implement escape decoding in this pass.
- **D12 — Generation is byte-deterministic:** sort every map-derived output
  (imports, namespaces, registrations, factories).
- **D13 — Transports** accept optional custom headers and normalize a trailing
  `/` on the base URL.
- **D14 — Playground file access** must reject any path that escapes the
  session directory (clean, relative, no `..`, no absolute path).
- **D15 — Contract verification** fetches `pulserpc-idl` and diffs against it;
  a failed fetch is explicitly incompatible with an `Error` message.
- **D16 — Type-name resolution:** exact key → exact base-name key → first
  key in sorted order sharing the base name.

---

## Ownership and conflict avoidance

- **core** owns `pkg/parser/**`, `pkg/playground/**`, `pkg/webui/**`,
  `pkg/runtime/embed.go`, `tests/integration/**`, `scripts/test-servers.sh`,
  `Makefile`, `docs/RUNTIME_IMPLEMENTATION_GUIDE.md`, `docs/audit/**`,
  `docs/plans/**`.
- Each runtime plan owns its generator file(s), its `pkg/runtime/runtimes/<rt>/**`
  tree, its language docs under `docs/languages/<rt>/**`, and its own tests.
- **Never edit another plan's files.** If a shared change is needed, record it
  in the final report; the orchestrator routes it to core.
- Shared integration probes (M18–M28, R15–R24 limits/security) are written by
  **core**; runtime plans implement the behavior and add focused unit tests.
  Runtime plans may add tests under their own runtime directory.

## Workflow for each implementer

1. Work only in your worktree on your branch. Verify with `git branch --show-current`.
2. Implement P0 tasks first, then P1, then P2 if time permits.
3. Run the plan's test commands; keep the tree green.
4. Commit in Conventional Commits with a kebab-case scope, e.g.
   `fix(java): box optional built-ins`.
5. Report: files changed, tests run/results, deviations from the plan, and any
   shared change that core must make.

## Test commands

- Go: `GOTMPDIR=C:\Users\james\AppData\Local\Temp\opencode go test ./pkg/... ./tests/integration/... -count=1`
- Java: `mvn -q test` in `pkg/runtime/runtimes/java`; generated e2e per plan.
- Python: `python -m pytest pkg/runtime/runtimes/python/tests pkg/runtime/runtimes/python/pulserpc/tests -q`;
  Python 2 in `python:2.7-slim`.
- TypeScript: `npx tsx pulserpc/tests/test_*.ts` (or `make test`) in each runtime.
- C#: `dotnet test tests/PulseRPC.Tests.csproj`.
- Always prefix shell commands with:
  `$env:PATH = [Environment]::GetEnvironmentVariable("PATH","Machine") + ";" + [Environment]::GetEnvironmentVariable("PATH","User");`

## Definition of done

- Every assigned invariant is implemented, tested, and documented.
- New tests assert the behavior (not just source substrings) where practical.
- Unit + targeted integration pass; no unrelated changes.
- Review findings are fixed or explicitly deferred with a reason.
