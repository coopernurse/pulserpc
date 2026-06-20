# PulseRPC TypeScript Runtime

This directory contains the TypeScript runtime library for PulseRPC-generated code.

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

- `validateType(value: any, typeDef: TypeDef, allStructs: StructMap, allEnums: EnumMap, isOptional?: boolean): void`
- `validateString(value: any): void`
- `validateInt(value: any): void`
- `validateFloat(value: any): void`
- `validateBool(value: any): void`
- `validateArray(value: any, elementValidator: (v: any) => void): void`
- `validateMap(value: any, valueValidator: (v: any) => void): void`
- `validateEnum(value: any, enumName: string, allowedValues: string[]): void`
- `validateStruct(value: any, structName: string, structDef: StructDef, allStructs: StructMap, allEnums: EnumMap): void`

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

Generated code uses CommonJS (`module.exports`) for maximum Node.js compatibility.

