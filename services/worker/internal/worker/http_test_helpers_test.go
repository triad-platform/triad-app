package worker

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func runHandler(handler http.Handler, req *http.Request) *http.Response {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	result := rec.Result()
	if result.Body == nil {
		result.Body = io.NopCloser(strings.NewReader(""))
	}
	return result
}
