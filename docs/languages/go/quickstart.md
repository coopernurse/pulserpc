---
title: Quickstart
parent: Go
grand_parent: Language Guides
nav_order: 2
layout: default
---

# Go Quickstart

Build a complete PulseRPC service in Go with our e-commerce checkout example.

> **Time Estimate**: 25-30 minutes
> This quickstart takes about 30 minutes to complete and covers all the essentials.

## Prerequisites

- Go 1.21 or later
- PulseRPC CLI installed ([Installation Guide](../../get-started/installation))

> **Note**: Make sure your Go version is 1.21 or later. Check with `go version`.

## 1. Create Project and Define the Service (2 min)

```bash
mkdir checkout-service && cd checkout-service
go mod init checkout-service
```

Create `checkout.pulse` with your service definition:

```idl
namespace checkout

// Enums for order status and payment methods

enum OrderStatus {
    pending
    paid
    shipped
    delivered
    cancelled
}

enum PaymentMethod {
    credit_card
    debit_card
    paypal
    apple_pay
}

// Core domain entities

struct Product {
    productId    string
    name         string
    description  string
    price        float
    stock        int
    imageUrl     string  [optional]
}

struct CartItem {
    productId    string
    quantity     int
    price        float
}

struct Cart {
    cartId       string
    items        []CartItem
    subtotal     float
}

struct Address {
    street       string
    city         string
    state        string
    zipCode      string
    country      string
}

struct Order {
    orderId           string
    cart              Cart
    shippingAddress   Address
    paymentMethod     PaymentMethod
    status            OrderStatus
    total             float
    createdAt         int
}

// Request/Response structures

struct AddToCartRequest {
    cartId       string  [optional]
    productId    string
    quantity     int
}

struct CreateOrderRequest {
    cartId              string
    shippingAddress     Address
    paymentMethod       PaymentMethod
}

struct CheckoutResponse {
    orderId      string
    message      string  [optional]
}

// Error Codes for createOrder:
//   1001 - CartNotFound: Cart doesn't exist
//   1002 - CartEmpty: Cart has no items
//   1003 - PaymentFailed: Payment method rejected
//   1004 - OutOfStock: Insufficient inventory
//   1005 - InvalidAddress: Shipping address validation failed

// Service interfaces

interface CatalogService {
    // Returns a list of all available products
    listProducts() []Product

    // Returns details for a specific product, or null if not found
    getProduct(productId string) Product  [optional]
}

interface CartService {
    // Adds an item to the cart (creates cart if cartId not provided)
    addToCart(request AddToCartRequest) Cart

    // Returns the cart contents, or null if cart doesn't exist
    getCart(cartId string) Cart  [optional]

    // Removes all items from the cart, returns true if successful
    clearCart(cartId string) bool
}

interface OrderService {
    // Converts a cart to an order
    createOrder(request CreateOrderRequest) CheckoutResponse

    // Returns the order details, or null if order doesn't exist
    getOrder(orderId string) Order  [optional]
}
```

## 2. Generate Code (1 min)

Generate the Go code from your IDL:

```bash
mkdir -p pkg/checkout
pulserpc -plugin go-client-server -dir pkg/checkout checkout.pulse
```

This creates:
- `pkg/checkout/all_types.go` - Shared type maps
- `pkg/checkout/checkout.go` - Type definitions (with embedded IDL)
- `pkg/checkout/server.go` - PulseRPC server framework
- `pkg/checkout/client.go` - HTTP client framework
- `pkg/pulserpc/rpc.go`, `types.go`, `validation.go` - Shared runtime (created once)

> **Note**: The generated code uses the namespace from your IDL as the package name (`checkout` in this example).
> The shared `pkg/pulserpc` runtime is created only once and reused for all generated packages in your project.

## 3. Project Structure

Your directory should look like this:

```
checkout-service/
├── go.mod
├── checkout.pulse
└── pkg/
    ├── pulserpc/
    │   ├── rpc.go
    │   ├── types.go
    │   └── validation.go
    └── checkout/
        ├── all_types.go
        ├── checkout.go
        ├── server.go
        └── client.go
```

## 4. Create Your Server (10-15 min)

Create `cmd/server/main.go` that implements your service handlers:

```bash
mkdir -p cmd/server
```

```go
package main

import (
    "fmt"
    "math/rand"
    "time"

    "checkout-service/pkg/checkout"
    "checkout-service/pkg/pulserpc"
)

// Helper function to create string pointers for optional fields
func strPtr(s string) *string {
    return &s
}

var products = []*checkout.Product{
    {ProductId: "prod001", Name: "Wireless Mouse", Description: "Ergonomic mouse",
     Price: 29.99, Stock: 50, ImageUrl: strPtr("https://example.com/mouse.jpg")},
    {ProductId: "prod002", Name: "Mechanical Keyboard", Description: "RGB keyboard",
     Price: 89.99, Stock: 25, ImageUrl: strPtr("https://example.com/keyboard.jpg")},
}

type CatalogService struct{}

func (s *CatalogService) ListProducts() []*checkout.Product {
    return products
}

func (s *CatalogService) GetProduct(productId string) (*checkout.Product, error) {
    for _, p := range products {
        if p.ProductId == productId {
            return p, nil
        }
    }
    return nil, nil
}

type CartService struct {
    carts map[string]*checkout.Cart
}

func NewCartService() *CartService {
    return &CartService{
        carts: make(map[string]*checkout.Cart),
    }
}

func (s *CartService) AddToCart(request *checkout.AddToCartRequest) (*checkout.Cart, error) {
    var cartId string
    if request.CartId == nil {
        cartId = fmt.Sprintf("cart_%d", rand.Intn(9000)+1000)
    } else {
        cartId = *request.CartId
    }

    cart, ok := s.carts[cartId]
    if !ok {
        cart = &checkout.Cart{CartId: cartId, Items: []checkout.CartItem{}, Subtotal: 0}
        s.carts[cartId] = cart
    }

    // Find product
    var product *checkout.Product
    for _, p := range products {
        if p.ProductId == request.ProductId {
            product = p
            break
        }
    }

    // Add item (note: Items is []CartItem, not []*checkout.CartItem)
    cart.Items = append(cart.Items, checkout.CartItem{
        ProductId: request.ProductId,
        Quantity:  request.Quantity,
        Price:     product.Price,
    })

    // Recalculate subtotal
    var subtotal float64
    for _, item := range cart.Items {
        subtotal += item.Price * float64(item.Quantity)
    }
    cart.Subtotal = subtotal

    return cart, nil
}

func (s *CartService) GetCart(cartId string) (*checkout.Cart, error) {
    return s.carts[cartId], nil
}

func (s *CartService) ClearCart(cartId string) (bool, error) {
    if cart, ok := s.carts[cartId]; ok {
        cart.Items = []checkout.CartItem{}
        cart.Subtotal = 0
        return true, nil
    }
    return false, nil
}

type OrderService struct {
    carts  map[string]*checkout.Cart
    orders map[string]*checkout.Order
}

func NewOrderService(cartService *CartService) *OrderService {
    return &OrderService{
        carts:  cartService.carts,
        orders: make(map[string]*checkout.Order),
    }
}

func (s *OrderService) CreateOrder(request *checkout.CreateOrderRequest) (*checkout.CheckoutResponse, error) {
    cart, ok := s.carts[request.CartId]
    if !ok {
        return nil, pulserpc.NewRPCError(1001, "CartNotFound: Cart does not exist")
    }

    if len(cart.Items) == 0 {
        return nil, pulserpc.NewRPCError(1002, "CartEmpty: Cannot create order from empty cart")
    }

    // Create order
    orderId := fmt.Sprintf("order_%d", rand.Intn(90000)+10000)
    order := &checkout.Order{
        OrderId:         orderId,
        Cart:            *cart,  // Dereference pointer (struct uses value type)
        ShippingAddress: request.ShippingAddress,
        PaymentMethod:   request.PaymentMethod,
        Status:          checkout.OrderStatusPending,
        Total:           cart.Subtotal,
        CreatedAt:       int(time.Now().Unix()),  // Convert int64 to int
    }
    s.orders[orderId] = order

    return &checkout.CheckoutResponse{OrderId: orderId}, nil
}

func (s *OrderService) GetOrder(orderId string) (*checkout.Order, error) {
    return s.orders[orderId], nil
}

func main() {
    server := checkout.NewPulseRPCServer("0.0.0.0", 8080)
    cartSvc := NewCartService()

    server.Register("CatalogService", &CatalogService{})
    server.Register("CartService", cartSvc)
    server.Register("OrderService", NewOrderService(cartSvc))

    fmt.Println("Server starting on http://localhost:8080")
    server.ServeForever()
}
```

> **Note**: The generated code uses build tags to separate server and client code. Use `-tags server_only` when building the server.

## 5. Build and Run Your Server

```bash
go build -tags server_only -o bin/server ./cmd/server
./bin/server
```

Or run directly:

```bash
go run -tags server_only ./cmd/server
```

## 6. Create Your Client (5-10 min)

Create `cmd/client/main.go` to call your service:

```bash
mkdir -p cmd/client
```

```go
package main

import (
    "fmt"
    "checkout-service/pkg/checkout"
)

func main() {
    transport := checkout.NewHTTPTransport("http://localhost:8080", nil)
    catalog := checkout.NewCatalogServiceClient(transport)
    cart := checkout.NewCartServiceClient(transport)
    orders := checkout.NewOrderServiceClient(transport)

    // List products
    products, _ := catalog.ListProducts()
    fmt.Println("=== Products ===")
    for _, p := range products {
        fmt.Printf("%s - $%.2f\n", p.Name, p.Price)
    }

    // Add to cart
    result, _ := cart.AddToCart(checkout.AddToCartRequest{
        ProductId: products[0].ProductId,
        Quantity:  2,
    })
    fmt.Printf("\nCart: %s, Subtotal: $%.2f\n", result.CartId, result.Subtotal)

    // Create order
    response, _ := orders.CreateOrder(checkout.CreateOrderRequest{
        CartId: result.CartId,
        ShippingAddress: checkout.Address{
            Street:  "123 Main St",
            City:    "San Francisco",
            State:   "CA",
            ZipCode: "94105",
            Country: "USA",
        },
        PaymentMethod: checkout.PaymentMethodCreditCard,
    })
    fmt.Printf("✓ Order created: %s\n", response.OrderId)
}
```

## 7. Run Your Client

```bash
go run -tags client_only ./cmd/client
```

## Error Codes

Return errors using the pulserpc package:

```go
return nil, pulserpc.NewRPCError(1002, "CartEmpty: Cannot create order from empty cart")
```

| Code | Name |
|------|------|
| 1001 | CartNotFound |
| 1002 | CartEmpty |
| 1003 | PaymentFailed |
| 1004 | OutOfStock |
| 1005 | InvalidAddress |

## Complete Example Structure

```
checkout-service/
├── go.mod                 # Your module file
├── checkout.pulse         # Your IDL
└── pkg/
    ├── pulserpc/          # Shared runtime (created once, reused)
    │   ├── rpc.go
    │   ├── types.go
    │   └── validation.go
    └── checkout/          # Generated types
        ├── all_types.go   # Shared type maps
        ├── checkout.go    # Type definitions (with embedded IDL)
        ├── server.go      # Generated server framework
        └── client.go      # Generated client framework
└── cmd/
    ├── server/
    │   └── main.go        # Your server implementation
    └── client/
        └── main.go        # Your client implementation
```

## Next Steps

- [Go Reference](reference.html) - Type mappings and patterns
- [IDL Syntax](../../idl-guide/syntax.html) - Full IDL reference
