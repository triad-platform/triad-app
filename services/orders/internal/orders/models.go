package orders

const OrdersCreatedSubject = "orders.created.v1"

type CreateOrderRequest struct {
	UserID   string      `json:"user_id"`
	Items    []OrderItem `json:"items"`
	Currency string      `json:"currency"`
}

type OrderItem struct {
	SKU        string `json:"sku"`
	Qty        int    `json:"qty"`
	PriceCents int    `json:"price_cents"`
}

type CreateOrderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

type OrdersCreatedEvent struct {
	Type       string `json:"type"`
	Version    int    `json:"version"`
	OrderID    string `json:"order_id"`
	UserID     string `json:"user_id"`
	RequestID  string `json:"request_id"`
	TotalCents int    `json:"total_cents"`
	Currency   string `json:"currency"`
	CreatedAt  string `json:"created_at"`
}
