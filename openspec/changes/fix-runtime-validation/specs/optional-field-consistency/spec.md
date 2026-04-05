## ADDED Requirements

### Requirement: Optional fields accept null values

When validating a field marked as `[optional]` in the IDL, the runtime SHALL accept `null` as a valid value for that field.

#### Scenario: Null is valid for optional field
- **WHEN** the IDL defines a field as `[optional]` and the JSON value is `null`
- **THEN** validation SHALL pass

#### Scenario: Null is valid for optional field even when other fields present
- **WHEN** the IDL defines a struct with optional field `email` and required fields `firstName`, `lastName`
- **AND** the JSON value for `email` is `null`
- **THEN** validation SHALL pass

### Requirement: Optional fields accept absent fields

When validating a struct field marked as `[optional]`, the runtime SHALL accept the complete absence of that field from the JSON object.

#### Scenario: Missing optional field is valid
- **WHEN** the IDL defines a struct with an optional field `email`
- **AND** the JSON object does not contain the key `email`
- **THEN** validation SHALL pass

### Requirement: TypeScript optional fields must not accept undefined

When validating a field marked as `[optional]` in TypeScript runtime, the runtime SHALL NOT accept `undefined` as a valid value.

#### Scenario: TypeScript undefined is rejected for optional field
- **WHEN** the TypeScript runtime validates a field marked as `[optional]`
- **AND** the JSON value is `undefined`
- **THEN** validation SHALL fail with a type error indicating the field cannot be undefined

#### Scenario: TypeScript undefined is rejected for optional field in nested object
- **WHEN** the TypeScript runtime validates a struct with an optional field `email`
- **AND** the JSON value has `"email": undefined`
- **THEN** validation SHALL fail with a type error indicating the field cannot be undefined

### Requirement: Non-optional fields reject null

When validating a field that is NOT marked as `[optional]`, the runtime SHALL reject `null` as a value for that field.

#### Scenario: Null rejected for required field
- **WHEN** the IDL defines a field without `[optional]` and the JSON value is `null`
- **THEN** validation SHALL fail with an error indicating the field is required

### Requirement: Consistent optional field handling across all runtimes

Go, Python, C#, and Java runtimes SHALL only accept `null` for optional fields (not `undefined`, which is not a concept in these languages).

#### Scenario: Go accepts null for optional field
- **WHEN** the Go runtime validates a field marked as `[optional]`
- **AND** the value is `null`
- **THEN** validation SHALL pass

#### Scenario: Python accepts None for optional field
- **WHEN** the Python runtime validates a field marked as `[optional]`
- **AND** the value is `None`
- **THEN** validation SHALL pass

#### Scenario: C# accepts null for optional field
- **WHEN** the C# runtime validates a field marked as `[optional]`
- **AND** the value is `null`
- **THEN** validation SHALL pass

#### Scenario: Java accepts null for optional field
- **WHEN** the Java runtime validates a field marked as `[optional]`
- **AND** the value is `null`
- **THEN** validation SHALL pass