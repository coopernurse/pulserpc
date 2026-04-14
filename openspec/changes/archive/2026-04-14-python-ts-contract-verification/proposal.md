## Why

The Go, Java, and C# runtimes support contract validation and diffing - the client can compare its IDL against what the server returns via `pulserpc-idl` and log errors/warnings if there are deltas. The Python and TypeScript runtimes are missing this capability.

## What Changes

- Add `DiffIDL()` function to Python and TypeScript runtimes
- Add `ContractDelta`, `VerificationResult`, and `ContractAuditor` types to Python and TypeScript runtimes
- Add `verify_compatibility()` method to Python `Client`
- Add `verifyCompatibility()` method (async) to TypeScript `Client`
- Add `set_local_idl()` to Python `Client` and `setLocalIDL()` to TypeScript `Client`
- Add built-in auditors (NoOp, Logging, FailFast) for both runtimes
- Add client options dict support for auditor and verifyOnBootstrap configuration
- Python client reads local IDL from `idl.json` at runtime (generated alongside client code)
- TypeScript client reads local IDL from `idl.json` at runtime
- Add shared diff test cases to ensure behavioral consistency across all runtimes
- Ensure quickstart and generator tests pass with the new functionality

## Capabilities

### New Capabilities

- `python-contract-verification`: Contract compatibility verification for Python runtime (mirrors existing Go/Java/C# spec)
- `ts-contract-verification`: Contract compatibility verification for TypeScript runtime (mirrors existing Go/Java/C# spec)

### Modified Capabilities

<!-- No requirement changes to existing specs - this is purely additive -->

## Impact

- **New files in Python runtime**: `diff.py`, extended `contract.py`, extended `client.py`, extended `types.py`
- **New files in TypeScript runtime**: `diff.ts`, extended `contract.ts`, extended `client.ts`, extended `types.ts`
- **New test files**: `tests/test_diff.py` (Python), `tests/test_diff.ts` (TypeScript)
- **Shared test data**: Cross-language diff test cases for consistency testing
- **Generator impact**: Python and TypeScript generators continue to write `idl.json` (already exists), no code embedding needed
