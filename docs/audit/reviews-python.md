# Python — code review findings (branch audit-fixes-python, `7f4bb96`)

Verdict: APPROVE (no critical/high). Under the revised policy all surviving
findings are fixed regardless of verdict.

Axes: correctness incomplete, security, quality unavailable, performance,
regression incomplete.

## Surviving changes (all fixed)

1. [Medium · security] Unbounded JSON-RPC batch processing enables request amplification / DoS — `pkg/runtime/runtimes/python/pulserpc/server.py:74-90`
   - Evidence: newly added `_call_batch` loops over every element of an attacker-controlled array with no count limit, invoking handlers per item; generated HTTP adapter reads the body with no size cap. Confirmed introduced by this branch.
   - Change: enforce a configurable maximum batch size (reject with -32600) and cap response memory; add body-size limit at the adapter.

2. [Medium · performance] Qualified-name resolution re-sorts the full type table on every validation — `pkg/runtime/runtimes/python/pulserpc/rpctypes.py:70,76,85,91`
   - Evidence: `resolve_qualified_name` calls `sorted(definitions)` up to twice per lookup; tables are fixed for a Contract lifetime. Regression from O(1) dict `.get()`. Measured 12µs@50 keys → 102µs@500 keys. Confirmed.
   - Change: build a resolution index once per Contract (or cache the sorted key list) for O(1) lookups with unchanged semantics.

## Deferred

None.
