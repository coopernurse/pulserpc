## ADDED Requirements

### Requirement: IDL checksum field

The parser SHALL compute a SHA-256 checksum of the structural IDL content and include it as a top-level `checksum` field in `idl.json`.

#### Scenario: Checksum is present in idl.json
- **WHEN** IDL is parsed and serialized to JSON
- **THEN** the output contains `"checksum": "<base64url-encoded-sha256>"` at the top level

#### Scenario: Checksum is non-empty string
- **WHEN** IDL is parsed
- **THEN** the checksum field is a non-empty string of 44 base64url characters

### Requirement: Whitespace and comments excluded

The checksum computation SHALL exclude all whitespace and comments from the source .pulse file.

#### Scenario: Adding whitespace doesn't change checksum
- **WHEN** user adds extra newlines to a .pulse file
- **THEN** the computed checksum remains identical

#### Scenario: Moving struct above enum doesn't change checksum
- **WHEN** user moves a struct definition above an enum definition
- **THEN** the computed checksum remains identical

#### Scenario: Reordering interface methods doesn't change checksum
- **WHEN** user reorders methods within an interface
- **THEN** the computed checksum remains identical

### Requirement: Structural elements included

The checksum SHALL be computed from the following structural elements:

- `rootNamespace`
- Interface names (fully-qualified), methods (sorted by name), parameters (sorted by name), return type, returnOptional flag, raises list (sorted)
- Struct names (fully-qualified), extends chain, fields (sorted by name, with types and optional flag)
- Enum names (fully-qualified), values (sorted by name)
- Error names (fully-qualified), error codes

#### Scenario: Same interface with different parameter order produces same checksum
- **WHEN** method has parameters `(a string, b int)` in file version A
- **AND** same method has parameters `(b int, a string)` in file version B
- **THEN** both versions produce identical checksums

#### Scenario: Same struct with different field order produces same checksum
- **WHEN** struct has fields `(name string, age int)` in file version A
- **AND** same struct has fields `(age int, name string)` in file version B
- **THEN** both versions produce identical checksums

### Requirement: Namespace-qualified type names

Type references SHALL be resolved to fully-qualified namespace names before checksum computation.

#### Scenario: Same type referenced with different qualification produces same checksum
- **WHEN** struct field uses `BaseResponse` in namespace `checkout`
- **AND** same field uses `checkout.BaseResponse`
- **THEN** both produce identical checksums

### Requirement: Error codes included, messages excluded

The checksum computation SHALL include error codes but exclude error message strings.

#### Scenario: Changing error message doesn't change checksum
- **WHEN** error declaration has message `"Not found"` in version A
- **AND** same error has message `"Item not found"` in version B
- **THEN** both versions produce identical checksums

#### Scenario: Changing error code changes checksum
- **WHEN** error declaration has code `1001` in version A
- **AND** same error has code `1002` in version B
- **THEN** the checksums are different

### Requirement: Inheritance chain recorded, fields not flattened

The checksum SHALL record the `extends` relationship using fully-qualified names but SHALL NOT flatten inherited fields into the child struct's checksum contribution.

#### Scenario: Same inheritance produces same checksum
- **WHEN** struct `BookWithStatus extends Book` in version A
- **AND** same relationship exists in version B with identical field definitions
- **THEN** both versions produce identical checksums

### Requirement: Deterministic algorithm

The checksum algorithm SHALL produce identical output for the same IDL regardless of the order in which elements appear in the source file.

#### Scenario: Identical content in different order produces same checksum
- **GIVEN** two .pulse files with identical interfaces, structs, enums, and errors
- **WHEN** elements are arranged in different orders
- **THEN** both files produce the same checksum

#### Scenario: Different content produces different checksum
- **GIVEN** two .pulse files with different structural content
- **THEN** the checksums are different

## Unit Testing Recommendations

### Property-Based Tests (using go-proptest or similar)

1. **Order Invariance**: For any parsed IDL, shuffling the order of top-level declarations (interfaces, structs, enums, errors) within the source text should not change the computed checksum.

2. **Whitespace Invariance**: Adding, removing, or modifying whitespace (spaces, tabs, newlines) at any position should not change the checksum.

3. **Comment Invariance**: Adding, removing, or modifying comments at any position should not change the checksum.

4. **Type Reference Resolution**: Type references using bare names vs fully-qualified names should produce the same checksum when they refer to the same type.

5. **Parameter Sort Invariance**: Permuting parameter order within a method should not change the checksum.

6. **Field Sort Invariance**: Permuting field order within a struct should not change the checksum.

7. **Method Sort Invariance**: Permuting method order within an interface should not change the checksum.

### Traditional Unit Tests

1. **Checksum Format**: Verify checksum is 44 characters of base64url.

2. **Known Input**: Use `book.pulse` to compute a known checksum and verify it doesn't change (regression test).

3. **Empty IDL**: Verify checksum can be computed for minimal IDL.

4. **Full Book Example**: Compute checksum for `book.pulse` and verify it matches expected value (stored as constant).

### Integration Tests

1. **Generator Integration**: Verify that all generators (Go, Python, TypeScript, Java, C#) include checksum in generated idl.json.

2. **Round-trip**: Parse idl.json, recompute checksum, verify it matches the stored checksum.
