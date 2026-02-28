package worker

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/triad-platform/triad-app/pkg/metricsx"
)

type OrdersCreatedEvent struct {
	OrderID    string `json:"order_id"`
	UserID     string `json:"user_id"`
	RequestID  string `json:"request_id"`
	TotalCents int    `json:"total_cents"`
	Currency   string `json:"currency"`
	CreatedAt  string `json:"created_at"`
}

type IdempotencyStore interface {
	Reserve(ctx context.Context, key string, ttl time.Duration) (bool, error)
}

type Notifier interface {
	NotifyOrderCreated(ctx context.Context, event OrdersCreatedEvent) error
}

type Processor struct {
	IdempotencyStore IdempotencyStore
	Notifier         Notifier
	Metrics          *metricsx.Registry
	KeyPrefix        string
	IdempotencyTTL   time.Duration
}

func DecodeOrdersCreated(data []byte) (OrdersCreatedEvent, error) {
	var event OrdersCreatedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return OrdersCreatedEvent{}, err
	}
	return event, nil
}

func (p *Processor) ProcessOrdersCreated(ctx context.Context, event OrdersCreatedEvent) (bool, error) {
	start := time.Now()
	defer p.observeDuration("process_orders_created_duration", time.Since(start))
	p.inc("messages_received_total")

	if event.OrderID == "" {
		p.inc("messages_errors_total")
		return false, errors.New("missing order_id")
	}
	if p.IdempotencyStore == nil {
		p.inc("messages_errors_total")
		return false, errors.New("idempotency store not configured")
	}
	if p.Notifier == nil {
		p.inc("messages_errors_total")
		return false, errors.New("notifier not configured")
	}

	reserved, err := p.IdempotencyStore.Reserve(ctx, p.key(event.OrderID), p.idempotencyTTL())
	if err != nil {
		p.inc("messages_errors_total")
		p.inc("idempotency_errors_total")
		return false, err
	}
	if !reserved {
		// Duplicate event replay; no side effects should run twice.
		p.inc("messages_duplicates_total")
		return false, nil
	}

	if err := p.Notifier.NotifyOrderCreated(ctx, event); err != nil {
		p.inc("messages_errors_total")
		p.inc("notifier_errors_total")
		return false, err
	}
	p.inc("messages_processed_total")
	return true, nil
}

func (p *Processor) HandleOrdersCreatedMessage(ctx context.Context, data []byte) (bool, error) {
	event, err := DecodeOrdersCreated(data)
	if err != nil {
		return false, err
	}
	return p.ProcessOrdersCreated(ctx, event)
}

func (p *Processor) key(orderID string) string {
	prefix := p.KeyPrefix
	if prefix == "" {
		prefix = "worker:orders-created:"
	}
	return prefix + orderID
}

func (p *Processor) idempotencyTTL() time.Duration {
	if p.IdempotencyTTL <= 0 {
		return 24 * time.Hour
	}
	return p.IdempotencyTTL
}

func (p *Processor) inc(name string) {
	if p.Metrics != nil {
		p.Metrics.Inc(name)
	}
}

func (p *Processor) observeDuration(name string, d time.Duration) {
	if p.Metrics != nil {
		p.Metrics.ObserveDuration(name, d)
	}
}
