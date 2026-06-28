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

## Transport Context (ctx)

All handler methods receive a `ctx` parameter as the first positional argument after `self`. It contains transport-level metadata (headers, auth tokens, etc.) and may be `None` if no context was provided.

```python
def listProducts(self, ctx):
    # ctx is a dict with transport metadata (may be None)
    if ctx and 'headers' in ctx:
        auth = ctx['headers'].get('Authorization')
    return products_db
```

The `ctx` parameter is passed as the first positional argument in both Python 3 and Python 2: `func(ctx, *params)`.

---

## Error Handling

Raise `RPCError` with custom codes:

```python
from pulserpc import RPCError

# Standard JSON-RPC errors
raise RPCError(-32602, "Invalid params")

# Custom application errors (use codes >= 1000)
raise RPCError(1001, "CartNotFound: Cart does not exist")
raise RPCError(1002, "CartEmpty: Cannot create order from empty cart")
```

Common error codes:
- `-32700`: Parse error
- `-32600`: Invalid request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error
- `1000+`: Custom application errors

## Server Implementation

Extend generated service classes. All handler methods receive `ctx` as the first positional argument after `self` for transport-level metadata:

```python
from server import PulseRPCServer, CatalogService

class CatalogServiceImpl(CatalogService):
    def listProducts(self, ctx):
        # ctx is for transport-level metadata (headers, auth)
        # Return list of Product dicts
        return [
            {"productId": "p1", "name": "Item 1", "price": 10.0, "stock": 5},
            {"productId": "p2", "name": "Item 2", "price": 20.0, "stock": 3}
        ]

    def getProduct(self, ctx, productId):
        # ctx is for transport-level metadata (headers, auth)
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

### Error Callbacks

Pass an `on_error` callback to the server for custom error handling on unexpected handler exceptions:

```python
from pulserpc import Server, Contract, RPCError

def handle_error(e: Exception) -> None:
    # Log to your monitoring system, metrics, etc.
    print(f"Handler error: {e}")

server = Server(contract, on_error=handle_error)
```

When `on_error` is set, it is invoked with the exception rather than printing a traceback. If `on_error` is not set (default `None`), unhandled exceptions print a traceback to stderr.

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

> **Note:** The `Contract.validate()` method, `ValidationResult`, and `ValidationError` classes are only available in the Python 3 runtime. `Contract.from_file()` is available in both runtimes. Python 2 uses the older `Contract` API without `validate()` and validation classes (see the [Python 2 Server Guide](python2.html)).

### Automatic Validation

PulseRPC automatically validates requests and responses against your IDL:

```python
# This will raise RPCError (-32602) if validation fails
cart = cart.addToCart({
    'productId': 'prod001',
    'quantity': 2
})
```

Validation checks:
- Required fields are present
- Types match IDL definition
- Enum values are valid
- Optional fields allow `None`

### Manual Validation

Validate arbitrary data against any named type using the `Contract` class:

```python
from pulserpc import Contract

# Load IDL from file or use parsed JSON
contract = Contract.from_file("idl.json")

# Validate a struct — returns ValidationResult
result = contract.validate("Person", {
    "username": "alice",
    "age": 30,
    "email": "alice@example.com"
})

if result.valid:
    process(data)
else:
    print(result.error)           # Error summary with all failures
    print(result.invalid_fields)  # [".age", ".email"] — field selectors
```

### ValidationResult Reference

| Field | Type | Description |
|-------|------|-------------|
| `valid` | `bool` | `True` if the value is valid |
| `error` | `Optional[str]` | Human-readable error summary (`None` if valid) |
| `invalid_fields` | `Optional[List[str]]` | Path selectors for each invalid field (`None` if valid) |

### ValidationError Reference

Each collected error has:

| Field | Type | Description |
|-------|------|-------------|
| `path` | `str` | Dot-separated path to the invalid value (e.g. `.address.city`, `.items[1].quantity`) |
| `message` | `str` | Human-readable error description |

### Examples

**Validating form input:**

```python
result = contract.validate("SignupRequest", {
    "username": username,
    "email": email,
    "age": age
})
if not result.valid:
    # Map errors back to form fields
    for field in (result.invalid_fields or []):
        form.add_error(field.lstrip("."), result.error)
```

**Validating an enum value:**

```python
result = contract.validate("Color", "red")     # result.valid == True
result = contract.validate("Color", "yellow")   # result.valid == False
```

**Validating nested data with path tracking:**

```python
result = contract.validate("Order", {
    "orderId": "ord_123",
    "items": [
        {"productId": "p1", "quantity": 2},
        {"productId": "p2", "quantity": -1}
    ]
})
# result.invalid_fields == [".items[1].quantity"]
```

### Loading a Contract

```python
# From an idl.json file
contract = Contract.from_file("path/to/idl.json")

# From parsed JSON (e.g. from the generated ALL_STRUCTS/ALL_ENUMS)
import json
with open("idl.json") as f:
    contract = Contract(json.load(f))
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
