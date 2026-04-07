## Why

The TypeScript code generator currently generates synchronous `call` methods in `server.ts`, but real-world RPC handlers often need to perform async operations (database calls, external API requests). This forces developers to manually modify generated code or work around sync limitations.

## What Changes

- Modify TypeScript code generator to generate `async call()` methods returning `Promise<JsonRpcResponse | null>`
- Update the generated server template to use `await` when invoking handlers
- **BREAKING**: Generated `server.ts` will require async handling on the consumer side

## Capabilities

### New Capabilities
- `ts-async-server-call`: Generate async-capable server stubs with Promise-based `call` methods

### Modified Capabilities
- (none - this is a generator implementation change, not a spec-level behavior change)

## Impact

- **Generated code**: `pkg/runtime/runtimes/ts/pulserpc/server.ts` template
- **Generator**: TypeScript code generator in `pkg/codegen/`
- **Consumers**: Any service using the TypeScript generator will need to handle async handlers
