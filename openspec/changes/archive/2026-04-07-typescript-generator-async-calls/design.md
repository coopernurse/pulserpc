## Context

The TypeScript code generator creates abstract interface classes for services and a runtime `Server` class. The runtime `Server.call()` method currently invokes handlers synchronously, but real-world services often need async handlers (database calls, external API requests, etc.).

## Goals / Non-Goals

**Goals:**
- Make `Server.call()` async with `Promise<JsonRpcResponse | null>` return type
- Make handler invocation use `await` for async compatibility
- Update abstract interface method signatures to be async

**Non-Goals:**
- Breaking existing sync handlers immediately (we'll await the result either way)
- Changes to other runtimes (Go, Python, etc.)
- Changes to client-side code

## Decisions

1. **Use async/await in Server.call()**: The runtime `Server.call()` will become `async call()` and use `await func(...args)`. JavaScript/TypeScript will handle both sync and async functions seamlessly - `await` on a non-Promise returns the value directly.

2. **Update test_server.ts generator**: The test server generator at `pkg/generator/ts_client_server.go:1031` calls `this.rpcServer.call(data)` synchronously. This will need to become `await this.rpcServer.call(data)`.

3. **Update abstract interface stubs**: The abstract method signatures in generated `server.ts` files should return `Promise<T>` types to indicate they can be implemented as async. The `async` keyword is NOT used on abstract methods (TypeScript doesn't allow `async abstract`).

## Risks / Trade-offs

- [Risk] **Breaking change**: Existing code that calls `server.call()` synchronously will need updating to `await server.call()`. Mitigation: This is a necessary improvement; consumers will adapt.
- [Trade-off] The generated abstract interface methods use `Promise<T>` return types (not `async abstract` since TypeScript disallows that combination), which requires implementers to use `async` when providing concrete implementations.

## Migration Plan

1. Modify `pkg/runtime/runtimes/ts/pulserpc/server.ts`:
   - Change `call(req: JsonRpcRequest): JsonRpcResponse | null` to `async call(req: JsonRpcRequest): Promise<JsonRpcResponse | null>`
   - Change `const result = func(...args);` to `const result = await func(...args);`

2. Modify `pkg/generator/ts_client_server.go`:
   - Update `writeInterfaceStubTs` and `writeInterfaceStubTsForNamespace` to add `async` keyword to abstract method signatures

3. Update test server generator to use `await` when calling `server.call()`

## Open Questions

- None at this time.
