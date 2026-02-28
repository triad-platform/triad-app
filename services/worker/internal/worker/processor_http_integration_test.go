package worker

import (
	"context"
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestProcessor_HTTPNotifier_ReplaySendsSingleNotification(t *testing.T) {
	t.Parallel()

	var (
		mu    sync.Mutex
		calls int
	)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/v1/notify" {
			mu.Lock()
			calls++
			mu.Unlock()
			w.WriteHeader(http.StatusAccepted)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	notifier := NewHTTPNotifier("http://notifications", 2*time.Second)
	notifier.Client = &http.Client{
		Timeout:   2 * time.Second,
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) { return runHandler(handler, req), nil }),
	}

	p := &Processor{
		IdempotencyStore: &statefulStore{keys: map[string]struct{}{}},
		Notifier:         notifier,
		KeyPrefix:        "worker:orders-created:",
		IdempotencyTTL:   time.Minute,
	}

	event := OrdersCreatedEvent{
		OrderID:    "o-http-replay",
		UserID:     "u-1",
		TotalCents: 900,
		Currency:   "USD",
		CreatedAt:  "2026-02-27T00:00:00Z",
	}

	processed1, err := p.ProcessOrdersCreated(context.Background(), event)
	if err != nil {
		t.Fatalf("first process failed: %v", err)
	}
	if !processed1 {
		t.Fatal("first process should be processed=true")
	}

	processed2, err := p.ProcessOrdersCreated(context.Background(), event)
	if err != nil {
		t.Fatalf("second process failed: %v", err)
	}
	if processed2 {
		t.Fatal("second process should be processed=false due to idempotency")
	}

	mu.Lock()
	gotCalls := calls
	mu.Unlock()
	if gotCalls != 1 {
		t.Fatalf("notifications endpoint should be called exactly once: got=%d want=1", gotCalls)
	}
}
