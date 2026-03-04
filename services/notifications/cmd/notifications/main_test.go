package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/triad-platform/triad-app/pkg/metricsx"
)

func TestNotifyEndpoint(t *testing.T) {
	t.Parallel()

	metrics := metricsx.NewRegistry("triad_notifications")
	r := newRouter(zerolog.Nop(), metrics)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid json",
			body:       "{",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing order_id",
			body:       `{"user_id":"u-1","total_cents":1500,"currency":"USD","created_at":"2026-02-27T00:00:00Z"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "valid payload",
			body:       `{"order_id":"o-1","user_id":"u-1","total_cents":1500,"currency":"USD","created_at":"2026-02-27T00:00:00Z"}`,
			wantStatus: http.StatusAccepted,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodPost, "/v1/notify", bytes.NewBufferString(tc.body))
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Fatalf("status mismatch: got=%d want=%d body=%q", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestMetricsEndpoint(t *testing.T) {
	t.Parallel()

	metrics := metricsx.NewRegistry("triad_notifications")
	r := newRouter(zerolog.Nop(), metrics)

	req := httptest.NewRequest(http.MethodPost, "/v1/notify", bytes.NewBufferString(`{"order_id":"o-1","user_id":"u-1","total_cents":1500,"currency":"USD","created_at":"2026-02-27T00:00:00Z"}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("notify status mismatch: got=%d want=%d", rec.Code, http.StatusAccepted)
	}

	metricsReq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	metricsRec := httptest.NewRecorder()
	r.ServeHTTP(metricsRec, metricsReq)
	if metricsRec.Code != http.StatusOK {
		t.Fatalf("metrics status mismatch: got=%d want=%d", metricsRec.Code, http.StatusOK)
	}
	body := metricsRec.Body.String()
	if !strings.Contains(body, "triad_notifications_requests_total 1") {
		t.Fatalf("expected requests counter in metrics body, got=%q", body)
	}
	if !strings.Contains(body, "triad_notifications_accepted_total 1") {
		t.Fatalf("expected accepted counter in metrics body, got=%q", body)
	}
}
