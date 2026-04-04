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

{% include quickstart/checkout.idl %}

## Defining Service Errors

The IDL uses the `errors` keyword to declare error codes and `raises()` clauses to specify which errors each method can raise:

```idl
errors {
    1001 CartNotFound "Cart doesn't exist"
    1002 CartEmpty "Cart has no items"
    1003 PaymentFailed "Payment method rejected"
    1004 OutOfStock "Insufficient inventory"
    1005 InvalidAddress "Shipping address validation failed"
}

interface OrderService {
    createOrder(request CreateOrderRequest) CheckoutResponse raises(CartNotFound, CartEmpty, PaymentFailed, OutOfStock, InvalidAddress)
}
```

For details on error handling, see [Error Handling](../../idl-guide/errors.html).

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

{% include quickstart/go-server.md %}

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

{% include quickstart/go-client.md %}

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
