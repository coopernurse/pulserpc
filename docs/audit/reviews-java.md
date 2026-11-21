# Java — code review findings (branch audit-fixes-java, `d59b4fe`)

Verdict: APPROVE (no critical/high). Under the revised policy all surviving
findings are fixed regardless of verdict.

Axes: correctness incomplete, security, quality unavailable, performance,
regression unavailable.

## Surviving changes (all fixed)

1. [Low · security] `Response.validateMap` skips the id check when the response omits `id` — `pkg/runtime/runtimes/java/com/bitmechanic/pulserpc/Response.java:87`
   - Evidence: `if (expectedId != null && responseId != null && !expectedId.equals(responseId))` — the `responseId != null` conjunct makes the check a no-op for a missing id. The typed `validate()` at `:71` correctly rejects it. Confirmed by reading both.
   - Change: drop the `responseId != null` guard; add an `EnvelopeTest` case for a response with no `id`.

2. [Medium · performance] Client parses every response body twice — `pkg/runtime/runtimes/java/com/bitmechanic/pulserpc/HTTPTransport.java:46,49`
   - Evidence: `jsonParser.fromJson(httpResponse.body(), Map.class)` then `jsonParser.fromJson(httpResponse.body(), Response.class)` on the same string. Confirmed.
   - Change: parse once into a Map, validate, and build the Response from the parsed data.

3. [Medium · performance] Generated server serializes then re-parses every successful result — `pkg/generator/java_client_server.go:1178`
   - Evidence: `Object plainResult = jsonParser.fromJson(jsonParser.toJson(result), Object.class);` before validation. Confirmed.
   - Change: validate the typed result directly, or only convert for types with user-defined definitions.

4. [Medium · performance] Type-name resolution re-sorts and re-copies all keys on every lookup — `pkg/runtime/runtimes/java/com/bitmechanic/pulserpc/Types.java:41` and `Contract.java:158`
   - Evidence: `new ArrayList<>(definitions.keySet()); Collections.sort(keys)` on every non-exact match, reached recursively from validation and twice per request from the generated server. Confirmed.
   - Change: cache the sorted key list (or a base-name index) once at construction.

## Deferred

None.

## Follow-up

Fixed in commit `<pending>` on `audit-fixes-java` after this review.
