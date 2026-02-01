import { PulseRPCServer, CatalogService, CartService, OrderService } from './server';
import { RPCError } from './pulserpc/rpc';

const products = [
  {
    productId: 'prod001',
    name: 'Wireless Mouse',
    description: 'Ergonomic mouse',
    price: 29.99,
    stock: 50,
    imageUrl: 'https://example.com/mouse.jpg'
  },
  {
    productId: 'prod002',
    name: 'Mechanical Keyboard',
    description: 'RGB keyboard',
    price: 89.99,
    stock: 25,
    imageUrl: 'https://example.com/keyboard.jpg'
  }
];

const carts = new Map<string, any>();
const orders = new Map<string, any>();

class CatalogServiceImpl extends CatalogService {
  listProducts(): any[] {
    return products;
  }

  getProduct(productId: string): any | null {
    return products.find((p: any) => p.productId === productId) || null;
  }
}

class CartServiceImpl extends CartService {
  addToCart(request: any): any {
    let cartId = request.cartId || `cart_${Math.floor(Math.random() * 9000 + 1000)}`;

    let cart = carts.get(cartId);
    if (!cart) {
      cart = {
        cartId,
        items: [],
        subtotal: 0
      };
      carts.set(cartId, cart);
    }

    const product = products.find((p: any) => p.productId === request.productId);
    if (!product) {
      throw new RPCError(-32602, 'Product not found');
    }

    cart.items.push({
      productId: request.productId,
      quantity: request.quantity,
      price: product.price
    });

    cart.subtotal = cart.items.reduce((sum: number, item: any) => sum + item.price * item.quantity, 0);
    return cart;
  }

  getCart(cartId: string): any | null {
    return carts.get(cartId) || null;
  }

  clearCart(cartId: string): boolean {
    const cart = carts.get(cartId);
    if (cart) {
      cart.items = [];
      cart.subtotal = 0;
      return true;
    }
    return false;
  }
}

class OrderServiceImpl extends OrderService {
  createOrder(request: any): any {
    const cart = carts.get(request.cartId);
    if (!cart) {
      throw new RPCError(1001, 'CartNotFound: Cart does not exist');
    }

    if (!cart.items || cart.items.length === 0) {
      throw new RPCError(1002, 'CartEmpty: Cannot create order from empty cart');
    }

    const orderId = `order_${Math.floor(Math.random() * 90000 + 10000)}`;
    const order = {
      orderId,
      cart,
      shippingAddress: request.shippingAddress,
      paymentMethod: request.paymentMethod,
      status: 'pending',
      total: cart.subtotal,
      createdAt: Math.floor(Date.now() / 1000)
    };

    orders.set(orderId, order);
    return { orderId, message: 'Order created successfully' };
  }

  getOrder(orderId: string): any | null {
    return orders.get(orderId) || null;
  }
}

const server = new PulseRPCServer('0.0.0.0', 8080);
server.register('CatalogService', new CatalogServiceImpl());
server.register('CartService', new CartServiceImpl());
server.register('OrderService', new OrderServiceImpl());
server.serveForever();
