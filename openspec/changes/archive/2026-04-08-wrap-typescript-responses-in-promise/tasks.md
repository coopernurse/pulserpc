## 1. Generator Changes

- [x] 1.1 Update `writeInterfaceStubTs` in `pkg/generator/ts_client_server.go` to wrap return types in `Promise<>`
- [x] 1.2 Handle optional return types with `Promise<T | null>` pattern
- [x] 1.3 Ensure void-returning methods remain as `void` (not `Promise<void>`)

## 2. Verification

- [x] 2.1 Generate TypeScript code and verify interface methods have `Promise<>` return types
- [x] 2.2 Verify client methods remain unchanged (they already use `Promise<>`)

## 3. Runtime and Tests

- [x] 3.1 Check TypeScript runtime compatibility (if runtime types assume sync interfaces)
- [x] 3.2 Update quickstart TypeScript tests if they implement the interface directly
- [x] 3.3 Regenerate quickstart examples to verify end-to-end
