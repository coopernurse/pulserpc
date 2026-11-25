## ADDED Requirements

### Requirement: Preserve explicitly empty optional arrays in generated Go

The Go generator SHALL preserve its default optional array tags unless `-preserve-empty-optional-arrays` is enabled. When enabled, generated Go bindings SHALL omit nil optional arrays and serialize non-nil empty optional arrays as `[]`. Other field types retain the selected `-omit-empty` behavior.

#### Scenario: Generated server returns an empty optional array

- **WHEN** the flag is enabled and a handler returns a non-nil empty optional array
- **THEN** the response includes the corresponding JSON field with value `[]`

#### Scenario: Generated server returns a nil optional array

- **WHEN** the flag is enabled and a handler returns a nil optional array
- **THEN** the response omits the corresponding JSON field

#### Scenario: Validation takes the JSON fallback

- **WHEN** Go response validation serializes a response before checking the IDL
- **THEN** it preserves signed 64-bit integer values exactly
