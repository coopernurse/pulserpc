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

        // Test error case: empty cart
        System.out.println("\n=== Testing Error Case ===");
        cart.clearCart(result.getCartId());
        try {
            orders.createOrder(orderReq);
            System.out.println("✗ Should have failed!");
        } catch (RPCError e) {
            System.out.println("✓ Got expected error: " + e.getCode() + " - " + e.getMessage());
        }
    }
}
