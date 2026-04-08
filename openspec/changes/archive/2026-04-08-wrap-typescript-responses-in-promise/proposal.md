## Why

The TypeScript generator produces interface declarations (in `server.ts`) with synchronous return types, but the corresponding client implementations return `Promise<T>`. This inconsistency forces developers to cast or duplicate signatures when implementing the interface, and creates a mismatch between what the interface contract promises and what the client actually delivers.

## What Changes

- TypeScript interface methods (in `server.ts` stubs) will wrap return types in `Promise<>`
- The generator will be updated to detect when methods have return types and wrap them appropriately
- The runtime library will be updated if any runtime types need adjustment for async interface compatibility
- Quickstart tests will be updated to reflect the new interface signatures

## Capabilities

### New Capabilities

- `ts-interface-async-return`: TypeScript interface methods declared in `server.ts` will return `Promise<T>` instead of `T` when a return type is present

### Modified Capabilities

- None (this is an implementation detail change that doesn't change spec-level behavior)

## Impact

- **Code Generated**: `server.ts` interface stubs will have `Promise<>` return types
- **Runtime**: May need minor adjustments if runtime types assume synchronous interfaces
- **Tests**: Quickstart TypeScript tests will need signature updates
