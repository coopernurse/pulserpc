---
title: TypeScript Reference
layout: default
---

# TypeScript Reference

## Type Mappings

| IDL Type | TypeScript Type | Example |
|----------|-----------------|---------|
| `string` | `string` | `"hello"` |
| `int` | `number` | `42` |
| `float` | `number` | `3.14` |
| `bool` | `boolean` | `true`, `false` |
| `[]Type` | `Type[]` | `[1, 2, 3]` |
| `map[string]Type` | `{[key: string]: Type}` | `{"key": "value"}` |
| `Enum` | String union type | `"pending" \| "paid"` |
| `Struct` | Class | `new Product({...})` |
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

## Generated Classes

Each struct in your IDL becomes a TypeScript class:

```typescript
import * as checkout from './checkout';

// Create instances
const product = new checkout.Product({
  productId: 'prod001',
  name: 'Wireless Mouse',
  description: 'Ergonomic mouse',
  price: 29.99,
  stock: 50,
  imageUrl: 'https://example.com/mouse.jpg'  // optional field
});

const cart = new checkout.Cart({
  cartId: 'cart_1234',
  items: [],
  subtotal: 0
});
```

## Optional Fields

Optional fields can be `undefined`:

```typescript
// Create with optional field
const product = new checkout.Product({
  productId: 'prod001',
  name: 'Wireless Mouse',
  description: 'Ergonomic mouse',
  price: 29.99,
  stock: 50,
  imageUrl: undefined  // optional field can be undefined
});

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
const order = new checkout.Order({
  orderId: 'order_123',
  cart: cart,
  shippingAddress: address,
  paymentMethod: checkout.PaymentMethod.creditCard,
  status: checkout.OrderStatus.pending,
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

Extend generated service classes:

```typescript
import { PulseRPCServer, CatalogService } from './server';
import * as checkout from './checkout';

class CatalogServiceImpl extends CatalogService {
  private products: checkout.Product[] = [
    new checkout.Product({ productId: 'p1', name: 'Item 1', price: 10.0, stock: 5 }),
    new checkout.Product({ productId: 'p2', name: 'Item 2', price: 20.0, stock: 3 })
  ];

  listProducts(): checkout.Product[] {
    return this.products;
  }

  getProduct(productId: string): checkout.Product | null {
    for (const p of this.products) {
      if (p.productId === productId) {
        return p;
      }
    }
    return null;  // Return null for optional type
  }
}

// Start server
const server = new PulseRPCServer(8080);
server.registerCatalogService(new CatalogServiceImpl());
server.start();
```

## Client Usage

```typescript
import { HTTPTransport } from './client';
import * as checkout from './checkout';
import { CatalogServiceClient } from './checkout';

const transport = new HTTPTransport('http://localhost:8080');
const catalog = new CatalogServiceClient(transport);

// Method calls return TypeScript objects
const products: checkout.Product[] = catalog.listProducts();
for (const p of products) {
  console.log(`${p.name}: $${p.price}`);
}

// Optional methods return null if not found
const product: checkout.Product | null = catalog.getProduct('prod001');
if (product !== null) {
  console.log(product.name);
}
```

## Async/Await Pattern

PulseRPC TypeScript can use async/await:

```typescript
class OrderServiceImpl extends OrderService {
  async createOrder(request: checkout.CreateOrderRequest): Promise<checkout.CheckoutResponse> {
    // Async operations
    const orderId = await this.generateOrderId();
    const validated = await this.validateCart(request.cartId);

    if (!validated) {
      throw new RPCException(1002, 'CartEmpty: Cannot create order from empty cart');
    }

    return new checkout.CheckoutResponse({ orderId });
  }
}
```

## Validation

### Automatic Validation

PulseRPC automatically validates requests and responses against your IDL:

```typescript
// This will throw RPCException (-32602) if validation fails
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
5. **Error boundaries**: Catch RPCException at appropriate levels

## Working with Nested Structs

```typescript
// Nested structs work naturally
const order = new checkout.Order({
  orderId: 'order_123',
  cart: new checkout.Cart({
    cartId: 'cart_123',
    items: [new checkout.CartItem({...})],
    subtotal: 59.98
  }),
  shippingAddress: new checkout.Address({
    street: '123 Main St',
    city: 'San Francisco',
    state: 'CA',
    zipCode: '94105',
    country: 'USA'
  }),
  paymentMethod: checkout.PaymentMethod.creditCard,
  status: checkout.OrderStatus.pending,
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
import { PulseRPCServer } from './server';

const app = express();
app.use(express.json());

const pulserpc = new PulseRPCServer(8080);

// Mount PulseRPC server on Express
app.use('/rpc', (req, res) => {
  // Forward Express requests to PulseRPC
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
