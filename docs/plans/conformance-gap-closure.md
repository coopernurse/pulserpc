# Conformance Gap Closure Plan

Companion to the gap register derived from `docs/audit/invariants.md`,
`docs/audit/invariants-additional.md`, and the canonical D1–D16 set in
`docs/RUNTIME_IMPLEMENTATION_GUIDE.md`.

## Goal

Turn the multi-runtime conformance job from an **envelope-and-error-code** gate
into a **behavioral** gate. Today the seven-runtime matrix proves generated code
compiles, starts, dispatches the `conform.pulse` methods, and returns the right
JSON-RPC error codes. It does not prove that results are correct, that canonical
D1/D4/D5 are enforced, or that clients validate responses.

## Current gate (baseline)

`ci-conformance.yml` → `make test-all-runtimes` → `test_all_runtimes.sh` →
`test_http_api.sh` (~41 probes) + `test_limits.sh` (2 probes) for each of
python, ts, csharp, java, go, python2, ts-cjs. `make test-quickstarts` additionally
compiles/runs the generated examples.

Structural weaknesses:

- `test_method` checks only that `result` exists; no computed value is asserted.
- D4 (`-32603` redaction) and D5 (body/batch caps) are **SKIP**-based: 5 of 7
  runtimes skip the redaction probe, and any runtime can drop the caps and stay
  green.
- D1 numeric boundaries, declared `raises()` codes, missing `method`, batch
  edges, and non-POST verbs beyond GET/PUT are not probed in the gate.
- No client is ever exercised: response validation, id matching, headers,
  timeouts, and contract verification are outside the gate.
- Generator artifact checks (C1–C13) are opt-in behind
  `PULSERPC_CROSS_GENERATOR=1`.

## Gap register (condensed)

| # | Priority | Invariants | Gap |
|---|---|---|---|
| 1 | P0 | R2, R5, R6, R26, M10, M11 | No result values asserted |
| 2 | P0 | D4, R15, S6 | Redaction probe SKIPs on 5/7 runtimes |
| 3 | P0 | D5, R16–R18, S7 | Body/batch caps SKIP; response cap untested |
| 4 | P1 | D1, R2, R25 | No int32 boundary / NaN / bool-as-int probes |
| 5 | P1 | R7, raises | Declared error 1001 not in the gate |
| 6 | P1 | M2 | Missing/empty/non-string `method` not probed |
| 7 | P1 | R20, R21 | No concurrency; non-atomic ids/cache races |
| 8 | P1 | M18/M19/M28 | Batch edge cases (boundary, duplicates, ordering) |
| 9 | P1 | M5, M14, R10, M24, D13, D6, D15, R17 | No client-side conformance |
| 10 | P1 | C1–C13, D8/D9/D12 | Generator artifacts gated out of CI |
| 11 | P2 | D3 | Only GET/PUT; 405 message text unasserted |
| 12 | P2 | R3 | Multi-error accumulation unasserted |
| 13 | P2 | D10/D11/G26–G39 | Grammar negatives — unit-only (appropriate) |
| 14 | P2 | D14/S1–S4, R28/S5 | Playground/path/meta — Go-only or unit-only |

## Work packages

Wave 1 closes gaps 1–8 and 11–12. Each package owns disjoint files; the
orchestrator owns `tests/integration/test_all_runtimes.sh`, the root `Makefile`,
and this document, and performs all merging and wiring.

| WP | Worktree | Owns | Scope |
|---|---|---|---|
| WP1 | `cg-probes` | `tests/integration/test_http_api.sh` | Result-value assertions; numeric D1 matrix; raises 1001; missing/empty/non-string method; batch edges; non-POST verbs + message text; multi-error accumulation. **Must not** alter the existing `-32603` redaction block or existing probe description strings (meta-tests depend on them). |
| WP2 | `cg-limits` | `tests/integration/test_limits.sh`, `tests/integration/integration_harness_test.go` | Flip SKIP→FAIL for body and batch caps; add exact-boundary probes (100 entries, ~1 MiB); update limit meta-tests. **Test-only** — report (do not fix) runtimes that appear not to enforce caps. |
| WP3 | `cg-redaction` | `pkg/generator/*_client_server.go`, `tests/integration/servers/python2_conform_server.py`, new `tests/integration/test_internal_errors.sh` | Make the generated `A.repeat` handler throw with a fixed sentinel when `count < 0` in all seven runtimes; new hard-gate script asserts `-32603`, exact `"Internal error"` message, no `data`, sentinel absent. |
| WP4 | `cg-concurrency` | new `tests/integration/test_concurrency.sh` | N parallel requests per server: assert all echoed ids distinct, every response correct, no cross-talk. HTTP-level only (client id uniqueness is WP5). |

Wave 2 (after wave 1 merges and CI reports):

| WP | Scope |
|---|---|
| WP5 | Client-side harness: generated `TestClient` drivers against a mock server returning mismatched ids, both `result`+`error`, missing `jsonrpc`, wrong types. Closes 9. |
| WP6 | Generator artifacts: wire `make test-cross-generator` into CI once the gated deltas are resolved; add generate-twice byte compare and served-IDL diff. Closes 10. |
| WP7 | Response cap (R17), timeout (D6), optional built-ins (D7/C10), enum-return (R27). |

## Acceptance criteria

- Every new probe is a hard FAIL, not a WARN or SKIP.
- Existing probe description strings remain stable (`integration_harness_test.go`
  meta-tests match on them).
- New scripts are wired into `test_all_runtimes.sh` and the root `Makefile` by
  the orchestrator after merge.
- `go test ./tests/integration` passes locally; full validation is the CI
  Conformance job on the merged branch.

## Risks

- WP1's value assertions must normalize JSON number rendering (`5` vs `5.0`)
  across runtimes; floats need a tolerance.
- WP2 flipping SKIP→FAIL may expose runtimes that never implemented the caps;
  those become a generator/runtime fix-up wave.
- WP3 touches all seven generator test-server paths; a missed runtime breaks
  container startup. Verify generation for each plugin before merging.

## Sequencing

1. Wave 1: four parallel worktrees (WP1–WP4).
2. Orchestrator merges, wires new scripts, pushes.
3. CI Conformance is the arbiter; dispatch fix-up agents for any real violations
   revealed by the new hard gates.
4. Wave 2: WP5–WP7, then update this document with final status.
