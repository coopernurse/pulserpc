## Why

The Python quickstart currently uses magic numbers for error codes (e.g., `raise RPCError(1001, ...)`) instead of named constants. This makes code harder to read and maintain. Error codes are documented in comments but not enforced by the IDL.

## What Changes

- Add `errors {}` block to the quickstart `checkout.pulse` file with proper IDL syntax
- Remove error code comments from the pulse file
- Update Python code generator to produce error constant classes (like `Err` and `ErrJsonRpc`)
- Update `my_server.py` to use generated error constants instead of magic numbers

## Capabilities

### New Capabilities

- `error-constants-generation`: Generate Python error constant classes from `errors {}` block in IDL, producing `Err` and `ErrJsonRpc` classes similar to the existing `errors.py` pattern

### Modified Capabilities

- None - this is an enhancement to existing code generation, not a change to requirements

## Impact

- Code generator: `pkg/generator/python_client_server.go` - add error constant generation
- Example file: `examples/quickstart/checkout.pulse` - add errors block
- Example file: `examples/quickstart/python/my_server.py` - use generated constants
