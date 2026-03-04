package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewRouter_HealthEndpoints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		wantCode int
	}{
		{name: "healthz", path: "/healthz", wantCode: http.StatusOK},
		{name: "readyz", path: "/readyz", wantCode: http.StatusOK},
		{name: "unknown route", path: "/does-not-exist", wantCode: http.StatusNotFound},
	}

	r := newRouter()

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tc.wantCode {
				t.Fatalf("status code mismatch for %s: got=%d want=%d body=%q", tc.path, rec.Code, tc.wantCode, rec.Body.String())
			}
		})
	}
}

func TestGateway_OrderForwarding(t *testing.T) {
	t.Parallel()

	var capturedReq *http.Request
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			capturedReq = req
			return &http.Response{
				StatusCode: http.StatusCreated,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"order_id":"o-1","status":"created"}`)),
			}, nil
		}),
	}
	r := newRouterWithConfig(gatewayConfig{
		OrdersURL:       "http://orders:8081",
		RequestTimeout:  time.Second,
		UpstreamTimeout: time.Second,
		Client:          client,
	}, nil)

	body := `{"user_id":"u_1","items":[{"sku":"sku_1","qty":1,"price_cents":100}],"currency":"USD"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-1")
	req.Header.Set(requestIDHeader, "req-123")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status code mismatch: got=%d want=%d body=%q", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if capturedReq == nil {
		t.Fatal("expected upstream request to be sent")
	}
	if got, want := capturedReq.URL.String(), "http://orders:8081/v1/orders"; got != want {
		t.Fatalf("upstream url mismatch: got=%q want=%q", got, want)
	}
	if got, want := capturedReq.Header.Get("Idempotency-Key"), "idem-1"; got != want {
		t.Fatalf("idempotency header mismatch: got=%q want=%q", got, want)
	}
	if got, want := capturedReq.Header.Get(requestIDHeader), "req-123"; got != want {
		t.Fatalf("request id mismatch: got=%q want=%q", got, want)
	}
	if got := rec.Header().Get(requestIDHeader); got == "" {
		t.Fatal("response should include request id")
	}
}

func TestGateway_AssignsRequestIDWhenMissing(t *testing.T) {
	t.Parallel()

	var upstreamRequestID string
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			upstreamRequestID = req.Header.Get(requestIDHeader)
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
			}, nil
		}),
	}
	r := newRouterWithConfig(gatewayConfig{
		OrdersURL:       "http://orders:8081",
		RequestTimeout:  time.Second,
		UpstreamTimeout: time.Second,
		Client:          client,
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(`{"user_id":"u_1","items":[{"sku":"sku_1","qty":1,"price_cents":100}],"currency":"USD"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-2")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status code mismatch: got=%d want=%d body=%q", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if upstreamRequestID == "" {
		t.Fatal("expected generated request id to be sent upstream")
	}
	if got := rec.Header().Get(requestIDHeader); got == "" {
		t.Fatal("expected generated request id in response headers")
	}
}

func TestGateway_UpstreamTimeout(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()
			return nil, req.Context().Err()
		}),
	}
	r := newRouterWithConfig(gatewayConfig{
		OrdersURL:       "http://orders:8081",
		RequestTimeout:  20 * time.Millisecond,
		UpstreamTimeout: time.Second,
		Client:          client,
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/orders", strings.NewReader(`{"user_id":"u_1","items":[{"sku":"sku_1","qty":1,"price_cents":100}],"currency":"USD"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "idem-timeout")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("status code mismatch: got=%d want=%d body=%q", rec.Code, http.StatusGatewayTimeout, rec.Body.String())
	}
}

func TestGateway_AsyncStatus(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.String() {
			case "http://worker:9091/metrics":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(
						"# TYPE triad_worker_messages_processed_total counter\n" +
							"triad_worker_messages_processed_total 3\n" +
							"triad_worker_messages_duplicates_total 1\n" +
							"triad_worker_messages_errors_total 0\n",
					)),
				}, nil
			case "http://notifications:8082/metrics":
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(
						"# TYPE triad_notifications_accepted_total counter\n" +
							"triad_notifications_accepted_total 3\n" +
							"triad_notifications_validation_errors_total 1\n",
					)),
				}, nil
			default:
				return nil, io.EOF
			}
		}),
	}

	r := newRouterWithConfig(gatewayConfig{
		EnableDevDiagnostics:    true,
		WorkerMetricsURL:        "http://worker:9091/metrics",
		NotificationsMetricsURL: "http://notifications:8082/metrics",
		Client:                  client,
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/v1/dev/async-status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code mismatch: got=%d want=%d body=%q", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp asyncStatusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.WorkerMessagesProcessed != 3 {
		t.Fatalf("worker processed mismatch: got=%d want=3", resp.WorkerMessagesProcessed)
	}
	if resp.NotificationsAccepted != 3 {
		t.Fatalf("notifications accepted mismatch: got=%d want=3", resp.NotificationsAccepted)
	}
}

func TestGateway_AsyncStatusDisabled(t *testing.T) {
	t.Parallel()

	r := newRouterWithConfig(gatewayConfig{}, nil)
	req := httptest.NewRequest(http.MethodGet, "/v1/dev/async-status", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status code mismatch: got=%d want=%d body=%q", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
