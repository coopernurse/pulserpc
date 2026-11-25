## ADDED Requirements

### Requirement: Exact IDL field names in Go requests

When enabled during Go server request validation, exact IDL field-name validation SHALL reject any request struct key missing from the resolved IDL definition. This SHALL include aliases for Go field names and SHALL cover inherited and nested structs. Ordinary map keys, response validation and defaults SHALL remain unaffected.

#### Scenario: Go field alias

- **WHEN** a request struct includes a Go field alias alongside or in place of the IDL key
- **THEN** request validation rejects the alias

#### Scenario: Nested map value

- **WHEN** an arbitrary map key identifies a value whose type is an IDL struct
- **THEN** validation accepts the map key and enforces the declared keys of its value
