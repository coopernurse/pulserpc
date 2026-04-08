## ADDED Requirements

### Requirement: TypeScript interface methods return Promise for non-void types

The TypeScript code generator SHALL wrap return types in `Promise<>` for interface methods declared in `server.ts` when a return type exists and is not void.

#### Scenario: Interface method with return type wrapped in Promise
- **WHEN** TypeScript code is generated for a method with a non-void return type
- **THEN** the interface stub in `server.ts` declares the method with `Promise<T>` return type

#### Scenario: Interface method with optional return type wrapped in Promise
- **WHEN** TypeScript code is generated for a method with an optional non-void return type
- **THEN** the interface stub declares the method with `Promise<T | null>` return type

#### Scenario: Interface method without return type stays void
- **WHEN** TypeScript code is generated for a method with no return type (void)
- **THEN** the interface stub declares the method with `void` return type
