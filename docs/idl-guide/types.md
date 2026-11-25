---
title: IDL Types & Fields
layout: default
---

# IDL Types & Fields

PulseRPC supports a rich type system for defining your service contracts.

## Built-in Types

| IDL Type | Description | Go | Java | C# | Python | TypeScript |
|----------|-------------|-----|------|----|--------|-----------|
| `string` | Text/UTF-8 strings | `string` | `String` | `string` | `str` | `string` |
| `int` | Signed 64-bit integers (`[-9223372036854775808, 9223372036854775807]`) | `int64` | `long`/`Long` | `long` | `int` | `number` (safe integers only) |
| `float` | 64-bit floating point | `float64` | `double`/`Double` | `double` | `float` | `number` |
| `bool` | Boolean values | `bool` | `boolean`/`Boolean` | `bool` | `bool` | `boolean` |

> **`int` is signed 64-bit (D1).** Java, C#, Go, and Python runtimes support the
> full range. TypeScript types `int` as `number`, and JavaScript numbers can
> represent integers exactly only up to 2^53-1 (`9007199254740991`), so the
> TypeScript runtime validates the safe-integer range. This is a breaking change
> from the earlier signed 32-bit `int`.

## Arrays

Ordered lists of a type:

```idl
struct Cart {
    items []CartItem
    tags  []string
}
```

**Language mapping:**
- Go: `[]T`
- Java: `List<T>`
- Python: `List[T]`
- TypeScript: `T[]`

## Maps

Key-value dictionaries (keys are always strings):

```idl
struct Metadata {
    tags map[string]string
    headers map[string]string
}
```

**Language mapping:**
- Go: `map[string]T`
- Java: `Map<String, T>`
- Python: `Dict[str, T]`
- TypeScript: `Record<string, T>`

## Optional Fields

Fields marked `[optional]` can be null or omitted:

```idl
struct User {
    userId    string
    email     string
    phone     string  [optional]  // Can be null
    bio       string  [optional]  // Can be omitted
}
```

**Validation rules:**
- Required fields must be present and non-null
- Optional fields can be omitted or null
- Arrays/maps are optional if their element type is optional

## Struct Inheritance

Extend existing structs to reuse fields:

```idl
struct BaseResponse {
    status string
    message string
}

struct UserResponse extends BaseResponse {
    user User
}

// UserResponse has: status, message, user
```

**Rules:**
- Child struct inherits all parent fields
- Can add new fields
- Multiple inheritance not supported

## Complex Example

```idl
namespace ecommerce

enum ContactType {
    email
    phone
    sms
}

struct Contact {
    type  ContactType
    value string
}

struct Address {
    street     string
    city       string
    state      string
    zipCode    string
    country    string
    isPrimary  bool  [optional]
}

struct User {
    userId      string
    name        string
    email       string
    contacts    []Contact
    metadata    map[string]string
    address     Address  [optional]
    createdAt   int
    verified    bool
}
```

## Type Validation

PulseRPC runtimes automatically validate:
- **Required fields** are present
- **Types match** (string, int, float, bool)
- **Arrays** contain correct element types
- **Maps** have string keys and correct value types

See [Validation](validation.html) for details.

## Next Steps

- [Validation](validation.html) - How runtime validation works
- [Syntax](syntax.html) - Complete IDL syntax reference
- [Quickstart](../get-started/quickstart-overview.html) - Hands-on practice
