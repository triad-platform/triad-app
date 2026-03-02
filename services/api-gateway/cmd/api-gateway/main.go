package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/triad-platform/triad-app/pkg/config"
	"github.com/triad-platform/triad-app/pkg/httpx"
	"github.com/triad-platform/triad-app/pkg/logx"
	"github.com/triad-platform/triad-app/pkg/metricsx"
)

const (
	requestIDHeader = "X-Request-Id"
	ordersPath      = "/v1/orders"
)

type gatewayConfig struct {
	OrdersURL               string
	WorkerMetricsURL        string
	NotificationsMetricsURL string
	EnableDevDiagnostics    bool
	RequestTimeout          time.Duration
	UpstreamTimeout         time.Duration
	Client                  *http.Client
}

func ordersURL() string {
	return config.Getenv("ORDERS_URL", "http://localhost:8081")
}

func workerMetricsURL() string {
	return strings.TrimSpace(os.Getenv("WORKER_METRICS_URL"))
}

func notificationsMetricsURL() string {
	return strings.TrimSpace(os.Getenv("NOTIFICATIONS_METRICS_URL"))
}

func enableDevDiagnostics() bool {
	return strings.EqualFold(config.Getenv("ENABLE_DEV_DIAGNOSTICS", "false"), "true")
}

func newRouter() http.Handler {
	metrics := metricsx.NewRegistry("triad_api_gateway")
	return newRouterWithConfig(gatewayConfig{
		OrdersURL:               ordersURL(),
		WorkerMetricsURL:        workerMetricsURL(),
		NotificationsMetricsURL: notificationsMetricsURL(),
		EnableDevDiagnostics:    enableDevDiagnostics(),
		RequestTimeout:          5 * time.Second,
		UpstreamTimeout:         3 * time.Second,
	}, metrics)
}

func newRouterWithConfig(cfg gatewayConfig, metrics *metricsx.Registry) http.Handler {
	if metrics == nil {
		metrics = metricsx.NewRegistry("triad_api_gateway")
	}
	r := chi.NewRouter()
	r.Use(requestIDMiddleware)
	r.Use(metricsMiddleware(metrics))
	if cfg.RequestTimeout <= 0 {
		cfg.RequestTimeout = 5 * time.Second
	}
	r.Use(middleware.Timeout(cfg.RequestTimeout))
	r.Get("/healthz", httpx.Healthz)
	r.Get("/readyz", httpx.Readyz)
	r.Get("/metrics", metrics.Handler().ServeHTTP)
	r.Post(ordersPath, forwardOrdersHandler(cfg, metrics))
	if cfg.EnableDevDiagnostics {
		r.Get("/v1/dev/async-status", asyncStatusHandler(cfg, metrics))
	}
	return r
}

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (w *statusRecorder) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func metricsMiddleware(metrics *metricsx.Registry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			next.ServeHTTP(rec, r)

			metrics.Inc("http_requests_total")
			metrics.Inc("http_response_status_" + strconv.Itoa(rec.statusCode) + "_total")
			metrics.ObserveDuration("http_request_duration", time.Since(start))
		})
	}
}

func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get(requestIDHeader))
		if requestID == "" {
			requestID = newRequestID()
			r.Header.Set(requestIDHeader, requestID)
		}
		w.Header().Set(requestIDHeader, requestID)
		next.ServeHTTP(w, r)
	})
}

func newRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "req-fallback"
	}
	return hex.EncodeToString(b)
}

func forwardOrdersHandler(cfg gatewayConfig, metrics *metricsx.Registry) http.HandlerFunc {
	ordersURL := strings.TrimRight(cfg.OrdersURL, "/")
	upstreamTimeout := cfg.UpstreamTimeout
	if upstreamTimeout <= 0 {
		upstreamTimeout = 3 * time.Second
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: upstreamTimeout}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		metrics.Inc("orders_forward_requests_total")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			metrics.Inc("orders_forward_errors_total")
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}

		upstreamReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, ordersURL+ordersPath, bytes.NewReader(body))
		if err != nil {
			metrics.Inc("orders_forward_errors_total")
			http.Error(w, "failed to create upstream request", http.StatusBadGateway)
			return
		}

		copyHeaderIfPresent(r, upstreamReq, "Content-Type")
		copyHeaderIfPresent(r, upstreamReq, "Idempotency-Key")
		copyHeaderIfPresent(r, upstreamReq, requestIDHeader)

		resp, err := client.Do(upstreamReq)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) || errors.Is(r.Context().Err(), context.DeadlineExceeded) {
				metrics.Inc("orders_forward_timeouts_total")
				http.Error(w, "upstream timeout", http.StatusGatewayTimeout)
				return
			}
			metrics.Inc("orders_forward_errors_total")
			http.Error(w, "upstream request failed", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		if contentType := resp.Header.Get("Content-Type"); contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}

type asyncStatusResponse struct {
	WorkerMessagesProcessed       int64 `json:"worker_messages_processed"`
	WorkerMessagesDuplicates      int64 `json:"worker_messages_duplicates"`
	WorkerMessagesErrors          int64 `json:"worker_messages_errors"`
	NotificationsAccepted         int64 `json:"notifications_accepted"`
	NotificationsValidationErrors int64 `json:"notifications_validation_errors"`
}

func asyncStatusHandler(cfg gatewayConfig, metrics *metricsx.Registry) http.HandlerFunc {
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(cfg.WorkerMetricsURL) == "" || strings.TrimSpace(cfg.NotificationsMetricsURL) == "" {
			metrics.Inc("dev_async_status_errors_total")
			http.Error(w, "async diagnostics not configured", http.StatusServiceUnavailable)
			return
		}

		workerMetrics, err := fetchMetrics(r.Context(), client, cfg.WorkerMetricsURL)
		if err != nil {
			metrics.Inc("dev_async_status_errors_total")
			http.Error(w, "failed to read worker metrics", http.StatusBadGateway)
			return
		}
		notificationsMetrics, err := fetchMetrics(r.Context(), client, cfg.NotificationsMetricsURL)
		if err != nil {
			metrics.Inc("dev_async_status_errors_total")
			http.Error(w, "failed to read notifications metrics", http.StatusBadGateway)
			return
		}

		resp := asyncStatusResponse{
			WorkerMessagesProcessed:       workerMetrics["triad_worker_messages_processed_total"],
			WorkerMessagesDuplicates:      workerMetrics["triad_worker_messages_duplicates_total"],
			WorkerMessagesErrors:          workerMetrics["triad_worker_messages_errors_total"],
			NotificationsAccepted:         notificationsMetrics["triad_notifications_accepted_total"],
			NotificationsValidationErrors: notificationsMetrics["triad_notifications_validation_errors_total"],
		}

		metrics.Inc("dev_async_status_requests_total")
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			metrics.Inc("dev_async_status_errors_total")
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	}
}

func fetchMetrics(ctx context.Context, client *http.Client, url string) (map[string]int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("unexpected metrics status")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseMetricsCounters(string(body)), nil
}

func parseMetricsCounters(body string) map[string]int64 {
	out := map[string]int64{}
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		v, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		out[fields[0]] = v
	}
	return out
}

func copyHeaderIfPresent(from *http.Request, to *http.Request, key string) {
	v := strings.TrimSpace(from.Header.Get(key))
	if v != "" {
		to.Header.Set(key, v)
	}
}

func main() {
	log := logx.New()

	port := config.Getenv("PORT", "8080")

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           newRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Msgf("api-gateway listening on :%s", port)
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
	log.Info().Msg("api-gateway shutdown complete")
}
