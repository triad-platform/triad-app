# notifications

Notifications receives notification requests and records notification actions.

## Responsibilities

1. Expose `POST /v1/notify` for async notification requests.
2. Validate payload and deduplicate if needed.
3. Execute notification action (initially log-only in Phase 1).
4. Return clear delivery status to caller.

## API and Events

1. API
   - `POST /v1/notify` (target endpoint for worker calls)
2. Health
   - `GET /healthz`
   - `GET /readyz`

Current status:
- Service runtime and health endpoints exist in `cmd/notifications/main.go`.
- `POST /v1/notify` handler is still TODO.

## Dependencies

1. Optional Redis dedupe store (recommended for idempotent notify requests).
2. Optional external provider adapters (email/SMS/push) in later phases.
3. Structured logging stack for traceable delivery events.

## Run Locally

From `triad-app/`:

```bash
make up
make smoke
go run ./services/notifications/cmd/notifications
```

Default service port: `8082`

## Test Commands

From `triad-app/`:

```bash
go test ./services/notifications/...
```

Phase 1 required tests (to add):
1. Unit tests for payload validation.
2. API tests for success/error response behavior.
3. Idempotency tests (if dedupe is added in Phase 1).

## Implementation Checklist (Phase 1)

1. Add `POST /v1/notify` route and request model.
2. Validate required fields and enforce timeout-safe handler behavior.
3. Implement log-based notification sink.
4. Add request ID propagation to logs.
5. Return deterministic response schema.
