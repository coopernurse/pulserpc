---
title: Error Handling
parent: IDL Guide
nav_order: 4
layout: default
---

# Error Handling

PulseRPC provides declarative error definitions for documenting and handling errors in your services.

## Declaring Errors

Use the `errors` block to define error codes:

```idl
namespace myservice

errors {
    1001 NotFound "Not Found"
    1002 InvalidInput "Invalid Input"
    1003 PermissionDenied "Permission Denied"
    1004 AlreadyExists "Already Exists"
}
```

## Error Declaration Syntax

Each error has three components:

1. **Code**: Integer - sent as the JSON-RPC `code` field
2. **Name**: Identifier - referenced in `raises()` clauses
3. **Message**: String literal - sent as the JSON-RPC `message` field

```
<code> <name> <message>
```

Example:
```idl
1001 NotFound "Not Found"
//     ^         ^
//     |         +-- Message (string literal)
//     +------------ Name (identifier)
// Code (integer)
```

## Using Raises Clauses

Methods declare which errors they can raise:

```idl
interface UserService {
    getUser(userId string) User raises(NotFound)
    createUser(user User) UserResponse raises(InvalidInput, PermissionDenied)
    deleteUser(userId string) string raises(NotFound, PermissionDenied)
}
```

### Rules

- `raises()` is optional - methods without it can only succeed
- Multiple errors are comma-separated
- Error names can be qualified (namespaced)

## Importing Errors

Errors can be defined in a separate file and imported:

**common/errors.pulse:**
```idl
namespace common

errors {
    1001 NotFound "Not Found"
    1002 InvalidInput "Invalid Input"
}
```

**myservice.pulse:**
```idl
import "common/errors.pulse"

namespace myservice

interface Service {
    getValue(id string) string raises(common.NotFound)
}
```

## JSON-RPC Error Response

When an error occurs, the JSON-RPC response includes:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "error": {
    "code": 1001,
    "message": "Not Found",
    "data": null
  }
}
```

The `code` and `message` values come from the error declaration.

## Reserved Error Codes

JSON-RPC 2.0 defines these standard error codes:

| Code | Message | Meaning |
|------|---------|---------|
| -32700 | Parse error | Invalid JSON |
| -32600 | Invalid Request | JSON-RPC request is invalid |
| -32601 | Method not found | Method does not exist |
| -32602 | Invalid params | Invalid method parameters |
| -32603 | Internal error | Internal server error |

You can use codes outside these ranges for application-specific errors.

## Best Practices

### Use Semantic Error Codes

```idl
// Good: Semantic, domain-specific
errors {
    1001 UserNotFound "User not found"
    1002 InvalidEmail "Invalid email address"
    1003 DuplicateEmail "Email already registered"
}

// Avoid: Generic codes
errors {
    1 Error "Error"
    2 Error2 "Another error"
}
```

### Group Related Operations

```idl
interface UserService {
    // User lookups
    getUser(id string) User raises(UserNotFound)

    // User creation
    createUser(user User) UserResponse raises(InvalidEmail, DuplicateEmail)

    // User modification
    updateUser(id string, updates User) User raises(UserNotFound, InvalidEmail)
}
```

### Document Error Conditions

```idl
// Standard error codes for user operations
errors {
    // User not found in database
    1001 UserNotFound "User not found"

    // Email format validation failed
    1002 InvalidEmail "Invalid email format"

    // Email already registered to another account
    1003 DuplicateEmail "Email already registered"
}
```

## Validation Rules

The parser enforces these rules:

1. **Namespace required**: Errors must be in a namespace
2. **Unique codes**: Each error code must be unique within a file
3. **Unique names**: Error names cannot conflict with types (structs, enums, interfaces)
4. **Valid references**: `raises()` can only reference declared errors
5. **Integer codes**: Error codes must be integer literals
6. **String messages**: Error messages must be string literals

## Examples

### Simple Example

```idl
namespace checkout

errors {
    1001 OutOfStock "Out of Stock"
    1002 InvalidQuantity "Invalid Quantity"
}

interface OrderService {
    createOrder(items []OrderItem) Order raises(OutOfStock, InvalidQuantity)
}
```

### Complex Example with Imports

```idl
// errors.pulse
namespace api

errors {
    1001 NotFound "Resource not found"
    1002 InvalidInput "Invalid input"
    1003 Unauthorized "Unauthorized access"
}
```

```idl
// service.pulse
import "errors.pulse"

namespace userservice

struct User {
    userId string
    email  string
}

interface UserService {
    getUser(userId string) User raises(api.NotFound)
    createUser(user User) User raises(api.InvalidInput, api.Unauthorized)
}
```

## Language Mapping

Error declarations generate language-specific constructs:

| Language | Generated Code |
|----------|----------------|
| Go | Constants with error codes and helpers |
| Python | Exception classes with code/message |
| TypeScript | Error classes and type guards |
| Java | Exception classes with error codes |
| C# | Exception classes with error codes |

See the [Language Reference](../languages/) for details on error handling in each language.

## Next Steps

- [Syntax](syntax.html) - Complete IDL syntax reference
- [Validation](validation.html) - How runtime validation works
- [Quickstart](../get-started/quickstart-overview.html) - Hands-on practice
