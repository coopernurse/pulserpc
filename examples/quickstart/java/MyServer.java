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
