## 1. Runtime policy types

- [x] 1.1 Add `pkg/runtime/runtimes/go/pulserpc/options.go` with `ErrorPhase`, `ErrorPhaseDecodeParams`, `ErrorPhaseValidateParams`, `ParamsMode`, `ParamsModeAny`, `ParamsModePositionalOnly`, `ServerConfig`, and `DefaultServerConfig()`.
- [x] 1.2 Document that the `ServerConfig` zero value disables batching and that callers must start from `DefaultServerConfig()`.

## 2. Runtime enforcement

- [x] 2.1 Add `config ServerConfig` to `Server`; add `NewServerWithConfig` and `Config()` (returning a copy); make `NewServer` delegate with `DefaultServerConfig()`.
- [x] 2.2 Enforce `RequireNonNullID` after the nil-request check and again after middleware rewrite; use `errorResponse` with a null id so the error is not suppressed as a notification.
- [x] 2.3 Reject object params with `-32600` when `ParamsModePositionalOnly`; keep absent/null params as zero arguments.
- [x] 2.4 Route params decode failures (including parameter-count mismatches) through an internal typed `ErrorPhaseDecodeParams` error and `Contract.ValidateRequest` failures through `ErrorPhaseValidateParams`.
- [x] 2.5 Add a response helper that calls `MapError`, falls back to `*RPCError` passthrough, then `-32602 "Invalid params"` with the error text as data; leave envelope errors unmapped.
- [x] 2.6 Add recursive strict unknown-field detection in `invokeHandler` before conversion, sharing key candidates with `convertMapToStruct` and handling pointers, slices/arrays, maps, embedded structs, and `json:"-"`. Cache the per-type key schema and bound recursive embedded types.

## 3. Numeric decoding and conversion

- [x] 3.1 Decode generated-adapter bodies with `json.Decoder` + `UseNumber()` and reject trailing data with `-32700`; normalize params (not the request id) back to `float64` for middleware and handlers.
- [x] 3.2 Accept `json.Number` in `ValidateIntE` and `ValidateFloatE` while keeping the pre-change numeric-kind acceptance and fractional/non-finite/range rules.
- [x] 3.3 Special-case `json.Number` in `convertValueToType` so it converts to ints/uints/floats and never silently becomes a string.
- [x] 3.4 Decode client responses through `DecodeResponse` so the id stays exact and other numbers stay `float64`; compare ids exactly with fast paths plus `math/big.Rat`, keeping `idsEqual(7, 7.0)` and string equality.

## 4. Generator

- [x] 4.1 Add `NewPulseRPCServerWithConfig(host, port, config)` and keep `NewPulseRPCServer` delegating with `pulserpc.DefaultServerConfig()`.
- [x] 4.2 Read `config := s.rpcServer.Config()` in the adapter; use `config.MaxRequestBodyBytes` (0 -> legacy constant) in `http.MaxBytesReader` before HTTP middleware.
- [x] 4.3 Reject invalid UTF-8 when enabled and reject batches when `!config.AllowBatch`.
- [x] 4.4 Decode with `UseNumber()` and reject trailing data; add `bytes` and `unicode/utf8` imports.
- [x] 4.5 Keep `errorResponse`/`sendErrorResponse` shape and HTTP 200.

## 5. Tests

- [x] 5.1 Add runtime tests for default leniency, `RequireNonNullID`, `ParamsModePositionalOnly`, strict unknown fields (top-level, nested, pointer, slice, map) and the default-off behavior.
- [x] 5.2 Add runtime tests for `MapError` phases, data-free mapped errors, the nil fallback, and unmapped envelope errors.
- [x] 5.3 Add runtime tests for exact `json.Number` id round-trip, `json.Number` int/float params including fractional/out-of-range rejection, request-number normalization, response decoding, and exact client id comparison.
- [x] 5.4 Add generator static assertions for the new constructor, `Config()`, `UseNumber`, trailing-data token, request normalization, response decoding, `utf8.Valid`, `config.AllowBatch`, and the custom body-limit branch, including body-cap-before-middleware ordering.
- [x] 5.5 Add a generated-module end-to-end test covering batches, absent/null ids, named params, mapped unknown fields, zero-arg params, oversize bodies, invalid UTF-8, exact ids, middleware param/id types, and default batch acceptance; run `go test ./...` in the generated module.
- [x] 5.6 Update `go_invariants_test.go` and `go_context_test.go` for the new adapter strings.

## 6. Documentation and openspec

- [x] 6.1 Add a "Server configuration" section to `docs/languages/go/reference.md` with the field table, defaults, zero-value warning, params mode, error phases, unknown fields, body/UTF-8 behavior, and exact numeric ids.
- [x] 6.2 Update the limits table so the default 1 MiB cap returns `-32600` and a custom cap returns `-32700`.
- [x] 6.3 Add this openspec change (`proposal.md`, `design.md`, `tasks.md`, `.openspec.yaml`, `specs/go-server-options/spec.md`).

## 7. Verification

- [x] 7.1 Run `gofmt` on changed Go files and `go build` on the changed packages.
- [x] 7.2 Run the runtime tests (`go test ./pkg/runtime/runtimes/go/pulserpc/`).
- [x] 7.3 Run the generator tests (`go test ./pkg/generator/`); the only failure is the pre-existing `TestTsCjsOutputIsCjsParseable` tooling check.
- [x] 7.4 Run the generated-module end-to-end test.
- [ ] 7.5 `go vet` on the changed packages (blocked by the pre-existing `convert_to_map_test.go` duplicate json tag diagnostic).
- [x] 7.6 Commit the work; do not push or tag.
