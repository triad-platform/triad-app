# orders

Orders is the system-of-record service for order creation in PulseCart.

## Responsibilities

1. Expose `POST /v1/orders`.
2. Validate order payloads.
3. Enforce producer-side idempotency using `Idempotency-Key`.
4. Persist orders and order items in Postgres.
5. Publish `orders.created.v1` after successful write.

## API and Events

1. API
   - `POST /v1/orders`
   - Request/response shape is defined in `contracts/api/orders.http`.
2. Event
   - Subject: `orders.created.v1` (target)
   - Contract: `contracts/events/orders.created.json`

Current status:
- Health endpoints are live (`/healthz`, `/readyz`).
- `POST /v1/orders` handler scaffolding exists in `internal/orders/handler.go`.
- Persistence/idempotency/event publishing are still TODO.

## Dependencies

1. Postgres (`pulsecart-postgres`, port `5432`) for durable order data.
2. Redis (`pulsecart-redis`, port `6379`) for idempotency key checks.
3. NATS (`pulsecart-nats`, port `4222`) for async event publishing.

## Run Locally

From `triad-app/`:

```bash
make up
make smoke
go run ./services/orders/cmd/orders
```

Default service port: `8081`

## Test Commands

From `triad-app/`:

```bash
go test ./services/orders/...
```

Phase 1 required tests (to add):
1. Unit tests for payload validation and idempotency header handling.
2. Integration tests for Postgres insert + Redis key behavior.
3. Integration tests for NATS publish-on-success semantics.

## Implementation Checklist (Phase 1)

1. Add DB, Redis, and NATS dependencies to `Handler`.
2. Implement migration and insert logic for orders/items.
3. Add idempotency key read/check/set flow.
4. Emit `orders.created.v1` with versioned schema fields.
5. Return stable `201` response with generated `order_id`.
