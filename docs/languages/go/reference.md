---
title: Reference
parent: Go
grand_parent: Language Guides
nav_order: 3
layout: default
---

# Go Reference

## Type Mappings

| IDL Type | Go Type | Example |
|----------|---------|---------|
| `string` | `string` | `"hello"` |
| `int` | `int` | `42` |
| `float` | `float64` | `3.14` |
| `bool` | `bool` | `true`, `false` |
| `[]Type` | `[]Type` | `[]int{1, 2, 3}` |
| `map[string]Type` | `map[string]Type` | `map[string]string{"key": "value"}` |
| `Enum` | `EnumName` + `EnumValue_Value` | `OrderStatusPending` |
| `Struct` | `*Struct` | `&Product{...}` |
| `T [optional]` | `*T` (pointer + `omitempty`) | `*string`, `*int` |

> **`int` is signed 32-bit on the wire.** Go's native `int` may be wider, but
> validation accepts only integral JSON numbers in
> `[-2147483648, 2147483647]` (D1). Fractional, NaN/Inf, bool, and out-of-range
> values are rejected with `-32602`.
>
> **Optional fields omit their value when nil.** Optional fields are pointers
> tagged with `omitempty`, so a nil value is absent from the serialized struct
> rather than encoded as `null`.

## Generated Structs

Each struct in your IDL becomes a Go struct:

```go
import "checkout"

// Create instances
product := &checkout.Product{
    ProductId: "prod001",
    Name:      "Wireless Mouse",
    Description: "Ergonomic mouse",
    Price:     29.99,
    Stock:     50,
    ImageUrl:  stringPtr("https://example.com/mouse.jpg"), // optional field
}

cart := &checkout.Cart{
    CartId:   "cart_1234",
    Items:    []*checkout.CartItem{},
    Subtotal: 0.0,
}
```

## Optional Fields

Optional fields become pointers. Use helper functions:

```go
func stringPtr(s string) *string {
    return &s
}

func intPtr(i int) *int {
    return &i
}

// Create with optional field
product := &checkout.Product{
    ProductId: "prod001",
    Name:      "Wireless Mouse",
    Price:     29.99,
    Stock:     50,
    ImageUrl:  nil,  // optional field can be nil
}

// Check optional field
if product.ImageUrl != nil {
    fmt.Println(*product.ImageUrl)
}
```

## Enums

Enums become constants with `EnumName_Value` naming:

```go
import "checkout"

// Use enum constants
order := &checkout.Order{
    Status: checkout.OrderStatusPending,
    PaymentMethod: checkout.PaymentMethodCreditCard,
}

// Compare enums
if order.Status == checkout.OrderStatusPending {
    fmt.Println("Order is pending")
}
```

## Error Handling

Return errors using `checkout.NewRPCError()`:

```go
import "checkout"

// Standard JSON-RPC errors
return nil, checkout.NewRPCError(-32602, "Invalid params")

// Custom application errors (use codes >= 1000)
return nil, checkout.NewRPCError(1001, "CartNotFound: Cart does not exist")
return nil, checkout.NewRPCError(1002, "CartEmpty: Cannot create order from empty cart")
```

Common error codes:
- `-32700`: Parse error
- `-32600`: Invalid request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error
- `1000+`: Custom application errors

## Server Implementation

Implement interface methods:

```go
import (
    "checkout"
)

type CatalogService struct{}

func (s *CatalogService) ListProducts() ([]*checkout.Product, error) {
    return []*checkout.Product{
        {ProductId: "p1", Name: "Item 1", Price: 10.0, Stock: 5},
        {ProductId: "p2", Name: "Item 2", Price: 20.0, Stock: 3},
    }, nil
}

func (s *CatalogService) GetProduct(productId string) (*checkout.Product, error) {
    for _, p := range products {
        if p.ProductId == productId {
            return p, nil
        }
    }
    return nil, nil  // Return nil for optional type
}

// Start server
func main() {
    server := checkout.NewServer("0.0.0.0", 8080)
    server.Register("CatalogService", &CatalogService{})
    server.ServeForever()
}
```

## Client Usage

```go
import (
    "checkout"
)

transport := checkout.NewHTTPTransport("http://localhost:8080", nil)
catalog := checkout.NewCatalogServiceClient(transport)

// Method calls return structs or nil
products, err := catalog.ListProducts()
if err != nil {
    log.Fatal(err)
}

for _, p := range products {
    fmt.Printf("%s: $%.2f\n", p.Name, p.Price)
}

// Optional methods return nil if not found
product, err := catalog.GetProduct("prod001")
if err != nil {
    log.Fatal(err)
}
if product != nil {
    fmt.Println(product.Name)
}
```

## Validation

PulseRPC automatically validates:
- Required fields are present
- Types match IDL definition
- Enum values are valid

```go
// This will return error (-32602) if validation fails
cart, err := cartService.AddToCart(&checkout.AddToCartRequest{
    ProductId: "prod001",
    Quantity:  2,
})
```

## IDL Metadata

Generation writes `idl.json` and embeds the same document as the `IDL_JSON`
constant in `server.go`. Both forms use one schema (D8):

```json
{
  "rootNamespace": "checkout",
  "checksum": "<43-char base64url SHA-256>",
  "interfaces": [],
  "structs": [],
  "enums": [],
  "errors": []
}
```

Every top-level key is present even when its collection is empty. The checksum
is identical in both forms and covers declaration structure, not comments or
error-message text.

## Limits and Transport Behavior

Generated adapters and the runtime enforce the shared resource limits (D5/D6):

| Limit | Value | Error |
|-------|-------|-------|
| Request body | 1 MiB | `-32600` `"Request body too large"` |
| Response body (client read) | 10 MiB | client error |
| Batch entries | 100 | `-32600` `"Batch exceeds maximum size"` |
| HTTP client timeout | 30 s | transport error |

Additional behavior:

- Non-POST requests return HTTP **405** with a JSON-RPC `-32600` body and
  `"id": null`.
- Handler errors and panics return `-32603` `"Internal error"` with no `data`
  member. Details are logged server-side only.
- Requests with a boolean, object, or array `id` are rejected with `-32600`.
  An explicit `"id": null` is a notification and receives no response.
- Batch responses keep the array shape. Non-object batch entries receive their
  own `-32600`; notification-only batches return HTTP 204 with no body.
- `NewHTTPTransport` accepts optional custom headers and removes a trailing `/`
  from the base URL.
- Generation is byte-deterministic: namespaces, imports, and type lookups are
  sorted rather than derived from Go map iteration order.

## Best Practices

1. **Use pointers for optionals**: Always check for nil before dereferencing
2. **Handle errors explicitly**: Go requires explicit error checking
3. **Use descriptive error codes**: Custom errors should have codes >= 1000
4. **Keep handlers simple**: Business logic in handlers, validation in framework
5. **Use struct literals**: Clearer than setting fields one-by-one

## Working with Nested Structs

```go
// Nested structs work naturally
order := &checkout.Order{
    OrderId: "order_123",
    Cart: &checkout.Cart{
        CartId:   "cart_123",
        Items:    []*checkout.CartItem{...},
        Subtotal: 59.98,
    },
    ShippingAddress: &checkout.Address{
        Street:  "123 Main St",
        City:    "San Francisco",
        State:   "CA",
        ZipCode: "94105",
        Country: "USA",
    },
    PaymentMethod: checkout.PaymentMethodCreditCard,
    Status:        checkout.OrderStatusPending,
    Total:         59.98,
    CreatedAt:     time.Now().Unix(),
}
```

## Concurrency

Go server handlers are called concurrently. Use mutexes for shared state:

```go
import "sync"

type CartService struct {
    mu    sync.RWMutex
    carts map[string]*checkout.Cart
}

func (s *CartService) GetCart(cartId string) (*checkout.Cart, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.carts[cartId], nil
}

func (s *CartService) AddToCart(req *checkout.AddToCartRequest) (*checkout.Cart, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    // ... modify s.carts
}
```
