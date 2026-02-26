package httpx

import (
	"net/http"
)

func Healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func Readyz(w http.ResponseWriter, _ *http.Request) {
	// TODO: real readiness checks per service (db/nats/redis)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}
