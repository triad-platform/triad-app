package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
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
	dbURL := config.Getenv("DATABASE_URL", "postgres://pulsecart:pulsecart@localhost:5432/pulsecart?sslmode=disable")
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

	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
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
