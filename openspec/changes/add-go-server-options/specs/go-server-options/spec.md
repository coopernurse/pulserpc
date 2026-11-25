## ADDED Requirements

### Requirement: Default server policy preserves v0.4.0 behavior

`pulserpc.DefaultServerConfig()` SHALL return a policy that allows batches,
allows notifications, accepts both positional and named params, uses the legacy
1 MiB request body cap with the legacy oversize error, does not reject invalid
UTF-8 or unknown request fields, and does not map errors. `NewServer` and
`NewPulseRPCServer` SHALL delegate with this policy. The `ServerConfig` zero
value SHALL be documented as disabling batching.

#### Scenario: Default constructor accepts a batch
- **WHEN** a server is built with `NewPulseRPCServer` and receives a JSON array body
- **THEN** each member is dispatched and the response keeps the array shape

#### Scenario: Default constructor accepts notifications
- **WHEN** a request has no `id` and dispatch succeeds or fails
- **THEN** no response is written

#### Scenario: Zero value disables batching
- **WHEN** a server is built from `pulserpc.ServerConfig{}` and receives a JSON array body
- **THEN** the adapter returns `-32600` before dispatch

### Requirement: Batches can be disabled

When `ServerConfig.AllowBatch` is false, the generated adapter SHALL reject a
JSON array request body with `-32600` before dispatching any member.

#### Scenario: Batch rejected when disabled
- **WHEN** `AllowBatch` is false and the body is `[{"jsonrpc":"2.0",...}]`
- **THEN** the response is a single `-32600` error object
- **AND** no handler runs

### Requirement: Request ids can be required

When `ServerConfig.RequireNonNullID` is true, the runtime SHALL reject a request
whose `id` is absent or `null` with `-32600` and a response id of `null`. The
check SHALL run after the nil-request check and again after RPC middleware
rewrites the request. The error SHALL be written even though the request would
otherwise be a notification, and the handler SHALL NOT run.

#### Scenario: Absent id rejected
- **WHEN** `RequireNonNullID` is true and the request has no `id`
- **THEN** the response is `-32600` with `"id": null`

#### Scenario: Null id rejected
- **WHEN** `RequireNonNullID` is true and the request has `"id": null`
- **THEN** the response is `-32600` with `"id": null`

#### Scenario: Handler does not run
- **WHEN** a request without an id is rejected by `RequireNonNullID`
- **THEN** the handler is not invoked

### Requirement: Positional-only params mode

When `ServerConfig.ParamsMode` is `ParamsModePositionalOnly`, the runtime SHALL
reject object (named) params with `-32600`. Absent and `null` params SHALL still
count as zero arguments, including for a zero-argument handler. The named-params
rejection SHALL NOT be routed through `MapError`.

#### Scenario: Named params rejected
- **WHEN** `ParamsMode` is `ParamsModePositionalOnly` and params is an object
- **THEN** the response is `-32600`

#### Scenario: Absent params call a zero-arg method
- **WHEN** `ParamsMode` is `ParamsModePositionalOnly` and the request omits params
- **THEN** a zero-argument handler runs successfully

### Requirement: Configurable request body limit and UTF-8 rejection

`MaxRequestBodyBytes <= 0` SHALL keep the legacy 1 MiB cap and the legacy
`-32600 "Request body too large"` response. A positive `MaxRequestBodyBytes`
SHALL cap the body at that value and return `-32700 "Parse error"` for an
oversized body. `RejectInvalidUTF8` true SHALL return `-32700 "Parse error"` for
a body that is not valid UTF-8; false SHALL keep `encoding/json`'s replacement
behavior. The `http.MaxBytesReader` cap SHALL be installed before HTTP
middleware.

#### Scenario: Default oversize error
- **WHEN** `MaxRequestBodyBytes` is 0 and the body exceeds 1 MiB
- **THEN** the response is `-32600 "Request body too large"`

#### Scenario: Custom oversize error
- **WHEN** `MaxRequestBodyBytes` is 32 KiB and the body exceeds it
- **THEN** the response is `-32700 "Parse error"`

#### Scenario: Invalid UTF-8 rejected
- **WHEN** `RejectInvalidUTF8` is true and the body contains an invalid byte
- **THEN** the response is `-32700 "Parse error"`

#### Scenario: Body cap precedes middleware
- **WHEN** HTTP middleware reads the request body
- **THEN** the read is bounded by the configured cap

### Requirement: Unknown request fields can be rejected

When `ServerConfig.RejectUnknownRequestFields` is true, the runtime SHALL reject
unknown keys in struct-typed request params with `-32602` (or the mapped error).
Detection SHALL recurse through pointers, slices/arrays, maps, and
nested/embedded structs, SHALL honor `json` tag names and `json:"-"`, and SHALL
not reject unknown top-level JSON-RPC envelope members. When false, unknown keys
SHALL continue to be ignored.

#### Scenario: Top-level unknown field rejected
- **WHEN** strict mode is on and a struct param contains an unknown key
- **THEN** the response is `-32602` and the handler does not run

#### Scenario: Nested unknown field rejected
- **WHEN** strict mode is on and a nested struct, pointer target, slice element, or map value contains an unknown key
- **THEN** the response is an error naming the unknown field

#### Scenario: Known fields accepted
- **WHEN** strict mode is on and every key maps to a struct field
- **THEN** the handler runs

#### Scenario: Default is lenient
- **WHEN** strict mode is off and a struct param contains an unknown key
- **THEN** the key is ignored and the handler runs

### Requirement: Decode and validation errors can be mapped

`ServerConfig.MapError` SHALL receive `ErrorPhaseDecodeParams` for params-shape,
parameter-count, named-param conversion, and unknown-field failures, and
`ErrorPhaseValidateParams` for `Contract.ValidateRequest` failures. A non-nil
returned `*RPCError` SHALL be returned with HTTP 200 and exactly its code,
message, and data. Returning nil SHALL fall back to the pre-change response
(`*RPCError` passthrough, otherwise `-32602 "Invalid params"` with the error
text as data). Envelope errors (invalid `jsonrpc`/method/id, named params
rejected by `ParamsMode`) SHALL NOT be mapped and SHALL remain `-32600`.

#### Scenario: Decode phase
- **WHEN** `MapError` is set and a request has an unknown named parameter or an unknown struct field
- **THEN** `MapError` receives `ErrorPhaseDecodeParams`

#### Scenario: Validation phase
- **WHEN** `MapError` is set and a parameter fails contract validation
- **THEN** `MapError` receives `ErrorPhaseValidateParams`

#### Scenario: Mapped error has no injected detail
- **WHEN** `MapError` returns `NewRPCError(4101, "Invalid input")`
- **THEN** the response has code 4101, message "Invalid input", no `data` member, and HTTP 200

#### Scenario: Nil falls back
- **WHEN** `MapError` returns nil
- **THEN** the standard `-32602` response is returned

#### Scenario: Envelope errors stay unmapped
- **WHEN** `MapError` is set and the request has an invalid id, bad `jsonrpc`, or named params in positional-only mode
- **THEN** the response is `-32600` and `MapError` is not called

### Requirement: Numeric ids are exact

The generated adapter SHALL decode request bodies with `json.Decoder` and
`UseNumber()` and SHALL reject a second JSON value as `-32700`. Numeric ids
SHALL be echoed exactly, independent of `ServerConfig`. Runtime validation and
conversion SHALL accept `json.Number` for `int` and `float` params without
losing fractional/out-of-range rejection, and the client envelope check SHALL
compare numeric ids exactly rather than through `float64`.

#### Scenario: Large numeric id round-trips
- **WHEN** a request id is `9007199254740993`
- **THEN** the response contains exactly `"id":9007199254740993`

#### Scenario: Trailing data is a parse error
- **WHEN** the body contains two JSON values
- **THEN** the response is `-32700 "Parse error"`

#### Scenario: json.Number params convert
- **WHEN** a request carries `json.Number("2")` for an `int` param
- **THEN** the handler receives `2`

#### Scenario: Fractional int rejected
- **WHEN** a request carries `json.Number("1.5")` for an `int` param
- **THEN** validation rejects it with `-32602`

#### Scenario: Client id comparison stays exact
- **WHEN** the client envelope check compares `7` with `7.0`
- **THEN** they are equal, and string ids still compare by string equality
