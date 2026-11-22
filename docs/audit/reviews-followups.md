# Follow-up regression review — `main` at `20ec0da`

Review of the 37-commit invariant audit after it was pushed. All runtime
suites were green, so these were latent issues the tests did not exercise.
Every finding below was fixed on `fix-audit-followups` with a regression test.

## High

1. **Java envelope id comparison broke Gson clients.** `Response.validate` and
   `Response.validateMap` used `Object.equals`; Gson decodes JSON numbers as
   `Double` while request ids are `Integer`, so every response failed with
   `-32600 id mismatch`. Fixed with a numeric-aware `idsMatch` helper (matching
   Go's `idsEqual`). Added `EnvelopeTest.testNumericIdMatchesAcrossParsers`;
   verified end-to-end with `-json-lib gson`.

2. **Go response-envelope validation was incomplete.** `validateEnvelope` did
   not check `jsonrpc`, accepted a null/non-object `error`, and the runtime
   `Client.Call` returned `(nil, nil)` for `{"error": null}`. Fixed to require
   `jsonrpc == "2.0"`, a well-formed error object, and exactly one of
   result/error. Exported `ValidateResponseEnvelope` so generated transports
   enforce the identical rule; added `transport_envelope_test.go` and a
   generator test.

3. **ts-node and ts-cjs resolved type names differently.** The ts-cjs
   performance fix added base-name resolution; ts-node remained exact-only.
   Ported the base-name index to ts-node and made both TS runtimes use a
   deterministic sorted index. Fixed the Go runtime's ambiguity policy to the
   same rule (first sorted key). Tests added in both TS runtimes and Go.

## Medium

4. **Generated Java and C# servers replied to notifications on error paths.**
   Only success paths honoured `isNotification`. Added a wrapper around the
   core dispatcher in both generated servers that drops any error envelope for
   a request without an `id`. Verified over HTTP: invalid-method and valid
   notifications return no body; id-bearing requests still return errors.

5. **Python 2 `validate_float` accepted booleans.** Added the bool guard and a
   test; verified in `python:2.7-slim`.

6. **Python 3 `validate_int` kept the 1e-9 tolerance that Go removed.** Made
   Python exact (`value == int(value)`), updated the comment and tests.

7. **Generated Java test client sent empty structs.** The new server-side IDL
   validation (java-04) rejected `new RepeatRequest()` because required fields
   were null. The generator now emits `make<Struct>()` factories that populate
   required scalar, enum, array, map and nested-struct fields. Verified with
   the generated Java e2e (Jackson and Gson).

8. **Gson dropped `"result": null`.** Gson's default omits null map values, so
   an optional-return method produced an envelope with neither result nor
   error. `GsonJsonParser` now serializes nulls, matching Jackson. Added an
   `EnvelopeTest` case.

## Low / consistency

9. Java `errorEnvelope` now always includes `"id"` (null for parse errors),
   matching the JSON-RPC parse-error shape.
10. TS generated test server returns `-32600` for an empty batch instead of a
    204.
11. C# `HttpTransport` validates the envelope before throwing on an error
    response, so `jsonrpc`/id/XOR are checked for error responses too.
12. Generated Java clients use a unique atomic request id instead of the fixed
    `1`.

## Verification

- Go: `pkg/parser`, `pkg/generator`, `pkg/openapi`, `pkg/playground`,
  `pkg/runtime/...`, `tests/integration` — pass; `go vet` clean.
- Python: 136 passed. Python 2: full suite passes in `python:2.7-slim`.
- Java: Maven suite passes; generated Java e2e passes with Jackson and Gson.
- C#: 100 passed; generated C# e2e passes.
- ts-node: all suites pass. ts-cjs: all suites pass.
- Generated Go builds, vets, and its test server/client pass end-to-end.
- Notification suppression verified over HTTP for generated Java and C#:
  notifications (including error paths) produce no body; id-bearing requests
  return normal JSON-RPC responses.
