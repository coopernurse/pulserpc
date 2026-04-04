---
title: Python Quickstart
layout: default
---

# Python Quickstart

Build a complete PulseRPC service in Python with our e-commerce checkout example.

## Prerequisites

- Python 3.8 or later
- PulseRPC CLI installed ([Installation Guide](../../get-started/installation))

## 1. Define the Service (2 min)

Create `checkout.pulse` with your service definition:

{% include quickstart/checkout.idl %}

This IDL defines:
- **3 interfaces**: CatalogService, CartService, OrderService
- **7 structs**: Product, CartItem, Cart, Address, Order, AddToCartRequest, CreateOrderRequest, CheckoutResponse
- **2 enums**: OrderStatus, PaymentMethod
- **5 errors** (1001-1005): CartNotFound, CartEmpty, PaymentFailed, OutOfStock, InvalidAddress

## Defining Service Errors

The IDL uses the `errors` keyword to declare error codes and `raises()` clauses to specify which errors each method can raise:

```idl
errors {
    1001 CartNotFound "Cart doesn't exist"
    1002 CartEmpty "Cart has no items"
    1003 PaymentFailed "Payment method rejected"
    1004 OutOfStock "Insufficient inventory"
    1005 InvalidAddress "Shipping address validation failed"
}

interface OrderService {
    createOrder(request CreateOrderRequest) CheckoutResponse raises(CartNotFound, CartEmpty, PaymentFailed, OutOfStock, InvalidAddress)
}
```

For details on error handling, see [Error Handling](../../idl-guide/errors.html).

## 2. Generate Code (1 min)

Generate the Python code from your IDL:

```bash
pulserpc -plugin python-client-server checkout.pulse
```

This creates:
- `checkout.py` - IDL metadata and helpers (structs are dicts, enums are strings)
- `server.py` - PulseRPCServer framework with abstract service classes
- `client.py` - HTTPTransport and service client classes
- `pulserpc/` - Runtime library (RPCError, validation, types)

For multi-namespace projects, add `-dir` and `-package` so each namespace is generated into its own package directory:

```bash
pulserpc -plugin python-client-server -dir ./generated -package myapp.lib.rpc checkout.pulse
```

This creates a proper Python package structure:

```
generated/
├── myapp/
│   └── lib/
│       └── rpc/
│           ├── __init__.py              # Imports from pulserpc runtime
│           ├── pulserpc/                # Runtime library
│           │   ├── __init__.py
│           │   ├── rpc.py
│           │   ├── server.py
│           │   └── ...
│           ├── catalog/                  # Namespace: catalog
│           │   ├── __init__.py
│           │   ├── types.py
│           │   ├── server.py
│           │   └── client.py
│           └── orders/                  # Namespace: orders
│               ├── __init__.py
│               ├── types.py
│               ├── server.py
│               └── client.py
└── idl.json
```

Import packages in your code:

```python
from myapp.lib.rpc.pulserpc import RPCError, Server, HttpTransport
from myapp.lib.rpc.catalog.client import CatalogServiceClient
from myapp.lib.rpc.orders.client import OrderServiceClient
```

The Python `Client` auto-discovers interfaces via the `pulserpc-idl` RPC method, so you can also use:

```python
from myapp.lib.rpc.pulserpc import Client, HttpTransport

transport = HttpTransport("http://localhost:8080")
client = Client(transport)  # Auto-discovers all interfaces

# Works for any interface
client.CatalogService.listProducts()
client.OrderService.createOrder({...})
```

The IDL is embedded directly in `server.py` for the `pulserpc-idl` RPC method.

Note: the Python generator only creates classes for interfaces (service stubs). Structs are plain dicts and enums are strings, so use maps and lists directly in your handlers and client code.

## 3. Implement the Server (10-15 min)

Create a file `my_server.py` that implements your service handlers:

{% include quickstart/python-server.md %}

Start your server:

```bash
python3 my_server.py
```

Server runs on http://localhost:8080

## 4. Implement the Client (5-10 min)

Create `my_client.py` to call your service:

{% include quickstart/python-client.md %}

Run your client:

```bash
python3 my_client.py
```

## 5. Expected Output

```
=== Products ===
Wireless Mouse - $29.99
Mechanical Keyboard - $89.99

=== Creating Cart ===
Cart: cart_XXXX, Subtotal: $59.98

=== Creating Order ===
✓ Order created: order_XXXXX

=== Testing Error Case ===
✓ Got expected error: 1002 - CartEmpty: Cannot create order from empty cart
```

## Error Codes

Your service implements these custom error codes:

| Code | Name | When Returned |
|------|------|---------------|
| 1001 | CartNotFound | Cart doesn't exist |
| 1002 | CartEmpty | Cart has no items |
| 1003 | PaymentFailed | Payment method rejected |
| 1004 | OutOfStock | Insufficient inventory |
| 1005 | InvalidAddress | Address validation failed |

Raise errors with `RPCError(code, message)`:

```python
raise RPCError(1002, "CartEmpty: Cannot create order from empty cart")
```

## Next Steps

- [Python Reference](reference.html) - Type mappings, patterns, best practices
- [IDL Syntax](../../idl-guide/syntax.html) - Full IDL language reference
- [Validation](../../idl-guide/validation.html) - How runtime validation works
