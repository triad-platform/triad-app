package orders

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateOrder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		body           string
		idempotencyKey string
		requestID      string
		store          IdempotencyStore
		orderStore     *stubOrderStore
		publisher      *stubPublisher
		wantStatusCode int
		wantStoreCalls int
		wantPublishes  int
	}{
		{
			name:           "invalid json",
			body:           "{",
			idempotencyKey: "idem-invalid-json",
			store:          &stubIdempotencyStore{reserveResult: true},
			orderStore:     &stubOrderStore{},
			publisher:      &stubPublisher{},
			wantStatusCode: http.StatusBadRequest,
			wantStoreCalls: 0,
			wantPublishes:  0,
		},
		{
			name:           "missing user_id",
			body:           `{"items":[{"sku":"sku_1","qty":1,"price_cents":100}],"currency":"USD"}`,
			idempotencyKey: "idem-missing-user",
			store:          &stubIdempotencyStore{reserveResult: true},
			orderStore:     &stubOrderStore{},
			publisher:      &stubPublisher{},
			wantStatusCode: http.StatusBadRequest,
			wantStoreCalls: 0,
			wantPublishes:  0,
		},
		{
			name:           "missing items",
			body:           `{"user_id":"u_123","items":[],"currency":"USD"}`,
			idempotencyKey: "idem-missing-items",
			store:          &stubIdempotencyStore{reserveResult: true},
			orderStore:     &stubOrderStore{},
			publisher:      &stubPublisher{},
			wantStatusCode: http.StatusBadRequest,
			wantStoreCalls: 0,
			wantPublishes:  0,
		},
		{
			name:           "invalid item values",
			body:           `{"user_id":"u_123","items":[{"sku":"sku_1","quantity":0,"unit_price":0}],"currency":"USD"}`,
			idempotencyKey: "idem-invalid-items",
			store:          &stubIdempotencyStore{reserveResult: true},
			orderStore:     &stubOrderStore{},
			publisher:      &stubPublisher{},
			wantStatusCode: http.StatusBadRequest,
			wantStoreCalls: 0,
			wantPublishes:  0,
		},
		{
			name:           "missing currency",
			body:           `{"user_id":"u_123","items":[{"sku":"sku_1","qty":1,"price_cents":100}]}`,
			idempotencyKey: "idem-missing-currency",
			store:          &stubIdempotencyStore{reserveResult: true},
			orderStore:     &stubOrderStore{},
			publisher:      &stubPublisher{},
			wantStatusCode: http.StatusBadRequest,
			wantStoreCalls: 0,
			wantPublishes:  0,
		},
		{
			name:           "missing idempotency header",
			body:           `{"user_id":"u_123","items":[{"sku":"sku_1","qty":1,"price_cents":100}],"currency":"USD"}`,
			idempotencyKey: "",
			store:          &stubIdempotencyStore{reserveResult: true},
			orderStore:     &stubOrderStore{},
			publisher:      &stubPublisher{},
			wantStatusCode: http.StatusBadRequest,
			wantStoreCalls: 0,
			wantPublishes:  0,
		},
		{
			name:           "idempotency store error",
			body:           `{"user_id":"u_123","items":[{"sku":"sku_1","qty":1,"price_cents":100}],"currency":"USD"}`,
			idempotencyKey: "idem-store-error",
			store:          &stubIdempotencyStore{reserveErr: errors.New("boom")},
			orderStore:     &stubOrderStore{},
			publisher:      &stubPublisher{},
			wantStatusCode: http.StatusServiceUnavailable,
			wantStoreCalls: 0,
			wantPublishes:  0,
		},
		{
			name:           "duplicate idempotency key",
			body:           `{"user_id":"u_123","items":[{"sku":"sku_1","qty":1,"price_cents":100}],"currency":"USD"}`,
			idempotencyKey: "idem-duplicate",
			store:          &stubIdempotencyStore{reserveResult: false},
			orderStore:     &stubOrderStore{},
			publisher:      &stubPublisher{},
			wantStatusCode: http.StatusConflict,
			wantStoreCalls: 0,
			wantPublishes:  0,
		},
		{
			name:           "persistence error",
			body:           `{"user_id":"u_123","items":[{"sku":"sku_1","qty":1,"price_cents":100}],"currency":"USD"}`,
			idempotencyKey: "idem-store-write-error",
			store:          &stubIdempotencyStore{reserveResult: true},
			orderStore:     &stubOrderStore{err: errors.New("db unavailable")},
			publisher:      &stubPublisher{},
			wantStatusCode: http.StatusServiceUnavailable,
			wantStoreCalls: 1,
			wantPublishes:  0,
		},
		{
			name:           "publish error",
			body:           `{"user_id":"u_123","items":[{"sku":"sku_1","qty":1,"price_cents":100}],"currency":"USD"}`,
			idempotencyKey: "idem-publish-error",
			store:          &stubIdempotencyStore{reserveResult: true},
			orderStore:     &stubOrderStore{persisted: PersistedOrder{OrderID: "o-publish", UserID: "u_123", TotalCents: 100, Currency: "USD", CreatedAt: time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)}},
			publisher:      &stubPublisher{err: errors.New("publish failed")},
			wantStatusCode: http.StatusServiceUnavailable,
			wantStoreCalls: 1,
			wantPublishes:  1,
		},
		{
			name:           "valid request",
			body:           `{"user_id":"u_123","items":[{"sku":"sku_1","quantity":1,"unit_price":100}],"currency":"USD"}`,
			idempotencyKey: "idem-valid",
			requestID:      "req-test-123",
			store:          &stubIdempotencyStore{reserveResult: true},
			orderStore:     &stubOrderStore{persisted: PersistedOrder{OrderID: "o-valid", UserID: "u_123", TotalCents: 100, Currency: "USD", CreatedAt: time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)}},
			publisher:      &stubPublisher{},
			wantStatusCode: http.StatusCreated,
			wantStoreCalls: 1,
			wantPublishes:  1,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			h := &Handler{
				IdempotencyStore: tc.store,
				OrderStore:       tc.orderStore,
				EventPublisher:   tc.publisher,
				IdempotencyTTL:   time.Minute,
			}

			req := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(tc.body))
			if tc.idempotencyKey != "" {
				req.Header.Set(idempotencyHeader, tc.idempotencyKey)
			}
			if tc.requestID != "" {
				req.Header.Set(requestIDHeader, tc.requestID)
			}
			rec := httptest.NewRecorder()

			h.CreateOrder(rec, req)

			if rec.Code != tc.wantStatusCode {
				t.Fatalf("status code mismatch: got=%d want=%d body=%q", rec.Code, tc.wantStatusCode, rec.Body.String())
			}
			if tc.orderStore.calls != tc.wantStoreCalls {
				t.Fatalf("store calls mismatch: got=%d want=%d", tc.orderStore.calls, tc.wantStoreCalls)
			}
			if tc.publisher.calls != tc.wantPublishes {
				t.Fatalf("publish calls mismatch: got=%d want=%d", tc.publisher.calls, tc.wantPublishes)
			}

			if tc.wantStatusCode != http.StatusCreated {
				return
			}

			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Fatalf("content-type mismatch: got=%q want=%q", got, "application/json")
			}

			var resp CreateOrderResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}

			if resp.Status != "created" {
				t.Fatalf("status mismatch: got=%q want=%q", resp.Status, "created")
			}
			if resp.OrderID != tc.orderStore.persisted.OrderID {
				t.Fatalf("order_id mismatch: got=%q want=%q", resp.OrderID, tc.orderStore.persisted.OrderID)
			}
			if tc.publisher.lastEvent.OrderID != tc.orderStore.persisted.OrderID {
				t.Fatal("published event should include persisted order_id")
			}
			if tc.publisher.lastEvent.TotalCents != tc.orderStore.persisted.TotalCents {
				t.Fatalf("published total_cents mismatch: got=%d want=%d", tc.publisher.lastEvent.TotalCents, tc.orderStore.persisted.TotalCents)
			}
			if tc.publisher.lastEvent.Type != "OrdersCreated" || tc.publisher.lastEvent.Version != 1 {
				t.Fatalf("published event metadata mismatch: got type=%q version=%d", tc.publisher.lastEvent.Type, tc.publisher.lastEvent.Version)
			}
			if tc.requestID != "" && tc.publisher.lastEvent.RequestID != tc.requestID {
				t.Fatalf("published request_id mismatch: got=%q want=%q", tc.publisher.lastEvent.RequestID, tc.requestID)
			}
		})
	}
}

func TestCreateOrder_IdempotencyBehavior(t *testing.T) {
	t.Parallel()

	store := &statefulIdempotencyStore{
		keys: map[string]struct{}{},
	}
	orderStore := &stubOrderStore{persisted: PersistedOrder{OrderID: "o-repeat", UserID: "u_123", TotalCents: 100, Currency: "USD", CreatedAt: time.Date(2026, 2, 27, 0, 0, 0, 0, time.UTC)}}
	publisher := &stubPublisher{}
	h := &Handler{
		IdempotencyStore: store,
		OrderStore:       orderStore,
		EventPublisher:   publisher,
		IdempotencyTTL:   time.Minute,
	}

	body := `{"user_id":"u_123","items":[{"sku":"sku_1","quantity":1,"unit_price":100}],"currency":"USD"}`
	key := "idem-repeat"

	req1 := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(body))
	req1.Header.Set(idempotencyHeader, key)
	rec1 := httptest.NewRecorder()
	h.CreateOrder(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first request status mismatch: got=%d want=%d body=%q", rec1.Code, http.StatusCreated, rec1.Body.String())
	}

	req2 := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(body))
	req2.Header.Set(idempotencyHeader, key)
	rec2 := httptest.NewRecorder()
	h.CreateOrder(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("second request status mismatch: got=%d want=%d body=%q", rec2.Code, http.StatusConflict, rec2.Body.String())
	}
	if orderStore.calls != 1 {
		t.Fatalf("store should be called once across duplicate retries: got=%d want=1", orderStore.calls)
	}
	if publisher.calls != 1 {
		t.Fatalf("publish should occur once across duplicate retries: got=%d want=1", publisher.calls)
	}
}

type stubIdempotencyStore struct {
	reserveResult bool
	reserveErr    error
}

func (s *stubIdempotencyStore) Reserve(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return s.reserveResult, s.reserveErr
}

type statefulIdempotencyStore struct {
	keys map[string]struct{}
}

func (s *statefulIdempotencyStore) Reserve(_ context.Context, key string, _ time.Duration) (bool, error) {
	if _, ok := s.keys[key]; ok {
		return false, nil
	}
	s.keys[key] = struct{}{}
	return true, nil
}

type stubOrderStore struct {
	calls     int
	persisted PersistedOrder
	err       error
}

func (s *stubOrderStore) CreateOrder(_ context.Context, _ CreateOrderParams) (PersistedOrder, error) {
	s.calls++
	if s.err != nil {
		return PersistedOrder{}, s.err
	}
	return s.persisted, nil
}

type stubPublisher struct {
	calls     int
	lastEvent OrdersCreatedEvent
	err       error
}

func (p *stubPublisher) PublishOrdersCreated(_ context.Context, event OrdersCreatedEvent) error {
	p.calls++
	p.lastEvent = event
	return p.err
}
