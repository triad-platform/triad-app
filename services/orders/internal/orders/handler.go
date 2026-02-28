package orders

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/triad-platform/triad-app/pkg/metricsx"
)

const (
	idempotencyHeader = "Idempotency-Key"
	requestIDHeader   = "X-Request-Id"
)

type IdempotencyStore interface {
	Reserve(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

type EventPublisher interface {
	PublishOrdersCreated(ctx context.Context, event OrdersCreatedEvent) error
}

type OrderStore interface {
	CreateOrder(ctx context.Context, params CreateOrderParams) (PersistedOrder, error)
}

type Handler struct {
	IdempotencyStore IdempotencyStore
	EventPublisher   EventPublisher
	OrderStore       OrderStore
	Metrics          *metricsx.Registry
	IdempotencyTTL   time.Duration
	// TODO: add DB and Logger dependencies
}

func Routes(h *Handler) chi.Router {
	r := chi.NewRouter()
	r.Post("/v1/orders", h.CreateOrder)
	return r
}

func (h *Handler) idempotencyTTL() time.Duration {
	if h.IdempotencyTTL <= 0 {
		return 24 * time.Hour
	}
	return h.IdempotencyTTL
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer h.observeDuration("create_order_duration", time.Since(start))
	h.inc("create_order_requests_total")

	idempotencyKey := strings.TrimSpace(r.Header.Get(idempotencyHeader))
	if idempotencyKey == "" {
		h.inc("create_order_validation_errors_total")
		http.Error(w, "missing Idempotency-Key header", http.StatusBadRequest)
		return
	}

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.inc("create_order_validation_errors_total")
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// minimal validation placeholder
	if req.UserID == "" || len(req.Items) == 0 || req.Currency == "" {
		h.inc("create_order_validation_errors_total")
		http.Error(w, "missing fields", http.StatusBadRequest)
		return
	}

	if h.IdempotencyStore == nil {
		h.inc("create_order_service_errors_total")
		http.Error(w, "idempotency store not configured", http.StatusServiceUnavailable)
		return
	}

	reserved, err := h.IdempotencyStore.Reserve(r.Context(), idempotencyKey, h.idempotencyTTL())
	if err != nil {
		h.inc("create_order_idempotency_errors_total")
		http.Error(w, "idempotency check failed", http.StatusServiceUnavailable)
		return
	}
	if !reserved {
		h.inc("create_order_duplicates_total")
		http.Error(w, "duplicate request", http.StatusConflict)
		return
	}

	if h.EventPublisher == nil {
		h.inc("create_order_service_errors_total")
		http.Error(w, "event publisher not configured", http.StatusServiceUnavailable)
		return
	}
	if h.OrderStore == nil {
		h.inc("create_order_service_errors_total")
		http.Error(w, "order store not configured", http.StatusServiceUnavailable)
		return
	}

	orderID := newOrderID()
	persistedOrder, err := h.OrderStore.CreateOrder(r.Context(), CreateOrderParams{
		OrderID:  orderID,
		UserID:   req.UserID,
		Items:    req.Items,
		Currency: req.Currency,
	})
	if err != nil {
		h.inc("create_order_persistence_errors_total")
		http.Error(w, "order persistence failed", http.StatusServiceUnavailable)
		return
	}

	event := OrdersCreatedEvent{
		Type:       "OrdersCreated",
		Version:    1,
		OrderID:    persistedOrder.OrderID,
		UserID:     persistedOrder.UserID,
		RequestID:  strings.TrimSpace(r.Header.Get(requestIDHeader)),
		TotalCents: persistedOrder.TotalCents,
		Currency:   persistedOrder.Currency,
		CreatedAt:  persistedOrder.CreatedAt.UTC().Format(time.RFC3339),
	}
	if err := h.EventPublisher.PublishOrdersCreated(r.Context(), event); err != nil {
		h.inc("create_order_publish_errors_total")
		http.Error(w, "event publish failed", http.StatusServiceUnavailable)
		return
	}

	// TODO: adopt transactional outbox for DB + event atomicity.
	resp := CreateOrderResponse{
		OrderID: persistedOrder.OrderID,
		Status:  "created",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
	h.inc("create_order_success_total")
}

func calculateTotalCents(items []OrderItem) int {
	total := 0
	for _, item := range items {
		total += item.Qty * item.PriceCents
	}
	return total
}

func newOrderID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "o-fallback"
	}
	return hex.EncodeToString(b)
}

func (h *Handler) inc(name string) {
	if h.Metrics != nil {
		h.Metrics.Inc(name)
	}
}

func (h *Handler) observeDuration(name string, d time.Duration) {
	if h.Metrics != nil {
		h.Metrics.ObserveDuration(name, d)
	}
}
