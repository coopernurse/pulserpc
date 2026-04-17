{% highlight python %}
#!/usr/bin/env python
import os
import sys
import json
from BaseHTTPServer import HTTPServer, BaseHTTPRequestHandler
import random
import time

from pulserpc import Server
from pulserpc.contract import Contract
from pulserpc import RPCError

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

carts_db = {}
orders_db = {}

class CatalogServiceImpl(object):
    def listProducts(self):
        return products_db

    def getProduct(self, productId):
        for p in products_db:
            if p["productId"] == productId:
                return p
        return None

class CartServiceImpl(object):
    def addToCart(self, request):
        cart_id = request.get("cartId") or "cart_%d" % random.randint(1000, 9999)

        if cart_id not in carts_db:
            carts_db[cart_id] = {"cartId": cart_id, "items": [], "subtotal": 0.0}

        cart = carts_db[cart_id]
        product = None
        for p in products_db:
            if p["productId"] == request.get("productId"):
                product = p
                break

        if not product:
            raise RPCError(-32602, "Product '%s' not found" % request.get("productId"))

        for item in cart["items"]:
            if item["productId"] == request.get("productId"):
                item["quantity"] += request.get("quantity", 0)
                item["price"] = product["price"]
                break
        else:
            cart["items"].append({
                "productId": request.get("productId"),
                "quantity": request.get("quantity", 0),
                "price": product["price"],
            })

        cart["subtotal"] = sum(item["price"] * item["quantity"] for item in cart["items"])
        return cart

    def getCart(self, cartId):
        return carts_db.get(cartId)

    def clearCart(self, cartId):
        if cartId in carts_db:
            carts_db[cartId]["items"] = []
            carts_db[cartId]["subtotal"] = 0.0
            return True
        return False

class OrderServiceImpl(object):
    def createOrder(self, request):
        if request.get("cartId") not in carts_db:
            raise RPCError(1001, "CartNotFound: Cart does not exist")

        cart = carts_db[request.get("cartId")]

        if not cart["items"]:
            raise RPCError(1002, "CartEmpty: Cannot create order from empty cart")

        addr = request.get("shippingAddress") or {}
        if not addr.get("street") or not addr.get("city") or not addr.get("zipCode"):
            raise RPCError(1005, "InvalidAddress: Shipping address validation failed")

        for item in cart["items"]:
            product = None
            for p in products_db:
                if p["productId"] == item["productId"]:
                    product = p
                    break
            if product and product["stock"] < item["quantity"]:
                raise RPCError(1004, "OutOfStock: Insufficient inventory")

        if random.random() < 0.1:
            raise RPCError(1003, "PaymentFailed: Card declined by issuer")

        order_id = "order_%d" % random.randint(10000, 99999)
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

        carts_db[request.get("cartId")]["items"] = []
        carts_db[request.get("cartId")]["subtotal"] = 0.0

        return {"orderId": order_id, "message": "Order created successfully"}

    def getOrder(self, orderId):
        return orders_db.get(orderId)

port = int(os.environ.get("SERVER_PORT", "8080"))

idl_path = os.path.join(os.path.dirname(__file__), "idl.json")
with open(idl_path) as f:
    idl_data = json.load(f)

contract = Contract(idl_data)
server = Server(contract)
server.add_handler("CatalogService", CatalogServiceImpl())
server.add_handler("CartService", CartServiceImpl())
server.add_handler("OrderService", OrderServiceImpl())

class PulseRPCHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers.getheader('Content-Length', 0))
        if content_length <= 0:
            self.send_error(400, "No content")
            return

        request_body = self.rfile.read(content_length)
        request_data = json.loads(request_body)
        response = server.call(request_data)

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        if response:
            self.wfile.write(json.dumps(response))

    def log_message(self, format, *args):
        pass

if __name__ == "__main__":
    print "Starting PulseRPC server on port %d..." % port
    httpd = HTTPServer(("0.0.0.0", port), PulseRPCHandler)
    print "Server running at http://localhost:%d" % port
    httpd.serve_forever()
{% highlight python %}