---
title: Runtime Validation
layout: default
---

# Runtime Validation

PulseRPC runtimes automatically validate request and response data against your IDL definitions.

## When Validation Happens

Validation occurs at **two points**:

1. **Client-side** - Before sending requests
2. **Server-side** - Before and after processing requests

## Required vs Optional Fields

```idl
struct User {
    userId    string        // Required
    email     string        // Required
    phone     string [optional]  // Optional
}
```

**Validation rules:**
- ✅ `{"userId": "123", "email": "user@example.com"}` - Valid (phone omitted)
- ✅ `{"userId": "123", "email": "user@example.com", "phone": null}` - Valid (phone is null)
- ❌ `{"userId": "123"}` - Invalid (email is required)
- ❌ `{"userId": 123, "email": "user@example.com"}` - Invalid (userId wrong type)

## Type Validation

### String

```idl
name string
```

✅ `"John Doe"`
❌ `123`
❌ `null` (unless marked optional)

### Int

```idl
age int
```

✅ `42`
❌ `"42"` (string, not int)
❌ `3.14` (float, not int)

### Float

```idl
price float
```

✅ `19.99`
✅ `20` (int coerces to float)
❌ `"19.99"`

### Bool

```idl
active bool
```

✅ `true`
✅ `false`
❌ `"true"`
❌ `1`

## Array Validation

```idl
struct Cart {
    items []CartItem
}
```

✅ `{"items": []}` - Empty array valid
✅ `{"items": [{"productId": "1", "quantity": 2}]}` - Valid item
❌ `{"items": null}` - Null array invalid (unless optional)
❌ `{"items": [{"productId": 1}]}` - Wrong type inside array

## Map Validation

```idl
metadata map[string]string
```

✅ `{"metadata": {"key": "value"}}`
✅ `{"metadata": {}}` - Empty map valid
❌ `{"metadata": {"key": 123}}` - Value type mismatch
❌ `{"metadata": []}` - Array, not map

## Nested Validation

Validation is recursive:

```idl
struct Order {
    cart  Cart
    user  User [optional]
}
```

Validates `Cart` and `User` structures recursively.

## Manual Validation

In addition to automatic request/response validation, you can validate arbitrary data against any named type using the `Contract.validate()` method. This is useful for form validation, testing, or validating data before passing it to business logic.

### Loading a Contract

```python
from pulserpc import Contract

# Load from idl.json
contract = Contract.from_file("path/to/idl.json")
```

```typescript
import { Contract } from './pulserpc/contract';

// Load from idl.json
const contract = Contract.fromFile('path/to/idl.json');
```

### Validating a Struct

```python
result = contract.validate("Person", {
    "username": "alice",
    "age": 30
})
```

```typescript
const result = contract.validate("Person", {
    username: "alice",
    age: 30
});
```

Returns `ValidationResult`:

| Field | Type | Description |
|-------|------|-------------|
| `valid` | `bool` | Whether the value passed validation |
| `error` | `string \| null` | Human-readable error summary (null if valid) |
| `invalidFields` | `string[] \| null` | Path selectors for each invalid field (null if valid) |

### Checking Results

```python
if result.valid:
    process(data)
else:
    print(result.error)           # ".age: Expected int, got string; .email: Expected string, got int"
    for field in result.invalid_fields:
        highlight_form_field(field)  # ".age", ".email"
```

```typescript
if (result.valid) {
  process(data);
} else {
  console.log(result.error);      // ".age: Expected int, got string; .email: Expected string, got int"
  result.invalidFields?.forEach(f => highlightFormField(f));  // ".age", ".email"
}
```

### Error Path Selectors

When validation fails, each error includes a path selector that pinpoints the exact location:

| Path | Meaning |
|------|---------|
| `.username` | Top-level field `username` in a struct |
| `.address.city` | Nested field `city` inside struct `address` |
| `.items[2]` | Element at index 2 of an array |
| `.children[1].age` | Field `age` of element at index 1 in array `children` |

This makes it straightforward to map validation errors back to form fields or API payload paths.

### Nested Validation

```python
result = contract.validate("Order", {
    "orderId": "ord_123",
    "items": [
        {"productId": "p1", "quantity": 2},
        {"productId": "p2", "quantity": -1}
    ]
})
if not result.valid:
    print(result.invalid_fields)  # [".items[1].quantity"]
```

```typescript
const result = contract.validate("Order", {
  orderId: "ord_123",
  items: [
    { productId: "p1", quantity: 2 },
    { productId: "p2", quantity: -1 }
  ]
});
if (!result.valid) {
  console.log(result.invalidFields);  // [".items[1].quantity"]
}
```

### Validating an Enum

```python
result = contract.validate("Color", "red")         # result.valid == True
result = contract.validate("Color", "yellow")       # result.valid == False
```

```typescript
const result = contract.validate("Color", "red");     // result.valid == true
const result = contract.validate("Color", "yellow");   // result.valid == false
```

## Validation Errors

When validation fails, PulseRPC returns an RPC error:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": -32602,
    "message": "Invalid params",
    "data": {
      "field": "email",
      "message": "Required field 'email' is missing"
    }
  }
}
```

## Custom Validation

For business logic validation, return error codes:

```idl
// Error codes:
//   1001 - CartNotFound
//   1002 - CartEmpty
//   1003 - PaymentFailed

interface OrderService {
    createOrder(request CreateOrderRequest) CheckoutResponse
}
```

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": 1002,
    "message": "CartEmpty: Cannot create order from empty cart"
  }
}
```

## Best Practices

1. **Validate early** - Client-side validation provides fast feedback
2. **Validate server-side** - Never trust client input
3. **Use optional fields** - Make fields optional only if truly nullable
4. **Custom error codes** - Use error codes for business logic failures
5. **Clear error messages** - Include helpful context in error messages

## Next Steps

- [IDL Syntax](syntax.html) - Define your IDL
- [Quickstart](../get-started/quickstart-overview.html) - See validation in action
