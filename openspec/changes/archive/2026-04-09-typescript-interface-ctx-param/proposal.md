## Why

Server-side service implementations need access to transport-specific metadata at request time (e.g., HTTP headers containing authentication/authorization information). Currently there is no mechanism to pass this data from the HTTP handler layer to the service method.

## What Changes

- **Modified**: TypeScript generator emits `ctx: Record<string, any>` as a required parameter on all interface method signatures in `server.ts`
- **Modified**: Runtime `Server.call()` accepts `ctx` as a second parameter and passes it through to the handler method
- **Modified**: Test server generator includes `ctx` parameter in generated method implementations

## Capabilities

### New Capabilities
- `typescript-interface-ctx-param`: TypeScript interface methods include a required `ctx` parameter for passing transport-specific metadata from the HTTP handler layer to service implementations

## Impact

- **Code**: `pkg/generator/ts_client_server.go` (interface stub generation), `pkg/runtime/runtimes/ts/pulserpc/server.ts` (Server.call method)
- **Generated Output**: All TypeScript service interfaces will have a `ctx` parameter on every method
- **Migration**: Existing TypeScript service implementations will need to add `ctx` parameter to all methods