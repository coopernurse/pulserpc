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
import { Server, Contract } from './pulserpc';
import { CatalogService } from './server';
import * as checkout from './checkout';

// Load IDL and create Contract
const idlData = JSON.parse(readFileSync('idl.json', 'utf-8'));
const contract = new Contract(idlData);

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

// Create server and register handler
const server = new Server({ contract });
server.addHandler('CatalogService', new CatalogServiceImpl());

// Use with HTTP server
import * as http from 'http';
const httpServer = http.createServer((req, res) => {
  if (req.method === 'POST') {
    let body = '';
    req.on('data', (chunk) => { body += chunk.toString(); });
    req.on('end', () => {
      const response = server.call(JSON.parse(body));
      res.writeHead(200, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify(response));
    });
  } else {
    res.writeHead(405);
    res.end();
  }
});
httpServer.listen(8080);
```

## Client Usage

```typescript
import { HttpTransport } from './pulserpc/transport';
import { CatalogServiceClient } from './client';
import * as checkout from './checkout';

const transport = new HttpTransport('http://localhost:8080');
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
      throw new RPCError(1002, 'CartEmpty: Cannot create order from empty cart');
    }

    return new checkout.CheckoutResponse({ orderId });
  }
}
```

## Validation

PulseRPC automatically validates:
- Required fields are present
- Types match IDL definition
- Enum values are valid

```typescript
// This will throw RPCError (-32602) if validation fails
const cart = cart.addToCart({
  cartId: null,
  productId: 'prod001',
  quantity: 2
});
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
import { Server, Contract } from './pulserpc';
import { CatalogService } from './server';
import * as http from 'http';

const app = express();
app.use(express.json());

// Load IDL and create Contract
const idlData = JSON.parse(readFileSync('idl.json', 'utf-8'));
const contract = new Contract(idlData);

// Create PulseRPC server
const rpcServer = new Server({ contract });
rpcServer.addHandler('CatalogService', new CatalogServiceImpl());

// Mount PulseRPC server on Express
app.use('/rpc', express.json(), (req, res) => {
  const response = rpcServer.call(req.body);
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

## Multi-Namespace Projects

When your IDL defines multiple namespaces, use the `-package` flag to generate properly structured TypeScript modules:

```bash
pulserpc -plugin ts-client-server -dir ./generated -package '@mycompany/api' api/service.pulse
```

### Output Structure

```
generated/
├── common/                     # Namespace: common
│   ├── index.ts               # Re-exports from types, server, client
│   ├── types.ts
│   ├── server.ts
│   └── client.ts
├── orders/                    # Namespace: orders
│   ├── index.ts
│   ├── types.ts
│   ├── server.ts
│   └── client.ts
├── pulserpc/                  # Runtime library
│   ├── index.ts
│   ├── rpc.ts
│   ├── server.ts
│   ├── client.ts
│   ├── contract.ts
│   ├── transport.ts
│   └── validation.ts
└── idl.json                   # Must be at root, not in namespace subdirs
```

### index.ts Re-Exports

Each namespace directory contains an `index.ts` that re-exports the public API:

```typescript
// orders/index.ts
export * from './types';
export * from './server';
export * from './client';
```

This allows convenient imports from the namespace:

```typescript
import { OrderService, CreateOrderRequest } from './orders';
// vs
import { OrderService } from './orders/server';
import { CreateOrderRequest } from './orders/types';
```

### Cross-Namespace Imports

Import types from other namespaces using relative paths:

```typescript
// In orders/server.ts, import from common namespace
import * as common from '../common/types';

class OrderServiceImpl extends OrderService {
  processOrder(request: common.CreateOrderRequest): common.Order {
    // Use common types across namespaces
  }
}
```

### Contract and IDL Loading

The TypeScript runtime requires `idl.json` to be at the project root:

```typescript
import { promises as fs } from 'fs';
import { Contract } from './pulserpc/contract';

// Load IDL from file (must be at project root)
const idlData = JSON.parse(await fs.readFile('idl.json', 'utf-8'));
const contract = new Contract(idlData);
```

## Runtime Reference

### Client Async Initialization

The `Client` requires async initialization to fetch IDL from the server:

```typescript
import { Client, HttpTransport } from './pulserpc';
import { CatalogServiceClient } from './client';

const transport = new HttpTransport('http://localhost:8080');
const client = new Client(transport);

// Wait for client to be ready before making calls
await client.ready();

// Now interface proxies are available
const catalog = client.CatalogService;
const products = await catalog.listProducts();
```

The `Client.ready()` promise resolves when:
1. IDL has been fetched from the server via `pulserpc-idl`
2. Interface proxies have been created

### Server Validation Options

The `Server` constructor accepts validation options:

```typescript
import { Server, Contract } from './pulserpc';

const contract = new Contract(idlData);

// Enable request validation
const server = new Server({ 
  contract,
  validateRequests: true 
});

// Enable response validation
const server2 = new Server({ 
  contract,
  validateResponses: true 
});

// Enable both
const server3 = new Server({ 
  contract,
  validateRequests: true,
  validateResponses: true 
});
```

When enabled:
- `validateRequests`: Validates incoming params against IDL before calling handler
- `validateResponses`: Validates handler return values against IDL before sending response

### Contract Validation API

The `Contract` class provides manual validation:

```typescript
import { Contract } from './pulserpc';

const contract = new Contract(idlData);

// Validate request parameters
contract.validateRequest('CatalogService', 'listProducts', []);

// Validate response
contract.validateResponse('CatalogService', 'getProduct', { productId: 'p1', ... });

// Throws error if validation fails
try {
  contract.validateRequest('CatalogService', 'listProducts', ['too', 'many', 'params']);
} catch (e) {
  console.error('Validation failed:', e.message);
}
```

## IDL JSON Artifact

The generator creates an `idl.json` file containing the parsed IDL metadata. This file is required at runtime for:

1. **TypeScript `Contract`** - Parses it to get interface/struct/enum definitions
2. **Python `Client`** - Fetches it automatically via `pulserpc-idl` RPC

Deploy `idl.json` alongside your generated code. The TypeScript runtime expects it at the project root (where you run the script).

```
