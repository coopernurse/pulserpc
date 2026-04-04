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

PulseRPC automatically validates:
- Required fields are present
- Types match IDL definition
- Enum values are valid

```typescript
// This will throw RPCException (-32602) if validation fails
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
