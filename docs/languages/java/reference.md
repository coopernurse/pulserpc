---
title: Java Reference
layout: default
---

# Java Reference

## Package Configuration

The `-package` flag specifies the base Java package for generated code. With multi-namespace support, namespaces become sub-packages under this base.

### Package Structure

When using `-package com.example.myapp` with namespace `checkout`:

| Generated File | Package Declaration |
|----------------|---------------------|
| `src/main/java/com/example/myapp/checkout/Checkout.java` | `package com.example.myapp.checkout;` |
| `src/main/java/com/example/myapp/checkout/CheckoutService.java` | `package com.example.myapp.checkout;` |
| `src/main/java/com/example/myapp/checkout/CheckoutClient.java` | `package com.example.myapp.checkout;` |
| `src/main/java/pulserpc/RPCError.java` | `package pulserpc;` |

**Key points:**
- Namespace packages are fully-qualified: `{package}.{namespace}`
- Runtime always lives in `pulserpc` package (not qualified by `-package`)
- Directory structure mirrors package structure

### Multi-Namespace Projects

For projects with multiple namespaces (e.g., `common.pulse`, `book.pulse`, `user.pulse`):

```
src/main/java/
├── pulserpc/                          # Runtime (package pulserpc)
│   ├── RPCError.java
│   ├── Client.java
│   └── Server.java
├── com/example/myapp/common/          # common namespace (package com.example.myapp.common)
│   ├── CommonTypes.java
│   └── CommonService.java
├── com/example/myapp/book/            # book namespace (package com.example.myapp.book)
│   ├── Book.java
│   └── BookService.java
└── com/example/myapp/user/            # user namespace (package com.example.myapp.user)
    ├── User.java
    └── UserService.java
```

Cross-namespace imports are fully-qualified:
```java
import com.example.myapp.common.CommonTypes;
import com.example.myapp.book.BookService;
```

Runtime imports use simple package:
```java
import pulserpc.RPCError;
import pulserpc.*;
```

### Maven Configuration

Add generated sources to your `pom.xml`:

```xml
<build>
    <sourceDirectory>${project.basedir}/src/main/java</sourceDirectory>
    <plugins>
        <plugin>
            <groupId>org.apache.maven.plugins</groupId>
            <artifactId>maven-compiler-plugin</artifactId>
            <version>3.11.0</version>
            <configuration>
                <source>11</source>
                <target>11</target>
            </configuration>
        </plugin>
    </plugins>
</build>
```

### Gradle Configuration

```groovy
sourceSets {
    main {
        java {
            srcDirs += 'src/main/java'
        }
    }
}
```

### IDE Configuration

**IntelliJ IDEA:**
1. Mark `src/main/java` as a source root
2. The package structure will auto-resolve from directory structure
3. Right-click project → Mark Directory as → Sources Root

**Eclipse:**
1. Project → Properties → Java Build Path → Source tab
2. Add `src/main/java` to source folders

## Type Mappings

| IDL Type | Java Type | Example |
|----------|-----------|---------|
| `string` | `String` | `"hello"` |
| `int` | `Long` | `42L` |
| `float` | `Double` | `3.14` |
| `bool` | `Boolean` | `true`, `false` |
| `[]Type` | `List<Type>` | `Arrays.asList(1, 2, 3)` |
| `map[string]Type` | `Map<String, Type>` | `Collections.singletonMap("key", "value")` |
| `Enum` | `Enum` | `OrderStatus.PENDING` |
| `Struct` | Class with getters | `new Product(...)` |
| `T [optional]` | `Optional<T>` | `Optional.of(value)` |

## Generated Classes

Each struct in your IDL becomes a Java class with builder pattern or constructor:

```java
import checkout.*;

// Create instances using constructor
Product product = new Product(
    "prod001",
    "Wireless Mouse",
    "Ergonomic mouse",
    29.99,
    50,
    "https://example.com/mouse.jpg"  // optional field
);

Cart cart = new Cart(
    "cart_1234",
    new ArrayList<>(),
    0.0
);
```

## Optional Fields

Optional return types use `Optional<T>`:

```java
import java.util.Optional;

// Methods with optional return type
public Optional<Product> getProduct(String productId) {
    for (Product p : products) {
        if (p.getProductId().equals(productId)) {
            return Optional.of(p);
        }
    }
    return Optional.empty();  // Not found
}

// Client usage
Optional<Product> result = catalog.getProduct("prod001");
if (result.isPresent()) {
    Product product = result.get();
    System.out.println(product.getName());
}
```

## Enums

Enums use proper Java enum with constants:

```java
import checkout.*;

// Use enum constants
Order order = new Order(
    orderId,
    cart,
    shippingAddress,
    PaymentMethod.CREDIT_CARD,
    OrderStatus.PENDING,
    total,
    createdAt
);

// Compare enums
if (order.getStatus() == OrderStatus.PENDING) {
    System.out.println("Order is pending");
}
```

## Error Handling

Throw `RPCError` with custom codes:

```java
import com.bitmechanic.pulserpc.RPCError;

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

```java
import checkout.*;
import com.bitmechanic.pulserpc.*;

class CatalogServiceImpl implements CatalogService {
    private List<Product> products = Arrays.asList(
        new Product("p1", "Item 1", "Description", 10.0, 5),
        new Product("p2", "Item 2", "Description", 20.0, 3)
    );

    public List<Product> listProducts() {
        return products;
    }

    public Optional<Product> getProduct(String productId) {
        return products.stream()
            .filter(p -> p.getProductId().equals(productId))
            .findFirst();
    }
}

// Start server
public static void main(String[] args) throws Exception {
    JsonParser jsonParser = new JacksonJsonParser(); // or GsonJsonParser
    Server server = new Server(8080, jsonParser);
    server.register("CatalogService", new CatalogServiceImpl());
    server.start();
}
```

## Client Usage

```java
import checkout.*;
import com.bitmechanic.pulserpc.*;

JsonParser jsonParser = new JacksonJsonParser();
Transport transport = new HTTPTransport("http://localhost:8080", jsonParser);
CatalogServiceClient catalog = new CatalogServiceClient(transport, jsonParser);

// Method calls return Java objects
List<Product> products = catalog.listProducts();
for (Product p : products) {
    System.out.println(p.getName() + " - $" + p.getPrice());
}
```

## Client Usage

```java
import checkout.*;
import com.bitmechanic.pulserpc.*;

Transport transport = new HTTPTransport("http://localhost:8080");
CatalogServiceClient catalog = new CatalogServiceClient(transport);

// Method calls return Java objects
List<Product> products = catalog.listProducts();
for (Product p : products) {
    System.out.println(p.getName() + " - $" + p.getPrice());
}

// Optional methods return Optional
Optional<Product> result = catalog.getProduct("prod001");
result.ifPresent(product -> {
    System.out.println(product.getName());
});
```

## JSON Library Support

PulseRPC supports both Jackson and Gson. Configure in `pom.xml`:

```xml
<!-- Jackson (default) -->
<dependency>
    <groupId>com.fasterxml.jackson.core</groupId>
    <artifactId>jackson-databind</artifactId>
    <version>2.15.0</version>
</dependency>

<!-- OR Gson -->
<dependency>
    <groupId>com.google.code.gson</groupId>
    <artifactId>gson</artifactId>
    <version>2.10.1</version>
</dependency>
```

## Validation

PulseRPC automatically validates:
- Required fields are present
- Types match IDL definition
- Enum values are valid

```java
// This will throw RPCError (-32602) if validation fails
Cart cart = cart.addToCart(new AddToCartRequest(
    null,  // cartId is optional
    "prod001",
    2
));
```

## Best Practices

1. **Use Optional correctly**: Return `Optional.of()` for values, `Optional.empty()` for null
2. **Stream for collections**: Use Java streams for filtering and mapping
3. **Immutable where possible**: Consider making generated classes immutable
4. **Use Jackson annotations**: Add `@JsonProperty` for custom field names
5. **Handle RPCError**: Catch and handle RPC errors appropriately

## Working with Nested Structs

```java
// Nested structs work naturally
Order order = new Order(
    "order_123",
    new Cart(
        "cart_123",
        Arrays.asList(new CartItem(...)),
        59.98
    ),
    new Address(
        "123 Main St",
        "San Francisco",
        "CA",
        "94105",
        "USA"
    ),
    PaymentMethod.CREDIT_CARD,
    OrderStatus.PENDING,
    59.98,
    System.currentTimeMillis() / 1000
);
```

## Maven Integration

Generated code includes `pom.xml` for building:

```bash
# Compile
mvn compile

# Run server
mvn exec:java -Dexec.mainClass="Server"

# Run client
mvn exec:java -Dexec.mainClass="Client"
```

## Using with Spring Boot

```java
@RestController
@RequestMapping("/api")
public class CheckoutController {
    private final CartService cartService;

    public CheckoutController(CartService cartService) {
        this.cartService = cartService;
    }

    @PostMapping("/cart")
    public Cart addToCart(@RequestBody AddToCartRequest request) {
        return cartService.addToCart(request);
    }
}
```

## Contract Compatibility Verification

PulseRPC provides contract compatibility verification to detect when client and server IDLs have diverged. This helps catch integration issues early.

### Using Auditors

```java
import com.bitmechanic.pulserpc.*;

// Create client with FailFast auditor - throws if incompatible contract detected
Client client = new Client("http://localhost:8080", jsonParser)
    .withAuditor(ContractAuditor.failFast())
    .verifyOnBootstrap();

// Or use explicit verification
VerificationResult result = client.verifyCompatibility();
if (!result.isCompatible()) {
    System.out.println("Contract incompatible: " + result.getDeltas().size() + " deltas found");
    for (ContractDelta delta : result.getDeltas()) {
        System.out.println("  - " + delta.getEntityType() + ": " + delta.getDescription());
    }
}
```

### Built-in Auditors

| Auditor | Behavior |
|---------|----------|
| `ContractAuditor.noOp()` | Does nothing - inspect results directly |
| `ContractAuditor.logging()` | Logs at appropriate levels (Error/Warning/Info) |
| `ContractAuditor.failFast()` | Throws exception if any Error-level deltas found |

### Severity Levels

| Level | Meaning |
|-------|---------|
| Error | Breaking change - will cause runtime failures |
| Warning | Potentially problematic - may cause issues |
| Info | Non-breaking - server can ignore extra fields |

### VerificationResult Properties

| Property | Type | Description |
|----------|------|-------------|
| `isCompatible()` | `boolean` | True if no Error-level deltas |
| `getServerChecksum()` | `String` | SHA-256 hash of server IDL |
| `getClientChecksum()` | `String` | SHA-256 hash of client IDL |
| `getDeltas()` | `List<ContractDelta>` | List of changes detected |
| `getTimestamp()` | `long` | When verification ran (epoch ms) |

### ContractDelta Properties

| Property | Type | Description |
|----------|------|-------------|
| `getEntityType()` | `EntityType` | Type of entity (Interface, Struct, Enum, Error) |
| `getEntityName()` | `String` | Name of the entity |
| `getChangeType()` | `ChangeType` | Type of change (Added, Removed, Modified) |
| `getDirection()` | `Direction` | ClientToServer or ServerToClient |
| `getSeverity()` | `Severity` | Error, Warning, or Info |
| `getDescription()` | `String` | Human-readable description |
