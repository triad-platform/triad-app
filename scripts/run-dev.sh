#!/usr/bin/env bash
set -euo pipefail

# Start deps
make up >/dev/null
make smoke

echo ""
echo "Starting services..."
echo "api-gateway   :8080"
echo "orders        :8081"
echo "notifications :8082"
echo "worker        (no port)"

# Run each service in background
go run ./services/api-gateway/cmd/api-gateway &
PID1=$!
go run ./services/orders/cmd/orders &
PID2=$!
go run ./services/notifications/cmd/notifications &
PID3=$!
go run ./services/worker/cmd/worker &
PID4=$!

trap 'echo "Stopping..."; kill $PID1 $PID2 $PID3 $PID4 2>/dev/null || true' EXIT
wait
