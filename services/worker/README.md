# worker

Worker is the async processing service for order-created events.

## Responsibilities

1. Subscribe to `orders.created.v1`.
2. Process each message exactly-once from the business perspective.
3. Enforce consumer-side idempotency.
4. Trigger notification flow (`notifications` service) or equivalent action.
5. Record structured processing logs for traceability.

## API and Events

1. Consumes event
   - Subject: `orders.created.v1` (target)
   - Contract source: `contracts/events/orders.created.json`
2. Produces side effects
   - Calls notifications API: `POST /v1/notify` (target behavior)

Current status:
- Startup/shutdown scaffolding is implemented in `cmd/worker/main.go`.
- NATS subscription, idempotency, and notification call path are still TODO.

## Dependencies

1. NATS (`pulsecart-nats`, `4222`) for event consumption.
2. Redis (`pulsecart-redis`, `6379`) for consumer idempotency keys.
3. Notifications service (default local port `8082`) for notification dispatch.

## Run Locally

From `triad-app/`:

```bash
make up
make smoke
go run ./services/worker/cmd/worker
```

No HTTP port is exposed by default.

## Test Commands

From `triad-app/`:

```bash
go test ./services/worker/...
```

Phase 1 required tests (to add):
1. Unit tests for event decode and validation.
2. Integration tests for NATS consume + ack/retry handling.
3. Idempotency replay tests (duplicate message should not duplicate side effects).

## Implementation Checklist (Phase 1)

1. Connect to NATS and subscribe to `orders.created.v1`.
2. Add consumer idempotency key strategy in Redis.
3. Add retry/backoff and dead-letter decision for poison messages.
4. Call notifications service with timeout and request ID propagation.
5. Emit processing outcome metrics/logs.
