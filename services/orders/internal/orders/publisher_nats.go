package orders

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"
)

type NATSEventPublisher struct {
	conn    *nats.Conn
	subject string
}

func NewNATSEventPublisher(conn *nats.Conn, subject string) *NATSEventPublisher {
	if subject == "" {
		subject = OrdersCreatedSubject
	}
	return &NATSEventPublisher{
		conn:    conn,
		subject: subject,
	}
}

func (p *NATSEventPublisher) PublishOrdersCreated(_ context.Context, event OrdersCreatedEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.conn.Publish(p.subject, payload)
}
