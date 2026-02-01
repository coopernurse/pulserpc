{% highlight go %}
package main

import (
	"fmt"
	"checkout-service/pkg/checkout"
)

func main() {
	transport := checkout.NewHTTPTransport("http://localhost:8080", nil)
	catalog := checkout.NewCatalogServiceClient(transport)
	cart := checkout.NewCartServiceClient(transport)
	orders := checkout.NewOrderServiceClient(transport)

	// List products
	products, _ := catalog.ListProducts()
	fmt.Println("=== Products ===")
	for _, p := range products {
		fmt.Printf("%s - $%.2f\n", p.Name, p.Price)
	}

	// Add to cart
	result, _ := cart.AddToCart(checkout.AddToCartRequest{
		ProductId: products[0].ProductId,
		Quantity:  2,
	})
	fmt.Printf("\nCart: %s, Subtotal: $%.2f\n", result.CartId, result.Subtotal)

	// Create order
	response, _ := orders.CreateOrder(checkout.CreateOrderRequest{
		CartId: result.CartId,
		ShippingAddress: checkout.Address{
			Street:  "123 Main St",
			City:    "San Francisco",
			State:   "CA",
			ZipCode: "94105",
			Country: "USA",
		},
		PaymentMethod: checkout.PaymentMethodCreditCard,
	})
	fmt.Printf("✓ Order created: %s\n", response.OrderId)
}
{% endhighlight %}
