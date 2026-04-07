## 1. Update Runtime Server

- [x] 1.1 Change `call()` signature to `async call(req: JsonRpcRequest): Promise<JsonRpcResponse | null>` in `pkg/runtime/runtimes/ts/pulserpc/server.ts`
- [x] 1.2 Change `const result = func(...args);` to `const result = await func(...args);` in handler invocation block

## 2. Update Code Generator

- [x] 2.1 Update `writeInterfaceStubTs` to add `async` keyword to abstract method signatures in `pkg/generator/ts_client_server.go`
- [x] 2.2 Update `writeInterfaceStubTsForNamespace` to add `async` keyword to abstract method signatures

## 3. Update Test Server Generator

- [x] 3.1 Change `this.rpcServer.call(data)` to `await this.rpcServer.call(data)` in `generateTestServerTs` function

## 4. Verify

- [x] 4.1 Run TypeScript generator tests to ensure async code compiles
- [x] 4.2 Verify generated server.ts has async abstract methods
