# PulseRPC TypeScript Runtime

> **Note:** This directory is a backward-compat alias for the canonical TypeScript runtime
> at `pkg/runtime/runtimes/ts/`. The two trees are identical; `ts-node` is kept for
> tooling that references the legacy name.

## Overview

The TypeScript runtime provides:
- Type validation functions for all PulseRPC types
- RPC error handling (`RPCError` class)
- Type helper functions for working with structs and enums

## Type Mappings

PulseRPC IDL types map to TypeScript as follows:

| PulseRPC Type | TypeScript Type |
|----------------|-----------------|
| `string`       | `string`        |
| `int`          | `number`        |
| `float`        | `number`        |
| `bool`         | `boolean`       |
| `[]Type`       | `Type[]`        |
| `map[string]Type` | `Record<string, Type>` or `{ [key: string]: Type }` |
| User-defined struct | TypeScript interface/type |
| User-defined enum | String literal union type |
| Optional types | `Type | null` |

### Int Validation

When validating `int` types, the runtime ensures the value has no fractional component. Values like `5.0` pass (effectively an integer), but `5.1` fails validation.

## API

### RPCError

```typescript
class RPCError extends Error {
  code: number;
  message: string;
  data?: any;

  constructor(code: number, message: string, data?: any);
}
```

### Validation Functions

All validation functions return `ValidationError[]`. Each error has `{ path: string, message: string }`.

- `validateType(value: any, typeDef: TypeDef, allStructs: StructMap, allEnums: EnumMap, isOptional?: boolean, path?: string): ValidationError[]`
- `validateString(value: any, path?: string): ValidationError[]`
- `validateInt(value: any, path?: string): ValidationError[]`
- `validateFloat(value: any, path?: string): ValidationError[]`
- `validateBool(value: any, path?: string): ValidationError[]`
- `validateArray(value: any, elementValidator: (v: any, p: string) => ValidationError[], path?: string): ValidationError[]`
- `validateMap(value: any, valueValidator: (v: any, p: string) => ValidationError[], path?: string): ValidationError[]`
- `validateEnum(value: any, enumName: string, allowedValues: string[], path?: string): ValidationError[]`
- `validateStruct(value: any, structName: string, structDef: StructDef, allStructs: StructMap, allEnums: EnumMap, path?: string): ValidationError[]`

### Contract API

- `Contract.fromFile(path: string): Contract` — load IDL from JSON file
- `contract.validate(typeName: string, value: any): ValidationResult` — manually validate data against a named type
- `contract.validateRequest(ifaceName: string, funcName: string, params: any[]): void` — throws `RPCError` on invalid params
- `contract.validateResponse(ifaceName: string, funcName: string, result: any): void` — throws `RPCError` on invalid response

### ValidationResult

| Field | Type | Description |
|-------|------|-------------|
| `valid` | `boolean` | `true` if the value is valid |
| `error` | `string \| undefined` | Human-readable error summary |
| `invalidFields` | `string[] \| undefined` | Path selectors for each invalid field |

### Type Helper Functions

- `findStruct(structName: string, allStructs: StructMap): StructDef | undefined`
- `findEnum(enumName: string, allEnums: EnumMap): EnumDef | undefined`
- `getStructFields(structName: string, allStructs: StructMap): FieldDef[]` (handles inheritance)

## Usage

The runtime library is automatically included when you generate TypeScript code from an IDL using the `ts-client-server` plugin:

```bash
pulserpc -plugin ts-client-server -dir output examples/book.pulse
```

This generates:
- `idl.ts` - IDL-specific type definitions
- `server.ts` - HTTP server with interface stubs
- `client.ts` - Client classes with transport abstraction
- `pulserpc/` - Runtime library (copied from this directory)

## Testing

Run tests using:

```bash
make test
```

This uses Docker to run tests in a consistent Node.js 18+ environment if Docker is available, otherwise uses local Node.js.

## Requirements

- Node.js 18+ (uses native `fetch` API)
- TypeScript (for type checking, though generated code is plain JavaScript-compatible)

## Module System

The runtime itself is ESM but supports dual ESM+CJS distribution (via tsup) so generated
code works in any module system: Node ESM, bundler ESM, or CommonJS.

