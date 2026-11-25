package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"checkout-service/pkg/checkout"
	"checkout-service/pkg/pulserpc"
)

// contextKey is a typed key for values attached by middleware. Typed keys keep
// request-scoped values from colliding with unrelated context values.
type contextKey string

const (
	apiKeyContext    contextKey = "api-key"
	rpcMethodContext contextKey = "rpc-method"
)

// Helper function to create string pointers for optional fields
func strPtr(s string) *string {
	return &s
}

var products = []checkout.Product{
	{ProductId: "prod001", Name: "Wireless Mouse", Description: "Ergonomic mouse",
		Price: 29.99, Stock: 50, ImageUrl: strPtr("https://example.com/mouse.jpg")},
	{ProductId: "prod002", Name: "Mechanical Keyboard", Description: "RGB keyboard",
		Price: 89.99, Stock: 25, ImageUrl: strPtr("https://example.com/keyboard.jpg")},
}

type CatalogService struct{}

func (s *CatalogService) ListProducts(ctx context.Context) ([]checkout.Product, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return products, nil
}

func (s *CatalogService) GetProduct(ctx context.Context, productId string) (*checkout.Product, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for i := range products {
		if products[i].ProductId == productId {
			return &products[i], nil
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

func (s *CartService) AddToCart(ctx context.Context, request checkout.AddToCartRequest) (checkout.Cart, error) {
	if err := ctx.Err(); err != nil {
		return checkout.Cart{}, err
	}
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
	for i := range products {
		if products[i].ProductId == request.ProductId {
			product = &products[i]
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

	return *cart, nil
}

func (s *CartService) GetCart(ctx context.Context, cartId string) (*checkout.Cart, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.carts[cartId], nil
}

func (s *CartService) ClearCart(ctx context.Context, cartId string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
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

func (s *OrderService) CreateOrder(ctx context.Context, request checkout.CreateOrderRequest) (checkout.CheckoutResponse, error) {
	if err := ctx.Err(); err != nil {
		return checkout.CheckoutResponse{}, err
	}
	cart, ok := s.carts[request.CartId]
	if !ok {
		return checkout.CheckoutResponse{}, pulserpc.NewRPCError(1001, "CartNotFound: Cart does not exist")
	}

	if len(cart.Items) == 0 {
		return checkout.CheckoutResponse{}, pulserpc.NewRPCError(1002, "CartEmpty: Cannot create order from empty cart")
	}

	// Create order
	orderId := fmt.Sprintf("order_%d", rand.Intn(90000)+10000)
	order := &checkout.Order{
		OrderId:         orderId,
		Cart:            *cart, // Dereference pointer (struct uses value type)
		ShippingAddress: request.ShippingAddress,
		PaymentMethod:   request.PaymentMethod,
		Status:          checkout.OrderStatusPending,
		Total:           cart.Subtotal,
		CreatedAt:       time.Now().Unix(),
	}
	s.orders[orderId] = order

	return checkout.CheckoutResponse{OrderId: orderId}, nil
}

func (s *OrderService) GetOrder(ctx context.Context, orderId string) (*checkout.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.orders[orderId], nil
}

// Compile-time checks: the reference handlers satisfy the generated interfaces.
var (
	_ checkout.CatalogService = (*CatalogService)(nil)
	_ checkout.CartService    = (*CartService)(nil)
	_ checkout.OrderService   = (*OrderService)(nil)
)

func main() {
	server := checkout.NewPulseRPCServer("0.0.0.0", 8080)
	cartSvc := NewCartService()

	// HTTP middleware: copy a header into the request context. Returning
	// (ctx, true) continues processing with the derived context.
	server.UseHTTP(func(w http.ResponseWriter, r *http.Request) (context.Context, bool) {
		ctx := context.WithValue(r.Context(), apiKeyContext, r.Header.Get("X-API-Key"))
		return ctx, true
	})

	// RPC middleware: attach the dispatched method for handlers that log or
	// authorize per method. The request is returned unchanged, so the wire
	// behavior is identical.
	server.UseRPC(func(ctx context.Context, request map[string]interface{}, response map[string]interface{}) (context.Context, map[string]interface{}, map[string]interface{}) {
		return context.WithValue(ctx, rpcMethodContext, request["method"]), request, nil
	})

	// Register validates every handler method (leading context.Context and an
	// error result), so a misconfigured server fails at startup.
	mustRegister := func(name string, implementation interface{}) {
		if err := server.Register(name, implementation); err != nil {
			panic(err)
		}
	}
	mustRegister("CatalogService", &CatalogService{})
	mustRegister("CartService", cartSvc)
	mustRegister("OrderService", NewOrderService(cartSvc))

	fmt.Println("Server starting on http://localhost:8080")
	server.ServeForever()
}
