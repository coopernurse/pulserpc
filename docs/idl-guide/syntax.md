---
title: Syntax
parent: IDL Guide
nav_order: 1
layout: default
---

# PulseRPC IDL Syntax

The PulseRPC Interface Definition Language (IDL) lets you define your service contract in a language-agnostic way.

## Namespaces

Every IDL file must declare a namespace:

```idl
namespace myservice
```

The namespace becomes the package/module name in generated code.

## Comments

```idl
// Comments start with //

// Multi-line comments are supported                                                                                                                                                                                                        
// by stacking single-line comments                                                                                                                                                                                                         
// on consecutive lines
```

## Enums

Define a set of valid values:

```idl
enum Status {
    pending
    active
    closed
}
```

## Error Declarations

Define error codes that methods can raise:

```idl
errors {
    1001 NotFound "Not Found"
    1002 InvalidInput "Invalid Input"
    1003 PermissionDenied "Permission Denied"
}
```

Each error declaration has three parts:
- **Error code**: Integer value for the JSON-RPC `code` field
- **Name**: Identifier used in `raises()` clauses
- **Message**: String literal for the JSON-RPC `message` field

### Error Codes

Error codes can be any integer. JSON-RPC reserves certain ranges:
- `-32700` to `-32000`: Standard JSON-RPC errors
- `-32099` to `-32000`: Server errors
- `0` and positive integers: Application-defined errors

### Using Errors in Methods

Methods can declare which errors they raise using the `raises()` clause:

```idl
interface UserService {
    getUser(userId string) User raises(NotFound)
    createUser(user User) UserResponse raises(InvalidInput, PermissionDenied)
}
```

### Namespaced Errors

Errors can be imported from other files:

```idl
// common/errors.pulse
namespace common

errors {
    1001 NotFound "Not Found"
    1002 InvalidInput "Invalid Input"
}

// service.pulse
import "common/errors.pulse"

namespace myservice

interface Service {
    getValue(id string) string raises(common.NotFound)
}
```

### Error Documentation

Errors can be documented with comments:

```idl
// Standard application errors
errors {
    // Returned when a requested resource is not found
    1001 NotFound "Not Found"

    // Returned when input validation fails
    1002 InvalidInput "Invalid Input"
}
```

See [Error Handling](errors.html) for comprehensive error documentation.

## Structs

Define data structures with fields:

```idl
struct User {
    userId    string
    email     string
    age       int
    active    bool
    createdAt int      [optional]
}
```

### Field Modifiers

- `[optional]` - Field can be null/omitted
- No modifier - Field is required

## Struct Inheritance

Extend existing structs:

```idl
struct BaseResponse {
    status string
    message string
}

struct UserResponse extends BaseResponse {
    user User
}
```

## Arrays

Define lists with `[]`:

```idl
struct Cart {
    items []CartItem
}
```

## Maps

Define key-value maps:

```idl
struct Metadata {
    tags map[string]string
}
```

## Interfaces

Define service interfaces:

```idl
interface UserService {
    // Returns a user by ID
    getUser(userId string) User

    // Creates a new user
    createUser(user User) UserResponse

    // Lists all users (optional return)
    listUsers() []User [optional]
}
```

### Interface Methods

- Methods define request and response types
- Return type can be marked `[optional]` to indicate null return

## Imports

Import other IDL files:

```idl
import "common.pulse"
```

## Complete Example

```idl
namespace checkout

enum OrderStatus {
    pending
    paid
    shipped
    cancelled
}

struct Product {
    productId    string
    name         string
    price        float
    stock        int
}

interface CatalogService {
    listProducts() []Product
    getProduct(productId string) Product [optional]
}

interface OrderService {
    createOrder(request CreateOrderRequest) OrderResponse
    getOrder(orderId string) Order [optional]
}
```

## Next Steps

- [Error Handling](errors.html) - Declaring and handling errors
- [Types & Fields](types.html) - Built-in types and validation
- [Validation](validation.html) - How runtime validation works
- [Quickstart](../get-started/quickstart-overview.html) - Build a complete example
