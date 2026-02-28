package worker

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestProcessOrdersCreated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		event         OrdersCreatedEvent
		store         *stubStore
		notifier      *stubNotifier
		wantProcessed bool
		wantErr       bool
		wantNotifies  int
	}{
		{
			name:          "missing order id",
			event:         OrdersCreatedEvent{},
			store:         &stubStore{reserveResult: true},
			notifier:      &stubNotifier{},
			wantProcessed: false,
			wantErr:       true,
			wantNotifies:  0,
		},
		{
			name:          "store error",
			event:         OrdersCreatedEvent{OrderID: "o-1"},
			store:         &stubStore{reserveErr: errors.New("store unavailable")},
			notifier:      &stubNotifier{},
			wantProcessed: false,
			wantErr:       true,
			wantNotifies:  0,
		},
		{
			name:          "duplicate event",
			event:         OrdersCreatedEvent{OrderID: "o-dup"},
			store:         &stubStore{reserveResult: false},
			notifier:      &stubNotifier{},
			wantProcessed: false,
			wantErr:       false,
			wantNotifies:  0,
		},
		{
			name:          "notifier error",
			event:         OrdersCreatedEvent{OrderID: "o-2"},
			store:         &stubStore{reserveResult: true},
			notifier:      &stubNotifier{err: errors.New("notify failed")},
			wantProcessed: false,
			wantErr:       true,
			wantNotifies:  1,
		},
		{
			name:          "success",
			event:         OrdersCreatedEvent{OrderID: "o-3", UserID: "u-1", TotalCents: 1200, Currency: "USD"},
			store:         &stubStore{reserveResult: true},
			notifier:      &stubNotifier{},
			wantProcessed: true,
			wantErr:       false,
			wantNotifies:  1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := &Processor{
				IdempotencyStore: tc.store,
				Notifier:         tc.notifier,
				KeyPrefix:        "worker:orders-created:",
				IdempotencyTTL:   time.Minute,
			}
			gotProcessed, err := p.ProcessOrdersCreated(context.Background(), tc.event)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error mismatch: gotErr=%v wantErr=%v err=%v", err != nil, tc.wantErr, err)
			}
			if gotProcessed != tc.wantProcessed {
				t.Fatalf("processed mismatch: got=%v want=%v", gotProcessed, tc.wantProcessed)
			}
			if tc.notifier.calls != tc.wantNotifies {
				t.Fatalf("notifier calls mismatch: got=%d want=%d", tc.notifier.calls, tc.wantNotifies)
			}
		})
	}
}

func TestProcessOrdersCreated_ReplayIdempotency(t *testing.T) {
	t.Parallel()

	store := &statefulStore{keys: map[string]struct{}{}}
	notifier := &stubNotifier{}
	p := &Processor{
		IdempotencyStore: store,
		Notifier:         notifier,
		KeyPrefix:        "worker:orders-created:",
		IdempotencyTTL:   time.Minute,
	}

	event := OrdersCreatedEvent{
		OrderID:    "o-replay",
		UserID:     "u-1",
		TotalCents: 2200,
		Currency:   "USD",
		CreatedAt:  "2026-02-26T00:00:00Z",
	}

	processed1, err := p.ProcessOrdersCreated(context.Background(), event)
	if err != nil {
		t.Fatalf("first process returned error: %v", err)
	}
	if !processed1 {
		t.Fatal("first process should be processed=true")
	}

	processed2, err := p.ProcessOrdersCreated(context.Background(), event)
	if err != nil {
		t.Fatalf("second process returned error: %v", err)
	}
	if processed2 {
		t.Fatal("second process should be processed=false due to idempotency replay protection")
	}

	if notifier.calls != 1 {
		t.Fatalf("notifier should be called exactly once for replayed event: got=%d want=1", notifier.calls)
	}
}

func TestHandleOrdersCreatedMessage(t *testing.T) {
	t.Parallel()

	store := &stubStore{reserveResult: true}
	notifier := &stubNotifier{}
	p := &Processor{
		IdempotencyStore: store,
		Notifier:         notifier,
	}

	processed, err := p.HandleOrdersCreatedMessage(context.Background(), []byte(`{"order_id":"o-raw","user_id":"u-1","total_cents":1500,"currency":"USD","created_at":"2026-02-26T00:00:00Z"}`))
	if err != nil {
		t.Fatalf("handle message returned error: %v", err)
	}
	if !processed {
		t.Fatal("expected processed=true")
	}
	if notifier.calls != 1 {
		t.Fatalf("notifier calls mismatch: got=%d want=1", notifier.calls)
	}
}

type stubStore struct {
	reserveResult bool
	reserveErr    error
}

func (s *stubStore) Reserve(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return s.reserveResult, s.reserveErr
}

type statefulStore struct {
	keys map[string]struct{}
}

func (s *statefulStore) Reserve(_ context.Context, key string, _ time.Duration) (bool, error) {
	if _, exists := s.keys[key]; exists {
		return false, nil
	}
	s.keys[key] = struct{}{}
	return true, nil
}

type stubNotifier struct {
	calls int
	err   error
}

func (n *stubNotifier) NotifyOrderCreated(_ context.Context, _ OrdersCreatedEvent) error {
	n.calls++
	return n.err
}
