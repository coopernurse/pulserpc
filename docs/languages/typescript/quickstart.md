---
title: TypeScript Quickstart
layout: default
---

# TypeScript Quickstart

Build a complete PulseRPC service in TypeScript with our e-commerce checkout example.

## Prerequisites

- Node.js 18 or later
- TypeScript 5.0 or later
- PulseRPC CLI installed ([Installation Guide](../../get-started/installation))

## 1. Define the Service (2 min)

Create `checkout.pulse` with your service definition:

{% include quickstart/checkout.idl %}

## 2. Generate Code (1 min)

Generate the TypeScript code from your IDL:

```bash
pulserpc -plugin ts-client-server checkout.pulse
```

This creates:
- `checkout.ts` - Type definitions
- `server.ts` - PulseRPC server framework
- `client.ts` - HTTP client framework
- `pulserpc/` - Runtime library

The IDL is embedded directly in `server.ts` for the `pulserpc-idl` RPC method.

## 3. Implement the Server (10-15 min)

Create `my_server.ts` that implements your service handlers:

{% include quickstart/ts-server.md %}

Create a `package.json` file in the same directory:

```json
{
  "name": "checkout-service",
  "version": "1.0.0",
  "type": "commonjs",
  "scripts": {
    "build": "tsc",
    "start": "node dist/my_server.js"
  },
  "dependencies": {
    "pulserpc-ts-runtime": "file:./pulserpc"
  },
  "devDependencies": {
    "@types/node": "^18.0.0",
    "typescript": "^5.0.0"
  }
}
```

Create a `tsconfig.json` file in the same directory:

```json
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "CommonJS",
    "lib": ["ES2020"],
    "types": ["node"],
    "moduleResolution": "node",
    "esModuleInterop": true,
    "skipLibCheck": true,
    "strict": false,
    "resolveJsonModule": true,
    "outDir": "./dist",
    "rootDir": "."
  },
  "include": ["*.ts"],
  "exclude": ["node_modules", "dist"]
}
```

Build and start your server:

```bash
npm install
npm run build
npm start
```

## 4. Implement the Client (5-10 min)

Create `my_client.ts` to call your service:

{% include quickstart/ts-client.md %}

Build and run your client:

```bash
npm run build
node dist/my_client.js
```

## Error Codes

Throw `RPCError` with custom error codes:

```typescript
throw new RPCError(1002, 'CartEmpty: Cannot create order from empty cart');
```

| Code | Name |
|------|------|
| 1001 | CartNotFound |
| 1002 | CartEmpty |
| 1003 | PaymentFailed |
| 1004 | OutOfStock |
| 1005 | InvalidAddress |

## Next Steps

- [TypeScript Reference](reference.html) - Type mappings and async patterns
- [IDL Syntax](../../idl-guide/syntax.html) - Full IDL reference
