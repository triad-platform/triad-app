package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
)

func TestNotifyEndpoint(t *testing.T) {
	t.Parallel()

	r := newRouter(zerolog.Nop())

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
