# ts-node — code review findings (branch audit-fixes-ts-node, `28cd2e5`)

Verdict: APPROVE (no critical/high). Under the revised policy all surviving
findings are fixed regardless of verdict.

Axes: correctness (no findings), security, quality, performance (no text),
regression (no text).

Canonical decisions respected: severity (removed field = Info, added required
field = Error) and validation data shape (TS keeps structured ValidationResult).

## Surviving changes (all fixed)

1. [Low · security] Prototype pollution via untrusted IDL `meta` keys — `pkg/runtime/runtimes/ts-node/pulserpc/contract.ts:177,202`
   - Evidence: `Object.assign(this.meta, rawMeta)` and `this.meta[key] = value` copy remote-controlled keys from a server's `pulserpc-idl` response; `__proto__` keys reach the inherited setter. Confirmed.
   - Change: reject `__proto__`/`constructor`/`prototype` keys, or build `meta` with `Object.create(null)` and `Object.defineProperty`. Add a test.

2. [Medium · quality] Duplicated parameter-count validation — `contract.ts:260-265` and `server.ts:266-273`
   - Evidence: identical `requiredCount` range check in both files with different error messages. Confirmed.
   - Change: single owner of the rule — remove the server-side check (or reuse a shared helper).

3. [Low · quality] Dead `extra` argument to `classifySeverity` — `diff.ts:315-320`
   - Evidence: `extra` is ignored for Field/Removed/ClientHasMore (returns Info unconditionally at `:526-527`); unused computation. Confirmed.
   - Change: restore the simple `severity: Severity.Info` literal.

## Deferred

None.
