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

```idl
namespace checkout

// Enums for order status and payment methods

enum OrderStatus {
    pending
    paid
    shipped
    delivered
    cancelled
}

enum PaymentMethod {
    credit_card
    debit_card
    paypal
    apple_pay
}

// Core domain entities

struct Product {
    productId    string
    name         string
    description  string
    price        float
    stock        int
    imageUrl     string  [optional]
}

struct CartItem {
    productId    string
    quantity     int
    price        float
}

struct Cart {
    cartId       string
    items        []CartItem
    subtotal     float
}

struct Address {
    street       string
    city         string
    state        string
    zipCode      string
    country      string
}

struct Order {
    orderId           string
    cart              Cart
    shippingAddress   Address
    paymentMethod     PaymentMethod
    status            OrderStatus
    total             float
    createdAt         int
}

// Request/Response structures

struct AddToCartRequest {
    cartId       string  [optional]
    productId    string
    quantity     int
}

struct CreateOrderRequest {
    cartId              string
    shippingAddress     Address
    paymentMethod       PaymentMethod
}

struct CheckoutResponse {
    orderId      string
    message      string  [optional]
}

// Error Codes for createOrder:
//   1001 - CartNotFound: Cart doesn't exist
//   1002 - CartEmpty: Cart has no items
//   1003 - PaymentFailed: Payment method rejected
//   1004 - OutOfStock: Insufficient inventory
//   1005 - InvalidAddress: Shipping address validation failed

// Service interfaces

interface CatalogService {
    // Returns a list of all available products
    listProducts() []Product

    // Returns details for a specific product, or null if not found
    getProduct(productId string) Product  [optional]
}

interface CartService {
    // Adds an item to the cart (creates cart if cartId not provided)
    addToCart(request AddToCartRequest) Cart

    // Returns the cart contents, or null if cart doesn't exist
    getCart(cartId string) Cart  [optional]

    // Removes all items from the cart, returns true if successful
    clearCart(cartId string) bool
}

interface OrderService {
    // Converts a cart to an order
    createOrder(request CreateOrderRequest) CheckoutResponse

    // Returns the order details, or null if order doesn't exist
    getOrder(orderId string) Order  [optional]
}
```

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
- `idl.json` - IDL metadata

## 3. Implement the Server (10-15 min)

Create `my_server.ts` that implements your service handlers:

```typescript
import { PulseRPCServer, CatalogService, CartService, OrderService } from './server';
import { RPCError } from './pulserpc/rpc';

const products = [
  {
    productId: 'prod001',
    name: 'Wireless Mouse',
    description: 'Ergonomic mouse',
    price: 29.99,
    stock: 50,
    imageUrl: 'https://example.com/mouse.jpg'
  },
  {
    productId: 'prod002',
    name: 'Mechanical Keyboard',
    description: 'RGB keyboard',
    price: 89.99,
    stock: 25,
    imageUrl: 'https://example.com/keyboard.jpg'
  }
];

const carts = new Map<string, any>();
const orders = new Map<string, any>();

class CatalogServiceImpl extends CatalogService {
  listProducts(): any[] {
    return products;
  }

  getProduct(productId: string): any | null {
    return products.find((p: any) => p.productId === productId) || null;
  }
}

class CartServiceImpl extends CartService {
  addToCart(request: any): any {
    let cartId = request.cartId || `cart_${Math.floor(Math.random() * 9000 + 1000)}`;

    let cart = carts.get(cartId);
    if (!cart) {
      cart = {
        cartId,
        items: [],
        subtotal: 0
      };
      carts.set(cartId, cart);
    }

    const product = products.find((p: any) => p.productId === request.productId);
    if (!product) {
      throw new RPCError(-32602, 'Product not found');
    }

    cart.items.push({
      productId: request.productId,
      quantity: request.quantity,
      price: product.price
    });

    cart.subtotal = cart.items.reduce((sum: number, item: any) => sum + item.price * item.quantity, 0);
    return cart;
  }

  getCart(cartId: string): any | null {
    return carts.get(cartId) || null;
  }

  clearCart(cartId: string): boolean {
    const cart = carts.get(cartId);
    if (cart) {
      cart.items = [];
      cart.subtotal = 0;
      return true;
    }
    return false;
  }
}

class OrderServiceImpl extends OrderService {
  createOrder(request: any): any {
    const cart = carts.get(request.cartId);
    if (!cart) {
      throw new RPCError(1001, 'CartNotFound: Cart does not exist');
    }

    if (!cart.items || cart.items.length === 0) {
      throw new RPCError(1002, 'CartEmpty: Cannot create order from empty cart');
    }

    const orderId = `order_${Math.floor(Math.random() * 90000 + 10000)}`;
    const order = {
      orderId,
      cart,
      shippingAddress: request.shippingAddress,
      paymentMethod: request.paymentMethod,
      status: 'pending',
      total: cart.subtotal,
      createdAt: Math.floor(Date.now() / 1000)
    };

    orders.set(orderId, order);
    return { orderId, message: 'Order created successfully' };
  }

  getOrder(orderId: string): any | null {
    return orders.get(orderId) || null;
  }
}

const server = new PulseRPCServer('0.0.0.0', 8080);
server.register('CatalogService', new CatalogServiceImpl());
server.register('CartService', new CartServiceImpl());
server.register('OrderService', new OrderServiceImpl());
server.serveForever();
```

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

```typescript
import { HTTPTransport, CatalogServiceClient, CartServiceClient, OrderServiceClient } from './client';

const transport = new HTTPTransport('http://localhost:8080');
const catalog = new CatalogServiceClient(transport);
const cart = new CartServiceClient(transport);
const orders = new OrderServiceClient(transport);

async function main() {
  const products = await catalog.listProducts();
  console.log('=== Products ===');
  for (const p of products) {
    console.log(`${p.name} - $${p.price}`);
  }

  const result = await cart.addToCart({
    cartId: null,
    productId: products[0].productId,
    quantity: 2
  });
  console.log(`\nCart: ${result.cartId}`);

  const response = await orders.createOrder({
    cartId: result.cartId,
    shippingAddress: {
      street: '123 Main St',
      city: 'San Francisco',
      state: 'CA',
      zipCode: '94105',
      country: 'USA'
    },
    paymentMethod: 'credit_card'
  });
  console.log(`✓ Order created: ${response.orderId}`);
}

main().catch(console.error);
```

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
