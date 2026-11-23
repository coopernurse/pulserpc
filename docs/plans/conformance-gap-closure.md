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

## Wave 1 results

All four work packages landed and were merged. New hard-gate scripts are wired
into the seven-runtime matrix and exposed as `make test-internal-errors` and
`make test-concurrency`; a meta-test guards the wiring and forbids reintroducing
`TESTS_SKIPPED` in the limit/redaction probes.

The first CI run with the tightened gates failed, which was the intended result:
it separated harness bugs from genuine product divergences.

**Harness corrections** (`test_http_api.sh`):
- String results were compared as unquoted text against JSON string literals.
- `HEAD` cannot carry a response body per HTTP semantics; status is asserted,
  Content-Type/body are not. Curl status strings are normalized to 3 digits.
- The multi-error probe was relaxed to the documented R3 model (fail-fast in
  Go/Java/Python2/C#, accumulating in Python3/TS).

**Product fixes:**
- Invalid `method` envelopes (missing/empty/non-string) now return `id: null`
  in all seven runtimes and in the generated Java server.
- Generated `A.divide` test handlers now implement the declared `1001`
  `ValidationError` on division by zero and real division otherwise.
- The Python 2 conformance server handles DELETE/PATCH/OPTIONS/HEAD with a
  405 JSON-RPC response instead of the base-class HTML 501.
- D4 redaction is deterministic in every runtime via the negative-count
  `A.repeat` handler throwing `PULSERPC_SECRET_TOKEN_7F3A`; the old SKIP-based
  probe was removed.

CI confirmed all checks green after the fix wave: CI (9 jobs), CI Conformance
(`Multi-runtime conformance (Docker)` and `Quickstart guides (Docker)`), and
Documentation CI. Gaps 1–8 and 11–12 are closed.

### Wave 2

- **Gap 10 closed.** Cross-generator determinism (C1/D12), IDL artifact
  checksum/schema checks (C2/C4), and a new embedded-`IDL_JSON` object check
  (C5) now run by default in `go test ./tests/integration/...`; the obsolete
  `PULSERPC_CROSS_GENERATOR` gate and its meta-test were removed.
- **D7/C10 closed.** `conform.pulse` now has optional `int`/`bool`/`float`
  fields and an `A.echoOptional` method implemented by all seven runtimes.
  Probes assert required fields exactly and each optional field individually,
  accepting D7's documented null-or-omitted representation. The first probe
  revision was too strict (exact whole-object equality) and was relaxed after
  CI exposed the legitimate divergence.

CI is green for the merged wave-2 head (`34d10be`): CI (9 jobs) and CI
Conformance (multi-runtime + quickstarts).

### Wave 3

- **Gap 9 closed.** A new `tests/integration/client_conformance_test.go`
  exercises nine canonical envelope cases (wrong version, id mismatch,
  result+error, neither, malformed error code/message, numeric-id equality,
  null required/optional results) against the Go client. The same case list was
  added to Java (new `ClientEnvelopeTest.java`), C#, Python, ts-node, and ts-cjs.
  Malformed error objects are normalized to a safe internal-error fallback in
  those runtimes rather than rejected outright.
- **R17/D6 locked in.** Generator invariant assertions now cover the 10 MiB
  response cap and 30s timeouts in Python/TS/C# generated transports; a Go
  behavioral timeout test and a C# 10s connect timeout were added. The caps and
  timeouts themselves already existed in the runtime transports from earlier
  commits.
- **R27/C11 closed.** `A.pick(MathOp) MathOp` returns an enum over the wire;
  all seven runtimes were probed for declared string serialization and invalid
  enum rejection. CI exposed a real Go bug: `convertToMap`/`ValidateEnumE`
  rejected named string types, so enum-returning methods produced `-32603`.
  The runtime now normalizes named primitives, with a regression test.

All originally identified gaps except the intentionally out-of-scope grammar
and playground/security items are now closed. Remaining follow-ups are
non-conformance concerns (e.g. C# `-package` namespace handling, generated Java
`verifyOnBootstrap()` wiring).

## Post-conformance performance pass

Seven parallel reviews (one per runtime) found two universal inefficiencies:
enum allowed-value sets and inherited struct field lists were rebuilt for every
validated value, and the latter used an O(F²) override scan. Both are now
precomputed once per contract in all seven runtimes with O(1) lookup. Runtime
specific wins:

- **Go:** response validation no longer round-trips typed structs through
  `json.Marshal`/`Unmarshal` (`convertToMap` now uses reflection honoring JSON
  tags); handler reflection is resolved once per call.
- **Python:** both resolution indexes are built once in `Contract.__init__` and
  threaded through validation, removing a global lock acquisition per param.
- **Python 2:** namespaced user-defined types resolve via a precomputed
  simple-name map instead of an O(S+E) `endswith` scan per value.
- **ts-node / ts-cjs:** enum and struct metadata cached in `WeakMap`s; required
  param counting no longer allocates a filter array per request.
- **C#:** handler `MethodInfo`/`ParameterInfo` resolved at registration;
  by-name params normalized in one case-insensitive dictionary; IDL entries
  resolved once per call.
- **Java:** handler method cache and single IDL resolution per call.

All observable behavior (error codes/messages, validation results, wire format)
is unchanged and pinned by new focused tests. Full local suites and both CI
workflows are green.

## Sequencing

1. Wave 1: four parallel worktrees (WP1–WP4) — done.
2. Orchestrator merged, wired new scripts, pushed.
3. CI Conformance arbitrated; fix-up wave applied and re-pushed.
4. Wave 2: WP5–WP7, then update this document with final status.
5. Wave 3: enum returns, client caps/timeouts, gap 9 — done.
6. Post-conformance performance pass: seven parallel runtime reviews and fixes — done.
