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
| `int` | `int64` | `42` |
| `float` | `float64` | `3.14` |
| `bool` | `bool` | `true`, `false` |
| `[]Type` | `[]Type` | `[]int64{1, 2, 3}` |
| `map[string]Type` | `map[string]Type` | `map[string]string{"key": "value"}` |
| `Enum` | `EnumName` + `EnumValue_Value` | `OrderStatusPending` |
| `Struct` | `Struct` (value type; `*Struct` only when `[optional]`) | `Product{...}` |
| `T [optional]` | `*T` (pointer + `omitempty`) | `*string`, `*int64` |

> **`int` is signed 64-bit on the wire.** Generated Go fields and method
> signatures use `int64`; validation accepts integral JSON numbers in
> `[-9223372036854775808, 9223372036854775807]` (D1). Fractional, NaN/Inf, bool,
> and out-of-range values are rejected with `-32602`. This is a breaking change
> from the earlier 32-bit `int`.
>
> **Optional fields omit their value when nil.** Optional fields are pointers
> tagged with `omitempty`, so a nil value is absent from the serialized struct
> rather than encoded as `null`.
>
> **Required struct fields are values.** Non-optional struct fields, slice
> elements, and method parameters use the struct value type (`Cart`,
> `[]CartItem`). Only `[optional]` structs become pointers (`*Cart`).

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
    Items:    []checkout.CartItem{},
    Subtotal: 0.0,
}
```

### Struct JSON tags

By default, only optional fields are tagged `omitempty`; required fields are
always serialized:

```go
type Product struct {
    ProductId string  `json:"product_id"`
    ImageUrl  *string `json:"image_url,omitempty"`
}
```

Pass `-omit-empty` to the Go generator to add `omitempty` to every generated
struct field:

```bash
pulserpc -plugin go-client-server -omit-empty -dir ./pkg/checkout checkout.pulse
```

With `-omit-empty` enabled, required fields also omit their zero value (`0`, `""`,
`false`, or a nil pointer/slice/map). This is a compile-time generator option,
not a `ServerConfig` field. Use it deliberately: omitting a required zero value
from a request can make the server report the field as missing.

Use `-preserve-empty-optional-arrays` when an optional array must distinguish omission
from an explicit empty array. It emits the Go `omitzero` JSON tag on optional
slice fields. A nil slice is omitted and a non-nil empty slice serializes to
`[]`. Generated code using this flag requires Go 1.24 or later. The flag takes
precedence for optional arrays when combined with `-omit-empty`.

## Optional Fields

Optional fields become pointers. Use helper functions:

```go
func stringPtr(s string) *string {
    return &s
}

func int64Ptr(i int64) *int64 {
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

Implement interface methods. Every method takes `ctx context.Context` first and
returns an error result:

```go
import (
    "context"

    "checkout"
)

type CatalogService struct{}

func (s *CatalogService) ListProducts(ctx context.Context) ([]checkout.Product, error) {
    if err := ctx.Err(); err != nil {
        return nil, err
    }
    return []checkout.Product{
        {ProductId: "p1", Name: "Item 1", Price: 10.0, Stock: 5},
        {ProductId: "p2", Name: "Item 2", Price: 20.0, Stock: 3},
    }, nil
}

func (s *CatalogService) GetProduct(ctx context.Context, productId string) (*checkout.Product, error) {
    for _, p := range products {
        if p.ProductId == productId {
            return p, nil
        }
    }
    return nil, nil // Return nil for an optional return type
}

// Start server
func main() {
    server := checkout.NewPulseRPCServer("0.0.0.0", 8080)
    // Register validates the handler shape: a leading context.Context and the
    // declared error result. Check the error so a misconfigured server fails
    // at startup instead of on the first request.
    if err := server.Register("CatalogService", &CatalogService{}); err != nil {
        log.Fatal(err)
    }
    server.ServeForever()
}
```

A method with no declared return type returns `error` only. Methods unrelated
to the registered interface are ignored; every interface method must be
implemented, and `Register` returns an error naming the offending method
otherwise.


For an IDL interface declared in a namespace, register either its declared
name or its qualified IDL name. Incoming calls must use the registered name.
Register both explicitly if both spellings are part of the service contract.

## Server configuration

`NewPulseRPCServerWithConfig` accepts a `pulserpc.ServerConfig` that controls
the server policy. Start from `pulserpc.DefaultServerConfig()` and override the
fields you need:

```go
cfg := pulserpc.DefaultServerConfig()
cfg.AllowBatch = false
cfg.RequireNonNullID = true
cfg.ParamsMode = pulserpc.ParamsModePositionalOnly
cfg.MaxRequestBodyBytes = 32 * 1024
cfg.RejectInvalidUTF8 = true
cfg.RejectUnknownRequestFields = true
cfg.MapError = func(phase pulserpc.ErrorPhase, err error) *pulserpc.RPCError {
    return pulserpc.NewRPCError(4101, "Invalid input")
}

server := checkout.NewPulseRPCServerWithConfig("0.0.0.0", 8080, cfg)
```

`NewPulseRPCServer(host, port)` is equivalent to
`NewPulseRPCServerWithConfig(host, port, pulserpc.DefaultServerConfig())` and
keeps the v0.4.0 behavior.

> **The `ServerConfig` zero value disables batching.** `AllowBatch` must be set
> explicitly. Always build the config from `DefaultServerConfig()` so you only
> change the policies you intend to change.

`AllowBatch`, `MaxRequestBodyBytes`, and `RejectInvalidUTF8` are enforced by the
generated HTTP adapter. `RequireNonNullID`, `ParamsMode`,
`RejectUnknownRequestFields`, `RequireIDLFieldNames`, `RejectNullRequestField`, `MapError`, and `MapMethodError` are enforced by the runtime
dispatcher, so they also apply to programmatic `Call`/`CallWithContext` users.

Regenerate the Go server with the matching PulseRPC release. Generated servers
now call `NewServerWithConfig` and `Config()`, so the generated code and the
copied `pulserpc` runtime must be upgraded together; `NewPulseRPCServer` and
`NewServer` remain source-compatible.

| Field | Default | Behavior |
|-------|---------|----------|
| `AllowBatch` | `true` | `false` rejects a JSON array body with `-32600` before dispatch. The zero value is `false`. |
| `RequireNonNullID` | `false` | `true` rejects an absent or `null` id with `-32600` and a `null` response id. The handler does not run and an error is written even though the request would otherwise be a notification. The id policy is applied before the `jsonrpc` and method checks, so an id-less request reports the id error first. |
| `ParamsMode` | `ParamsModeAny` | `ParamsModePositionalOnly` rejects object (named) params with `-32600`. Missing or `null` params still mean zero arguments. |
| `MaxRequestBodyBytes` | `0` | `<= 0` keeps the legacy 1 MiB cap and the legacy `-32600` `"Request body too large"` response. A positive value uses that byte cap and reports oversized bodies as `-32700` `"Parse error"`. |
| `RejectInvalidUTF8` | `false` | `true` rejects a body that is not valid UTF-8 with `-32700` `"Parse error"`. The default lets `encoding/json` replace invalid bytes with U+FFFD. |
| `RejectUnknownRequestFields` | `false` | `true` rejects unknown keys in struct-typed request params with `-32602` (or the mapped error). Detection recurses through pointers, slices, arrays, maps, and embedded structs, and honors `json` tags and `json:"-"`. Unknown top-level JSON-RPC envelope members are still ignored. |
| `RequireIDLFieldNames` | `false` | During request contract validation, restricts struct keys to exact IDL field names, including nested and inherited fields. Leaves map keys, envelope metadata, and response validation unchanged. |
| `RejectNullRequestField` | `nil` | Rejects explicit null for selected optional request struct fields. The callback receives the declared struct name and exact IDL field key. Omission, response validation, and direct contract validation are unaffected. |
| `MapError` | `nil` | Maps argument decode and validation failures to the returned `*RPCError`. Return `nil` to fall back to the standard response. |
| `MapMethodError` | `nil` | Maps the same failures with the qualified RPC method available. Return `nil` to fall through to `MapError`, then to the standard response. |

`RejectNullRequestField` may run concurrently for different requests. For a
field inherited through `extends`, the callback receives the resolved request
struct's name.

### Parameter error phases

`MapError` receives an `ErrorPhase` so an application can distinguish a
malformed payload from a value that decoded but failed contract validation:

- `pulserpc.ErrorPhaseDecodeParams` covers params-shape, parameter-count,
  named-param conversion, and unknown-request-field failures.
- `pulserpc.ErrorPhaseValidateParams` covers IDL contract validation failures
  such as a wrong value type, a missing required struct field, or a bad enum
  value.

A mapped error is returned with HTTP status 200 and exactly the code, message,
and data of the returned `RPCError` (the example above omits `data`). Envelope
protocol errors are never mapped and remain `-32600`: an invalid `jsonrpc`
version, method, or id, and named params rejected by `ParamsModePositionalOnly`.
Notifications still receive no response unless `RequireNonNullID` rejects them.

### Numeric IDs are exact

The generated adapter decodes request bodies with `json.Decoder.UseNumber()`,
so numeric ids never pass through `float64`. An id such as
`9007199254740993` (above 2^53) is echoed exactly. This applies with or without
a custom `ServerConfig`.

The request id always keeps the exact `json.Number` form. Before dispatch,
the adapter converts numbers inside `params` (and other request members) back to
`float64` **when the value is in the JavaScript safe-integer range**
(`±(2^53-1)`), so RPC middleware sees the same Go types as v0.4.0 for ordinary
values. Larger `int64` values stay `json.Number`: middleware must handle that
type (or convert it explicitly) when it inspects a large `int64` parameter, and
the runtime converts it to `int64` for the handler without losing precision.
(Even `-2^63`, which is exactly representable as a `float64`, stays
`json.Number` because JSON float formatting would rewrite its integer text.) On
the client side, `HTTPTransport` also preserves the response id as
`json.Number` and compares numeric ids exactly; result and error numbers stay
`float64` in the safe-integer range and remain `json.Number` for larger
integers. Upgrade client and server runtimes together, and note that a server
which still rounds a large id will now fail the client's envelope check instead
of silently accepting a mismatched id.

For Go IDL integer fields, decimal and exponent spellings are accepted when
their exact numeric value is an integer in the signed 64-bit range. The adapter
rejects fractional values even when a float64 conversion would round to an
integer. These rules apply to optional pointer fields as well.


> **JavaScript/TypeScript cannot use the full `int` range.** The JavaScript
> `number` type represents integers exactly only up to 2^53-1
> (`9007199254740991`). A TypeScript client or server therefore validates
> against the safe-integer range even though the wire type is signed 64-bit.
> Go, Java, C#, and Python runtimes support the full range.

## Client Usage

Generated client methods take a context first:

```go
import (
    "context"

    "checkout"
)

ctx := context.Background()
transport := checkout.NewHTTPTransport("http://localhost:8080", nil)
catalog := checkout.NewCatalogServiceClient(transport)

// Method calls return structs or nil
products, err := catalog.ListProducts(ctx)
if err != nil {
    log.Fatal(err)
}

for _, p := range products {
    fmt.Printf("%s: $%.2f\n", p.Name, p.Price)
}

// Optional methods return nil if not found
product, err := catalog.GetProduct(ctx, "prod001")
if err != nil {
    log.Fatal(err)
}
if product != nil {
    fmt.Println(product.Name)
}
```

Custom transports implement `Transport` (and optionally `NotifyTransport`):

```go
type Transport interface {
    Call(ctx context.Context, method string, params []interface{}) (map[string]interface{}, error)
}

type NotifyTransport interface {
    Notify(ctx context.Context, method string, params []interface{}) error
}
```

The generated `HTTPTransport` uses `http.NewRequestWithContext`, so cancelling
`ctx` aborts an in-flight call.

## Context and Middleware

Every request has two supported middleware layers.

### HTTP middleware (`UseHTTP`)

`UseHTTP` receives the concrete `http.ResponseWriter` and `*http.Request` and
runs at the very top of the generated handler, before the method check and body
read. Return a derived context to share header values with RPC middleware and
handlers. Return `false` after writing a response to abort without the adapter
writing a second one. A nil context falls back to `r.Context()`. The adapter's
request body cap (the configured `MaxRequestBodyBytes`, or the default 1 MiB) is
installed before this hook, so a middleware that inspects the body is still
bounded. Each hook receives `r.WithContext(ctx)` using the context returned by
the previous hook.

```go
import (
    "context"
    "net/http"
)

type contextKey string

const apiKeyContext contextKey = "api-key"

server.UseHTTP(func(w http.ResponseWriter, r *http.Request) (context.Context, bool) {
    apiKey := r.Header.Get("X-API-Key")
    if apiKey == "" {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusUnauthorized)
        w.Write([]byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32001,"message":"Unauthorized"}}`))
        return nil, false
    }
    ctx := context.WithValue(r.Context(), apiKeyContext, apiKey)
    return ctx, true
})
```

Use typed keys (`type contextKey string`) so request-scoped values cannot
collide with unrelated context values.

### RPC middleware (`UseRPC`)

`UseRPC` runs once per JSON-RPC request (per batch member), after envelope
validation (`id`, `jsonrpc`, `method`) and before `pulserpc-idl` handling,
parameter validation, and dispatch. It can derive the context, rewrite the
request, or return a non-nil response to short-circuit dispatch. A
short-circuited response is discarded for notifications. A middleware panic
becomes `-32603` "Internal error".

```go
server.UseRPC(func(ctx context.Context, request map[string]interface{}, response map[string]interface{}) (context.Context, map[string]interface{}, map[string]interface{}) {
    method, _ := request["method"].(string)

    // Example: reject everything except CatalogService.getProduct.
    if method != "CatalogService.getProduct" {
        return ctx, request, map[string]interface{}{
            "jsonrpc": "2.0",
            "id":      request["id"],
            "error":   map[string]interface{}{"code": -32001, "message": "Unauthorized"},
        }
    }

    // Example: attach the method for the handler; the request is unchanged.
    return context.WithValue(ctx, rpcMethodContext, method), request, nil
})
```

with `const rpcMethodContext contextKey = "rpc-method"` declared alongside your
other typed keys.

The generated server also exposes `UseRPC(pulserpc.Middleware)` directly on
`*PulseRPCServer`, plus `CallWithContext(ctx, request)` and the background
convenience `Call(request)`.

## Migration from context-free handlers

Generated Go APIs are context-aware as of v0.4.0:

- Add `ctx context.Context` as the first parameter of every generated interface
  method, client method, and test handler, and pass it through to the transport.
- Generated interface methods now return `(T, error)` for value-returning
  methods and `error` for void methods. The interface previously omitted
  `error`.
- `Transport.Call` and `NotifyTransport.Notify` take a context first; custom
  transports must add it. The runtime library's `Transport.Request` /
  `RequestWithID`, `Client.Call`, and `Client.Notify` also take a context.
- `Register` and `AddHandler` return `error`. Statement-form calls such as
  `server.Register("A", &AImpl{})` still compile, but check the error at
  startup. Registration rejects a handler method without a leading
  `context.Context` or without the declared `error` result.
- `Server.Call(request)` keeps working and uses `context.Background()`; use
  `CallWithContext(ctx, request)` to propagate cancellation.
- Optional one-shot bootstrap verification preserves values from the caller
  context but ignores its cancellation, so a cancelled first call cannot
  permanently skip the contract check.

The JSON-RPC wire format, validation rules, error codes, and golden responses
are unchanged.

## Validation

PulseRPC automatically validates:
- Required fields are present
- Types match IDL definition
- Enum values are valid

```go
// This will return error (-32602) if validation fails
ctx := context.Background()
cart, err := cartService.AddToCart(ctx, checkout.AddToCartRequest{
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
| Request body (default) | 1 MiB | `-32600` `"Request body too large"` |
| Request body (`MaxRequestBodyBytes > 0`) | configured | `-32700` `"Parse error"` |
| Response body (client read) | 10 MiB | client error |
| Batch entries | 100 | `-32600` `"Batch exceeds maximum size"` |
| HTTP client timeout | 30 s | transport error |

Additional behavior:

- Non-POST requests return HTTP **405** with a JSON-RPC `-32600` body and
  `"id": null`.
- Handler errors and panics return `-32603` `"Internal error"` with no `data`
  member. Details are logged server-side only.
- Requests with a boolean, object, or array `id` are rejected with `-32600`.
  An explicit `"id": null` is a notification and receives no response unless
  `RequireNonNullID` is enabled, which rejects absent and null ids with
  `-32600`.
- Numeric ids are decoded with `json.Decoder.UseNumber()` and echoed exactly, so
  integers above 2^53 do not lose precision.
- Batch responses keep the array shape. Non-object batch entries receive their
  own `-32600`; notification-only batches return HTTP 204 with no body.
- `NewHTTPTransport` accepts optional custom headers and removes a trailing `/`
  from the base URL.
- Generated client calls take a context first and use
  `http.NewRequestWithContext`, so cancelling the context aborts the in-flight
  HTTP request. `Server.Call` uses `context.Background()`; use
  `CallWithContext` to propagate a caller context.
- `UseHTTP` runs once per HTTP request and `UseRPC` runs once per JSON-RPC
  request (per batch member). Middleware does not change the wire format.
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
        Items:    []checkout.CartItem{...},
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

func (s *CartService) GetCart(ctx context.Context, cartId string) (*checkout.Cart, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.carts[cartId], nil
}

func (s *CartService) AddToCart(ctx context.Context, req checkout.AddToCartRequest) (checkout.Cart, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    // ... modify s.carts
}
```
