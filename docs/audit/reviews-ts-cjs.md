# ts-cjs — code review findings (branch audit-fixes-ts-cjs, `d187595`)

Verdict: APPROVE (no critical/high). Under the revised policy all surviving
findings are fixed regardless of verdict.

Axes: correctness, security, quality (no text), performance, regression (no text).
Canonical severity decision respected (removed field = Info).

## Surviving changes (all fixed)

1. [High · correctness] Client `namedToPositional` omits trailing-`undefined` trim — `pkg/runtime/runtimes/ts-cjs/pulserpc/client.ts:331-338`
   - Evidence: server helper (`server.ts:263-268`) trims trailing undefined so `validateRequest` sees a short array; client helper does not, so `validateType(undefined,...)` rejects even optional params. Confirmed by direct read.
   - Change: mirror the server trim in the client; add a client test for named params with omitted trailing optional.

2. [Medium · correctness] Falsy `error` envelope member bypasses validation — `client.ts:277-295`
   - Evidence: XOR via `hasOwnProperty` but error branch via `if (response.error)`; `error:null` falls through to undefined result. Confirmed.
   - Change: branch on `hasError` and validate error object shape.

3. [Low · correctness] `InProcTransport` fallback emits envelope with neither result nor error — `transport.ts:84-89` vs `client.ts:282-284`
   - Change: emit a well-formed response or document the constraint.

4. [Low · security] Prototype pollution via untrusted IDL `meta` keys — `contract.ts:165,178,183`
   - Evidence: `this.meta[key] = value` with remote keys; ts-node branch fixed this with `UNSAFE_META_KEYS`/`setMetaItem`. Confirmed.
   - Change: mirror the ts-node guard.

5. [Medium · performance] `findByTypeName` O(n) scan + allocation per lookup on validation hot path — `types.ts:96-119`
   - Evidence: exact-match O(1) fast path, otherwise `Object.entries(types)` + full scan; O(N·S) for N struct values, S structs. Confirmed.
   - Change: lazy base-name index cached per types object.

## Deferred

None.
