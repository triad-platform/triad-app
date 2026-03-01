package orders

import "encoding/json"

const OrdersCreatedSubject = "orders.created.v1"

type CreateOrderRequest struct {
	UserID   string      `json:"user_id"`
	Items    []OrderItem `json:"items"`
	Currency string      `json:"currency"`
}

type OrderItem struct {
	SKU        string `json:"sku"`
	Qty        int    `json:"-"`
	PriceCents int    `json:"-"`
}

func (o *OrderItem) UnmarshalJSON(data []byte) error {
	type rawOrderItem struct {
		SKU        string `json:"sku"`
		Qty        int    `json:"qty"`
		Quantity   int    `json:"quantity"`
		PriceCents int    `json:"price_cents"`
		UnitPrice  int    `json:"unit_price"`
	}

	var raw rawOrderItem
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	o.SKU = raw.SKU
	if raw.Quantity > 0 {
		o.Qty = raw.Quantity
	} else {
		o.Qty = raw.Qty
	}
	if raw.UnitPrice > 0 {
		o.PriceCents = raw.UnitPrice
	} else {
		o.PriceCents = raw.PriceCents
	}
	return nil
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
