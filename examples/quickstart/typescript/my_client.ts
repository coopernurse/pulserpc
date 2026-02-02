import { HTTPTransport, CatalogServiceClient, CartServiceClient, OrderServiceClient } from './client';
import { RPCError } from './pulserpc/rpc';

const port = parseInt(process.env.SERVER_PORT || '8080', 10);
const transport = new HTTPTransport(`http://localhost:${port}`);
const catalog = new CatalogServiceClient(transport);
const cart = new CartServiceClient(transport);
const orders = new OrderServiceClient(transport);

async function main() {
  const products = await catalog.listProducts();
  console.log('=== Products ===');
  for (const p of products) {
    console.log(`${p.name} - $${p.price}`);
  }

  const result = await cart.addToCart({
    cartId: null,
    productId: products[0].productId,
    quantity: 2
  });
  console.log(`\nCart: ${result.cartId}`);

  const response = await orders.createOrder({
    cartId: result.cartId,
    shippingAddress: {
      street: '123 Main St',
      city: 'San Francisco',
      state: 'CA',
      zipCode: '94105',
      country: 'USA'
    },
    paymentMethod: 'credit_card'
  });
  console.log(`✓ Order created: ${response.orderId}`);

  // Test error case: empty cart
  console.log('\n=== Testing Error Case ===');
  await cart.clearCart(result.cartId);
  try {
    await orders.createOrder({
      cartId: result.cartId,
      shippingAddress: {
        street: '123 Main St',
        city: 'San Francisco',
        state: 'CA',
        zipCode: '94105',
        country: 'USA'
      },
      paymentMethod: 'credit_card'
    });
    console.log('✗ Should have failed!');
  } catch (e) {
    if (e instanceof RPCError) {
      console.log(`✓ Got expected error: ${e.code} - ${e.message}`);
    } else {
      throw e;
    }
  }
}

main().catch(console.error);
