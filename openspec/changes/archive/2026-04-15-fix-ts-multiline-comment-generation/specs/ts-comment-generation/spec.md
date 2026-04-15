## ADDED Requirements

### Requirement: Multi-line comment rendering in TypeScript

The TypeScript code generator SHALL render multi-line comments with the `//` prefix on all lines, not just the first line.

#### Scenario: Multi-line struct field comment
- **WHEN** a struct field in IDL has a multi-line comment
- **THEN** the generated TypeScript interface SHALL prefix each line with `//`

#### Scenario: Multi-line enum value comment
- **WHEN** an enum value in IDL has a multi-line comment
- **THEN** the generated TypeScript enum SHALL prefix each line with `//`
