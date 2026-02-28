package worker

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

func TestRunNATSSubscriber_ReplayIdempotency(t *testing.T) {
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		natsURL = "nats://localhost:4222"
	}

	nc, err := nats.Connect(natsURL, nats.Timeout(500*time.Millisecond))
	if err != nil {
		t.Skipf("skipping integration test; NATS not reachable at %s: %v", natsURL, err)
	}
	defer nc.Close()

	store := &syncStatefulStore{keys: map[string]struct{}{}}
	notifier := &syncNotifier{}
	processor := &Processor{
		IdempotencyStore: store,
		Notifier:         notifier,
		KeyPrefix:        "worker:orders-created:",
		IdempotencyTTL:   time.Minute,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	subject := "orders.created.v1.integration"
	go func() {
		errCh <- RunNATSSubscriber(ctx, nc, subject, processor, zerolog.Nop())
	}()

	payload := []byte(`{"order_id":"o-integration-1","user_id":"u-1","total_cents":1500,"currency":"USD","created_at":"2026-02-27T00:00:00Z"}`)
	if err := nc.Publish(subject, payload); err != nil {
		t.Fatalf("publish #1 failed: %v", err)
	}
	if err := nc.Publish(subject, payload); err != nil {
		t.Fatalf("publish #2 failed: %v", err)
	}
	if err := nc.Flush(); err != nil {
		t.Fatalf("flush failed: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if notifier.Calls() == 1 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := notifier.Calls(); got != 1 {
		t.Fatalf("duplicate replay should notify exactly once: got=%d want=1", got)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("subscriber returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("subscriber did not exit after cancel")
	}
}

type syncStatefulStore struct {
	mu   sync.Mutex
	keys map[string]struct{}
}

func (s *syncStatefulStore) Reserve(_ context.Context, key string, _ time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.keys[key]; exists {
		return false, nil
	}
	s.keys[key] = struct{}{}
	return true, nil
}

type syncNotifier struct {
	mu    sync.Mutex
	calls int
}

func (n *syncNotifier) NotifyOrderCreated(_ context.Context, _ OrdersCreatedEvent) error {
	n.mu.Lock()
	n.calls++
	n.mu.Unlock()
	return nil
}

func (n *syncNotifier) Calls() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.calls
}
