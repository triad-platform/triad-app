package orders

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type CreateOrderParams struct {
	OrderID  string
	UserID   string
	Items    []OrderItem
	Currency string
}

type PersistedOrder struct {
	OrderID    string
	UserID     string
	TotalCents int
	Currency   string
	CreatedAt  time.Time
}

type PostgresOrderStore struct {
	conn *pgx.Conn
}

func NewPostgresOrderStore(conn *pgx.Conn) *PostgresOrderStore {
	return &PostgresOrderStore{conn: conn}
}

func (s *PostgresOrderStore) EnsureSchema(ctx context.Context) error {
	_, err := s.conn.Exec(ctx, `
CREATE TABLE IF NOT EXISTS orders (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	total_cents INTEGER NOT NULL,
	currency TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS order_items (
	id BIGSERIAL PRIMARY KEY,
	order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
	sku TEXT NOT NULL,
	qty INTEGER NOT NULL,
	price_cents INTEGER NOT NULL
);
`)
	return err
}

func (s *PostgresOrderStore) CreateOrder(ctx context.Context, params CreateOrderParams) (PersistedOrder, error) {
	totalCents := calculateTotalCents(params.Items)
	tx, err := s.conn.Begin(ctx)
	if err != nil {
		return PersistedOrder{}, err
	}
	defer tx.Rollback(ctx)

	var createdAt time.Time
	err = tx.QueryRow(
		ctx,
		`INSERT INTO orders (id, user_id, total_cents, currency) VALUES ($1, $2, $3, $4) RETURNING created_at`,
		params.OrderID, params.UserID, totalCents, params.Currency,
	).Scan(&createdAt)
	if err != nil {
		return PersistedOrder{}, err
	}

	for _, item := range params.Items {
		_, err = tx.Exec(
			ctx,
			`INSERT INTO order_items (order_id, sku, qty, price_cents) VALUES ($1, $2, $3, $4)`,
			params.OrderID, item.SKU, item.Qty, item.PriceCents,
		)
		if err != nil {
			return PersistedOrder{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return PersistedOrder{}, fmt.Errorf("commit order transaction: %w", err)
	}

	return PersistedOrder{
		OrderID:    params.OrderID,
		UserID:     params.UserID,
		TotalCents: totalCents,
		Currency:   params.Currency,
		CreatedAt:  createdAt,
	}, nil
}
