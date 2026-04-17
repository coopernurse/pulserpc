#!/bin/bash
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
QUICKSTART_DIR="$PROJECT_ROOT/examples/quickstart"
OUTPUT_DIR="/tmp/pulserpc_quickstart_python2_$$"
SERVER_PORT=8102
SERVER_URL="http://localhost:$SERVER_PORT"
TIMEOUT=30

cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    if [ -n "$SERVER_PID" ]; then
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
    fi
    rm -rf "$OUTPUT_DIR"
}

trap cleanup EXIT

echo -e "${GREEN}=== Python 2 Quickstart Test ===${NC}"
echo ""

if [ -f "$PROJECT_ROOT/target/pulserpc" ]; then
    PULSERPC="$PROJECT_ROOT/target/pulserpc"
elif [ -f "$PROJECT_ROOT/target/pulserpc-amd64" ]; then
    PULSERPC="$PROJECT_ROOT/target/pulserpc-amd64"
else
    echo -e "${RED}ERROR: PulseRPC binary not found. Run 'make build' first.${NC}"
    exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
    echo -e "${RED}ERROR: Docker not found. This test requires Docker.${NC}"
    exit 1
fi

echo -e "${YELLOW}Generating Python 2 code from checkout.pulse...${NC}"
mkdir -p "$OUTPUT_DIR"
"$PULSERPC" -plugin python-client-server -python-version 2 -dir "$OUTPUT_DIR" \
    "$QUICKSTART_DIR/checkout.pulse"

if [ ! -f "$OUTPUT_DIR/idl.json" ]; then
    echo -e "${RED}ERROR: idl.json not generated${NC}"
    exit 1
fi

if [ ! -d "$OUTPUT_DIR/pulserpc" ]; then
    echo -e "${RED}ERROR: pulserpc runtime not generated${NC}"
    exit 1
fi

echo -e "${GREEN}Python 2 code generation successful${NC}"

DOCKER_SCRIPT="/tmp/test_python2_server.sh"
cat > "$DOCKER_SCRIPT" << 'DOCKERSCRIPT'
#!/bin/bash
set -e
cd /workspace

SERVER_PORT=${SERVER_PORT:-8080}
SERVER_URL="http://localhost:$SERVER_PORT"

cat > my_server.py << 'SERVEREOF'
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
server = Server(contract, validate_requests=False, validate_responses=False)
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
SERVEREOF

cat > my_client.py << 'CLIENTEOF'
#!/usr/bin/env python
import os
import urllib2
import json

SERVER_PORT = os.environ.get("SERVER_PORT", "8080")
SERVER_URL = "http://localhost:%s" % SERVER_PORT

def rpc_call(method, params):
    request = {
        "jsonrpc": "2.0",
        "method": method,
        "params": params,
        "id": 1
    }
    req = urllib2.Request(SERVER_URL, json.dumps(request),
                          {"Content-Type": "application/json"})
    response = urllib2.urlopen(req)
    response_data = json.loads(response.read())
    if "result" in response_data:
        return response_data["result"]
    elif "error" in response_data:
        print "RPC Error: %s" % response_data["error"]
        return None
    return None

print "=== Products ==="
products = rpc_call("CatalogService.listProducts", [])
if products:
    for p in products:
        print "%s - $%.2f" % (p["name"], p["price"])

print ""
print "=== Creating Cart ==="
cart = rpc_call("CartService.addToCart", [{
    "cartId": None,
    "productId": "prod001",
    "quantity": 2
}])
if cart:
    print "Cart: %s, Subtotal: $%.2f" % (cart["cartId"], cart["subtotal"])

print ""
print "=== Creating Order ==="
order = rpc_call("OrderService.createOrder", [{
    "cartId": cart["cartId"],
    "shippingAddress": {
        "street": "123 Main St",
        "city": "Anytown",
        "state": "CA",
        "zipCode": "12345",
        "country": "USA"
    },
    "paymentMethod": "credit_card"
}])
if order:
    print "Order created: %s" % order["orderId"]

print ""
print "=== Testing Error Case ==="
error_order = rpc_call("OrderService.createOrder", [{
    "cartId": cart["cartId"],
    "shippingAddress": {
        "street": "123 Main St",
        "city": "Anytown",
        "state": "CA",
        "zipCode": "12345",
        "country": "USA"
    },
    "paymentMethod": "credit_card"
}])
if not error_order:
    print "Got expected error"
else:
    print "ERROR: Should have raised an error for empty cart"
CLIENTEOF

PYTHONPATH="/workspace:$PYTHONPATH" SERVER_PORT=$SERVER_PORT python my_server.py > server.log 2>&1 &
SERVER_PID=$!

sleep 2

WAIT_COUNT=0
while [ $WAIT_COUNT -lt 30 ]; do
    if curl -s "$SERVER_URL" > /dev/null 2>&1; then
        break
    fi
    sleep 1
    WAIT_COUNT=$((WAIT_COUNT + 1))
done

if [ $WAIT_COUNT -ge 30 ]; then
    echo "Server did not start"
    cat server.log
    exit 1
fi

PYTHONPATH="/workspace:$PYTHONPATH" SERVER_PORT=$SERVER_PORT python my_client.py

kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true

echo ""
echo "Test completed successfully"
DOCKERSCRIPT

echo -e "${YELLOW}Running test in Docker with moxel/python2...${NC}"
docker rm -f python2-quickstart-test >/dev/null 2>&1 || true

docker run --rm \
    -v "$OUTPUT_DIR:/workspace" \
    -v "$DOCKER_SCRIPT:/tmp/test_script.sh" \
    -e SERVER_PORT=$SERVER_PORT \
    --name python2-quickstart-test \
    moxel/python2 \
    sh -c "chmod +x /tmp/test_script.sh && /tmp/test_script.sh"

echo ""
echo -e "${GREEN}✓ Python 2 quickstart test passed!${NC}"