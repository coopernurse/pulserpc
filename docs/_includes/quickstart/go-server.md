{% highlight go %}
package main

import (
	"fmt"
	"math/rand"
	"time"

	"checkout-service/pkg/checkout"
	"checkout-service/pkg/pulserpc"
)

// Helper function to create string pointers for optional fields
func strPtr(s string) *string {
	return &s
}

var products = []*checkout.Product{
	{ProductId: "prod001", Name: "Wireless Mouse", Description: "Ergonomic mouse",
		Price: 29.99, Stock: 50, ImageUrl: strPtr("https://example.com/mouse.jpg")},
	{ProductId: "prod002", Name: "Mechanical Keyboard", Description: "RGB keyboard",
		Price: 89.99, Stock: 25, ImageUrl: strPtr("https://example.com/keyboard.jpg")},
}

type CatalogService struct{}

func (s *CatalogService) ListProducts() []*checkout.Product {
	return products
}

func (s *CatalogService) GetProduct(productId string) (*checkout.Product, error) {
	for _, p := range products {
		if p.ProductId == productId {
			return p, nil
		}
	}
	return nil, nil
}

type CartService struct {
	carts map[string]*checkout.Cart
}

func NewCartService() *CartService {
	return &CartService{
		carts: make(map[string]*checkout.Cart),
	}
}

func (s *CartService) AddToCart(request *checkout.AddToCartRequest) (*checkout.Cart, error) {
	var cartId string
	if request.CartId == nil {
		cartId = fmt.Sprintf("cart_%d", rand.Intn(9000)+1000)
	} else {
		cartId = *request.CartId
	}

	cart, ok := s.carts[cartId]
	if !ok {
		cart = &checkout.Cart{CartId: cartId, Items: []checkout.CartItem{}, Subtotal: 0}
		s.carts[cartId] = cart
	}

	// Find product
	var product *checkout.Product
	for _, p := range products {
		if p.ProductId == request.ProductId {
			product = p
			break
		}
	}

	// Add item (note: Items is []CartItem, not []*checkout.CartItem)
	cart.Items = append(cart.Items, checkout.CartItem{
		ProductId: request.ProductId,
		Quantity:  request.Quantity,
		Price:     product.Price,
	})

	// Recalculate subtotal
	var subtotal float64
	for _, item := range cart.Items {
		subtotal += item.Price * float64(item.Quantity)
	}
	cart.Subtotal = subtotal

	return cart, nil
}

func (s *CartService) GetCart(cartId string) (*checkout.Cart, error) {
	return s.carts[cartId], nil
}

func (s *CartService) ClearCart(cartId string) (bool, error) {
	if cart, ok := s.carts[cartId]; ok {
		cart.Items = []checkout.CartItem{}
		cart.Subtotal = 0
		return true, nil
	}
	return false, nil
}

type OrderService struct {
	carts  map[string]*checkout.Cart
	orders map[string]*checkout.Order
}

func NewOrderService(cartService *CartService) *OrderService {
	return &OrderService{
		carts:  cartService.carts,
		orders: make(map[string]*checkout.Order),
	}
}

func (s *OrderService) CreateOrder(request *checkout.CreateOrderRequest) (*checkout.CheckoutResponse, error) {
	cart, ok := s.carts[request.CartId]
	if !ok {
		return nil, pulserpc.NewRPCError(1001, "CartNotFound: Cart does not exist")
	}

	if len(cart.Items) == 0 {
		return nil, pulserpc.NewRPCError(1002, "CartEmpty: Cannot create order from empty cart")
	}

	// Create order
	orderId := fmt.Sprintf("order_%d", rand.Intn(90000)+10000)
	order := &checkout.Order{
		OrderId:         orderId,
		Cart:            *cart,  // Dereference pointer (struct uses value type)
		ShippingAddress: request.ShippingAddress,
		PaymentMethod:   request.PaymentMethod,
		Status:          checkout.OrderStatusPending,
		Total:           cart.Subtotal,
		CreatedAt:       int(time.Now().Unix()),  // Convert int64 to int
	}
	s.orders[orderId] = order

	return &checkout.CheckoutResponse{OrderId: orderId}, nil
}

func (s *OrderService) GetOrder(orderId string) (*checkout.Order, error) {
	return s.orders[orderId], nil
}

func main() {
	server := checkout.NewPulseRPCServer("0.0.0.0", 8080)
	cartSvc := NewCartService()

	server.Register("CatalogService", &CatalogService{})
	server.Register("CartService", cartSvc)
	server.Register("OrderService", NewOrderService(cartSvc))

	fmt.Println("Server starting on http://localhost:8080")
	server.ServeForever()
}
{% endhighlight %}
