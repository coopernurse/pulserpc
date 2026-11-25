## MODIFIED Requirements

### Requirement: Numeric ids are exact

The generated adapter SHALL decode request bodies with `json.Decoder` and
`UseNumber()` and SHALL reject trailing data as `-32700`. Numeric ids SHALL be
echoed exactly, independent of `ServerConfig`. The adapter SHALL normalize
numbers inside `params` (and other non-id request members) for the v0.4.0
middleware view: integers in the JavaScript safe-integer range (`±(2^53-1)`)
and fractional values become `float64`, while larger integers and whole-valued
decimal/exponent tokens that cannot round-trip through `float64` remain
`json.Number`. Each request's top-level `id` remains `json.Number`. Runtime
validation and conversion SHALL accept `json.Number` for `int` and `float`
params without losing fractional or out-of-range rejection, and SHALL accept
decimal/exponent tokens exactly when their value is an integer in the signed
64-bit range. The runtime and generated clients SHALL decode responses so the
response id stays exact while result and error numbers follow the same
normalization, and the client envelope check SHALL compare numeric ids exactly
rather than through `float64`.

#### Scenario: Large numeric id round-trips
- **WHEN** a request id is `9007199254740993`
- **THEN** the response contains exactly `"id":9007199254740993`

#### Scenario: Trailing data is a parse error
- **WHEN** the body contains two JSON values
- **THEN** the response is `-32700 "Parse error"`

#### Scenario: Middleware params keep the v0.4.0 type
- **WHEN** an HTTP request carries ordinary numeric params
- **THEN** RPC middleware sees `float64` params and a `json.Number` top-level id

#### Scenario: Wide params stay exact
- **WHEN** an HTTP request carries `9007199254740993` or `1e16` for a param
- **THEN** RPC middleware sees `json.Number` and the handler receives the exact declared native value

#### Scenario: json.Number params convert
- **WHEN** a programmatic request carries `json.Number("2")` for an `int` param
- **THEN** the handler receives `2`

#### Scenario: Fractional int rejected
- **WHEN** a request carries `json.Number("1.5")` for an `int` param
- **THEN** validation rejects it with `-32602`

#### Scenario: Exact decimal int accepted
- **WHEN** a request carries `9007199254740993e0` for an `int` param
- **THEN** the handler receives `9007199254740993` without rounding

#### Scenario: Float-collapsed fraction rejected
- **WHEN** a request carries `9007199254740992.5` for an `int` param
- **THEN** validation rejects it with `-32602`

#### Scenario: Client response id stays exact
- **WHEN** a client decodes a response with id `9007199254740993`
- **THEN** the id remains `json.Number` and the envelope check compares it exactly

#### Scenario: Client id comparison stays exact
- **WHEN** the client envelope check compares `7` with `7.0`, or a large `uint64` request id with its exact `json.Number` echo
- **THEN** they are equal, and string ids still compare by string equality
