# Wave 4 - Concurrency / Thread-Safety Audit

Scope: caches introduced by the 2026-11-22 performance pass:
`7ed4ea1` (Go), `6e5b339` (Python), `e85365a` (Python2), `3c14ef2`
(ts-node), `f0a2336` (ts-cjs), `3a7d555` (C#), `fadb530` (Java).

## Honesty / method note

In this wave I directly re-read the **Go** runtime sources only
(`validation.go`, `types.go`, `contract.go`). I did **not** re-open the C#,
Java, Python/Python2 or TS sources, and during inspection I did **not** run any
test suite or the race detector. Every row below that says UNVERIFIED reports
the hypothesis and the check that is still owed, not a confirmed result. The
one dynamic check attempted in this wave is recorded at the bottom.

## Findings table

| Severity | File:line | Hypothesis | Confirmed / Refuted + how | Fix / recommendation |
|---|---|---|---|---|
| medium (claimed high) | `pkg/runtime/runtimes/go/pulserpc/validation.go:379-416`, `:422-429` | `typeResolver.enumValues` is a plain `map[string]*enumValueSet` written lazily during concurrent requests -> concurrent-map-write race | **REFUTED by static inspection:** `newTypeResolver` builds `enumValues` once at construction (lines 396-413). `enumSetFor` only *reads* it (line 424) and, on a miss, builds a fresh set that is never stored (line 428). No post-construction write path exists. Dynamic confirmation UNVERIFIED. | None. Keep `enumValues` build-at-construction and never insert after publication. |
| medium (claimed high) | `pkg/runtime/runtimes/go/pulserpc/validation.go:379-386`, `:432-460` | Resolved struct-field cache `typeResolver.structFields` is a plain lazily-written map shared across concurrent requests | **REFUTED by static inspection:** all reads/writes go through `sync.RWMutex` `fieldsMu` (RLock 436-438, Lock 446-458) with double-checked publication. The expensive `resolveStructFields` runs into a call-local `scratch` map (line 443) with no lock held. Dynamic confirmation UNVERIFIED. | None. Retain the `RWMutex` and the lock-free local scratch. |
| low | `pkg/runtime/runtimes/go/pulserpc/types.go:122-184` | The per-chain memo map in `resolveStructFields` is shared across goroutines | **REFUTED by static inspection:** `cache` is a per-call `scratch` created in `getStructFields` (validation.go:443); `fields[i] = field` (types.go:169) mutates the freshly-appended slice, not a cached slice. | None. |
| low | `pkg/runtime/runtimes/go/pulserpc/contract.go:318-393`, `:691-717` | Reflection conversion (`convertToMap`) mutates shared state | **REFUTED by static inspection for contract.go:** `convertToMap`, `collectJSONFields`, `selectJSONFields` use only local maps/slices; package-level `jsonMarshalerType`/`textMarshalerType` (lines 395-398) are read-only. The `server.go` handler-method-reuse row was NOT re-inspected this wave -> UNVERIFIED. | None for contract.go. |
| medium | `pkg/runtime/runtimes/csharp/PulseRPC/Contract.cs` (`_structFields`, `ConcurrentDictionary.GetOrAdd`); `Server.cs` (`_handlerMethods` built in `AddHandler`) | Lazy `GetOrAdd` factory side effects; cached `MethodInfo`/`ParameterInfo` arrays exposed and mutated by callers; concurrent `AddHandler`-vs-`Call` | **UNVERIFIED** - sources not re-opened, `dotnet test` not run this wave. These are the risks to check, not findings. | Run `dotnet test pkg/runtime/runtimes/csharp/tests/PulseRPC.Tests.csproj --nologo -v q`; add a parallel-call stress test; confirm the factory is side-effect-free and cached arrays are never mutated by callers. |
| medium | `pkg/runtime/runtimes/java/com/bitmechanic/pulserpc/Types.java` (weak-keyed cache); `Server.java` (`methodCache` ConcurrentHashMap) | Unsafe publication of lazily-built immutable structures; cache keys not scoped to the definitions map; concurrent `addHandler`-vs-call | **UNVERIFIED** - sources not re-opened, `mvn test` not run this wave. Existing `ClientConcurrencyTest.java` covers only ID uniqueness. | Run `mvn test`; add a cache-stress test; confirm immutable structures are safely published and keys include the definitions-map identity. |
| low | Python/Python2 `contract` caches; global LRUs guarded by `_INDEX_CACHE_LOCK` / `_FIELDS_CACHE_LOCK` (`test_resolution_index.py`, R21) | A new cache is mutated during validation after `__init__` | **UNVERIFIED this wave** - not re-inspected, threaded test not run. Python 2 has 17 pre-existing Python-3-incompatibility failures - do not touch them. | Add a threaded validation test asserting caches are unchanged after concurrent validation. |
| low | TS-node/TS-cjs `WeakMap` caches keyed by definition objects | worker_threads / shared-object risk on the WeakMaps | **UNVERIFIED** - not re-inspected. JS is single-threaded per worker; no risk unless the same WeakMap is shared via SharedArrayBuffer. | Note the risk; no change forced if no shared objects exist. |

## What is SAFE, and why

- **Go (statically inspected this wave):** enum membership sets are built once in
  `newTypeResolver` and only read afterwards; struct-field results are guarded by
  `fieldsMu` (`sync.RWMutex`) with a lock-free call-local scratch; the per-chain
  memo is local to each call; `convertToMap` and its reflection helpers touch no
  shared mutable state. The "plain map written lazily during request handling"
  hypothesis is **disproven by inspection** for the files read (dynamic race
  confirmation still owed - see below).
- **Python global LRU caches:** guarded by `_INDEX_CACHE_LOCK` /
  `_FIELDS_CACHE_LOCK`, with R21 and `test_resolution_index.py` coverage - safe
  by design (carried forward from the earlier assessment; not re-verified this wave).
- **Python contract-level caches:** immutable after `__init__`, so no mutation
  occurs during validation - safe by design (not re-verified this wave).
- **TS-node / TS-cjs WeakMaps:** keyed by definition objects and confined to a
  single-threaded worker; no cross-worker mutation of the same map - safe by design
  (not re-verified this wave).

## Recommended CI hardening

Add a `go test -race ./pkg/runtime/runtimes/go/... ./pkg/generator -count=1`
step to `.github/workflows/ci.yml`, scoped to locally runnable packages and
keeping the existing required status-check names unchanged. (NOT applied in this
report-only wave.)

## Dynamic check result

**NOT RUN - blocked locally.** `go test -race` requires cgo and a C compiler;
this WSL Ubuntu image has neither `CGO_ENABLED=1` nor `gcc`/`clang`/`cc`.
Command and exact output:

```
$ go test -race ./pkg/runtime/runtimes/go/... ./pkg/generator -count=1
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
# (after exporting CGO_ENABLED=1)
# runtime/cgo
cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in $PATH
FAIL github.com/coopernurse/pulserpc/pkg/runtime/runtimes/go/pulserpc [build failed]
FAIL github.com/coopernurse/pulserpc/pkg/runtime/runtimes/go/tests [build failed]
FAIL github.com/coopernurse/pulserpc/pkg/generator [build failed]
FAIL
```
(toolchain: go1.25.4 linux/amd64). Therefore the Go "no data race" conclusion
remains STATIC-INSPECTION-ONLY and UNVERIFIED dynamically. This is precisely
why the `go test -race` CI step above is recommended: CI images provide gcc,
so the race detector will actually exercise the concurrent-validation path
there. No stress test for C#/Java/Python/TS was run in this wave.

## Orchestrator follow-up verification (read-only)

- **Java caches:** `Types.java:26-27,87-88,167-168` use
  `Collections.synchronizedMap(new WeakHashMap<>())` for `SORTED_KEYS`,
  `ENUM_METADATA`, and `STRUCT_FIELDS`; each get-then-put is two synchronized
  operations, so concurrent threads may redundantly build an equivalent
  immutable value but cannot corrupt the map. `Server.methodCache` is a
  `ConcurrentHashMap` populated with `putIfAbsent` in `addHandler`. No race.
- **TS shared array:** the only production `getStructFields` call sites are
  `ts-node/validation.ts:174` and `ts-cjs/validation.ts:164`, both `const fields = ...`
  followed by read-only iteration. The shared-reference change is benign.
- **C# method dictionary:** `AddHandler` (`Server.cs:64-73`) uses first-match over
  `GetMethods(Public|Instance)`; generated handler classes have one method per IDL
  function, so the old `GetMethod(..., IgnoreCase)` ambiguity case is unreachable
  in generated code. The OrdinalIgnoreCase dictionary still covers the capitalized
  alias.
- Still owed: a CI `go test -race` run (local WSL lacks a C compiler).