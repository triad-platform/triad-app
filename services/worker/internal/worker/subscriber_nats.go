package worker

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog"
)

func RunNATSSubscriber(
	ctx context.Context,
	nc *nats.Conn,
	subject string,
	processor *Processor,
	log zerolog.Logger,
) error {
	sub, err := nc.Subscribe(subject, func(msg *nats.Msg) {
		processed, procErr := processor.HandleOrdersCreatedMessage(ctx, msg.Data)
		if procErr != nil {
			log.Error().Err(procErr).Str("subject", msg.Subject).Msg("failed to process NATS message")
			return
		}
		if !processed {
			log.Info().Str("subject", msg.Subject).Msg("duplicate event ignored")
			return
		}
		log.Info().Str("subject", msg.Subject).Msg("message processed")
	})
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	if err := nc.Flush(); err != nil {
		return err
	}
	if err := nc.LastError(); err != nil {
		return err
	}

	<-ctx.Done()
	return sub.Drain()
}
