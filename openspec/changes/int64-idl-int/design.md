## Context

IDL `int` is currently signed 32-bit in the canon (D1) and in runtime
validation, although several target languages already use a wider native type.
The code generators map `int` inconsistently: Go uses `int`, which is 64-bit on
common platforms but 32-bit elsewhere, and the runtime rejects values above
`MaxInt32` on the wire. This change makes the contract explicit: `int` is a
signed 64-bit JSON integer everywhere.

The Go runtime also has a number-normalization step that predates 64-bit `int`.
The generated adapter decodes request bodies with `json.Decoder.UseNumber()` so
numeric ids stay exact, then calls `pulserpc.NormalizeRequestNumbers` to restore
the v0.4.0 `float64` view for middleware. That conversion silently rounds
integers above 2^53, which would corrupt a valid `int64` parameter before the
handler ever sees it.

## Goals / Non-Goals

**Goals:**

- One canonical `int` range: `[-9223372036854775808, 9223372036854775807]`.
- Go generated code uses `int64` for every IDL `int` position, including
  optional pointers, arrays, and maps.
- Go runtime validation and conversion accept the full range from `int`,
  `int64`, `float64`, and `json.Number` inputs without accepting fractions,
  NaN/Inf, bools, or out-of-range values.
- Go request/response number normalization never corrupts an integer above
  2^53.
- Canonical docs and OpenSpec describe the breaking change and the TypeScript
  safe-integer limitation.

**Non-Goals:**

- Changing `float`; it stays an IEEE-754 double.
- Changing the JSON-RPC envelope or the wire encoding of `int` (still a bare
  JSON number).
- Giving TypeScript/JavaScript full 64-bit precision; that is not possible with
  `number`.
- Changing numeric id handling; ids already keep their exact text in Go.

## Decisions

### `int` is signed 64-bit (D1)

The canonical range is the `int64` range. Every runtime validates integral JSON
numbers against it. Generated native types are `int64` (Go), `long`/`Long`
(Java), `long` (C#), `int` (Python), and `number` (TypeScript). OpenAPI mapping
already emitted `integer`/`int64` for Pulse `int`, so it is now accurate.

**Alternative considered:** keep 32-bit on the wire and widen only selected
languages. Rejected: it leaves an artificial limit, keeps generators mapping the
same IDL type differently, and has no interoperability benefit.

### Go maps `int` to `int64`, not native `int`

Go's `int` is platform-sized. Mapping it to `int64` makes generated signatures
stable across architectures (including 32-bit builds) and matches the wire
contract. Optional values become `*int64`; arrays and maps follow from the
element mapping (`[]int64`, `map[string]int64`). Generated sample handlers that
use an IDL `int` as a loop counter or slice length are written with `int64`
indices (`for i := int64(0); ...`) or `range`, because an `int` loop variable no
longer compares to an `int64` bound without a conversion.

### Validation accepts the full int64 range from every numeric form

`ValidateIntE` accepts Go `int`/`int64` directly (all values are in range),
`float64` through a range check, and `json.Number` by parsing `Int64` first.
Fractional values, NaN/Inf, bools, and non-numeric values stay invalid.

The `float64` rule is `[-2^63, 2^63)`, not `[-2^63, 2^63]`. `float64` cannot
represent `MaxInt64`; the nearest double is exactly `2^63`. Accepting `2^63`
would admit a value that is outside the `int64` range, so the upper bound is
exclusive. The lower bound `-2^63` is exactly representable and stays valid.

For `json.Number`, a text form with no fraction or exponent that fails
`Int64()` is out of range and is rejected directly. Falling back to
`Float64()` for an integer-form value such as `-9223372036854775809` would round
it back to `-2^63` and incorrectly accept it. Values with a fraction or
exponent still take the float path, so `1.5` is rejected as fractional and
`1e1` is accepted as the integral value 10.

### Go normalization converts only safe integers to float64

`normalizeJSONNumbers` handles a `json.Number` by parsing `int64` and returning
a `float64` only when the value is in the JavaScript safe-integer range
`±(2^53-1)`. Otherwise it leaves the `json.Number` in place. Non-integer/
exponent values still go through `Float64`, which is the natural representation
for `float` params.

The safe-range rule is stricter than "the float conversion is numerically
exact" on purpose. `-2^63` is exactly representable as a `float64`, but
`encoding/json` formats the double as `-9223372036854776000`, so a generated
client that round-trips the result through JSON would lose the integer text and
fail to unmarshal it into `int64`. Keeping every large `int64` as `json.Number`
avoids that.

Consequences:

- Small `int` params (through 2^53-1) remain `float64` in middleware,
  preserving the v0.4.0 view asserted by existing users and tests.
- Large `int` params stay `json.Number` in middleware and reach the handler
  exactly through `convertJSONNumber`.
- The same helper is used for decoded client responses, so large `int` results
  also stay `json.Number` rather than being rounded.
- Response validation accepts a top-level `json.Number` as a numeric scalar
  rather than treating its string kind as a string.

**Alternative considered:** keep `float64` for middleware and convert back to
`int64` from text later. Rejected: the text is already gone once converted, so
the value cannot be recovered; only the handler could be fixed, and only if the
adapter kept a side channel.

**Alternative considered:** leave every param as `json.Number` in middleware.
Rejected: it breaks the documented v0.4.0 `float64` view for ordinary values
and existing middleware. Exact-only conversion is the smallest compatible
change.

### `convertJSONNumber` refuses out-of-range integer text

The converter parses `Int64` first. If that fails and the text has no fraction
or exponent, it reports failure (the caller falls back to the target's zero
value) instead of converting through a rounded `float64`. Whole-valued decimals
such as `"5.0"` continue to convert, and target-width overflow checks are
unchanged. This closes the same rounding hole as the validator so a value
rejected by validation can never be silently truncated by conversion.

### TypeScript/JavaScript limitation is explicit

JavaScript `number` represents integers exactly only through 2^53-1. The
TypeScript runtime types `int` as `number` and validates the safe-integer
range, rejecting larger values rather than silently rounding them. Go, Java,
C#, and Python enforce the full `int64` range. Documentation states that a
TypeScript peer cannot round-trip the upper range.

## Risks / Trade-offs

- **[Breaking change]** Existing IDL consumers and generated Go callers see
  wider types and an acceptance range change. Mitigation: call it out as
  **BREAKING** in the proposal and reference docs; regenerate generated code.
- **[Mixed middleware types]** A middleware hook can now see `float64` for one
  `int` param and `json.Number` for another. Mitigation: document the
  safe-integer rule and recommend handling both; the runtime keeps converting to
  the declared `int64` for handlers.
- **[TypeScript cannot match the contract]** TS/JS cannot represent the full
  range. Mitigation: validate the safe-integer range and document the
  limitation; do not claim full support.
- **[Float boundary asymmetry]** The float upper bound is exclusive. Mitigation:
  the bounds and rationale are documented in D1, the Go reference, and tests
  that assert `float64(math.MaxInt64)` is rejected and `-2^63` is accepted.
- **[Generated sample code]** Loop counters and slice lengths that use an IDL
  `int` need explicit `int64` handling. Mitigation: generator templates were
  updated and the generated module is compiled in tests.
