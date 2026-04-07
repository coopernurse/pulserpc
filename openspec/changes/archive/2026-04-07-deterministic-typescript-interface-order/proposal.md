## Why

When regenerating TypeScript code from the same IDL definition, the order of interfaces, enums, and methods may change even though the content is semantically identical. This causes unnecessary noise in code reviews and makes it harder to verify that regeneration only produced deterministic output.

## What Changes

- Sort TypeScript interface and enum declarations alphabetically by name during code generation
- Sort method declarations alphabetically by name within interfaces
- Apply sorting consistently across both single-namespace and multi-namespace TypeScript generation modes

## Capabilities

### New Capabilities
- `deterministic-ts-generation`: Ensures TypeScript code generation produces consistent output ordering regardless of parsing internals

### Modified Capabilities
- None

## Impact

- `/pkg/generator/ts_client_server.go` - Main TypeScript code generation file