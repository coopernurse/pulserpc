using System;
using System.Collections.Generic;
using System.Linq;
using PulseRPC;
using checkout;

public class CatalogServiceImpl : ICatalogService
{
    private static readonly List<Product> Products = new List<Product>
    {
        new Product { ProductId = "prod001", Name = "Wireless Mouse", Description = "Ergonomic mouse", Price = 29.99, Stock = 50, ImageUrl = "https://example.com/mouse.jpg" },
        new Product { ProductId = "prod002", Name = "Mechanical Keyboard", Description = "RGB keyboard", Price = 89.99, Stock = 25, ImageUrl = "https://example.com/keyboard.jpg" }
    };

    public List<Product> listProducts()
    {
        return Products;
    }

    public Product? getProduct(string productId)
    {
        var product = Products.FirstOrDefault(p => p.ProductId == productId);
        if (product == null)
            throw new RPCError(-32602, "Product not found");
        return product;
    }

}

public class CartServiceImpl : ICartService
{
    internal readonly Dictionary<string, Cart> _carts = new Dictionary<string, Cart>();
    private readonly CatalogServiceImpl _catalogService;

    public CartServiceImpl(CatalogServiceImpl catalogService)
    {
        _catalogService = catalogService;
    }

    public Cart addToCart(AddToCartRequest request)
    {
        var cartId = request.CartId ?? $"cart_{new Random().Next(1000, 9999)}";

        if (!_carts.TryGetValue(cartId, out var cart))
        {
            cart = new Cart { CartId = cartId, Items = new List<CartItem>(), Subtotal = 0 };
            _carts[cartId] = cart;
        }

        var product = _catalogService.listProducts().FirstOrDefault(p => p.ProductId == request.ProductId);
        if (product == null)
            throw new RPCError(-32602, "Product not found");

        cart.Items.Add(new CartItem { ProductId = request.ProductId, Quantity = request.Quantity, Price = (double)product.Price });
        cart.Subtotal = (double)cart.Items.Sum(i => i.Price * i.Quantity);

        return cart;
    }

    public Cart? getCart(string cartId)
    {
        return _carts.TryGetValue(cartId, out var cart) ? cart : null;
    }

    public bool clearCart(string cartId)
    {
        if (_carts.TryGetValue(cartId, out var cart))
        {
            cart.Items.Clear();
            cart.Subtotal = 0;
            return true;
        }
        return false;
    }
}

class OrderServiceImpl : IOrderService
{
    private readonly Dictionary<string, Cart> _carts;
    private readonly Dictionary<string, Order> _orders = new Dictionary<string, Order>();

    public OrderServiceImpl(Dictionary<string, Cart> carts)
    {
        _carts = carts;
    }

    public CheckoutResponse createOrder(CreateOrderRequest request)
    {
        if (!_carts.TryGetValue(request.CartId, out var cart))
            throw new RPCError(1001, "CartNotFound: Cart does not exist");

        if (cart.Items.Count == 0)
            throw new RPCError(1002, "CartEmpty: Cannot create order from empty cart");

        var orderId = $"order_{new Random().Next(10000, 99999)}";
        var order = new Order
        {
            OrderId = orderId,
            Cart = cart,
            ShippingAddress = request.ShippingAddress,
            PaymentMethod = request.PaymentMethod,
            Status = OrderStatus.pending,
            Total = (double)cart.Subtotal,
            CreatedAt = (int)DateTimeOffset.UtcNow.ToUnixTimeSeconds()
        };

        _orders[orderId] = order;
        return new CheckoutResponse { OrderId = orderId };
    }

    public Order? getOrder(string orderId)
    {
        return _orders.TryGetValue(orderId, out var order) ? order : null;
    }
}

class Program
{
    static async Task Main(string[] args)
    {
        var server = new PulseRPCServer();
        var catalogService = new CatalogServiceImpl();
        var cartService = new CartServiceImpl(catalogService);

        server.RegisterCatalogService(catalogService);
        server.RegisterCartService(cartService);
        server.RegisterOrderService(new OrderServiceImpl(cartService._carts));

        Console.WriteLine("Server starting on http://localhost:8080");
        await server.RunAsync("0.0.0.0", 8080);
    }
}
