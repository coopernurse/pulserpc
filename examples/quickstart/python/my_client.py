#!/usr/bin/env python3
import os
from pulserpc import HttpTransport, Client, RPCError

# Connect to server
port = os.environ.get("SERVER_PORT", "8080")
transport = HttpTransport(f"http://localhost:{port}")
client = Client(transport)

# List products
print("=== Products ===")
products = client.CatalogService.listProducts()
for p in products:
    print(f"{p['name']} - ${p['price']:.2f}")

# Create cart and add items
print("\n=== Creating Cart ===")
cart_response = client.CartService.addToCart({
    'productId': products[0]['productId'],
    'quantity': 2
})
my_cart = cart_response
print(f"Cart: {my_cart['cartId']}, Subtotal: ${my_cart['subtotal']:.2f}")

# Add another item
cart_response = client.CartService.addToCart({
    'cartId': my_cart['cartId'],
    'productId': products[1]['productId'],
    'quantity': 1
})
my_cart = cart_response
print(f"Updated Subtotal: ${my_cart['subtotal']:.2f}")

# Create order
print("\n=== Creating Order ===")
try:
    response_data = client.OrderService.createOrder({
        'cartId': my_cart['cartId'],
        'shippingAddress': {
            'street': '123 Main St',
            'city': 'San Francisco',
            'state': 'CA',
            'zipCode': '94105',
            'country': 'USA'
        },
        'paymentMethod': 'credit_card'
    })
    response = response_data
    print(f"✓ Order created: {response['orderId']}")
except RPCError as e:
    print(f"✗ Error {e.code}: {e.message}")

# Test error case: empty cart
print("\n=== Testing Error Case ===")
client.CartService.clearCart(my_cart['cartId'])
try:
    client.OrderService.createOrder({
        'cartId': my_cart['cartId'],
        'shippingAddress': {
            'street': '123 Main St',
            'city': 'San Francisco',
            'state': 'CA',
            'zipCode': '94105',
            'country': 'USA'
        },
        'paymentMethod': 'credit_card'
    })
    print("✗ Should have failed!")
except RPCError as e:
    print(f"✓ Got expected error: {e.code} - {e.message}")
