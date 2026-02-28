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

## Usage

From `triad-app/`:

```bash
make e2e
```

On success, output ends with:

```text
E2E PASS
```

If it fails, inspect logs in `.tmp/e2e-logs/`.
