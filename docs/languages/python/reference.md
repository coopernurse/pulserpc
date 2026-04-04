---
title: Python Reference
layout: default
---

# Python Reference

## Type Mappings

| IDL Type | Python Type | Example |
|----------|-------------|---------|
| `string` | `str` | `"hello"` |
| `int` | `int` | `42` |
| `float` | `float` | `3.14` |
| `bool` | `bool` | `True`, `False` |
| `[]Type` | `list` | `[1, 2, 3]` |
| `map[string]Type` | `dict` | `{"key": "value"}` |
| `Enum` | `str` | `"pending"` |
| `Struct` | `dict` | `{"field": "value"}` |
| `T [optional]` | `None` or type | `None` or value |

## Structs as Dictionaries

Each struct in your IDL becomes a dictionary in Python:

```python
from checkout import Product, Cart, CartItem

# Create instances using dicts
product = {
    "productId": "prod001",
    "name": "Wireless Mouse",
    "description": "Ergonomic mouse",
    "price": 29.99,
    "stock": 50,
    "imageUrl": "https://example.com/mouse.jpg"  # optional field
}

cart = {
    "cartId": "cart_1234",
    "items": [],
    "subtotal": 0.0
}
```

When generating multi-namespace output, import generated symbols from the namespace package instead of the flat output file:

```python
from myapp.lib.rpc.book import BookServiceClient
from myapp.lib.rpc.pulserpc import RPCError
```

## Optional Fields

Optional fields can be `None`:

```python
# Define with optional field
product = {
    "productId": "prod001",
    "name": "Wireless Mouse",
    "description": "Ergonomic mouse",
    "price": 29.99,
    "stock": 50,
    "imageUrl": None  # optional field can be None
}

# Check for optional field
if product.get("imageUrl"):
    print(product["imageUrl"])
```

## Error Handling

Throw `RPCError` with custom codes:

```python
from pulserpc import RPCError

# Standard JSON-RPC errors
throw RPCError(-32602, "Invalid params")

# Custom application errors (use codes >= 1000)
throw RPCError(1001, "CartNotFound: Cart does not exist")
throw RPCError(1002, "CartEmpty: Cannot create order from empty cart")
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

```python
from server import PulseRPCServer, CatalogService

class CatalogServiceImpl(CatalogService):
    def listProducts(self):
        # Return list of Product dicts
        return [
            {"productId": "p1", "name": "Item 1", "price": 10.0, "stock": 5},
            {"productId": "p2", "name": "Item 2", "price": 20.0, "stock": 3}
        ]

    def getProduct(self, productId):
        # Return None for optional return type
        for p in products:
            if p["productId"] == productId:
                return p
        return None

# Start server
server = PulseRPCServer(host="0.0.0.0", port=8080)
server.register("CatalogService", CatalogServiceImpl())
server.serve_forever()
```

## Client Usage

```python
from client import HTTPTransport, CatalogServiceClient

transport = HTTPTransport("http://localhost:8080")
catalog = CatalogServiceClient(transport)

# Method calls return Python dicts or None
products = catalog.listProducts()
for product in products:
    print(f"{product['name']}: ${product['price']}")

# Optional methods return None if not found
product = catalog.getProduct("prod001")
if product:
    print(product['name'])
```

## Migration Notes

- If `-package` is not set, the generator keeps the legacy flat Python layout.
- With `-package`, namespace folders become importable packages under the chosen base path.
- Runtime imports always come from `pulserpc` under the same generated root.

## Troubleshooting

- `ModuleNotFoundError` usually means the generated root directory is not on `PYTHONPATH`.
- If a namespace import fails, verify the namespace directory name matches the IDL namespace exactly.
- If runtime imports fail, confirm you are importing `pulserpc` from the generated base package, not from the project root.

## Validation

PulseRPC automatically validates:
- Required fields are present
- Types match IDL definition
- Enum values are valid

```python
# This will raise RPCError (-32602) if validation fails
cart = cart.addToCart({
    'productId': 'prod001',
    'quantity': 2
})
```

## Pydantic Models

When generating Python code with the `--use-pydantic` flag, PulseRPC creates a `models.py` file containing Pydantic `BaseModel` classes for all struct and enum types:

```bash
pulserpc -plugin python-client-server -dir ./output --use-pydantic api/service.pulse
```

This generates `models.py` with Pydantic models:

```python
from pydantic import BaseModel
from typing import Optional
from enum import Enum

class Product(BaseModel):
    productId: str
    name: str
    price: float
    stock: int
    imageUrl: Optional[str] = None  # optional field

    class Config:
        # Pydantic v1 style
        schema_extra = {
            "example": {
                "productId": "prod001",
                "name": "Wireless Mouse",
                "price": 29.99,
                "stock": 50
            }
        }

class OrderStatus(str, Enum):
    pending = "pending"
    paid = "paid"
    shipped = "shipped"
    delivered = "delivered"
```

### Using Pydantic Models

```python
from checkout.models import Product, OrderStatus

# Create and validate
product = Product(
    productId="prod001",
    name="Wireless Mouse",
    price=29.99,
    stock=50
)

# Validation happens automatically
try:
    invalid_product = Product(
        productId="prod001",
        name="Mouse",
        price="not a number",  # Will raise ValidationError
        stock=50
    )
except Exception as e:
    print(f"Validation failed: {e}")
```

### Pydantic Version Compatibility

Generated Pydantic models use Pydantic v1 style (`BaseModel` with `class Config`). If using Pydantic v2, the models are compatible via `pydantic.v1 import BaseModel` import aliasing.

## Multi-Namespace Projects

When your IDL defines multiple namespaces, use the `-package` flag to generate properly structured Python packages:

```bash
pulserpc -plugin python-client-server -dir ./generated -package myapp.lib.rpc api/service.pulse
```

### Output Structure

```
generated/
├── myapp/
│   └── lib/
│       └── rpc/
│           ├── __init__.py           # Imports from pulserpc runtime
│           ├── pulserpc/              # Runtime library
│           │   ├── __init__.py
│           │   ├── rpc.py
│           │   ├── server.py
│           │   ├── client.py
│           │   ├── contract.py
│           │   ├── transport.py
│           │   └── validation.py
│           ├── common/                # Namespace: common
│           │   ├── __init__.py
│           │   ├── types.py
│           │   ├── server.py
│           │   └── client.py
│           └── orders/                # Namespace: orders
│               ├── __init__.py
│               ├── types.py
│               ├── server.py
│               └── client.py
└── idl.json
```

### Import Patterns

```python
# Import from same namespace
from myapp.lib.rpc.orders.types import CreateOrderRequest

# Import from another namespace
from myapp.lib.rpc.common.types import Address

# Import runtime
from myapp.lib.rpc.pulserpc import RPCError, Server, Client, HttpTransport
```

### Runtime Auto-Discovery

The Python `Client` automatically fetches the IDL from the server using the `pulserpc-idl` RPC method:

```python
from myapp.lib.rpc.pulserpc import Client, HttpTransport

# Client bootstraps by fetching IDL from server
transport = HttpTransport("http://localhost:8080")
client = Client(transport)

# Interfaces are auto-discovered and available as attributes
client.OrdersService.createOrder({"cartId": "cart_123", ...})
```

## In-Process Transport

For testing without network overhead, use `InProcTransport`:

```python
from myapp.lib.rpc.pulserpc import Client, InProcTransport, Server

# Create server with your handlers
server = Server(handlers={"CatalogService": CatalogServiceImpl()})

# Create in-process transport connected to server
transport = InProcTransport(server)
client = Client(transport)

# RPC calls happen in-process without HTTP
products = client.CatalogService.listProducts()
```

## Runtime Reference

### Client Validation

The `Client` constructor accepts validation flags:

```python
from myapp.lib.rpc.pulserpc import Client, HttpTransport

transport = HttpTransport("http://localhost:8080")

# Enable request validation (validates params before sending)
client = Client(transport, validate_request=True)

# Enable response validation (validates responses after receiving)
client = Client(transport, validate_response=True)

# Enable both
client = Client(transport, validate_request=True, validate_response=True)
```

### Fire-and-Forget Notifications

Use `notify()` for RPC calls that don't require a response:

```python
# Send notification (no response expected)
client.notify("LoggingService.logEvent", {"event": "user_login", "userId": "123"})

# Unlike regular calls, notify() never raises RPCError
# Useful for logging, analytics, or one-way events
```

## Best Practices

1. **Use dicts for struct values**: All struct values should be dictionaries
2. **Handle None for optionals**: Always check if optional return values are None
3. **Use descriptive error codes**: Custom errors should have codes >= 1000
4. **Validate early**: Let PulseRPC validate input, validate business logic in handlers
5. **Keep state in handlers**: Store in-memory state in service implementation classes

## Working with Nested Structs

```python
# Nested structs work naturally as dicts
order = {
    "orderId": "order_123",
    "cart": {
        "cartId": "cart_123",
        "items": [...],
        "subtotal": 59.98
    },
    "shippingAddress": {
        "street": "123 Main St",
        "city": "San Francisco",
        "state": "CA",
        "zipCode": "94105",
        "country": "USA"
    },
    "paymentMethod": "credit_card",
    "status": "pending",
    "total": 59.98,
    "createdAt": 1642000000
}
```
