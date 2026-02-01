---
title: C# Quickstart
layout: default
---

# C# Quickstart

Build a complete PulseRPC service in C# with our e-commerce checkout example.

## Prerequisites

- .NET 8.0 or later
- PulseRPC CLI installed ([Installation Guide](../../get-started/installation))

## 1. Define the Service (2 min)

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

Generate the C# code from your IDL:

```bash
pulserpc -plugin csharp-client-server checkout.pulse
```

This creates:
- `Checkout.cs` - Type definitions (in `checkout` namespace)
- `Server.cs` - RPC server framework (in `PulseRPC` namespace)
- `Client.cs` - RPC client framework (in `PulseRPC` namespace)
- `Contract.cs` - Shared interfaces and IDL metadata
- `PulseRPC/` - Runtime library

**Pro tip:** Organize your generated code into a `Shared/` directory to keep things tidy:

```bash
mkdir Shared TestServer TestClient
mv Checkout.cs Client.cs Contract.cs Server.cs PulseRPC/ Shared/
```

## 3. Implement the Server (10-15 min)

Create a server project file `TestServer/TestServer.csproj`:

```xml
<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <OutputType>Exe</OutputType>
  </PropertyGroup>

  <ItemGroup>
    <FrameworkReference Include="Microsoft.AspNetCore.App" />
  </ItemGroup>

  <ItemGroup>
    <Compile Include="../Shared/Checkout.cs" />
    <Compile Include="../Shared/Contract.cs" />
    <Compile Include="../Shared/Server.cs" />
    <Compile Include="../Shared/pulserpc/*.cs" />
  </ItemGroup>

</Project>
```

Create `TestServer/MyServer.cs` that implements your service handlers:

```csharp
using System;
using System.Collections.Generic;
using System.Linq;
using PulseRPC;
using checkout;

public class CatalogServiceImpl : ICatalogService
{
    private static readonly List<Product> Products = new List<Product>
    {
        new Product { ProductId = "prod001", Name = "Wireless Mouse", Description = "Ergonomic mouse", Price = 29.99, Stock = 50, ImageUrl = "https://example.com/mouse.jpg" },
        new Product { ProductId = "prod002", Name = "Mechanical Keyboard", Description = "RGB keyboard", Price = 89.99, Stock = 25, ImageUrl = "https://example.com/keyboard.jpg" }
    };

    public List<Product> listProducts()
    {
        return Products;
    }

    public Product? getProduct(string productId)
    {
        return Products.FirstOrDefault(p => p.ProductId == productId);
    }

}

public class CartServiceImpl : ICartService
{
    internal readonly Dictionary<string, Cart> _carts = new Dictionary<string, Cart>();
    private readonly CatalogServiceImpl _catalogService;

    public CartServiceImpl(CatalogServiceImpl catalogService)
    {
        _catalogService = catalogService;
    }

    public Cart addToCart(AddToCartRequest request)
    {
        var cartId = request.CartId ?? $"cart_{new Random().Next(1000, 9999)}";

        if (!_carts.TryGetValue(cartId, out var cart))
        {
            cart = new Cart { CartId = cartId, Items = new List<CartItem>(), Subtotal = 0 };
            _carts[cartId] = cart;
        }

        var product = _catalogService.listProducts().FirstOrDefault(p => p.ProductId == request.ProductId);
        if (product == null)
            throw new RPCError(-32602, "Product not found");

        cart.Items.Add(new CartItem { ProductId = request.ProductId, Quantity = request.Quantity, Price = (double)product.Price });
        cart.Subtotal = (double)cart.Items.Sum(i => i.Price * i.Quantity);

        return cart;
    }

    public Cart? getCart(string cartId)
    {
        return _carts.TryGetValue(cartId, out var cart) ? cart : null;
    }

    public bool clearCart(string cartId)
    {
        if (_carts.TryGetValue(cartId, out var cart))
        {
            cart.Items.Clear();
            cart.Subtotal = 0;
            return true;
        }
        return false;
    }
}

class OrderServiceImpl : IOrderService
{
    private readonly Dictionary<string, Cart> _carts;
    private readonly Dictionary<string, Order> _orders = new Dictionary<string, Order>();

    public OrderServiceImpl(Dictionary<string, Cart> carts)
    {
        _carts = carts;
    }

    public CheckoutResponse createOrder(CreateOrderRequest request)
    {
        if (!_carts.TryGetValue(request.CartId, out var cart))
            throw new RPCError(1001, "CartNotFound: Cart does not exist");

        if (cart.Items.Count == 0)
            throw new RPCError(1002, "CartEmpty: Cannot create order from empty cart");

        var orderId = $"order_{new Random().Next(10000, 99999)}";
        var order = new Order
        {
            OrderId = orderId,
            Cart = cart,
            ShippingAddress = request.ShippingAddress,
            PaymentMethod = request.PaymentMethod,
            Status = OrderStatus.pending,
            Total = (double)cart.Subtotal,
            CreatedAt = (int)DateTimeOffset.UtcNow.ToUnixTimeSeconds()
        };

        _orders[orderId] = order;
        return new CheckoutResponse { OrderId = orderId };
    }

    public Order? getOrder(string orderId)
    {
        return _orders.TryGetValue(orderId, out var order) ? order : null;
    }
}

class Program
{
    static async Task Main(string[] args)
    {
        var server = new PulseRPCServer();
        var catalogService = new CatalogServiceImpl();
        var cartService = new CartServiceImpl(catalogService);

        server.RegisterCatalogService(catalogService);
        server.RegisterCartService(cartService);
        server.RegisterOrderService(new OrderServiceImpl(cartService._carts));

        Console.WriteLine("Server starting on http://localhost:8080");
        await server.RunAsync("0.0.0.0", 8080);
    }
}
```

Start your server:

```bash
cd TestServer
dotnet run
```

## 4. Implement the Client (5-10 min)

Create a client project file `TestClient/TestClient.csproj`:

```xml
<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <OutputType>Exe</OutputType>
  </PropertyGroup>

  <ItemGroup>
    <FrameworkReference Include="Microsoft.AspNetCore.App" />
  </ItemGroup>

  <ItemGroup>
    <Compile Include="../Shared/Checkout.cs" />
    <Compile Include="../Shared/Contract.cs" />
    <Compile Include="../Shared/Client.cs" />
    <Compile Include="../Shared/pulserpc/*.cs" />
  </ItemGroup>

</Project>
```

Create `TestClient/MyClient.cs` to call your service:

```csharp
using System;
using System.Linq;
using System.Threading.Tasks;
using PulseRPC;
using checkout;

class Program
{
    static async Task Main(string[] args)
    {
        var transport = new HttpTransport("http://localhost:8080");
        var catalogClient = new CatalogServiceClient(transport);
        var cartClient = new CartServiceClient(transport);
        var ordersClient = new OrderServiceClient(transport);

        // List products (async - use *Async methods inside async Main)
        var products = await catalogClient.listProductsAsync();
        Console.WriteLine("=== Products ===");
        foreach (var p in products)
        {
            Console.WriteLine($"{p.Name} - ${p.Price}");
        }

        // Add to cart (async)
        var result = await cartClient.addToCartAsync(new AddToCartRequest
        {
            ProductId = products[0].ProductId,
            Quantity = 2
        });
        Console.WriteLine($"\nCart: {result.CartId}");

        // Create order (async)
        var response = await ordersClient.createOrderAsync(new CreateOrderRequest
        {
            CartId = result.CartId,
            ShippingAddress = new Address
            {
                Street = "123 Main St",
                City = "San Francisco",
                State = "CA",
                ZipCode = "94105",
                Country = "USA"
            },
            PaymentMethod = PaymentMethod.credit_card
        });
        Console.WriteLine($"Order created: {response.OrderId}");
    }
}
```

Run your client:

```bash
cd TestClient
dotnet run
```

## Error Codes

Throw `RPCError` with custom error codes:

```csharp
throw new RPCError(1002, "CartEmpty: Cannot create order from empty cart");
```

| Code | Name |
|------|------|
| 1001 | CartNotFound |
| 1002 | CartEmpty |
| 1003 | PaymentFailed |
| 1004 | OutOfStock |
| 1005 | InvalidAddress |

## Next Steps

- [C# Reference](reference.html) - Type mappings and async/await patterns
- [IDL Syntax](../../idl-guide/syntax.html) - Full IDL reference
