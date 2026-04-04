{% highlight typescript %}
import { HttpTransport } from './pulserpc/transport';
import { CatalogServiceClient, CartServiceClient, OrderServiceClient } from './client';

const transport = new HttpTransport('http://localhost:8080');
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
}

main().catch(console.error);
{% endblock %}