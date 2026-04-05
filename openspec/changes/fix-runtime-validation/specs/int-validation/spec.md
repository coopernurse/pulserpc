## ADDED Requirements

### Requirement: Integer fields accept whole-number JSON numbers

When validating a field with type `int`, the runtime SHALL accept any JSON number whose value equals a whole number (no fractional part), regardless of whether the JSON representation is `5`, `5.0`, or `-3.0`.

#### Scenario: Valid whole-number float for int field
- **WHEN** the IDL defines a field as `int` and the JSON value is `5.0`
- **THEN** validation SHALL pass

#### Scenario: Valid integer for int field
- **WHEN** the IDL defines a field as `int` and the JSON value is `5`
- **THEN** validation SHALL pass

#### Scenario: Valid negative whole-number float for int field
- **WHEN** the IDL defines a field as `int` and the JSON value is `-3.0`
- **THEN** validation SHALL pass

### Requirement: Integer fields reject fractional JSON numbers

When validating a field with type `int`, the runtime SHALL reject any JSON number with a non-zero fractional part.

#### Scenario: Float with fractional part rejected for int field
- **WHEN** the IDL defines a field as `int` and the JSON value is `5.1`
- **THEN** validation SHALL fail with a type error indicating expected int

#### Scenario: Float with fractional part rejected for negative int field
- **WHEN** the IDL defines a field as `int` and the JSON value is `-3.5`
- **THEN** validation SHALL fail with a type error indicating expected int

#### Scenario: Scientific notation whole number rejected for int field
- **WHEN** the IDL defines a field as `int` and the JSON value is `1e2` (which equals 100)
- **THEN** validation SHALL fail (scientific notation is not a valid int representation)

### Requirement: Integer fields reject non-numeric types

When validating a field with type `int`, the runtime SHALL reject values that are not numbers.

#### Scenario: String rejected for int field
- **WHEN** the IDL defines a field as `int` and the JSON value is `"5"`
- **THEN** validation SHALL fail with a type error indicating expected int

#### Scenario: Null rejected for int field
- **WHEN** the IDL defines a field as `int` and the JSON value is `null`
- **THEN** validation SHALL fail (null is not a valid int)

#### Scenario: Boolean rejected for int field
- **WHEN** the IDL defines a field as `int` and the JSON value is `true`
- **THEN** validation SHALL fail with a type error indicating expected int