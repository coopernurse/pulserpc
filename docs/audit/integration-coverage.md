# Integration-Test Coverage Assessment

Companion to [invariants.md](invariants.md) and
[invariants-additional.md](invariants-additional.md). Four coverage reviewers
(grammar, generator, runtime, message/security) mapped every invariant to the
integration suite at `f2de19b`.

**What counts as integration coverage:** the cross-runtime harness under
`tests/integration/` (the `.sh` scripts plus `integration_harness_test.go`),
per-runtime `make test-integration` targets, and the generator/quickstart
harnesses that generate code, start a server, and exchange wire traffic.
Unit tests (`*_test.go`, `tests/test_*.py`, `pulserpc/tests/*.ts`, JUnit,
xunit) are **not** integration coverage and are classified as `U`.

**Statuses:** **I** asserted end-to-end · **I-indirect** exercised end-to-end
but the key property is not asserted · **U** unit-only · **N** none.

---

## Executive summary

The integration suite is a **happy-path consumer of one fixture family**
(`examples/conform.pulse` + `conform-inc.pulse`), not a contract test suite.
It reliably proves that generated code compiles, starts, dispatches the
conform methods, and returns basic JSON-RPC errors. It does **not** assert most
grammar negatives, protocol-envelope details, checksum/IDL-artifact contents,
security behavior, resource limits, or concurrency.

For the 64 newly catalogued invariants:

| Status | Count | Share |
|---|---:|---:|
| I (asserted end-to-end) | 6 | 9% |
| I-indirect | 14 | 22% |
| U (unit-only) | 28 | 44% |
| N (no coverage) | 16 | 25% |

In other words, **~69% of the new invariants have no end-to-end assertion**,
and 25% have no test of any kind.

Three suite-level problems amplify the gap:

1. **The seven-runtime matrix is nominal.** `test_all_runtimes.sh` loops seven
   runtimes, but `scripts/test-servers.sh` configures only
   `python, ts, csharp, java, go`. `python2` and `ts-cjs` are skipped by the
   `grep -q "\"$runtime:"` guard (`test_all_runtimes.sh:80-85`).
2. **Several high-value scripts are orphaned.** Only
   `test_all_runtimes.sh` and `test_generator_java.sh` are wired in the root
   `Makefile`; per-runtime Makefiles wire the Go/C#/TS generator scripts.
   `test_numeric_types.sh`, `test_enum_case.sh`, `test_raises_propagation.sh`,
   `test_static_client_ts.sh`, `test_java_compile.sh`, `test_errors.sh`, and the
   OpenAPI scripts are not run by any Makefile target.
3. **`test_http_api.sh` only checks error codes and field presence.** It greps
   for `"error"`/`"result"` and for the expected field name; it does not assert
   result values, `jsonrpc`, echoed `id`, HTTP status, `Content-Type`,
   `error.message`, or `error.data`.

---

## New invariant coverage (the core ask)

### Grammar / parser

| Invariant | Status | Evidence / notes |
|---|---|---|
| G26 no string escapes | **N** | No fixture/script uses `\"` or a backslash |
| G27 return type mandatory, no `void` | I-indirect | All `conform.pulse` methods have a return type; no `void` attempt |
| G28 parameters positional/order-significant | I-indirect | Positional dispatch exercised; order not asserted (`add` is symmetric) |
| G29 import regex / trailing-comment failure | **N** | No import with a trailing token/comment |
| G30 import dedup by literal path | **N** | No `a.pulse` vs `./a.pulse` case |
| G31 imports relative/absolute | I-indirect | Relative only (`conform.pulse:15`) |
| G32 root/import namespace collision | **N** | No colliding fixture |
| G33 asymmetric import prefixing | I-indirect | Works via `inc.Response`/`inc.MathOp`; not asserted directly |
| G34 checksum excludes messages/comments | U | `checksum_test.go` only |
| G35 checksum ignores parameter order | U | No integration checksum comparison |
| G36 decimal, optionally-negative codes | I-indirect | `1001` asserted; negatives only unit |
| G37 one namespace, order-independent | I-indirect | Always first element |
| G38 root declarations unprefixed | **I** | Dispatch is literally `"A.add"` |
| G39 empty files may omit namespace | I-indirect | Generic rejection only |

### Generator / output

| Invariant | Status | Evidence / notes |
|---|---|---|
| C1 byte-deterministic generation | **N** | No script generates twice/diffs. The TS unit test (`ts_client_server_test.go:1225-1250`) compares `collectedFiles[relPath]` to itself in the same iteration and is **vacuous**. Java import order varies run-to-run (verified). |
| C2 uniform `idl.json` schema (`rootNamespace`) | **N** | Go omits `rootNamespace`; no script inspects keys |
| C3 uniform IDL metadata placement | I-indirect | Python `<ns>/idl.json` asserted (`test_quickstart_python.sh:59`); Go root/TS namespace/Java-C# embed only implicit |
| C4 `idl.json` checksum == embedded checksum | U | `go_generator_audit_test.go` only |
| C5 `IDL_JSON` is a JSON object | U | Readiness probes discard the body |
| C6 TS config overwrite refusal | U | Scripts always use fresh dirs |
| C7 TS module precedence / `-ts-no-detect` | U | Integration passes explicit `-ts-module` |
| C8 stable per-runtime layout | **I** | Paths asserted per generator |
| C9 Go `omitempty` on optional only | **N** | No test contains `omitempty` |
| C10 nullable optional built-ins | **N** | No fixture has optional `int`/`bool`/`float`; Java/C# primitive bug unreachable |
| C11 enums as wire strings | I-indirect | String enum input asserted; output form not asserted |
| C12 generator defaults / required flags | I (partial) | Jackson default + inline runtime asserted; missing `-package` path not |
| C13 generated code compiles | **I** | `mvn test-compile`, `dotnet build`, `tsc`, `go run`, Python import |

### Runtime / wire

| Invariant | Status | Evidence / notes |
|---|---|---|
| R15 internal error detail leakage | **N** | No handler-throws probe; C# still emits message+stack trace |
| R16 request-body cap | U | Only the generated Python source string is checked |
| R17 response-body cap | U | Only the generated Go source string is checked |
| R18 batch-count cap | U | Python unit only |
| R19 HTTP timeouts | U | No slow-server probe; generated Go has no timeout |
| R20 concurrent client ids | **N** | No concurrency test; runtime clients use non-atomic counters |
| R21 Python cache thread safety | **N** | No threading test; `_index_cache` has no lock |
| R22 POST-only status/shape | **N** | All probes use POST; 405/501/200 divergence untested |
| R23 notification no body | I (narrow) | Only ts-cjs, only an all-notification batch (`test_generator_ts.sh:93-99`) |
| R24 `id` is string or number | **N** | No boolean/object id, no id-type assertion |
| R25 uniform `int` range | **N** | No values ≥ 2^31 anywhere |
| R26 `float` is double | I-indirect | Float calls succeed; precision not asserted |
| R27 enums as strings | I | Enum input/case asserted for 5 runtimes (`test_enum_case.sh`); python2/ts-cjs absent |
| R28 `meta` prototype safety | U | TS unit only; no hostile `meta` over the wire |
| R29 custom headers | U | Constructor-only unit; Java transport cannot set headers |
| R30 trailing-slash normalization | U | No base-URL variant; Java/C# trimming untested |
| R31 verify against fetched server IDL | U | Generated Go/Java/C# never call `VerifyCompatibility` in an integration script |
| R32 comments in `idl.json`, excluded from checksum | U | Parser/checksum units only |

### Message validation / security

| Invariant | Status | Evidence / notes |
|---|---|---|
| M18 empty batch → single `-32600` | U | Python unit; no live `[]` |
| M19 non-object batch entries | U | Python unit only |
| M20 object-form params | U | Never sent over HTTP |
| M21 `id: null` = notification | U | Only a generated-source string check |
| M22 `error.data` omitted when nil | U | Object-model unit only |
| M23 null result → `"result": null` | I-indirect | `grep "result"` cannot distinguish null |
| M24 numeric-id comparison | U | Java/Go unit only; no echoed-id assertion |
| M25 error object shape | I-indirect | Codes asserted; `message` never |
| M26 malformed JSON → `-32700` | U | Java generated-source substring only |
| M27 `jsonrpc:"2.0"` both directions | I-indirect | Request side yes; response side never asserted |
| M28 notifications in batch | I-indirect | All-notification batch only; mixed batch unasserted |
| S1 playground path containment | **N** | No `../` or encoded-traversal probe; `Manager.GetFile` has no containment check |
| S2 embed forward slashes | U | Host-OS unit only |
| S3 ZIP forward slashes | U | `zip_test.go` only |
| S4 OS path separators | U | Namespace-path units only; CI is Linux |
| S5 `meta` prototype pollution | U | TS unit only |
| S6 internal detail redaction | U | Go panic unit only |
| S7 amplification limits | U | Python batch unit only |
| S8 non-POST rejection | **N** | No non-POST probe against generated servers |

---

## Existing-invariant coverage (summary)

The original catalog fares no better end-to-end. The rows with real
integration assertions are:

- **G3, G6, G9, G11, G14, G17, G38** — identifiers, four scalars, optional
  fields/returns, decimal error codes, type resolution, `extends`, unprefixed
  root names. Mostly indirect through `conform.pulse`.
- **R2, R7, R11, R27** — scalar int acceptance, `RPCError.code` for a declared
  error, ts-cjs notification handling, enum strings.
- **M3, M9, M10, M11, M17** — method-not-found code, arity, param type,
  enum/fractional rejection, harness fails on wrong codes.

Everything else is unit-only or uncovered. Notable existing gaps:

- **G8 maps** — no `.pulse` fixture in the repo uses `map[`; generator/runtime
  map paths are unit-only.
- **G18 cycles** — all cycle tests are unit-level; no generated artifact is
  built from a cyclic IDL.
- **G20-G22 checksum** — no integration script reads or compares a checksum.
- **R3-R6, R8-R10, R12-R14** — aggregation, struct required/optional,
  `extends`, resolution, constants, transports, response validation, `meta`,
  contract diff, void returns: unit-only or uncovered.
- **M1/M2/M5/M6/M8/M12-M16** — response envelope, malformed JSON, params shape,
  extra fields, `data` shape, response validation, verification, severity:
  unit-only or uncovered.
- **M4/M7/M23/M25/M27/M28** — exercised but only for ts-cjs or only at
  presence level.

---

## Top gaps, ordered by risk

1. **S1 — playground path traversal has zero coverage.** `Manager.GetFile` joins
   the session dir with an untrusted filename and the web handler passes the raw
   URL suffix. No `../`, encoded traversal, or absolute-path test exists.
2. **R15/S6 — internal error leakage has zero end-to-end coverage.** No runtime
   is probed with a throwing handler; C# still returns `e.Message` and
   `e.StackTrace`.
3. **M26/M18/M21/M19/M20 — protocol-error paths are unit-only.** No live
   malformed JSON, empty batch, `id:null`, non-object batch entry, or
   object-form params probe.
4. **R20/R21/R19 — concurrency, caching, and timeouts are untested.** No
   parallel calls, no threaded Python validation, no slow-server timeout.
5. **R16/R17/R18/S7 — amplification limits are untested and inconsistent.**
   Only Python caps body/batch; only Go caps responses.
6. **R22/S8 — POST-only behavior has no test and is divergent.**
7. **R24/M24 — id type and echo are never asserted.** A regression to
   `id:null`/wrong-id responses would pass.
8. **C1/C2/C9/C10 — generator-artifact correctness is untested (and C1's unit
   test is vacuous).** Determinism, `rootNamespace`, Go `omitempty`, and
   optional-primitive nullability all slip through.
9. **G26-G35 — grammar negatives are unit-only or absent.** Import trailing
   comments, literal-path dedup, root/import namespace collisions, string
   escapes, cycle detection, and the parameter-order checksum gap are unguarded.
10. **The runtime matrix and Makefile wiring are incomplete.** `python2` and
    `ts-cjs` are skipped by `test_all_runtimes.sh`; six useful scripts are
    orphaned.
11. **R31/M15 — contract verification is never executed end-to-end.** Only
    generated-source string checks exist.
12. **M12 — unknown-field permissiveness has no test**, so the documented
    "never reject extras" contract could invert silently.

---

## Recommended integration tests

Small, high-leverage additions, in priority order:

1. **Protocol probe set in `test_http_api.sh`** (apply to every runtime):
   assert `jsonrpc`, echoed `id`, HTTP status, `Content-Type`; malformed JSON →
   `-32700`; empty batch → single `-32600`; non-object batch entry; `id:null`
   notification → no body; object-form params; extra fields accepted;
   `error.message`/`error.data` shape; non-POST rejection.
2. **Security probe:** a handler that throws a secret-bearing exception; assert
   the client sees only a generic `-32603` (and no stack trace). Add oversize
   body/response and over-limit batch probes.
3. **Concurrency probe:** N parallel calls per client; assert all ids are
   distinct and every response matches. Add a threaded Python validation loop.
4. **Generator artifact checks:** generate twice and byte-compare (C1); parse
   `idl.json` and assert `rootNamespace` + `checksum` (C2/C4); compare
   `idl.json` to the served `pulserpc-idl` result (C4/C5); add an
   optional-primitive struct fixture and assert null round-trips (C10).
5. **Grammar negative fixtures:** import with trailing comment, duplicate
   resolved import, root/import namespace collision, string escape, dotted
   namespace, cycle, duplicate members, `map[...]`, nested arrays.
6. **Fix the vacuous determinism unit test** and add a real `Generate`×2
   comparison.
7. **Wire the orphaned scripts** into `make test-integration` / `quality-full`,
   and add `python2`/`ts-cjs` to `scripts/test-servers.sh`.
8. **Playground traversal test:** `../`, `%2e%2e`, absolute paths against
   `GetFile` and the web handler.

These map directly to the N/U rows above and would move the most important
invariants from "unit-only" to "asserted end-to-end" without requiring a new
harness.
