## Why

The PulseRPC runtime implementations across Go, Python, TypeScript, C#, and Java have inconsistent type validation behavior. This causes cross-language compatibility issues, particularly around numeric types (int vs float), optional field handling, and error propagation via the `raises()` clause.

## What Changes

1. **Unify int validation across all runtimes** - JSON numbers with whole values (like `5.0`) should pass for `int` fields, but values with fractional parts (like `5.1`) should fail validation.

2. **Fix TypeScript optional field handling** - TypeScript currently allows `undefined` for optional fields, but Go/Python/C#/Java only allow `null`. Make TypeScript consistent by rejecting `undefined`.

3. **Implement `raises()` propagation in all languages** - The `raises()` clause on method definitions should propagate errors to clients consistently across all runtimes. Currently only Go correctly returns `RPCError` to clients.

4. **Add integration tests for type validation** - Create tests that verify numeric edge cases, optional field handling, and enum case sensitivity work correctly across all runtimes.

## Capabilities

### New Capabilities

- `int-validation`: Unified integer validation that accepts whole-number JSON numbers (e.g., `5.0`) and rejects fractional numbers (e.g., `5.1`)
- `optional-field-consistency`: Consistent handling of optional fields across all languages - only `null` is valid, not `undefined`
- `raises-propagation`: Error propagation via `raises()` clause works identically across Go, Python, TypeScript, C#, and Java runtimes

### Modified Capabilities

- (none)

## Impact

- **Runtimes affected**: Go, Python, TypeScript, C#, Java validation modules
- **Test coverage**: New integration tests for numeric types, optional fields, enum case sensitivity
- **Breaking changes**: TypeScript optional field behavior will change (rejects `undefined` instead of accepting it)