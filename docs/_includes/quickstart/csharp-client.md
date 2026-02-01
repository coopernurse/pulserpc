{% highlight csharp %}
using System;
using System.Linq;
using System.Threading.Tasks;
using PulseRPC;
using checkout;

class Program
{
    static async Task Main(string[] args)
    {
        var transport = new HttpTransport("http://localhost:8080");
        var catalogClient = new CatalogServiceClient(transport);
        var cartClient = new CartServiceClient(transport);
        var ordersClient = new OrderServiceClient(transport);

        // List products (async - use *Async methods inside async Main)
        var products = await catalogClient.listProductsAsync();
        Console.WriteLine("=== Products ===");
        foreach (var p in products)
        {
            Console.WriteLine($"{p.Name} - ${p.Price}");
        }

        // Add to cart (async)
        var result = await cartClient.addToCartAsync(new AddToCartRequest
        {
            ProductId = products[0].ProductId,
            Quantity = 2
        });
        Console.WriteLine($"\nCart: {result.CartId}");

        // Create order (async)
        var response = await ordersClient.createOrderAsync(new CreateOrderRequest
        {
            CartId = result.CartId,
            ShippingAddress = new Address
            {
                Street = "123 Main St",
                City = "San Francisco",
                State = "CA",
                ZipCode = "94105",
                Country = "USA"
            },
            PaymentMethod = PaymentMethod.credit_card
        });
        Console.WriteLine($"Order created: {response.OrderId}");
    }
}
{% endhighlight %}
