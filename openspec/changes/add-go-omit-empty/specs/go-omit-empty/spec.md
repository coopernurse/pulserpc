## ADDED Requirements

### Requirement: Optional-only omitempty remains the default

When `-omit-empty` is unset or false, generated Go struct tags SHALL keep the
existing behavior: optional fields include `omitempty`, required fields do not.

#### Scenario: Default required field
- **WHEN** a required `int` field is generated without `-omit-empty`
- **THEN** its JSON tag has no `omitempty`

#### Scenario: Default optional field
- **WHEN** an optional field is generated without `-omit-empty`
- **THEN** its JSON tag includes `omitempty`

### Requirement: OmitEmpty applies omitempty to every struct field

When `-omit-empty=true`, every generated Go struct field SHALL include
`omitempty` in its JSON tag, including required scalar, string, and bool
fields. Optional fields keep their existing `omitempty`.

#### Scenario: Required field with OmitEmpty
- **WHEN** a required `int` field is generated with `-omit-empty=true`
- **THEN** its JSON tag is ``json:"field,omitempty"``

#### Scenario: Optional field with OmitEmpty
- **WHEN** an optional field is generated with `-omit-empty=true`
- **THEN** its JSON tag still includes `omitempty`

#### Scenario: Embedded parent field
- **WHEN** a struct extends another struct
- **THEN** the embedded parent field is emitted without a JSON tag and is not
  affected by `OmitEmpty`

### Requirement: OmitEmpty is a generator option

`OmitEmpty` SHALL be a Go generator flag, not a `ServerConfig` runtime field,
because generated struct tags are compiled into the output.

#### Scenario: Runtime config does not expose OmitEmpty
- **WHEN** a server is configured through `pulserpc.ServerConfig`
- **THEN** there is no `OmitEmpty` field that changes struct tags at runtime
