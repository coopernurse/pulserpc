{% highlight typescript %}
import { readFileSync } from 'fs';
import { Server, Contract } from './pulserpc';
import { CatalogService, CartService, OrderService } from './server';
import { RPCError } from './pulserpc/rpc';
import * as http from 'http';

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
  listProducts(_ctx: Record<string, any>): any[] {
    return products;
  }

  getProduct(_ctx: Record<string, any>, productId: string): any | null {
    return products.find((p: any) => p.productId === productId) || null;
  }
}

class CartServiceImpl extends CartService {
  addToCart(_ctx: Record<string, any>, request: any): any {
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

  getCart(_ctx: Record<string, any>, cartId: string): any | null {
    return carts.get(cartId) || null;
  }

  clearCart(_ctx: Record<string, any>, cartId: string): boolean {
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
  createOrder(_ctx: Record<string, any>, request: any): any {
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

  getOrder(_ctx: Record<string, any>, orderId: string): any | null {
    return orders.get(orderId) || null;
  }
}

const idlData = JSON.parse(readFileSync('idl.json', 'utf-8'));
const contract = new Contract(idlData);
const rpcServer = new Server({ contract, validateRequests: true, validateResponses: true });
rpcServer.addHandler('CatalogService', new CatalogServiceImpl());
rpcServer.addHandler('CartService', new CartServiceImpl());
rpcServer.addHandler('OrderService', new OrderServiceImpl());

class RPCHandler {
  private rpcServer: Server;

  constructor(rpcServer: Server) {
    this.rpcServer = rpcServer;
  }

  handle(req: http.IncomingMessage, res: http.ServerResponse): void {
    let body = '';
    req.on('data', (chunk) => { body += chunk.toString(); });
    req.on('end', () => {
      try {
        const data = JSON.parse(body);
        const response = this.rpcServer.call(data);
        if (response === null || response === undefined) {
          res.writeHead(204);
          res.end();
        } else {
          res.writeHead(200, { 'Content-Type': 'application/json' });
          res.end(JSON.stringify(response));
        }
      } catch (err: any) {
        const errorResponse = {
          jsonrpc: '2.0',
          error: { code: -32700, message: `Parse error: ${err.message}` },
          id: null,
        };
        res.writeHead(200, { 'Content-Type': 'application/json' });
        res.end(JSON.stringify(errorResponse));
      }
    });
  }
}

const port = parseInt(process.env.SERVER_PORT || '8080', 10);
const handler = new RPCHandler(rpcServer);
const httpServer = http.createServer((req, res) => {
  if (req.method === 'POST') {
    handler.handle(req, res);
  } else {
    res.writeHead(405, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'Method Not Allowed' }));
  }
});

httpServer.listen(port, '0.0.0.0', () => {
  console.log(`Server listening on http://0.0.0.0:${port}`);
});
{% endhighlight %}