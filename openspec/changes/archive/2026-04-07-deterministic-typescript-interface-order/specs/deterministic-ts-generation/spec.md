## ADDED Requirements

### Requirement: TypeScript enum declarations are sorted alphabetically
TypeScript code generation SHALL sort enum declarations alphabetically by enum name before emitting them into the generated output.

#### Scenario: Same input produces same enum order
- **WHEN** Code generator processes an IDL file with enums named `Zoo`, `Alpha`, `Mango`
- **THEN** The generated TypeScript output SHALL have enums ordered `Alpha`, `Mango`, `Zoo`

### Requirement: TypeScript interface declarations are sorted alphabetically
TypeScript code generation SHALL sort interface/struct declarations alphabetically by name before emitting them into the generated output.

#### Scenario: Same input produces same interface order
- **WHEN** Code generator processes an IDL file with structs named `User`, `Address`, `Profile`
- **THEN** The generated TypeScript output SHALL have interfaces ordered `Address`, `Profile`, `User`

### Requirement: TypeScript interface methods are sorted alphabetically
TypeScript code generation SHALL sort method declarations alphabetically by method name within each interface before emitting them into the generated output.

#### Scenario: Same input produces same method order
- **WHEN** Code generator processes an interface with methods named `deleteUser`, `getUser`, `createUser`
- **THEN** The generated TypeScript interface SHALL have methods ordered `createUser`, `deleteUser`, `getUser`

### Requirement: Regeneration produces identical output
Regenerating TypeScript code from the same IDL definition SHALL produce byte-for-byte identical output when:
- The IDL source files are identical
- The generator version is the same

#### Scenario: Regeneration is idempotent
- **WHEN** User runs the code generator twice on the same IDL file
- **THEN** Both generated TypeScript files SHALL be identical (no spurious diffs)
