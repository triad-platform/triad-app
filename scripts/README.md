# scripts

Local developer workflows for PulseCart.

## Scripts

1. `run-dev.sh`
   - Starts local dependencies and all services.
2. `e2e-local.sh`
   - Runs local end-to-end verification:
     - Starts dependencies/services
     - Sends `POST /v1/orders` via gateway
     - Verifies duplicate request returns `409`
     - Verifies async worker -> notifications path executes exactly once
3. `e2e-cloud.sh`
   - Runs public dev-environment smoke verification:
     - Waits for the public `/healthz` endpoint
     - Sends `POST /v1/orders` to `pulsecart-dev.cloudevopsguru.com`
     - Verifies duplicate request returns `409`

## Usage

From `triad-app/`:

```bash
make e2e
make smoke-cloud
```

On success, output ends with:

```text
E2E PASS
```

Cloud smoke output ends with:

```text
CLOUD SMOKE PASS
```

If it fails, inspect logs in `.tmp/e2e-logs/`.
