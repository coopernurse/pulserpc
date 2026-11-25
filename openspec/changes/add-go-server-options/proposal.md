## Why

The Go runtime server has no supported way to tighten its JSON-RPC policy. A
production deployment that wants to refuse batches, require request ids, accept
only positional params, bound request bodies below the fixed 1 MiB cap, reject
invalid UTF-8, reject unknown struct fields, or map validation failures to an
application error must either wrap the generated server and duplicate envelope
handling or accept the lenient v0.4.0 behavior.

The generated adapter also decodes request bodies with
`json.Unmarshal(body, &requestData)`, which turns every JSON number into a
`float64`. A numeric request id above 2^53 (for example `9007199254740993`) is
silently rounded and echoed as `9007199254740992`, so a client can see a
response that does not match the request it sent.

## What Changes

- Add `pulserpc.ServerConfig`, `DefaultServerConfig()`, `ErrorPhase` and
  `ParamsMode` to the Go runtime.
- Add runtime `NewServerWithConfig` and `Server.Config()`; keep `NewServer`
  delegating with `DefaultServerConfig()`.
- Add `AllowBatch`, `RequireNonNullID`, `ParamsModePositionalOnly`,
  `MaxRequestBodyBytes`, `RejectInvalidUTF8`, `RejectUnknownRequestFields`, and
  `MapError` enforcement in the runtime and generated adapter.
- Add recursive unknown-field detection for struct-typed params, including
  pointers, slices/arrays, maps, and embedded structs.
- Add generated `NewPulseRPCServerWithConfig(host, port, config)`; keep
  `NewPulseRPCServer(host, port)` delegating with the default policy.
- Decode request bodies with `json.Decoder` + `UseNumber()` and reject trailing
  data, so numeric ids round-trip exactly. Normalize params (but not the id)
  back to `float64` so middleware and handlers keep the v0.4.0 view. Update
  runtime validation and conversion so `json.Number` also works for `int` and
  `float` params.
- Decode client responses so the id stays exact while result/error numbers stay
  `float64`, and compare numeric ids by exact rational value (`math/big.Rat`)
  in the client envelope check instead of `float64`.
- Update the Go reference, add runtime and generator tests (including a
  generated-module end-to-end test), and record the change in openspec.

## Capabilities

### New Capabilities

- `go-server-options`: configurable Go server policy and exact numeric-id
  handling.

### Modified Capabilities

- None.

## Impact

**Affected code:**
- `pkg/runtime/runtimes/go/pulserpc/options.go` — new policy types and defaults.
- `pkg/runtime/runtimes/go/pulserpc/server.go` — config-aware dispatch,
  unknown-field detection, `json.Number` conversion, error mapping.
- `pkg/runtime/runtimes/go/pulserpc/validation.go` — `json.Number` support for
  `int` and `float`.
- `pkg/runtime/runtimes/go/pulserpc/transport.go` — response decoding with an
  exact id, request-number normalization, and exact numeric id comparison.
- `pkg/generator/go_client_server.go` — configurable constructor, body limit,
  UTF-8 and batch checks, `UseNumber` decoding, exact response decoding.
- `pkg/generator/go_*_test.go` — updated adapter assertions and new coverage.

**Documentation:**
- `docs/languages/go/reference.md`

**Compatibility:**
- `NewPulseRPCServer` and `NewServer` keep their existing behavior.
- Wire output changes only where an option is enabled; the numeric-id fix is
  the one unconditional wire improvement.
- RPC middleware and handlers still see numeric params as `float64`; only the
  request id keeps its exact `json.Number` text.
- Client and server runtimes should be upgraded together: a client now rejects
  a response whose large numeric id does not match exactly instead of silently
  accepting the rounded id.
- The `ServerConfig` zero value disables batching (a deliberate fail-closed
  default), so callers must start from `DefaultServerConfig()`.
