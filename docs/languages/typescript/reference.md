---
title: TypeScript Reference
layout: default
---

# TypeScript Reference

## Type Mappings

| IDL Type | TypeScript Type | Example |
|----------|-----------------|---------|
| `string` | `string` | `"hello"` |
| `int` | `number` (signed 64-bit on the wire; safe-integer range validated) | `42` |
| `float` | `number` (IEEE-754 double) | `3.14` |
| `bool` | `boolean` | `true`, `false` |
| `[]Type` | `Type[]` | `[1, 2, 3]` |
| `map[string]Type` | `{[key: string]: Type}` | `{"key": "value"}` |
| `Enum` | Generated `enum` with string values | `"pending" \| "paid"` |
| `Struct` | Interface | `{ ... }` (plain object) |
| `T [optional]` | `T \| undefined` | `string \| undefined` |

## Module Styles

PulseRPC generates TypeScript code in three module styles, controlled by the `-ts-module` CLI flag:

| Style | Aliases | Default | Description | Import Format |
|-------|---------|---------|-------------|---------------|
| `esm-node` | `esm`, `node` | default | Node-flavored ESM with explicit `.js` import suffixes | `import { X } from './foo.js'` |
| `esm-bundler` | `bundler` | | ESM without `.js` suffixes (for Vite, webpack, Next.js) | `import { X } from './foo'` |
| `cjs` | `commonjs` | | CommonJS with `require()` and `module.exports` | `const { X } = require('./foo')` |

### Auto-Detection

When `-ts-module` is not set, PulseRPC resolves the style using this precedence:

1. **Explicit `-ts-module` flag** — always wins if set.
2. **`tsconfig.json` `compilerOptions.module`** — walks up from the output directory (up to 10 levels). Recognized values map as: `Node16`, `NodeNext`, `ES2022`, `ES2020`, `ESNext` → `esm-node`; `Bundler`, `Preserve` → `esm-bundler`; `CommonJS`, `Node10` → `cjs`.
3. **`package.json` `type` field** — walks up from the output directory (up to 10 levels). `"module"` or absent → `esm-node`; `"commonjs"` → `cjs`.
4. **Default** — `esm-node`.

Use `-ts-no-detect` to skip steps 2–3 and always use the default (or the explicit flag if set).

When the explicit flag disagrees with a detected value, a warning is printed to stderr but generation proceeds with the explicit value.

### Generated Config Files

Use `-ts-gen-package-json` and `-ts-gen-tsconfig` to generate matching project config files at `-dir`. These flags **error** if a `package.json` or `tsconfig.json` already exists at that location.

Each style produces matching compiler settings:

| Style | `package.json` `type` | `tsconfig.json` `module` | `tsconfig.json` `moduleResolution` |
|-------|----------------------|--------------------------|------------------------------------|
| `esm-node` | `"module"` | `NodeNext` | `NodeNext` |
| `esm-bundler` | `"module"` | `Bundler` | `Bundler` |
| `cjs` | `"commonjs"` | `CommonJS` | `Node10` |

### When To Use Each Style

| Style | Best for |
|-------|----------|
| `esm-node` | Standard Node.js projects using `"type": "module"` or `.mjs` files. Requires Node 16+. |
| `esm-bundler` | Projects built with Vite, webpack, Next.js, or any bundler that resolves imports without `.js` suffixes. |
| `cjs` | Node.js projects using `"type": "commonjs"`, legacy setups, or projects not yet migrated to ESM. |

### Examples

```bash
# Auto-detect from project files, or use esm-node (default)
pulserpc -plugin ts-client-server -dir ./src api/service.pulse

# Explicit bundler style with generated config files
pulserpc -plugin ts-client-server -dir ./src -ts-module=esm-bundler -ts-gen-package-json -ts-gen-tsconfig api/service.pulse

# CommonJS
pulserpc -plugin ts-client-server -dir ./src -ts-module=cjs api/service.pulse

# CommonJS with generated config files, no auto-detection
pulserpc -plugin ts-client-server -dir ./src -ts-module=cjs -ts-gen-package-json -ts-gen-tsconfig -ts-no-detect api/service.pulse
```

## Generated Interfaces

Each struct in your IDL becomes a TypeScript `interface`; construct plain objects:

```typescript
import * as checkout from './checkout';

// Create plain objects typed as the generated interface
const product: checkout.Product = {
  productId: 'prod001',
  name: 'Wireless Mouse',
  description: 'Ergonomic mouse',
  price: 29.99,
  stock: 50,
  imageUrl: 'https://example.com/mouse.jpg'  // optional field
};

const cart: checkout.Cart = {
  cartId: 'cart_1234',
  items: [],
  subtotal: 0
};
```

## Optional Fields

Optional fields can be `undefined`:

```typescript
// Create with optional field
const product: checkout.Product = {
  productId: 'prod001',
  name: 'Wireless Mouse',
  description: 'Ergonomic mouse',
  price: 29.99,
  stock: 50,
  imageUrl: undefined  // optional field can be undefined
};

// Check optional field
if (product.imageUrl !== undefined) {
  console.log(product.imageUrl);
}
```

## Enums

Enums are string types at runtime but have type safety:

```typescript
import * as checkout from './checkout';

// Use enum values
const order: checkout.Order = {
  orderId: 'order_123',
  cart: cart,
  shippingAddress: address,
  paymentMethod: 'credit_card',
  status: 'pending',
  total: 59.98,
  createdAt: Date.now()
});

// Compare enums
if (order.status === checkout.OrderStatus.pending) {
  console.log('Order is pending');
}
```

## Error Handling

Throw `RPCError` with custom codes:

```typescript
import { RPCError } from './pulserpc/rpc';

// Standard JSON-RPC errors
throw new RPCError(-32602, 'Invalid params');

// Custom application errors (use codes >= 1000)
throw new RPCError(1001, 'CartNotFound: Cart does not exist');
throw new RPCError(1002, 'CartEmpty: Cannot create order from empty cart');
```

Common error codes:
- `-32700`: Parse error
- `-32600`: Invalid request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error
- `1000+`: Custom application errors

## Server Implementation

The generator emits `types.ts` (interfaces/enums) and `server.ts` (one abstract
class per interface). Implement the abstract methods, passing `ctx` as the first
argument; every method returns a `Promise`.

```typescript
import { readFileSync } from 'fs';
import { Server, Contract } from './pulserpc/index.js';
import { CatalogService } from './checkout/server.js';
import type { Product } from './checkout/types.js';

class CatalogServiceImpl extends CatalogService {
  private products: Product[] = [
    { productId: 'p1', name: 'Item 1', price: 10.0, stock: 5 },
    { productId: 'p2', name: 'Item 2', price: 20.0, stock: 3 },
  ];

  async listProducts(ctx: Record<string, any>): Promise<Product[]> {
    return this.products;
  }

  async getProduct(ctx: Record<string, any>, productId: string): Promise<Product | null> {
    return this.products.find((p) => p.productId === productId) ?? null;
  }
}

// Wire the implementations into the runtime Server
const contract = new Contract(JSON.parse(readFileSync('./checkout/idl.json', 'utf-8')));
const rpcServer = new Server({ contract, validateRequests: true, validateResponses: true });
rpcServer.addHandler('CatalogService', new CatalogServiceImpl());

// rpcServer.call(request) returns the JSON-RPC response (or null for a notification).
// Serve it from your HTTP framework of choice (see the quickstart for a Node adapter).
```

Generated `server.ts` does **not** start an HTTP listener; you supply the HTTP
adapter and call `rpcServer.call(...)`.

## Client Usage

The generated `client.ts` exports one `*Client` class per interface plus the
`Transport`/`HttpTransport` types. Methods are async and do not take `ctx`.

```typescript
import { HttpTransport, CatalogServiceClient } from './checkout/client.js';
import type { Product } from './checkout/types.js';

const transport = new HttpTransport('http://localhost:8080');
const catalog = new CatalogServiceClient(transport);

const products: Product[] = await catalog.listProducts();
for (const p of products) {
  console.log(`${p.name}: $${p.price}`);
}

const product: Product | null = await catalog.getProduct('prod001');
if (product !== null) {
  console.log(product.name);
}
```

## Async/Await Pattern

All generated service methods are `async` and return `Promise`s, so use
`async`/`await` throughout:

```typescript
class OrderServiceImpl extends OrderService {
  async createOrder(ctx: Record<string, any>, request: checkout.CreateOrderRequest): Promise<checkout.CheckoutResponse> {
    const orderId = await this.generateOrderId();
    const validated = await this.validateCart(request.cartId);

    if (!validated) {
      throw new RPCError(1002, 'CartEmpty: Cannot create order from empty cart');
    }

    return { orderId };
  }
}
```

## Validation

### Automatic Validation

PulseRPC automatically validates requests and responses against your IDL:

```typescript
// This will throw RPCError (-32602) if validation fails
const cart = cart.addToCart({
  cartId: null,
  productId: 'prod001',
  quantity: 2
});
```

Validation checks:
- Required fields are present
- Types match IDL definition
- Enum values are valid
- Optional fields allow `null`

### Manual Validation

Validate arbitrary data against any named type using the `Contract` class:

```typescript
import { Contract } from './pulserpc/contract';

// Load IDL from file or use parsed JSON
const contract = Contract.fromFile('idl.json');

// Validate a struct — returns ValidationResult
const result = contract.validate('Person', {
  username: 'alice',
  age: 30,
  email: 'alice@example.com'
});

if (result.valid) {
  process(data);
} else {
  console.log(result.error);           // Error summary with all failures
  console.log(result.invalidFields);   // [".age", ".email"] — field selectors
}
```

### ValidationResult Reference

| Field | Type | Description |
|-------|------|-------------|
| `valid` | `boolean` | `true` if the value is valid |
| `error` | `string \| undefined` | Human-readable error summary (`undefined` if valid) |
| `invalidFields` | `string[] \| undefined` | Path selectors for each invalid field (`undefined` if valid) |

### ValidationError Reference

Each collected error has:

| Field | Type | Description |
|-------|------|-------------|
| `path` | `string` | Dot-separated path to the invalid value (e.g. `.address.city`, `.items[1].quantity`) |
| `message` | `string` | Human-readable error description |

### Examples

**Validating form input:**

```typescript
const result = contract.validate('SignupRequest', {
  username,
  email,
  age
});
if (!result.valid) {
  // Map errors back to form fields
  result.invalidFields?.forEach(field => {
    form.addError(field.replace(/^\./, ''), result.error);
  });
}
```

**Validating an enum value:**

```typescript
const result = contract.validate('Color', 'red');     // result.valid == true
const result = contract.validate('Color', 'yellow');   // result.valid == false
```

**Validating nested data with path tracking:**

```typescript
const result = contract.validate('Order', {
  orderId: 'ord_123',
  items: [
    { productId: 'p1', quantity: 2 },
    { productId: 'p2', quantity: -1 }
  ]
});
// result.invalidFields == [".items[1].quantity"]
```

### Loading a Contract

```typescript
// From an idl.json file
const contract = Contract.fromFile('path/to/idl.json');

// From parsed JSON
import * as fs from 'fs';
const json = JSON.parse(fs.readFileSync('idl.json', 'utf-8'));
const contract = new Contract(json);
```

## Type Safety

Generated code provides full TypeScript types:

```typescript
// Full type checking
const products: checkout.Product[] = catalog.listProducts();

// Type errors caught at compile time
products.forEach((p: checkout.Product) => {
  console.log(p.name);  // OK
  console.log(p.unknownField);  // Compile error
});

// Function signatures match IDL
cart.addToCart(request: checkout.AddToCartRequest): checkout.Cart
cart.getCart(cartId: string): checkout.Cart | null
cart.clearCart(cartId: string): boolean
```

## Best Practices

1. **Use strict mode**: Enable `strict: true` in `tsconfig.json`
2. **Type assertions**: Avoid `as`, use proper type guards
3. **Null checks**: Always check for `null` on optional returns
4. **Async patterns**: Use async/await for I/O operations
5. **Error boundaries**: Catch RPCError at appropriate levels

## Working with Nested Structs

```typescript
// Nested structs are plain objects
const order: checkout.Order = {
  orderId: 'order_123',
  cart: {
    cartId: 'cart_123',
    items: [{ ... }],
    subtotal: 59.98
  },
  shippingAddress: {
    street: '123 Main St',
    city: 'San Francisco',
    state: 'CA',
    zipCode: '94105',
    country: 'USA'
  },
  paymentMethod: 'credit_card',
  status: 'pending',
  total: 59.98,
  createdAt: Math.floor(Date.now() / 1000)
});
```

## Build Integration

Add to `package.json`:

```json
{
  "scripts": {
    "build": "tsc",
    "start": "node dist/server.js",
    "dev": "tsc && node dist/server.js"
  },
  "devDependencies": {
    "typescript": "^5.0.0",
    "@types/node": "^20.0.0"
  }
}
```

## Using with Express

```typescript
import express from 'express';
import { readFileSync } from 'fs';
import { Server, Contract } from './pulserpc/index.js';
import { CatalogService } from './checkout/server.js';
import { CatalogServiceImpl } from './catalog-service-impl.js';

const contract = new Contract(JSON.parse(readFileSync('./checkout/idl.json', 'utf-8')));
const rpcServer = new Server({ contract, validateRequests: true, validateResponses: true });
rpcServer.addHandler('CatalogService', new CatalogServiceImpl());

const app = express();
app.use(express.json());

app.post('/rpc', async (req, res) => {
  const response = await rpcServer.call(req.body);
  if (response === null) {
    res.status(204).end();
    return;
  }
  res.json(response);
});

app.listen(3000);
```

## Using with Node.js Native Modules

```typescript
// Async file operations
import { promises as fs } from 'fs';

class ProductServiceImpl extends ProductService {
  async loadProducts(): Promise<checkout.Product[]> {
    const data = await fs.readFile('products.json', 'utf-8');
    return JSON.parse(data);
  }
}
```

## Wire Behavior and Limits

These rules apply identically to `ts-node` (ESM) and `ts-cjs` (CommonJS)
runtimes and to the generated test server.

### Integer range (D1)

IDL `int` is a **signed 64-bit** integer on the wire:
`-9223372036854775808` through `9223372036854775807`.

TypeScript types both `int` and `float` as `number`, and JavaScript numbers are
IEEE-754 doubles that cannot represent every 64-bit integer exactly. Rather than
silently rounding, both TypeScript runtimes accept only integers in JavaScript's
safe-integer range, `[-9007199254740991, 9007199254740991]` (±(2^53 − 1),
`Number.MIN_SAFE_INTEGER`..`Number.MAX_SAFE_INTEGER`). Larger integral values are
rejected with `-32602` and a message explaining that JavaScript cannot represent
the full int64 range exactly. The bounds are exported as `INT64_SAFE_MIN` and
`INT64_SAFE_MAX`. The old `INT32_MIN`/`INT32_MAX` exports remain as deprecated
aliases so existing imports keep compiling.

This is a TypeScript/JavaScript runtime limitation, not a change to the wire
type: Go, Java, C#, and Python accept the full signed 64-bit range, so keep
`int` values within ±(2^53 − 1) when a TypeScript client or server is involved.
Fractional values, `NaN`, `Infinity`, and booleans are also rejected with
`-32602`. This is a breaking change from earlier releases, which used a signed
32-bit range.

```typescript
validateInt(2 ** 31);              // []      valid (previously rejected as int32)
validateInt(2 ** 32);              // []      valid
validateInt(2 ** 53 - 1);          // []      largest exactly representable integer
validateInt(2 ** 53);              // [error] outside the safe-integer range
validateInt(2 ** 63 - 1);          // [error] int64 max; not exact as a number
```

`float` remains an IEEE-754 double (`number`); enums are their declared
string names on the wire, never ordinals.

### JSON-RPC ids (D2/R24)

`id` must be a string, a number, or `null`. A boolean, object, or array id is
rejected with `-32600` and the response `id` is `null`. A request with
`id: null` (or no `id` member) is a notification: the handler runs but no
response is written.

### Non-POST requests (D3/R22)

The generated test server accepts only `POST`. Any other method returns HTTP
**405** with a JSON-RPC body:

```json
{"jsonrpc":"2.0","error":{"code":-32600,"message":"Invalid Request - only POST allowed"},"id":null}
```

### Internal errors (D4/R15)

Handler exceptions and response-validation failures are logged server-side
(`console.error`) and returned as:

```json
{"jsonrpc":"2.0","error":{"code":-32603,"message":"Internal error"},"id":1}
```

No exception text, stack trace, or validation detail is sent to the client.
Declared `RPCError` values keep their code, message, and `data`; `-32602`
request-validation messages and data are also user-facing and preserved.
`error.data` is omitted entirely when there is no data.

### Resource limits (D5/R16-R18)

| Limit | Value | Behaviour |
|-------|-------|-----------|
| Request body | 1 MiB | `-32600 "Request body too large"`; the adapter stops buffering |
| Batch entries | 100 | `-32600 "Batch exceeds maximum size"`; no entries are dispatched |
| Response body (client) | 10 MiB | `HttpTransport` rejects with `Response body too large: ...` |
| HTTP timeout (client) | 30 s | `HttpTransport` aborts via `AbortController` |

Constants are exported by the runtime: `HTTP_TIMEOUT_MS` and
`MAX_RESPONSE_BYTES`. `HttpTransport` also accepts custom headers and
normalizes a single trailing `/` from the base URL.

An empty batch returns one `-32600` error object, not an array. Non-object
batch entries produce a per-entry `-32600` inside the response array. A batch
containing only notifications returns HTTP 204 with no body.

### Optional values (D7)

TypeScript represents optional values as `T | undefined` in generated types.
On the wire, an optional field may be omitted or `null`; required fields may
not be `null`. Request validation accepts trailing optional parameters being
omitted from either positional or named-parameter requests.

### Deterministic output (D12)

Generation is byte-deterministic: namespaces, imports, runtime files, and
type registrations are emitted in sorted order, so regenerating the same IDL
produces identical bytes. The generator test suite compares full output trees
across runs.
