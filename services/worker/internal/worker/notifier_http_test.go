package worker

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestHTTPNotifier_NotifyOrderCreated(t *testing.T) {
	t.Parallel()

	var got OrdersCreatedEvent
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method mismatch: got=%s want=%s", r.Method, http.MethodPost)
		}
		if r.URL.Path != "/v1/notify" {
			t.Fatalf("path mismatch: got=%s want=%s", r.URL.Path, "/v1/notify")
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("content-type mismatch: got=%s want=application/json", ct)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	})

	n := NewHTTPNotifier("http://notifications", 2*time.Second)
	n.Client = &http.Client{
		Timeout:   2 * time.Second,
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) { return runHandler(handler, req), nil }),
	}
	event := OrdersCreatedEvent{
		OrderID:    "o-1",
		UserID:     "u-1",
		TotalCents: 1500,
		Currency:   "USD",
		CreatedAt:  "2026-02-27T00:00:00Z",
	}
	if err := n.NotifyOrderCreated(context.Background(), event); err != nil {
		t.Fatalf("notify failed: %v", err)
	}

	if got.OrderID != event.OrderID {
		t.Fatalf("order_id mismatch: got=%s want=%s", got.OrderID, event.OrderID)
	}
}

func TestHTTPNotifier_NotifyOrderCreated_NonAccepted(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	n := NewHTTPNotifier("http://notifications", 2*time.Second)
	n.Client = &http.Client{
		Timeout:   2 * time.Second,
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) { return runHandler(handler, req), nil }),
	}
	err := n.NotifyOrderCreated(context.Background(), OrdersCreatedEvent{OrderID: "o-1", UserID: "u-1", TotalCents: 10, Currency: "USD"})
	if err == nil {
		t.Fatal("expected error for non-202 status")
	}
}
