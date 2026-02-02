#!/usr/bin/env python3
import os
from server import PulseRPCServer, CatalogService, CartService, OrderService
from pulserpc import RPCError
import random
import time

# Initialize random for deterministic behavior
random.seed(0)
_call_count = 0

# In-memory storage
products_db = [
    {
        "productId": "prod001",
        "name": "Wireless Mouse",
        "description": "Ergonomic mouse",
        "price": 29.99,
        "stock": 50,
        "imageUrl": "https://example.com/mouse.jpg",
    },
    {
        "productId": "prod002",
        "name": "Mechanical Keyboard",
        "description": "RGB keyboard",
        "price": 89.99,
        "stock": 25,
        "imageUrl": "https://example.com/keyboard.jpg",
    },
]

carts_db = {}  # cart_id -> Cart
orders_db = {}  # order_id -> Order

class CatalogServiceImpl(CatalogService):
    def listProducts(self):
        return products_db

    def getProduct(self, productId):
        for p in products_db:
            if p["productId"] == productId:
                return p
        return None

class CartServiceImpl(CartService):
    def addToCart(self, request):
        # Use a counter instead of random for deterministic cart IDs
        global _call_count
        _call_count += 1
        
        if request.get("cartId"):
            cart_id = request.get("cartId")
        else:
            cart_id = f"cart_{_call_count}"

        if cart_id not in carts_db:
            carts_db[cart_id] = {"cartId": cart_id, "items": [], "subtotal": 0.0}

        cart = carts_db[cart_id]
        product = next(
            (p for p in products_db if p["productId"] == request.get("productId")), None
        )

        if not product:
            raise RPCError(-32602, f"Product '{request.get('productId')}' not found")

        # Add or update item
        for item in cart["items"]:
            if item["productId"] == request.get("productId"):
                item["quantity"] += request.get("quantity", 0)
                item["price"] = product["price"]
                break
        else:
            cart["items"].append(
                {
                    "productId": request.get("productId"),
                    "quantity": request.get("quantity", 0),
                    "price": product["price"],
                }
            )

        cart["subtotal"] = sum(
            item["price"] * item["quantity"] for item in cart["items"]
        )
        return cart

    def getCart(self, cartId):
        return carts_db.get(cartId)

    def clearCart(self, cartId):
        if cartId in carts_db:
            carts_db[cartId]["items"] = []
            carts_db[cartId]["subtotal"] = 0.0
            return True
        return False

class OrderServiceImpl(OrderService):
    def createOrder(self, request):
        # Validate cart exists
        if request.get("cartId") not in carts_db:
            raise RPCError(1001, "CartNotFound: Cart does not exist")

        cart = carts_db[request.get("cartId")]

        # Check if cart is empty
        if not cart["items"]:
            raise RPCError(1002, "CartEmpty: Cannot create order from empty cart")

        # Validate address
        addr = request.get("shippingAddress") or {}
        if not addr.get("street") or not addr.get("city") or not addr.get("zipCode"):
            raise RPCError(1005, "InvalidAddress: Shipping address validation failed")

        # Check stock
        for item in cart["items"]:
            product = next(
                (p for p in products_db if p["productId"] == item["productId"]), None
            )
            if product and product["stock"] < item["quantity"]:
                raise RPCError(1004, "OutOfStock: Insufficient inventory")

        # Simulate payment (deterministic: always succeeds in tests)
        # For real implementations, use actual payment gateway integration
        # In production, you would integrate with Stripe, PayPal, etc.
        pass

        # Create order
        order_id = f"order_{random.randint(10000, 99999)}"
        order = {
            "orderId": order_id,
            "cart": cart,
            "shippingAddress": request.get("shippingAddress"),
            "paymentMethod": request.get("paymentMethod"),
            "status": "pending",
            "total": cart["subtotal"],
            "createdAt": int(time.time()),
        }
        orders_db[order_id] = order

        # Clear cart
        carts_db[request.get("cartId")]["items"] = []
        carts_db[request.get("cartId")]["subtotal"] = 0.0

        return {"orderId": order_id, "message": "Order created successfully"}

    def getOrder(self, orderId):
        return orders_db.get(orderId)

# Start server
if __name__ == "__main__":
    port = int(os.environ.get("SERVER_PORT", "8080"))
    server = PulseRPCServer(host="0.0.0.0", port=port)
    server.register("CatalogService", CatalogServiceImpl())
    server.register("CartService", CartServiceImpl())
    server.register("OrderService", OrderServiceImpl())
    server.serve_forever()
