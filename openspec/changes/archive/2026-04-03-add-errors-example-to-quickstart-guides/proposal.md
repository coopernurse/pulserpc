## Why

The quickstart guides currently show how to raise errors in server implementations, but don't demonstrate how to use PulseRPC's `errors` keyword and `raises()` clause to declaratively define which errors each interface method can raise. This is a core PulseRPC feature that helps with API documentation and enables generated code to provide type-safe error handling.

## What Changes

- Update `checkout.idl` to use the formal `errors` keyword and `raises()` clauses on interface methods
- Update all language quickstart guides (Python, Go, TypeScript, Java, C#) to include an example section showing the `errors` keyword and `raises()` usage in the IDL
- Add cross-references to the Error Handling IDL guide

## Capabilities

### New Capabilities
- `errors-keyword-quickstart-example`: Demonstrate the `errors` keyword and `raises()` clause syntax in the checkout IDL and quickstart guides

### Modified Capabilities
- None (no existing spec-level behavior changes)

## Impact

- Files modified: `docs/_includes/quickstart/checkout.idl`, all language quickstart guides under `docs/languages/*/quickstart.md`
- No API changes - purely documentation update
