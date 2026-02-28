package worker

import (
	"context"

	"github.com/rs/zerolog"
)

type LogNotifier struct {
	Logger zerolog.Logger
}

func (n *LogNotifier) NotifyOrderCreated(_ context.Context, event OrdersCreatedEvent) error {
	n.Logger.Info().
		Str("order_id", event.OrderID).
		Str("user_id", event.UserID).
		Str("request_id", event.RequestID).
		Int("total_cents", event.TotalCents).
		Str("currency", event.Currency).
		Msg("worker processed orders.created.v1 event")
	return nil
}
