package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/triad-platform/triad-app/pkg/config"
	"github.com/triad-platform/triad-app/pkg/httpx"
	"github.com/triad-platform/triad-app/pkg/logx"
	"github.com/triad-platform/triad-app/pkg/metricsx"
	"github.com/triad-platform/triad-app/services/orders/internal/orders"
)

func main() {
	log := logx.New()
	metrics := metricsx.NewRegistry("triad_orders")

	port := config.Getenv("PORT", "8081")
	dbURL := databaseURL()
	redisAddr := config.Getenv("REDIS_ADDR", "localhost:6379")
	natsURL := config.Getenv("NATS_URL", "nats://localhost:4222")

	dbConn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Postgres")
	}
	defer dbConn.Close(context.Background())

	orderStore := orders.NewPostgresOrderStore(dbConn)
	schemaCtx, schemaCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := orderStore.EnsureSchema(schemaCtx); err != nil {
		schemaCancel()
		log.Fatal().Err(err).Msg("failed to ensure orders schema")
	}
	schemaCancel()

	redisOptions := &redis.Options{
		Addr: redisAddr,
	}
	if strings.EqualFold(config.Getenv("REDIS_TLS_ENABLED", "false"), "true") {
		host, _, splitErr := net.SplitHostPort(redisAddr)
		if splitErr != nil {
			host = redisAddr
		}
		redisOptions.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: host,
		}
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to NATS")
	}
	defer nc.Close()

	r := chi.NewRouter()
	r.Get("/healthz", httpx.Healthz)
	r.Get("/readyz", httpx.Readyz)
	r.Get("/metrics", metrics.Handler().ServeHTTP)

	h := &orders.Handler{
		IdempotencyStore: orders.NewRedisIdempotencyStore(redisClient, "orders:idempotency:"),
		EventPublisher:   orders.NewNATSEventPublisher(nc, orders.OrdersCreatedSubject),
		OrderStore:       orderStore,
		Metrics:          metrics,
		IdempotencyTTL:   24 * time.Hour,
	}
	r.Mount("/", orders.Routes(h))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Msgf("orders listening on :%s", port)
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
	log.Info().Msg("orders shutdown complete")
}

func databaseURL() string {
	if explicit := strings.TrimSpace(config.Getenv("DATABASE_URL", "")); explicit != "" {
		return explicit
	}

	host := config.Getenv("DB_HOST", "localhost")
	port := config.Getenv("DB_PORT", "5432")
	name := config.Getenv("DB_NAME", "pulsecart")
	user := config.Getenv("DB_USER", "pulsecart")
	password := config.Getenv("DB_PASSWORD", "pulsecart")
	sslmode := config.Getenv("DB_SSLMODE", "disable")

	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, password),
		Host:     net.JoinHostPort(host, port),
		Path:     name,
		RawQuery: "sslmode=" + url.QueryEscape(sslmode),
	}).String()
}
