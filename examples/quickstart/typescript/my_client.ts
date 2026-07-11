import { HttpTransport, Client } from './pulserpc/index.js';

const port = parseInt(process.env.SERVER_PORT || '8080', 10);
const transport = new HttpTransport(`http://127.0.0.1:${port}`);
const client = await Client.create(transport);

async function main() {
  // Client.create() fetches IDL from server and creates interface proxies
  const products = await client.CatalogService.listProducts();
  console.log('=== Products ===');
  for (const p of products) {
    console.log(`${p.name} - $${p.price}`);
  }

  const result = await client.CartService.addToCart({
    cartId: null,
    productId: products[0].productId,
    quantity: 2
  });
  console.log(`\nCart: ${result.cartId}`);

  const response = await client.OrderService.createOrder({
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
  await client.CartService.clearCart(result.cartId);
  try {
    await client.OrderService.createOrder({
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
  } catch (e: any) {
    console.log(`✓ Got expected error: ${e.code} - ${e.message}`);
  }
}

main().catch(console.error);
