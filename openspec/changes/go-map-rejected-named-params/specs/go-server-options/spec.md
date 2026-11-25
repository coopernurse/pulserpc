## ADDED Requirements

### Requirement: Optional mapping for rejected named parameters

The Go runtime SHALL provide an opt-in positional parameter mode that rejects object parameters through the parameter decode mappers. It SHALL fall back to the existing invalid-request error and data. Existing modes SHALL remain unchanged.

#### Scenario: Method-specific rejection

- **WHEN** a registered method receives named parameters in mapped positional mode
- **THEN** the runtime applies its decode error mappers without invoking the handler

#### Scenario: No mapping

- **WHEN** no mapper replaces the rejection
- **THEN** the runtime returns the existing invalid-request response

## MODIFIED Requirements

### Requirement: Decode and validation errors can be mapped

`ServerConfig.MapError` SHALL receive `ErrorPhaseDecodeParams` for params-shape,
parameter-count, named-param conversion, named-param rejection in
`ParamsModePositionalOnlyMapped`, and unknown-field failures, and
`ErrorPhaseValidateParams` for `Contract.ValidateRequest` failures. A non-nil
returned `*RPCError` SHALL be returned with HTTP 200 and exactly its code,
message, and data. Returning nil SHALL fall back to the pre-change response
(`*RPCError` passthrough, otherwise `-32602 "Invalid params"` with the error
text as data). When `ServerConfig.MapMethodError` is also set, it SHALL be
consulted first with the full RPC method name and the same phase and error, and
a nil result SHALL fall through to `MapError`. Envelope errors (invalid
`jsonrpc`/method/id) SHALL NOT be mapped and SHALL remain `-32600`. Named params
rejected by `ParamsModePositionalOnly` SHALL NOT be mapped; rejection in
`ParamsModePositionalOnlyMapped` SHALL be mapped and SHALL fall back to the same
`-32600 "Invalid Request"` response with data `"named params are not allowed"`
when no mapper returns an error.

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
- **WHEN** `MapError` is set and the request has an invalid id or bad `jsonrpc`
- **THEN** the response is `-32600` and `MapError` is not called

#### Scenario: Mapped positional mode uses the mapper
- **WHEN** `ParamsMode` is `ParamsModePositionalOnlyMapped` and params is an object
- **THEN** the mappers receive `ErrorPhaseDecodeParams`, the handler does not run, and the mapper result (if any) is returned

#### Scenario: Legacy positional mode stays unmapped
- **WHEN** `ParamsMode` is `ParamsModePositionalOnly` and params is an object
- **THEN** the response is `-32600` and neither mapper is called
