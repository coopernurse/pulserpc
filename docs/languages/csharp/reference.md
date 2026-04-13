---
title: C# Reference
layout: default
---

# C# Reference

## Type Mappings

| IDL Type | C# Type | Example |
|----------|---------|---------|
| `string` | `string` | `"hello"` |
| `int` | `long` | `42L` |
| `float` | `double` | `3.14` |
| `bool` | `bool` | `true`, `false` |
| `[]Type` | `List<Type>` | `new List<int> { 1, 2, 3 }` |
| `map[string]Type` | `Dictionary<string, Type>` | `new Dictionary<string, Type>` |
| `Enum` | `enum` | `OrderStatus.Pending` |
| `Struct` | Class with properties | `new Product { ... }` |
| `T [optional]` | `Nullable<T>` or default | `null` or `default` |

## Generated Classes

Each struct in your IDL becomes a C# class:

```csharp
using Checkout;

// Create instances using object initializer
var product = new Product {
    ProductId = "prod001",
    Name = "Wireless Mouse",
    Description = "Ergonomic mouse",
    Price = 29.99,
    Stock = 50,
    ImageUrl = "https://example.com/mouse.jpg"  // optional field
};

var cart = new Cart {
    CartId = "cart_1234",
    Items = new List<CartItem>(),
    Subtotal = 0.0
};
```

## Optional Fields

Optional fields can be `null`:

```csharp
// Create with optional field
var product = new Product {
    ProductId = "prod001",
    Name = "Wireless Mouse",
    Price = 29.99,
    Stock = 50,
    ImageUrl = null  // optional field can be null
};

// Check optional field
if (product.ImageUrl != null) {
    Console.WriteLine(product.ImageUrl);
}
```

## Enums

Enums use proper C# enum with constants:

```csharp
using Checkout;

// Use enum constants
var order = new Order {
    OrderId = "order_123",
    Cart = cart,
    ShippingAddress = address,
    PaymentMethod = PaymentMethod.CreditCard,
    Status = OrderStatus.Pending,
    Total = 59.98,
    CreatedAt = DateTimeOffset.UtcNow.ToUnixTimeSeconds()
};

// Compare enums
if (order.Status == OrderStatus.Pending) {
    Console.WriteLine("Order is pending");
}
```

## Error Handling

Throw `RPCError` with custom codes:

```csharp
using PulseRPC;

// Standard JSON-RPC errors
throw new RPCError(-32602, "Invalid params");

// Custom application errors (use codes >= 1000)
throw new RPCError(1001, "CartNotFound: Cart does not exist");
throw new RPCError(1002, "CartEmpty: Cannot create order from empty cart");
```

Common error codes:
- `-32700`: Parse error
- `-32600`: Invalid request
- `-32601`: Method not found
- `-32602`: Invalid params
- `-32603`: Internal error
- `1000+`: Custom application errors

## Server Implementation

Implement generated interfaces:

```csharp
using PulseRPC;
using Checkout;

class CatalogServiceImpl : ICatalogService {
    private List<Product> products = new List<Product> {
        new Product { ProductId = "p1", Name = "Item 1", Price = 10.0, Stock = 5 },
        new Product { ProductId = "p2", Name = "Item 2", Price = 20.0, Stock = 3 }
    };

    public List<Product> ListProducts() {
        return products;
    }

    public Product GetProduct(string productId) {
        return products.FirstOrDefault(p => p.ProductId == productId);
    }
}

// Start server
class Program {
    static async Task Main(string[] args) {
        var server = new PulseRPCServer();
        server.RegisterCatalogService(new CatalogServiceImpl());
        await server.RunAsync("localhost", 8080);
    }
}
```

## Client Usage

```csharp
using PulseRPC;
using Checkout;

var transport = new HttpTransport("http://localhost:8080");
var catalog = new CatalogServiceClient(transport);

// Method calls return C# objects
var products = catalog.ListProducts();
foreach (var p in products) {
    Console.WriteLine($"{p.Name} - ${p.Price}");
}

// Optional methods return null if not found
var product = catalog.GetProduct("prod001");
if (product != null) {
    Console.WriteLine(product.Name);
}
```

## Async/Await Pattern

PulseRPC C# supports async/await:

```csharp
using System.Threading.Tasks;

class OrderServiceImpl : IOrderService {
    public async Task<CheckoutResponse> CreateOrder(CreateOrderRequest request) {
        // Async operations
        var orderId = await GenerateOrderIdAsync();
        var validated = await ValidateCartAsync(request.CartId);

        if (!validated) {
            throw new RPCException(1002, "CartEmpty: Cannot create order from empty cart");
        }

        return new CheckoutResponse { OrderId = orderId };
    }
}
```

## Validation

PulseRPC automatically validates:
- Required fields are present
- Types match IDL definition
- Enum values are valid

```csharp
// This will throw RPCException (-32602) if validation fails
var cart = cart.AddToCart(new AddToCartRequest {
    CartId = null,
    ProductId = "prod001",
    Quantity = 2
});
```

## LINQ Integration

Use LINQ for working with collections:

```csharp
using System.Linq;

// Filter
var inStockProducts = products.Where(p => p.Stock > 0).ToList();

// Project
var productNames = products.Select(p => p.Name).ToList();

// Find
var firstProduct = products.FirstOrDefault(p => p.ProductId == "prod001");

// Aggregate
var totalValue = products.Sum(p => p.Price * p.Stock);
```

## Best Practices

1. **Use object initializers**: Cleaner than constructor parameters
2. **Null check optionals**: Always check for null on optional fields
3. **Use LINQ**: Elegant collection manipulation
4. **Async for I/O**: Use async/await for database/network calls
5. **Dependency injection**: Inject services into constructors

## Working with Nested Structs

```csharp
// Nested structs work naturally
var order = new Order {
    OrderId = "order_123",
    Cart = new Cart {
        CartId = "cart_123",
        Items = new List<CartItem> { new CartItem { ... } },
        Subtotal = 59.98
    },
    ShippingAddress = new Address {
        Street = "123 Main St",
        City = "San Francisco",
        State = "CA",
        ZipCode = "94105",
        Country = "USA"
    },
    PaymentMethod = PaymentMethod.CreditCard,
    Status = OrderStatus.Pending,
    Total = 59.98,
    CreatedAt = DateTimeOffset.UtcNow.ToUnixTimeSeconds()
};
```

## .NET Project Integration

Generated code includes `.csproj` file:

```bash
# Build
dotnet build

# Run server
dotnet run --project Server.csproj

# Run client
dotnet run --project Client.csproj
```

## Using with ASP.NET Core

```csharp
using Microsoft.AspNetCore.Mvc;

[ApiController]
[Route("api")]
public class CheckoutController : ControllerBase {
    private readonly ICartService _cartService;

    public CheckoutController(ICartService cartService) {
        _cartService = cartService;
    }

    [HttpPost("cart")]
    public Cart AddToCart([FromBody] AddToCartRequest request) {
        return _cartService.AddToCart(request);
    }
}
```

## Dependency Injection

```csharp
using Microsoft.Extensions.DependencyInjection;

// Startup.cs
public void ConfigureServices(IServiceCollection services) {
    services.AddSingleton<ICatalogService, CatalogServiceImpl>();
    services.AddSingleton<ICartService, CartServiceImpl>();
    services.AddSingleton<IOrderService, OrderServiceImpl>();
}

// Controller
public class CartController : ControllerBase {
    private readonly ICartService _cartService;

    public CartController(ICartService cartService) {
        _cartService = cartService;
    }
}
```

## Configuration

Add to `appsettings.json`:

```json
{
  "PulseRPC": {
    "ServerUrl": "http://localhost:8080",
    "Timeout": 30000
  }
}
```

Load in startup:

```csharp
var config = new ConfigurationBuilder()
    .AddJsonFile("appsettings.json")
    .Build();

var serverUrl = config["PulseRPC:ServerUrl"];
```

## Exception Handling

```csharp
try {
    var response = orders.CreateOrder(request);
    Console.WriteLine($"Order created: {response.OrderId}");
}
catch (RPCException ex) {
    // Handle application errors
    Console.WriteLine($"Error {ex.Code}: {ex.Message}");
}
catch (Exception ex) {
    // Handle system errors
    Console.WriteLine($"System error: {ex.Message}");
}
```

## Serialization

PulseRPC C# uses System.Text.Json or Newtonsoft.Json:

```csharp
using System.Text.Json;

// Custom serialization options
var options = new JsonSerializerOptions {
    PropertyNamingPolicy = JsonNamingPolicy.CamelCase,
    WriteIndented = true
};

var json = JsonSerializer.Serialize(product, options);
```

## Contract Compatibility Verification

PulseRPC provides contract compatibility verification to detect when client and server IDLs have diverged. This helps catch integration issues early.

### Using Auditors

```csharp
using PulseRPC;

// Create client with FailFast auditor - throws if incompatible contract detected
var client = new CatalogServiceClient(transport)
    .WithAuditor(new FailFastAuditor())
    .VerifyOnBootstrap();

// Or use explicit verification
var result = await client.VerifyCompatibility();
if (!result.Compatible) {
    Console.WriteLine($"Contract incompatible: {result.Deltas.Count} deltas found");
    foreach (var delta in result.Deltas) {
        Console.WriteLine($"  - {delta.EntityType}: {delta.Description}");
    }
}
```

### Built-in Auditors

| Auditor | Behavior |
|---------|----------|
| `NoOpAuditor` | Does nothing - inspect results directly |
| `LoggingAuditor` | Logs at appropriate levels (Error/Warning/Info) |
| `FailFastAuditor` | Throws exception if any Error-level deltas found |

### Severity Levels

| Level | Meaning |
|-------|---------|
| Error | Breaking change - will cause runtime failures |
| Warning | Potentially problematic - may cause issues |
| Info | Non-breaking - server can ignore extra fields |

### VerificationResult Properties

| Property | Type | Description |
|----------|------|-------------|
| `Compatible` | `bool` | True if no Error-level deltas |
| `ServerChecksum` | `string` | SHA-256 hash of server IDL |
| `ClientChecksum` | `string` | SHA-256 hash of client IDL |
| `Deltas` | `List<ContractDelta>` | List of changes detected |
| `Timestamp` | `DateTime` | When verification ran |

### ContractDelta Properties

| Property | Type | Description |
|----------|------|-------------|
| `EntityType` | `EntityType` | Type of entity (Interface, Struct, Enum, Error) |
| `EntityName` | `string` | Name of the entity |
| `ChangeType` | `ChangeType` | Type of change (Added, Removed, Modified) |
| `Direction` | `Direction` | ClientToServer or ServerToClient |
| `Severity` | `Severity` | Error, Warning, or Info |
| `Description` | `string` | Human-readable description |
