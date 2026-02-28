package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/triad-platform/triad-app/pkg/config"
	"github.com/triad-platform/triad-app/pkg/httpx"
	"github.com/triad-platform/triad-app/pkg/logx"
)

func newRouter(log zerolog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", httpx.Healthz)
	r.Get("/readyz", httpx.Readyz)
	r.Post("/v1/notify", func(w http.ResponseWriter, r *http.Request) {
		var req notifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if err := req.validate(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Info().
			Str("order_id", req.OrderID).
			Str("user_id", req.UserID).
			Str("request_id", req.RequestID).
			Int("total_cents", req.TotalCents).
			Str("currency", req.Currency).
			Msg("notification accepted")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(notifyResponse{Status: "accepted"})
	})
	return r
}

func main() {
	log := logx.New()

	port := config.Getenv("PORT", "8082")

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           newRouter(log),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Msgf("notifications listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info().Msg("notifications shutdown complete")
}

type notifyRequest struct {
	OrderID    string `json:"order_id"`
	UserID     string `json:"user_id"`
	RequestID  string `json:"request_id"`
	TotalCents int    `json:"total_cents"`
	Currency   string `json:"currency"`
	CreatedAt  string `json:"created_at"`
}

func (r notifyRequest) validate() error {
	if r.OrderID == "" {
		return errors.New("missing order_id")
	}
	if r.UserID == "" {
		return errors.New("missing user_id")
	}
	if r.Currency == "" {
		return errors.New("missing currency")
	}
	if r.TotalCents <= 0 {
		return errors.New("missing or invalid total_cents")
	}
	return nil
}

type notifyResponse struct {
	Status string `json:"status"`
}
