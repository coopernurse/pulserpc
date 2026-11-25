## 1. Canonical contract

- [x] 1.1 Define IDL `int` as signed 64-bit (`[-9223372036854775808, 9223372036854775807]`) and record the breaking change in the proposal and spec.
- [x] 1.2 Document the `float64` boundary caveat (`[-2^63, 2^63)`) and the TypeScript/JavaScript safe-integer limitation.

## 2. Go generator

- [x] 2.1 Map IDL `int` to `int64` in `mapTypeToGoType` and `getGoTypeWithNamespace`; verify optional (`*int64`), array, and map mappings follow.
- [x] 2.2 Update generated sample handlers that use an IDL `int` as an index/length (`repeat`, `repeat_num`) to compile with `int64`.
- [x] 2.3 Update Go generator tests to assert `int64` fields and signatures, and exercise the generated module.

## 3. Go runtime

- [x] 3.1 Update `ValidateIntE` to accept `int`, `int64`, `float64`, and `json.Number` across the full signed 64-bit range; reject fractional, NaN/Inf, bool, non-numeric, and out-of-range values.
- [x] 3.2 Implement the `float64` bounds `[-2^63, 2^63)` so `2^63` is rejected and `-2^63` is accepted; update range error messages to "signed 64-bit range".
- [x] 3.3 Parse `json.Number` with `Int64` first; reject integer-form text outside `int64` without rounding through `float64`; apply float rules to fractional/exponent forms.
- [x] 3.4 In `normalizeJSONNumbers`, convert integer `json.Number` to `float64` only within the JavaScript safe-integer range (`±(2^53-1)`); keep `json.Number` for larger values (including `-2^63`) for both request params and response results. Treat a top-level `json.Number` response as numeric during response validation.
- [x] 3.5 Harden `convertJSONNumber` so integer-form text outside `int64` fails instead of rounding; verify full-range conversion and no string fallback.

## 4. Go tests

- [x] 4.1 Update `pkg/runtime/runtimes/go/tests/validation_test.go` boundaries to int64, including `float64` upper-bound rejection and `json.Number` out-of-range rejection.
- [x] 4.2 Update `pkg/runtime/runtimes/go/pulserpc/server_options_test.go` handlers/fields to `int64`, add large `json.Number` param coverage, and assert safe-integer normalization.
- [x] 4.3 Update generator tests that assert Go `int` types (`go_invariants_test.go`, `go_server_options_test.go`, and generated-module fixtures).
- [x] 4.4 Add generated-module end-to-end coverage for `9007199254740993` and `9223372036854775807`: handler receives/returns the exact value and the raw JSON contains the exact number.
- [x] 4.5 Add middleware coverage: small int params are `float64`; large int params are `json.Number`.
- [x] 4.6 Update the Go quickstart (`examples/quickstart/go/server.go`) and its docs include to pass `time.Now().Unix()` directly to an `int` field.

## 5. Other runtimes

- [x] 5.1 Java generator/runtime: IDL `int` -> `long`/`Long`, full int64 validation. (Owned by the Java change.)
- [x] 5.2 C# generator/runtime: IDL `int` -> `long`, full int64 validation. (Owned by the C# change.)
- [x] 5.3 Python runtime: full int64 validation. (Owned by the Python change.)
- [x] 5.4 TypeScript generator/runtime: `int` -> `number` with safe-integer validation and documentation. (Owned by the TypeScript change.)
- [x] 5.5 Integration tests and conformance fixtures for the new range. (Owned by the integration change.)

## 6. Canonical documentation

- [x] 6.1 Update `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` D1.
- [x] 6.2 Update `docs/plans/README.md` D1.
- [x] 6.3 Update `docs/idl-guide/types.md` with `int64`/`long`/`int`/`number`, the range, and the safe-integer note.
- [x] 6.4 Update `docs/languages/go/reference.md` type table, range note, optional helper, and the middleware `json.Number` caveat.
- [x] 6.5 Verify `docs/advanced/openapi-mapping-reference.md` already maps Pulse `int` to `integer`/`int64` (no change needed).

## 7. OpenSpec

- [x] 7.1 Add `openspec/changes/int64-idl-int/` with proposal, design, tasks, `.openspec.yaml`, and `specs/int64-idl-int/spec.md`.

## 8. Verification

- [x] 8.1 Run the Go runtime tests (`go test ./pkg/runtime/runtimes/go/... ./pkg/runtime/...`).
- [x] 8.2 Run targeted Go generator tests (`go test -run TestGo ./pkg/generator/`), including generated-module end-to-end tests.
- [x] 8.3 Run the full generator suite: the only failures are pre-existing TypeScript tooling/environment issues (`TestTsCjsOutputIsCjsParseable` cannot resolve the temp path from WSL, and `TestCrossNamespaceFixtureTypeScriptIdentifiers` cannot find `node` on PATH). No Go generator failures.
- [ ] 8.4 Run cross-language integration tests. (Owned by the integration change.)
