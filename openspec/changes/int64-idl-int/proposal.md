## Why

IDL `int` is documented and validated as a signed 32-bit value
(`[-2147483648, 2147483647]`) even though most target languages already use a
wider native type. Callers that need 64-bit identifiers, timestamps, or
counters must either split values, use `string`, or lose range. Standardizing
on signed 64-bit makes the wire type match `long`/`int64`/`int` in every
statically typed runtime and removes an artificial limit.

## What Changes

- **BREAKING:** IDL `int` becomes signed 64-bit on the wire:
  `[-9223372036854775808, 9223372036854775807]`.
- Go maps IDL `int` to `int64` in generated structs, fields, arrays, maps,
  method parameters, and return types (optional -> `*int64`).
- Go `ValidateIntE` accepts `int`, `int64`, `float64`, and `json.Number` across
  the full signed 64-bit range. Fractions, NaN/Inf, bools, non-numeric values,
  and out-of-range values remain rejected. `float64` inputs are accepted on
  `[-2^63, 2^63)` because `float64(math.MaxInt64)` rounds to `2^63`.
- Go request-number normalization converts a `json.Number` to `float64` only
  in the JavaScript safe-integer range (`±(2^53-1)`). Larger `int64` values stay
  `json.Number`, so middleware can see either representation for an `int` param
  depending on the value; handlers still receive an exact `int64`.
- Go `json.Number` conversion rejects integer-form text outside the `int64`
  range instead of rounding it through `float64`.
- Java, C#, Python, TypeScript, and the integration suite are updated by their
  own changes to the same canonical range. TypeScript/JavaScript cannot
  represent the full range exactly (`number` is exact only through 2^53-1), so
  the TypeScript runtime validates the safe-integer range.
- Canonical docs (`D1` in the runtime guide and plans index, the IDL types
  table, the Go reference) are updated. The OpenAPI mapping already maps Pulse
  `int` to `integer`/`int64`.
- New OpenSpec capability `int64-idl-int` records the contract and the
  per-runtime behavior.

## Capabilities

### New Capabilities

- `int64-idl-int`: signed 64-bit IDL `int` across generators, runtimes, and
  canonical documentation, including the JavaScript safe-integer limitation.

### Modified Capabilities

- None. This change replaces the previous 32-bit `int` canon rather than
  layering an option on top of it.

## Impact

**Affected code:**
- `pkg/generator/go_client_server.go` — `int` -> `int64` type mapping and
  sample handler loops that index with `int`.
- `pkg/runtime/runtimes/go/pulserpc/validation.go` — signed 64-bit range.
- `pkg/runtime/runtimes/go/pulserpc/transport.go` — safe-integer-only number
  normalization.
- `pkg/runtime/runtimes/go/pulserpc/server.go` — integer-form `json.Number`
  range guard.
- Go runtime and generator tests, including the generated-module end-to-end
  test.
- `examples/quickstart/go/server.go` and its docs include.
- Other language generators/runtimes and `tests/integration/*` (owned by their
  respective changes).

**Documentation:**
- `docs/RUNTIME_IMPLEMENTATION_GUIDE.md` (D1)
- `docs/plans/README.md` (D1)
- `docs/idl-guide/types.md`
- `docs/languages/go/reference.md`
- `docs/advanced/openapi-mapping-reference.md` (verified; no change required)

**Compatibility:**
- This is a **breaking** IDL/wire change. A v0.4.x peer that enforces 32-bit
  `int` rejects the new upper range.
- Generated Go code changes public signatures (`int` -> `int64`). Callers must
  regenerate and adjust literals/variables typed as `int`.
- Go middleware that asserted every numeric param is `float64` must also handle
  `json.Number` for integers outside the safe-integer range. Small integers keep
  the legacy `float64` view.
- TypeScript/JavaScript peers cannot round-trip values above 2^53-1 exactly;
  the TS runtime rejects them rather than silently corrupting them.
