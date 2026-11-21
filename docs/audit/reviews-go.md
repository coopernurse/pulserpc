# Go — code review findings (branch audit-fixes-go, `ce900cc`)

Verdict: APPROVE (no critical/high). Under the revised policy all surviving
findings are fixed regardless of verdict.

Axes: correctness unavailable (no text), security, quality unavailable,
performance, regression unavailable.

## Surviving changes (all fixed)

1. [Medium · security] Recovered panic values reflected verbatim to clients as JSON-RPC error `data` — `pkg/runtime/runtimes/go/pulserpc/server.go:47-52`, `:172-189`, `pkg/generator/go_client_server.go:960-967`
   - Evidence: `recover()` formats the panic value with `fmt.Sprintf("...%v", r)` into error data; `errorResponse` copies `data` verbatim; generated HTTP adapter serializes with `json.NewEncoder(w).Encode(response)`. Confirmed introduced by this branch.
   - Change: return generic data to the client; log diagnostics server-side with redaction. Never reflect raw panic/error values into `error.data`.

2. [Medium · performance] `resolveDefinition` linear-scans the whole type map on misses — `pkg/runtime/runtimes/go/pulserpc/types.go:103-131`, called from `validation.go:225,231`
   - Evidence: old code was two O(1) map lookups; new code does a full `range defs` with `strings.HasSuffix` on every user-defined field lookup. Confirmed regression from `git diff main...audit-fixes-go`.
   - Change: build a base-name→definition index once at contract construction for O(1) resolution while retaining ambiguity detection.

3. [Low · security] Unbounded `io.ReadAll` in generated client transport — `pkg/generator/go_client_server.go:1172`
   - Evidence: new code replaced streaming `json.NewDecoder(resp.Body).Decode` with `io.ReadAll(resp.Body)` and no `io.LimitReader`. Confirmed introduced by this branch.
   - Change: bound the read with `io.LimitReader` and fail with a parse error when exceeded.

## Deferred

None.

## Follow-up

Fixed in commit `<pending>` on `audit-fixes-go` after this review.
