package main

import (
	"context"
	"crypto/tls"
	"net/http"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
	"github.com/triad-platform/triad-app/pkg/config"
	"github.com/triad-platform/triad-app/pkg/logx"
	"github.com/triad-platform/triad-app/pkg/metricsx"
	workerpkg "github.com/triad-platform/triad-app/services/worker/internal/worker"
)

const ordersCreatedSubject = "orders.created.v1"

func natsURL() string {
	return config.Getenv("NATS_URL", "nats://localhost:4222")
}

func notificationsURL() string {
	return config.Getenv("NOTIFICATIONS_URL", "http://localhost:8082")
}

func metricsPort() string {
	return config.Getenv("WORKER_METRICS_PORT", "9091")
}

func main() {
	log := logx.New()
	metrics := metricsx.NewRegistry("triad_worker")

	_ = natsURL()
	redisAddr := config.Getenv("REDIS_ADDR", "localhost:6379")

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

	processor := &workerpkg.Processor{
		IdempotencyStore: workerpkg.NewRedisIdempotencyStore(redisClient, ""),
		Notifier:         workerpkg.NewHTTPNotifier(notificationsURL(), 3*time.Second),
		Metrics:          metrics,
		KeyPrefix:        "worker:orders-created:",
		IdempotencyTTL:   24 * time.Hour,
	}

	nc, err := nats.Connect(natsURL())
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to NATS")
	}
	defer nc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- workerpkg.RunNATSSubscriber(ctx, nc, ordersCreatedSubject, processor, log)
	}()

	metricsSrv := &http.Server{
		Addr:              ":" + metricsPort(),
		Handler:           metrics.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		if err := metricsSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("worker metrics server failed")
		}
	}()

	log.Info().
		Str("subject", ordersCreatedSubject).
		Msg("worker started and subscribed to NATS")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-stop:
		cancel()
	case err := <-errCh:
		if err != nil {
			log.Fatal().Err(err).Msg("worker subscriber failed")
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	select {
	case err := <-errCh:
		if err != nil {
			log.Error().Err(err).Msg("worker subscriber shutdown error")
		}
	case <-shutdownCtx.Done():
		log.Error().Msg("worker shutdown timed out")
	}
	_ = metricsSrv.Shutdown(shutdownCtx)

	log.Info().Msg("worker shutdown complete")
}
