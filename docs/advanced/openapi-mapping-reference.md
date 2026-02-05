---
title: OpenAPI Mapping Reference
parent: Advanced
nav_order: 2
layout: default
---

# OpenAPI Mapping Reference

Complete reference for type and construct mappings between OpenAPI specifications and Pulse IDL.

## Type Mapping Tables

### Primitive Types

#### OpenAPI to Pulse

| OpenAPI Type | Format | Pulse Type | Notes |
|--------------|--------|------------|-------|
| `string` | - | `string` | Direct mapping |
| `integer` | `int32` | `int` | Pulse `int` is 64-bit |
| `integer` | `int64` | `int` | Direct mapping |
| `number` | `float` | `float` | Pulse `float` is double precision |
| `number` | `double` | `float` | Direct mapping |
| `boolean` | - | `bool` | Direct mapping |

#### Pulse to OpenAPI

| Pulse Type | OpenAPI Type | Format |
|------------|--------------|--------|
| `string` | `string` | - |
| `int` | `integer` | `int64` |
| `float` | `number` | `double` |
| `bool` | `boolean` | - |

### Complex Types

#### OpenAPI to Pulse

| OpenAPI Schema | Pulse Type | Example |
|---------------|------------|---------|
| `type: array` | `[]Type` | `[]string` |
| `type: object` with `properties` | `struct` | See struct mapping |
| `type: object` with `additionalProperties` | `map[string]Type` | `map[string]string` |
| `type: string` with `enum` | `enum` | See enum mapping |
| `allOf` with single ref + object | `struct` with `extends` | See inheritance |
| `nullable: true` | Field marked `[optional]` | See optional fields |

#### Pulse to OpenAPI

| Pulse Type | OpenAPI Schema | Example |
|------------|----------------|---------|
| `[]Type` | `type: array` with `items` | `{"type": "array", "items": {"type": "string"}}` |
| `map[string]Type` | `type: object` with `additionalProperties` | `{"type": "object", "additionalProperties": {"type": "string"}}` |
| `enum` | `type: string` with `enum` array | `{"type": "string", "enum": ["a", "b"]}` |
| `struct` with `extends` | `allOf` composition | See inheritance |
| `[optional]` field | Not in `required` array | See optional fields |

## Struct Mapping

### OpenAPI to Pulse

**OpenAPI Schema:**
```yaml
components:
  schemas:
    User:
      type: object
      required:
        - id
        - name
      properties:
        id:
          type: integer
          format: int64
        name:
          type: string
        email:
          type: string
```

**Generated Pulse IDL:**
```pulse
struct User {
    id    int
    name  string
    email string   [optional]
}
```

**Rules:**
1. Schema `title` or schema name becomes struct name
2. `required` array determines `[optional]` annotations
3. Property names are converted to camelCase
4. `description` field becomes doc comment
5. `nullable: true` adds `[optional]` annotation

### Pulse to OpenAPI

**Pulse IDL:**
```pulse
// User represents a user account
struct User {
    userId    string
    firstName string
    lastName  string
    email     string   [optional]
}
```

**Generated OpenAPI Schema:**
```yaml
components:
  schemas:
    User:
      description: User represents a user account
      type: object
      properties:
        userId:
          type: string
        firstName:
          type: string
        lastName:
          type: string
        email:
          type: string
      required:
        - userId
        - firstName
        - lastName
```

**Rules:**
1. Struct name becomes schema name
2. Fields without `[optional]` are in `required` array
3. IDL comments become `description` fields
4. Field names preserve original casing

## Enum Mapping

### OpenAPI to Pulse

**OpenAPI Schema:**
```yaml
components:
  schemas:
    Status:
      type: string
      enum:
        - active
        - inactive
        - pending
      description: User account status
```

**Generated Pulse IDL:**
```pulse
// User account status
enum Status {
    active
    inactive
    pending
}
```

**Rules:**
1. Only `type: string` enums are supported
2. Enum values are converted to valid Pulse identifiers
3. `description` field becomes doc comment
4. Numeric enums are converted to struct with const values (warning issued)

### Pulse to OpenAPI

**Pulse IDL:**
```pulse
// Order status enumeration
enum OrderStatus {
    pending
    paid
    shipped
    delivered
    cancelled
}
```

**Generated OpenAPI Schema:**
```yaml
components:
  schemas:
    OrderStatus:
      description: Order status enumeration
      type: string
      enum:
        - pending
        - paid
        - shipped
        - delivered
        - cancelled
```

**Rules:**
1. All Pulse enums map to `type: string`
2. Enum values preserve original casing
3. IDL comments become `description` fields

## Inheritance (extends)

### OpenAPI to Pulse

**OpenAPI Schema:**
```yaml
components:
  schemas:
    Animal:
      type: object
      required:
        - name
      properties:
        name:
          type: string

    Dog:
      allOf:
        - $ref: '#/components/schemas/Animal'
        - type: object
          required:
            - breed
          properties:
            breed:
              type: string
```

**Generated Pulse IDL:**
```pulse
struct Animal {
    name  string
}

struct Dog extends Animal {
    breed  string
}
```

**Rules:**
1. `allOf` with exactly one `$ref` + object maps to `extends`
2. `allOf` with multiple `$ref`s issues warning (uses first)
3. `allOf` with no `$ref`s issues warning (flattens properties)
4. Order is preserved: parent types before child properties

### Pulse to OpenAPI

**Pulse IDL:**
```pulse
struct Animal {
    name  string
}

struct Dog extends Animal {
    breed  string
}
```

**Generated OpenAPI Schema:**
```yaml
components:
  schemas:
    Animal:
      type: object
      properties:
        name:
          type: string
      required:
        - name

    Dog:
      allOf:
        - $ref: '#/components/schemas/Animal'
        - type: object
          properties:
            breed:
              type: string
          required:
            - breed
```

**Rules:**
1. `extends` keyword maps to `allOf` composition
2. Parent struct is first item in `allOf` array
3. Child properties are in second `allOf` item

## Optional Fields

### OpenAPI to Pulse

Fields are optional in Pulse when:
1. Not in the `required` array
2. Marked with `nullable: true`

**OpenAPI Schema:**
```yaml
components:
  schemas:
    User:
      type: object
      required:
        - id
      properties:
        id:
          type: string
        name:
          type: string
          nullable: true
        email:
          type: string
```

**Generated Pulse IDL:**
```pulse
struct User {
    id     string
    name   string   [optional]
    email  string   [optional]
}
```

### Pulse to OpenAPI

Fields with `[optional]` are excluded from the `required` array.

**Pulse IDL:**
```pulse
struct User {
    userId    string
    email     string   [optional]
}
```

**Generated OpenAPI Schema:**
```yaml
components:
  schemas:
    User:
      type: object
      properties:
        userId:
          type: string
        email:
          type: string
      required:
        - userId
```

## Interface and Method Mapping

### OpenAPI to Pulse

#### Interface Grouping

Operations are grouped into interfaces by:
1. **Primary:** First tag in `tags` array
2. **Fallback:** Path prefix (e.g., `/pets/*` → `pets` interface)

**OpenAPI Paths:**
```yaml
paths:
  /pets:
    get:
      operationId: listPets
      tags:
        - pets
      responses:
        '200':
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Pet'
  /pets/{id}:
    get:
      operationId: getPet
      tags:
        - pets
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
```

**Generated Pulse IDL:**
```pulse
interface pets {
    getListPets() []listPetsResponse_item
    getShowPetById(petId string) showPetByIdResponse_item
}
```

#### Method Naming

Method names follow the pattern: `{httpMethod}{operationId}`

| HTTP Method | Prefix | Example operationId | Method Name |
|-------------|--------|---------------------|-------------|
| GET | `get` | `listPets` | `getListPets` |
| POST | `post` | `createPet` | `postCreatePet` |
| PUT | `put` | `updatePet` | `putUpdatePet` |
| DELETE | `delete` | `deletePet` | `deleteDeletePet` |
| PATCH | `patch` | `patchPet` | `patchPatchPet` |

If `operationId` is missing, it's derived from the path:
- `/pets/{id}` → `getPetsById`

#### Parameter Mapping

All parameters become method parameters:

| OpenAPI Parameter In | Pulse Parameter |
|----------------------|-----------------|
| `path` | Named parameter (e.g., `id string`) |
| `query` | Named parameter (e.g., `limit int`) |
| `header` | Dropped with warning |
| `cookie` | Dropped with warning |

#### Request Body Mapping

Request body becomes a parameter named after the operation or `body`:

```yaml
requestBody:
  required: true
  content:
    application/json:
      schema:
        $ref: '#/components/schemas/NewPet'
```

Becomes:
```pulse
postCreatePet(createPetBody NewPet) ...
```

#### Response Mapping

Success responses determine return type:

| Response Code | Behavior |
|---------------|----------|
| 200 | Use schema as return type |
| 201 | Use schema as return type |
| 204 | Return type is `[optional]` |
| default | Used if 200/201 not present |

### Pulse to OpenAPI

#### Path Generation

Each interface method becomes a POST endpoint:

**Pattern:** `POST /{interface}/{method}`

**Pulse IDL:**
```pulse
interface UserService {
    getUser(userId string) User
    createUser(user CreateUserRequest) User
}
```

**Generated Paths:**
```yaml
paths:
  /UserService/getUser:
    post:
      operationId: UserService_getUser
      tags:
        - UserService
      requestBody:
        # ... userId parameter
      responses:
        '200':
          # ... User schema

  /UserService/createUser:
    post:
      operationId: UserService_createUser
      tags:
        - UserService
      requestBody:
        # ... user parameter
      responses:
        '200':
          # ... User schema
```

#### operationId Format

Format: `{interface}_{method}`

Examples:
- `UserService.getUser` → `UserService_getUser`
- `OrderService.placeOrder` → `OrderService_placeOrder`

#### Tags

Each Pulse interface becomes an OpenAPI tag:

**Pulse IDL:**
```pulse
interface CatalogService { }
interface CartService { }
```

**Generated Tags:**
```yaml
tags:
  - name: CatalogService
    description: Pulse interface: CatalogService
  - name: CartService
    description: Pulse interface: CartService
```

## Namespace Mapping

### OpenAPI to Pulse

Namespace is derived from `info.title`:

1. Convert to lowercase
2. Replace non-alphanumeric characters with underscores
3. Ensure valid identifier

**Examples:**

| `info.title` | Generated Namespace |
|--------------|---------------------|
| `Petstore API` | `petstore_api` |
| `My-Great Service` | `my_great_service` |
| `API123` | `api123` |

### Pulse to OpenAPI

`info.title` is derived from namespace:

**Pulse IDL:**
```pulse
namespace my_service
```

**Generated OpenAPI:**
```yaml
info:
  title: My_Service
  description: |
    Namespace: my_service

    Interfaces: 2
    Structs: 5
    Enums: 1
```

## Warning Conditions

### OpenAPI to Pulse Warnings

| Condition | Warning | Fallback |
|-----------|---------|----------|
| `oneOf` encountered | "oneOf encountered at {path} (using first type)" | First schema type |
| `anyOf` encountered | "anyOf encountered at {path} (using first type)" | First schema type |
| `allOf` with multiple refs | "allOf with multiple $ref at {path} (using first)" | First $ref |
| Header parameter | "header parameter '{name}' dropped" | Parameter ignored |
| Cookie parameter | "cookie parameter '{name}' dropped" | Parameter ignored |
| Non-2xx response | "response code {code} dropped" | Response ignored |
| Security scheme | "security scheme '{name}' dropped" | Scheme ignored |
| Binary format | "binary format not supported at {path}" | **Error** |

### Pulse to OpenAPI Warnings

| Condition | Warning |
|-----------|---------|
| Reserved word conflict | "parameter '{name}' conflicts with OpenAPI reserved word" |

## Circular Reference Handling

### OpenAPI to Pulse

Circular references are detected and broken with `[optional]`:

**OpenAPI Schema:**
```yaml
components:
  schemas:
    Node:
      type: object
      properties:
        value:
          type: string
        next:
          $ref: '#/components/schemas/Node'
```

**Generated Pulse IDL:**
```pulse
struct Node {
    value  string
    next   Node   [optional]   // [optional] breaks circular reference
}
```

**Warning Issued:**
```
Warning: circular reference detected at #/components/schemas/Node/next (breaking with [optional])
```

## Examples

See the [OpenAPI Examples](../../examples/openapi/) directory for complete working examples:

- `petstore.yaml` - Standard Petstore OpenAPI 3.0 spec
- `petstore.pulse` - Generated Pulse IDL
- `simple-api.pulse` - Simple Pulse IDL for round-trip testing
- `simple-api.openapi.yaml` - Generated OpenAPI spec

## Additional Resources

- [OpenAPI Specification](https://spec.openapis.org/oas/latest.html)
- [OpenAPI 3.0 to 3.1 Migration Guide](https://openapi.org/swagger/ specification/versions)
- [IDL Syntax Reference](../idl-guide/syntax.html)
- [OpenAPI Translation Guide](../get-started/openapi-translation.html)
