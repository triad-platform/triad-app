# api-gateway

API Gateway is the edge service for PulseCart service routing.

## Responsibilities

1. Expose stable external endpoints for clients.
2. Route order traffic to the `orders` service.
3. Attach/propagate request IDs and correlation headers.
4. Enforce request timeouts and upstream error mapping.
5. Provide edge health/readiness endpoints.

## API and Events

1. Public endpoints (Phase 1 target)
   - `POST /v1/orders` (forward to orders service)
2. Health
   - `GET /healthz`
   - `GET /readyz`

Current status:
- Service runtime and health routes exist in `cmd/api-gateway/main.go`.
- `/v1/orders` forwarding, request ID middleware, and timeout middleware are TODO.

## Dependencies

1. Orders service endpoint (recommended env var: `ORDERS_URL`, default `http://localhost:8081`).
2. Shared request logging and middleware from `pkg/httpx` (as it grows).

## Run Locally

From `triad-app/`:

```bash
make up
make smoke
go run ./services/api-gateway/cmd/api-gateway
```

Default service port: `8080`

## Test Commands

From `triad-app/`:

```bash
go test ./services/api-gateway/...
```

Phase 1 required tests (to add):
1. Unit tests for request ID middleware behavior.
2. Integration tests for upstream forwarding to orders.
3. Timeout and error-translation tests.

## Implementation Checklist (Phase 1)

1. Add `POST /v1/orders` route in gateway.
2. Forward payload and headers to orders service.
3. Propagate/emit request ID and trace correlation fields.
4. Enforce upstream timeout and cancellation.
5. Return consistent client-facing errors.
