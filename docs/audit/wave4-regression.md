# Wave 4 Regression Audit — Performance Commits

## Scope and method

This report maps the behavior-sensitive paths changed by the seven "performance" commits
reviewed in this worktree (branch `audit-regression`, base `d57f9b2`). Six diffs were read
in full and are covered below: Go `7ed4ea1`, Python `6e5b339`, Python2 `e85365a`,
ts-node `3c14ef2`, ts-cjs `f0a2336`, C# `3a7d555`. Java `fadb530` was **NOT examined** —
see the residual-gap section. This is a report-only wave: no code fixes and no probes
were added in this pass.

## Behavior-sensitive case matrix (expected outputs)

Enum names/values below are illustrative (e.g. an enum `MathOp` with values `add`,
`subtract`); the assertion is on the exact message bytes and ordering for each runtime.

| # | Case / trigger | Changed path | Go (7ed4ea1) | Python (6e5b339) | Python2 (e85365a) | ts-node (3c14ef2) | ts-cjs (f0a2336) | C# (3a7d555) |
|---|---|---|---|---|---|---|---|---|
| 1 | Enum invalid value (`"multiply"`) error text | set/frozenset caching | `invalid value for enum MathOp: 'multiply'. Allowed values: [add subtract]` (ordered slice via `%v`, space-joined, no quotes) | `Invalid value for enum MathOp: 'multiply'. Allowed values: ['add', 'subtract']` (Python list repr) | same Python list repr | `Invalid value for enum MathOp: 'multiply'. Allowed values: add, subtract` (join `", "`) | same join `", "` | `Invalid value for enum MathOp: 'multiply'. Allowed values: [add, subtract]` (bracketed, join `", "`) |
| 2 | Enum non-string value error text | same | `expected string for enum ...` via `fmt` `%T` | `Expected string for enum ..., got <typename>` | same | `Expected string for enum ..., got <typeof>` | same | `Expected string for enum ..., got <TypeName>` |
| 3 | Enum value case sensitivity (`"Add"` vs `add`) | membership set exact strings | membership via `map[string]struct{}` keyed by exact name | `frozenset` exact strings | `frozenset` exact strings | `Set` exact strings | `Set` exact strings | `HashSet` with `StringComparer.Ordinal` |
| 4 | Inherited struct field order (parent fields first, child appended, override in place) | O(1) `fieldIndex` overrides | `resolveStructFields` first-position map | `_resolve_struct_fields` name→position | `get_struct_fields` `field_index` | `getStructFields` `indexByName` | `getStructFields` `fieldIndexes` | `AddOrReplace` with `fieldIndex` (Ordinal) |
| 5 | `invalid_fields` / validation-error path ordering | flattened field order feeds validators | errors emitted in field order from cached list | `param[i].field` paths in field order | field-validation `ValueError` messages in field order | `ValidationError.path` in field order | same | same |
| 6 | Named primitive / enum return normalization | Go `convertToMap` reflection walk | named string type → `rv.String()`; int → `float64`; enum returns normalized to primitive; fallbacks for custom marshalers/recursive/unexported-embedded/`[]byte`/exotic tags | n/a | n/a | n/a | n/a | n/a |
| 7 | C# by-name params, mixed-case keys (`"Value"`/`"value"`) | `NormalizeParams` O(P+K) | n/a | n/a | n/a | n/a | n/a | exact match via `dict.Contains(name)` first, then `byName` OrdinalIgnoreCase lookup |
| 8 | C# by-name duplicate keys differing only in case | first-occurrence-wins claim | n/a | n/a | n/a | n/a | n/a | `byName` keeps first `dict.Keys` occurrence (UNVERIFIED — see risks) |
| 9 | C# method-name parsing (wire name vs capitalized alias) | reflection cache in `AddHandler` | `invokeHandler` reuses resolved `reflect.Value` | n/a | n/a | n/a | n/a | case-insensitive method dictionary (UNVERIFIED overload semantics — see risks) |
| 10 | Client envelope: wrong version / id mismatch / result+error | C# `Client.Call` parse-once | n/a | n/a | n/a | n/a | n/a | `ParseMethodName(method)` computed once and reused for request + response validation |
| 11 | Required-param count (`checkParamCount`) | inline counting loop | n/a | n/a | n/a | `requiredCount` counted in place (same boundary condition) | same | n/a |
| 12 | Response validation failure | result validation with threaded caches | `validateStructE` via `resolver.getStructFields` | `validate_type(..._fields, _enum_values)` | `validate_type(..., self._validation_context)` | `validateType` with cached struct/enum meta | same | `ValidateResponse(..., fn, paramDefs)` |

## Verified findings (from diffs actually inspected)

- **C# `ValidateEnum` error text is byte-identical.** Before: `Invalid value for enum {enumName}: '{s}'. Allowed values: [{string.Join(", ", allowedValues)}]`. After: cached `Display = $"[{string.Join(", ", allowed)}]"` used in the same format string. Verified via `git show 3a7d555^:pkg/runtime/runtimes/csharp/PulseRPC/Validation.cs`. Membership changed from `allowedValues.Contains(s)` to `HashSet<string>(allowed, StringComparer.Ordinal)` — exact-match semantics unchanged.
- **C# parse-once in `Client.Call` is safe.** `ParseMethodName(method)` is a pure function; the original invoked it twice (request validation and response validation), the new code invokes it once and reuses the value. No observable change.
- **Error-text preservation in Python / Python2 / TS.** All three cache only *membership* (`frozenset`, `Set`, `HashSet`) while retaining the ordered names list for the error string. Format strings and join separators are unchanged, so case sensitivity and exact message bytes are preserved. Go's `ValidateEnumE` keeps `allowed: values` (ordered) in `newEnumValueSetFromStrings`, so its `%v` error text is unchanged.
- **Struct field ordering preserved for unique field names.** Every runtime builds parent fields first in declaration order, appends child fields, and overrides an inherited field *in place* by name. With parser-validated unique field names the resulting order (and therefore `invalid_fields` ordering) is unchanged. Go's `resolveStructFields` explicitly keeps the first position for a duplicate name.
- **Additive public API changes only (C#/Python).** C# added overloads (`ValidateRequest`/`ValidateResponse` with pre-resolved IDL entries, `Contract.GetStructFields`); the original public `ValidateEnum(object?, string, List<string>, string)` overloads remain. Python appended optional parameters to `validate_enum`/`validate_struct`/`validate_type`. These are additive, not breaking, but should be pinned by guards (coverage gap below).

## Candidate regression risks — UNVERIFIED (need probes)

1. **TS `getStructFields` returns a shared cached array instead of a fresh one.** ts-node returns `index.byName.get(structName)` and ts-cjs returns `byDef.get(structDef)` — the same array object across calls. The original code built and returned a new array each call. Any caller that mutates the returned array could contaminate later validations (and cross-runtime golden bytes could diverge). Needs a probe; if callers are read-only this is benign.
2. **C# method resolution semantics changed for overloaded handlers.** Original `GetMethod(name, BindingFlags.IgnoreCase)` (plus an explicit capitalized-name fallback) could throw `AmbiguousMatchException` for overloads; the new case-insensitive dictionary built in `AddHandler` silently keeps the first method with a given name. Also the explicit capitalized alias fallback was removed in favor of `StringComparer.OrdinalIgnoreCase`. Need a probe with a handler that has overloads / mixed-case method names.
3. **C# by-name duplicate-case keys.** `byName` uses `OrdinalIgnoreCase` with first-occurrence-wins; the original loop broke on the first case-insensitive match. Claim is equivalence, but it depends on `dict.Keys` iteration order being the same in both the unknown-key rejection pass and the value-resolution pass. Needs a probe with keys `"Value"` and `"value"` present together.
4. **Python2 `field_index` last-vs-first duplicate handling.** `_resolve_struct_fields`'s parent loop maps a duplicated parent field name to the LAST occurrence (`field_index[name] = len(fields)` without a first-only check), whereas Go explicitly keeps the first. Likely unreachable because the parser rejects duplicate field names, but unverified.
5. **Go `convertToMap` named-primitive and fallback paths.** The reflection walk claims byte-identical shapes for generated enum/named string returns, `omitempty`, embedded/promoted fields, `[]byte` base64, and custom marshalers (via JSON fallback). Each fallback branch needs a characterization probe before declaring parity.
6. **API-surface guards.** The Python 19-symbol guard exists; no equivalent guard was checked/added for TS/C#/Java public surfaces (`RPCException` vs `RPCError`, `VerifyOnBootstrap`, constructor signatures, the new C# overloads). Guard coverage is a residual gap.

## Residual gap: Java not examined

The Java perf commit `fadb530 perf(java): cache enum/struct metadata and handler reflection` was **NOT examined** in this pass: the attempt to read its diff used an incorrect path and returned empty output. Its behavior-sensitive paths (enum error text, inherited field order, by-name/method-name handling, response validation, `RPCException`/`VerifyOnBootstrap` API surface) remain completely uncovered by this report and by any new probe from this wave. This is the largest residual gap.

## Coverage: existing tests vs newly added

- Existing per-runtime unit tests added in the perf commits cover enum error text, inherited overrides, and cache reuse at the unit level (e.g. Python `test_validation_cache.py`, Python2 `test_validation.py`, ts-node/ts-cjs `test_validation`/`test_types`, Go `convert_to_map_test.go`). These are single-runtime checks.
- There is **no cross-runtime golden response suite**: no CI job or Makefile target records exact response bytes across all seven generated servers for a shared matrix. Documented divergences to allow would include R3 accumulating-vs-fail-fast validation, D7 null-or-omit, and Python2's reduced `data` shape.
- This report-only wave added **no probes, no tests, and no golden harness**.

## Key question: any observable behavior difference?

**Not yet proven either way.** For ordinary inputs the inspected diffs preserve exact error text, validation results, and field ordering, and two risks were positively cleared (C# error text; C# parse-once). However, the six UNVERIFIED candidates above — especially the TS shared cached array and the C# overloaded-handler/duplicate-case-key paths — still need characterization probes, and Java was not examined at all. No behavior difference has been confirmed, but equivalence has not been demonstrated across all seven runtimes.
