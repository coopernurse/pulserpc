---
title: Java Quickstart
layout: default
---

# Java Quickstart

Build a complete PulseRPC service in Java with our e-commerce checkout example.

## Prerequisites

- Java 11 or later
- Maven 3.6 or later
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

Generate the Java code from your IDL:

```bash
pulserpc -plugin java-client-server -base-package com.example.myapp checkout.pulse
```

This creates:
- `src/main/java/com/example/myapp/` - Type definitions and Server/Client frameworks
- `src/main/resources/idl.json` - IDL metadata
- `src/main/java/com/bitmechanic/pulserpc/` - Runtime library

Create a `pom.xml` in your project root:

```xml
<project>
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>myapp</artifactId>
    <version>1.0-SNAPSHOT</version>

    <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
        <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    </properties>

    <dependencies>
        <dependency>
            <groupId>com.fasterxml.jackson.core</groupId>
            <artifactId>jackson-databind</artifactId>
            <version>2.15.2</version>
        </dependency>
    </dependencies>

    <build>
        <plugins>
            <plugin>
                <groupId>org.codehaus.mojo</groupId>
                <artifactId>exec-maven-plugin</artifactId>
                <version>3.1.0</version>
            </plugin>
        </plugins>
    </build>
</project>
```

## 3. Implement the Server (10-15 min)

Create `src/main/java/com/example/myapp/MyServer.java` that implements your service handlers:

```java
package com.example.myapp;

import com.example.myapp.checkout.*;
import com.bitmechanic.pulserpc.*;
import java.util.*;

public class MyServer {
    static List<Product> products = new ArrayList<>();

    static {
        Product p1 = new Product();
        p1.setProductId("prod001");
        p1.setName("Wireless Mouse");
        p1.setDescription("Ergonomic mouse");
        p1.setPrice(29.99);
        p1.setStock(50);
        p1.setImageUrl("https://example.com/mouse.jpg");
        products.add(p1);

        Product p2 = new Product();
        p2.setProductId("prod002");
        p2.setName("Mechanical Keyboard");
        p2.setDescription("RGB keyboard");
        p2.setPrice(89.99);
        p2.setStock(25);
        p2.setImageUrl("https://example.com/keyboard.jpg");
        products.add(p2);
    }

    static Map<String, Cart> carts = new HashMap<>();
    static Map<String, Order> orders = new HashMap<>();

    static class CatalogServiceImpl implements CatalogService {
        public List<Product> listProducts() {
            return products;
        }

        public Product getProduct(String productId) {
            return products.stream()
                .filter(p -> p.getProductId().equals(productId))
                .findFirst().orElse(null);
        }
    }

    static class CartServiceImpl implements CartService {
        public Cart addToCart(AddToCartRequest request) {
            String cartId = request.getCartId();
            if (cartId == null || cartId.isEmpty()) {
                cartId = "cart_" + (int)(Math.random() * 9000 + 1000);
            }

            Cart cart = carts.get(cartId);
            if (cart == null) {
                cart = new Cart();
                cart.setCartId(cartId);
                cart.setItems(new ArrayList<>());
                cart.setSubtotal(0.0);
                carts.put(cartId, cart);
            }

            Product product = products.stream()
                .filter(p -> p.getProductId().equals(request.getProductId()))
                .findFirst().orElseThrow(() -> new RPCError(-32602, "Product not found"));

            CartItem item = new CartItem();
            item.setProductId(request.getProductId());
            item.setQuantity(request.getQuantity());
            item.setPrice(product.getPrice());
            cart.getItems().add(item);
            cart.setSubtotal(cart.getItems().stream().mapToDouble(i -> i.getPrice() * i.getQuantity()).sum());

            return cart;
        }

        public Cart getCart(String cartId) {
            return carts.get(cartId);
        }

        public boolean clearCart(String cartId) {
            Cart cart = carts.get(cartId);
            if (cart != null) {
                cart.getItems().clear();
                cart.setSubtotal(0.0);
                return true;
            }
            return false;
        }
    }

    static class OrderServiceImpl implements OrderService {
        public CheckoutResponse createOrder(CreateOrderRequest request) {
            Cart cart = carts.get(request.getCartId());
            if (cart == null) {
                throw new RPCError(1001, "CartNotFound: Cart does not exist");
            }

            if (cart.getItems().isEmpty()) {
                throw new RPCError(1002, "CartEmpty: Cannot create order from empty cart");
            }

            String orderId = "order_" + (int)(Math.random() * 90000 + 10000);
            Order order = new Order();
            order.setOrderId(orderId);
            order.setCart(cart);
            order.setShippingAddress(request.getShippingAddress());
            order.setPaymentMethod(request.getPaymentMethod());
            order.setStatus(OrderStatus.pending);
            order.setTotal(cart.getSubtotal());
            order.setCreatedAt((int)(System.currentTimeMillis() / 1000));
            orders.put(orderId, order);

            CheckoutResponse resp = new CheckoutResponse();
            resp.setOrderId(orderId);
            resp.setMessage("Order created successfully");
            return resp;
        }

        public Order getOrder(String orderId) {
            return orders.get(orderId);
        }
    }

    public static void main(String[] args) throws Exception {
        JsonParser jsonParser = new JacksonJsonParser();
        Server server = new Server(8080, jsonParser);
        server.register("CatalogService", new CatalogServiceImpl());
        server.register("CartService", new CartServiceImpl());
        server.register("OrderService", new OrderServiceImpl());
        server.start();
    }
}
```

Start your server:

```bash
mvn compile exec:java -Dexec.mainClass="com.example.myapp.MyServer"
```

## 4. Implement the Client (5-10 min)

Create `src/main/java/com/example/myapp/MyClient.java` to call your service:

```java
package com.example.myapp;

import com.example.myapp.checkout.*;
import com.bitmechanic.pulserpc.*;
import java.util.*;

public class MyClient {
    public static void main(String[] args) throws Exception {
        JsonParser jsonParser = new JacksonJsonParser();
        Transport transport = new HTTPTransport("http://localhost:8080", jsonParser);
        CatalogServiceClient catalog = new CatalogServiceClient(transport, jsonParser);
        CartServiceClient cart = new CartServiceClient(transport, jsonParser);
        OrderServiceClient orders = new OrderServiceClient(transport, jsonParser);

        // List products
        List<Product> products = catalog.listProducts();
        System.out.println("=== Products ===");
        for (Product p : products) {
            System.out.println(p.getName() + " - $" + p.getPrice());
        }

        // Add to cart
        AddToCartRequest addReq = new AddToCartRequest();
        addReq.setProductId(products.get(0).getProductId());
        addReq.setQuantity(2);
        Cart result = cart.addToCart(addReq);
        System.out.println("\nCart: " + result.getCartId());

        // Create order
        CreateOrderRequest orderReq = new CreateOrderRequest();
        orderReq.setCartId(result.getCartId());
        
        Address addr = new Address();
        addr.setStreet("123 Main St");
        addr.setCity("San Francisco");
        addr.setState("CA");
        addr.setZipCode("94105");
        addr.setCountry("USA");
        orderReq.setShippingAddress(addr);
        orderReq.setPaymentMethod(PaymentMethod.credit_card);

        CheckoutResponse response = orders.createOrder(orderReq);
        System.out.println("✓ Order created: " + response.getOrderId());
    }
}
```

Run your client:

```bash
mvn compile exec:java -Dexec.mainClass="com.example.myapp.MyClient"
```

## Error Codes

Throw `RPCError` with custom error codes:

```java
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

- [Java Reference](reference.html) - Type mappings and Jackson/GSon support
- [IDL Syntax](../../idl-guide/syntax.html) - Full IDL reference
