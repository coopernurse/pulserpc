## ADDED Requirements

### Requirement: IDL `int` is signed 64-bit

IDL `int` SHALL be a signed 64-bit integer on the wire with the range
`[-9223372036854775808, 9223372036854775807]`. This is a **breaking** change
from the previous signed 32-bit range. A value is valid only when it is an
integral JSON number inside the range; fractional numbers, NaN/Infinity,
booleans, and values outside the range SHALL be invalid in every runtime.
`float` SHALL remain an IEEE-754 double.

#### Scenario: Upper bound accepted
- **WHEN** a request or response carries `9223372036854775807` for an `int`
- **THEN** validation accepts it

#### Scenario: Lower bound accepted
- **WHEN** a request or response carries `-9223372036854775808` for an `int`
- **THEN** validation accepts it

#### Scenario: Just outside the range rejected
- **WHEN** a value is `9223372036854775808` or `-9223372036854775809`
- **THEN** validation rejects it as out of range

#### Scenario: Fractional and non-numeric rejected
- **WHEN** a value is `1.5`, `NaN`, `Infinity`, or `true` for an `int`
- **THEN** validation rejects it

### Requirement: Go generated code uses `int64`

The Go generator SHALL map IDL `int` to Go `int64`, including optional fields
(`*int64`), array elements (`[]int64`), map values (`map[string]int64`), method
parameters, and return types. Generated sample handlers SHALL compile with
`int64` values, using explicit `int64` types or conversions where a Go `int`
index/length would otherwise be required.

#### Scenario: Struct field
- **WHEN** a struct declares `count int`
- **THEN** the generated Go field is `Count int64`

#### Scenario: Optional field
- **WHEN** a struct declares `score int [optional]`
- **THEN** the generated Go field is `*int64`

#### Scenario: Method signature
- **WHEN** an interface declares `add(a int, b int) int`
- **THEN** the generated Go interface and client use `int64` for `a`, `b`, and
  the result

#### Scenario: Collections
- **WHEN** an interface returns `[]int` or a struct has `map[string]int`
- **THEN** the generated Go type is `[]int64` or `map[string]int64`

### Requirement: Go validation accepts every int64 input form

`pulserpc.ValidateIntE` SHALL accept Go `int`, Go `int64`, `float64`, and
`json.Number` over the full signed 64-bit range. It SHALL reject fractional
values, NaN/Inf, booleans, non-numeric values, and values outside the range.
Error messages SHALL refer to the signed 64-bit range.

For `float64` inputs the accepted interval SHALL be `[-2^63, 2^63)`: `-2^63` is
exactly representable and valid, while `2^63` (the `float64` rounding of
`9223372036854775807`) is rejected. For `json.Number`, validation SHALL parse
`Int64` first; an integer-form value that does not fit SHALL be rejected as out
of range rather than rounded through `float64`. Fractional/exponent forms SHALL
use the float rules.

#### Scenario: Max int64 json.Number accepted
- **WHEN** validation receives `json.Number("9223372036854775807")`
- **THEN** it accepts the value

#### Scenario: Float upper bound rejected
- **WHEN** validation receives `float64(math.MaxInt64)`
- **THEN** it rejects the value because the float is `2^63`

#### Scenario: Float lower bound accepted
- **WHEN** validation receives `float64(math.MinInt64)`
- **THEN** it accepts the value because `-2^63` is exact

#### Scenario: Integer text outside int64 rejected
- **WHEN** validation receives `json.Number("-9223372036854775809")`
- **THEN** it rejects the value as out of range and does not round it to
  `-2^63`

#### Scenario: Fraction still rejected
- **WHEN** validation receives `json.Number("1.5")`
- **THEN** it rejects the value as fractional

### Requirement: Go number normalization never corrupts large integers

The generated adapter SHALL keep using `json.Decoder.UseNumber()` and
`pulserpc.NormalizeRequestNumbers`. Normalization SHALL convert a `json.Number`
integer to `float64` only when the value is in the JavaScript safe-integer range
`±(2^53-1)`. A larger integer SHALL remain `json.Number`, including `-2^63`
(even though it is exactly representable as a double) so its JSON integer text
is preserved. The same rule SHALL apply to decoded response results and error
data. Small integers SHALL remain `float64` for middleware compatibility, and
response validation SHALL treat a `json.Number` as a numeric scalar rather than
a string.

#### Scenario: Small int param stays float64
- **WHEN** an HTTP request carries `params:[1,2]` and RPC middleware inspects it
- **THEN** each param is `float64`

#### Scenario: Large int param stays json.Number in middleware
- **WHEN** an HTTP request carries `9007199254740993` or `9223372036854775807`
  for an `int` param and RPC middleware inspects it
- **THEN** the param is `json.Number`

#### Scenario: Large int param reaches the handler exactly
- **WHEN** an HTTP request carries `9007199254740993` or `9223372036854775807`
  for an `int` param
- **THEN** the generated `int64` handler receives exactly that value
- **AND** the raw response contains the exact integer text

#### Scenario: Large response result is not rounded
- **WHEN** a client decodes a response whose result contains a large integer
- **THEN** the value stays `json.Number` instead of becoming a rounded
  `float64`

### Requirement: Go `json.Number` conversion covers int64

`convertJSONNumber` SHALL convert integer `json.Number` text to the target
integer type across the full `int64` range and SHALL report failure for values
outside the target range. It SHALL NOT fall back to treating a number as a
string. Whole-valued decimals such as `"5.0"` SHALL still convert, and
integer-form text outside `int64` SHALL fail rather than round.

#### Scenario: Full-range conversion
- **WHEN** a handler parameter is `int64` and the request carries
  `json.Number("9223372036854775807")`
- **THEN** conversion produces `9223372036854775807`

#### Scenario: Out-of-range conversion refused
- **WHEN** a handler parameter is `int64` and the request carries
  `json.Number("9223372036854775808")`
- **THEN** conversion refuses the value

#### Scenario: Number never becomes a string
- **WHEN** the target type is `string` and the input is `json.Number("123")`
- **THEN** the result is the empty string, not `"123"`

### Requirement: TypeScript/JavaScript validates the safe-integer range

The TypeScript runtime SHALL type `int` as `number` and validate the
JavaScript safe-integer range `[-(2^53-1), 2^53-1]`
(`[-9007199254740991, 9007199254740991]`). It SHALL reject larger values rather
than accept a rounded number. Documentation SHALL state that TypeScript cannot
round-trip the full signed 64-bit range.

#### Scenario: Safe bound accepted
- **WHEN** a TypeScript runtime receives `9007199254740991`
- **THEN** it accepts the value

#### Scenario: Above safe range rejected
- **WHEN** a TypeScript runtime receives `9007199254740992` or
  `9223372036854775807`
- **THEN** it rejects the value as outside the safe-integer range

### Requirement: Other runtimes support the full int64 range

Java, C#, Python, and Go runtimes SHALL accept the full signed 64-bit range for
`int` and SHALL reject fractional, NaN/Inf, boolean, and out-of-range values.

#### Scenario: Python arbitrary precision
- **WHEN** a Python runtime receives `9223372036854775807`
- **THEN** it accepts the value

#### Scenario: Java and C# long
- **WHEN** a Java or C# runtime receives `9223372036854775807`
- **THEN** it accepts the value as `long`

### Requirement: Canonical documentation states the 64-bit range

`D1` in the runtime implementation guide and the plans index, the IDL types
table, and the Go reference SHALL document signed 64-bit `int`, the full range,
the Go `int64` mapping, the `float64` boundary caveat, and the
TypeScript/JavaScript safe-integer limitation. The OpenAPI mapping SHALL
continue to map Pulse `int` to `integer`/`int64`.

#### Scenario: D1 matches the runtime
- **WHEN** a maintainer reads D1
- **THEN** it states the signed 64-bit range and the per-language native types

#### Scenario: Go reference matches generated code
- **WHEN** a maintainer reads the Go type table
- **THEN** it maps IDL `int` to Go `int64` and documents the middleware
  `json.Number` caveat for values outside the safe-integer range
