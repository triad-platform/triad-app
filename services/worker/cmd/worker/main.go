package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/triad-platform/triad-app/pkg/config"
	"github.com/triad-platform/triad-app/pkg/logx"
)

func main() {
	log := logx.New()

	_ = config.Getenv("NATS_URL", "nats://localhost:4222")

	// TODO:
	// - connect to NATS
	// - subscribe to OrdersCreated
	// - process event (idempotent)
	// - call notifications service OR log action

	log.Info().Msg("worker started (TODO: connect to NATS and subscribe)")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = ctx
	log.Info().Msg("worker shutdown complete")
}
