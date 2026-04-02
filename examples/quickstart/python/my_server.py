#!/usr/bin/env python3
import os
import json
from http.server import HTTPServer, BaseHTTPRequestHandler
from typing import Any
from checkout.server import CatalogService, CartService, OrderService
from pulserpc import Server, Contract, RPCError
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

# Create JSON-RPC handler
class PulseRPCHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        # Read request body
        content_length = int(self.headers.get('Content-Length', 0))
        if content_length == 0:
            self._send_error_response(None, -32700, "Parse error", "Empty request body")
            return

        body = self.rfile.read(content_length)

        # Parse JSON request
        try:
            req = json.loads(body.decode('utf-8'))
        except json.JSONDecodeError as e:
            self._send_error_response(None, -32700, "Parse error", f"Invalid JSON: {e}")
            return

        # Handle request
        response = rpc_server.call(req)
        if response is None:
            self._send_response(204, b'')
        else:
            self._send_json_response(200, response)

    def _send_json_response(self, status: int, data: Any) -> None:
        """Send a JSON response"""
        response_body = json.dumps(data).encode('utf-8')
        self.send_response(status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(response_body)))
        self.end_headers()
        self.wfile.write(response_body)

    def _send_response(self, status: int, body: bytes) -> None:
        """Send a response with raw body"""
        self.send_response(status)
        if len(body) > 0:
            self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        if len(body) > 0:
            self.wfile.write(body)

    def _send_error_response(self, request_id: Any, code: int, message: str, data: Any = None) -> None:
        """Send a JSON-RPC 2.0 error response"""
        error = {'code': code, 'message': message}
        if data is not None:
            error['data'] = data
        response = {'jsonrpc': '2.0', 'error': error, 'id': request_id}
        self._send_json_response(200, response)

    def log_message(self, format: str, *args: Any) -> None:
        """Suppress default logging"""
        pass

# Start server
if __name__ == "__main__":
    port = int(os.environ.get("SERVER_PORT", "8080"))

    # Load IDL and create Contract
    with open('idl.json', 'r') as f:
        idl_data = json.load(f)
    contract = Contract(idl_data)

    # Create Server instance
    rpc_server = Server(contract, validate_requests=True, validate_responses=True)
    rpc_server.add_handler("CatalogService", CatalogServiceImpl())
    rpc_server.add_handler("CartService", CartServiceImpl())
    rpc_server.add_handler("OrderService", OrderServiceImpl())

    # Start HTTP server
    http_server = HTTPServer(("0.0.0.0", port), PulseRPCHandler)
    print(f"PulseRPC server listening on http://0.0.0.0:{port}")
    http_server.serve_forever()
