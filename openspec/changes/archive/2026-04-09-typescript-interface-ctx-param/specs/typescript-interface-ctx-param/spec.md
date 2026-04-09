## ADDED Requirements

### Requirement: TypeScript interface methods include ctx parameter

The TypeScript code generator SHALL emit all interface method signatures in `server.ts` with a required `ctx: Record<string, any>` parameter appended after the IDL-defined parameters.

#### Scenario: Generator creates method signatures with ctx
- **WHEN** code generator processes an IDL file defining interface `FooService` with method `Create(req CreateRequest) CreateResponse`
- **THEN** the generated abstract class in `server.ts` contains `abstract Create(req: CreateRequest, ctx: Record<string, any>): Promise<CreateResponse>;`

#### Scenario: Generator creates void method signatures with ctx
- **WHEN** code generator processes an IDL file defining interface `NotifyService` with method `Notify(msg string) void`
- **THEN** the generated abstract class in `server.ts` contains `abstract Notify(msg: string, ctx: Record<string, any>): Promise<void>;`

#### Scenario: Generator handles methods with multiple parameters
- **WHEN** code generator processes an IDL file defining interface `CalcService` with method `Add(a int, b int) int`
- **THEN** the generated abstract class in `server.ts` contains `abstract Add(a: number, b: number, ctx: Record<string, any>): Promise<number>;`

### Requirement: Runtime Server.call accepts ctx parameter

The runtime `Server.call()` method SHALL accept `ctx` as an optional second parameter and pass it as the final argument when invoking handler methods.

#### Scenario: Server.call passes ctx to handler
- **WHEN** `Server.call(req, ctx)` is called with `ctx = { userId: "123", auth: "Bearer token" }`
- **AND** the handler method is `async Create(req: CreateRequest, ctx: Record<string, any>): Promise<Response>`
- **THEN** the handler receives the ctx object with userId and auth

### Requirement: Test server generator includes ctx parameter

The test server generator SHALL emit method implementations that include the `ctx` parameter.

#### Scenario: Test server generator emits ctx in implementations
- **WHEN** test server generator processes an IDL file with interface `FooService` method `Create(req CreateRequest) CreateResponse`
- **THEN** the generated implementation class contains `Create(req: any, ctx: any): any { ... }`