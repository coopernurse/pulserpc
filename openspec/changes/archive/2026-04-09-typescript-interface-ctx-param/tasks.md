## 1. Generator Changes

- [x] 1.1 Update `writeInterfaceStubTs` in `pkg/generator/ts_client_server.go` to emit `ctx: Record<string, any>` parameter after IDL parameters
- [x] 1.2 Update `writeInterfaceStubTsForNamespace` in `pkg/generator/ts_client_server.go` to emit `ctx: Record<string, any>` parameter after IDL parameters
- [x] 1.3 Update `writeTestMethodImplTs` in `pkg/generator/ts_client_server.go` to emit `ctx: any` parameter in test implementations

## 2. Runtime Changes

- [x] 2.1 Update `Server.call` in `pkg/runtime/runtimes/ts/pulserpc/server.ts` to accept `ctx` as second parameter and pass to handler method

## 3. Verification

- [x] 3.1 Run TypeScript generator tests to ensure generated code compiles
- [x] 3.2 Verify generated interface signatures include ctx parameter
- [x] 3.3 Run `make quality-full` and fix any issues