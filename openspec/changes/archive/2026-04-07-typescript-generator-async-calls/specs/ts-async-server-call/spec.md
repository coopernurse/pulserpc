## ADDED Requirements

### Requirement: TypeScript server stubs support async handlers

The TypeScript code generator SHALL generate server stubs with async-compatible `call` method and abstract interface methods.

#### Scenario: Server.call() is async
- **WHEN** a JSON-RPC request is processed by the runtime Server
- **THEN** the `call` method is declared as `async call(req: JsonRpcRequest): Promise<JsonRpcResponse | null>`
- **AND** handler invocation uses `await func(...args)` to support both sync and async handlers

#### Scenario: Abstract interface methods are async
- **WHEN** the code generator creates abstract interface classes in server.ts
- **THEN** each abstract method is declared with `async` keyword
- **AND** the return type is wrapped in `Promise<>`

#### Scenario: Test server uses await when calling call()
- **WHEN** the test server generator creates HTTP handler code
- **THEN** it uses `await this.rpcServer.call(data)` instead of synchronous call
