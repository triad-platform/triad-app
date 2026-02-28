#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

LOG_DIR="${ROOT_DIR}/.tmp/e2e-logs"
mkdir -p "$LOG_DIR"

API_LOG="${LOG_DIR}/api-gateway.log"
ORDERS_LOG="${LOG_DIR}/orders.log"
NOTIFY_LOG="${LOG_DIR}/notifications.log"
WORKER_LOG="${LOG_DIR}/worker.log"

rm -f "$API_LOG" "$ORDERS_LOG" "$NOTIFY_LOG" "$WORKER_LOG"

cleanup() {
  local code=$?
  if [[ -n "${PID_API:-}" ]]; then kill "$PID_API" 2>/dev/null || true; fi
  if [[ -n "${PID_ORDERS:-}" ]]; then kill "$PID_ORDERS" 2>/dev/null || true; fi
  if [[ -n "${PID_NOTIFY:-}" ]]; then kill "$PID_NOTIFY" 2>/dev/null || true; fi
  if [[ -n "${PID_WORKER:-}" ]]; then kill "$PID_WORKER" 2>/dev/null || true; fi
  wait 2>/dev/null || true
  if [[ $code -ne 0 ]]; then
    echo "E2E failed. Logs:"
    echo "  $API_LOG"
    echo "  $ORDERS_LOG"
    echo "  $NOTIFY_LOG"
    echo "  $WORKER_LOG"
  fi
  exit "$code"
}
trap cleanup EXIT

wait_for_http() {
  local name="$1"
  local url="$2"
  local i
  for i in {1..40}; do
    if curl -sf "$url" >/dev/null; then
      echo "ready: $name"
      return 0
    fi
    sleep 0.25
  done
  echo "timeout waiting for $name at $url"
  return 1
}

assert_port_free() {
  local port="$1"
  if lsof -nP -iTCP:"${port}" -sTCP:LISTEN -t >/dev/null 2>&1; then
    echo "port ${port} is already in use; stop existing local services and retry"
    return 1
  fi
  return 0
}

list_listener_pids() {
  local ports=("$@")
  local pids=""
  local port
  local port_pids
  for port in "${ports[@]}"; do
    port_pids="$(lsof -nP -iTCP:"${port}" -sTCP:LISTEN -t 2>/dev/null || true)"
    if [[ -n "${port_pids// }" ]]; then
      pids="${pids}"$'\n'"${port_pids}"
    fi
  done
  printf "%s\n" "$pids" | sed '/^$/d' | sort -u
}

kill_pids_gracefully() {
  local pids="$1"
  if [[ -z "$pids" ]]; then
    return 0
  fi

  echo "$pids" | xargs kill 2>/dev/null || true
  sleep 0.5

  local remaining=""
  local pid
  for pid in $pids; do
    if kill -0 "$pid" 2>/dev/null; then
      remaining="${remaining} ${pid}"
    fi
  done
  if [[ -n "${remaining// }" ]]; then
    echo "$remaining" | xargs kill -9 2>/dev/null || true
  fi
}

kill_pids_force() {
  local pids="$1"
  if [[ -z "$pids" ]]; then
    return 0
  fi
  echo "$pids" | xargs kill -9 2>/dev/null || true
}

stop_stale_local_services() {
  local pids=""

  # Kill any stale listeners on service ports from previous runs.
  pids="$(list_listener_pids 8080 8081 8082 9091)"
  if [[ -n "${pids// }" ]]; then
    echo "Stopping stale services on ports 8080/8081/8082/9091..."
    kill_pids_force "$pids"
  fi

  # Best-effort cleanup for old worker go-run commands (worker has no HTTP port).
  pids="$(pgrep -f 'go run ./services/worker/cmd/worker' 2>/dev/null || true)"
  if [[ -n "${pids// }" ]]; then
    echo "Stopping stale worker processes..."
    kill_pids_gracefully "$pids"
  fi
}

assert_process_alive() {
  local pid="$1"
  local name="$2"
  local logfile="$3"
  if ! kill -0 "$pid" 2>/dev/null; then
    echo "${name} failed to start; tail of ${logfile}:"
    if [[ -f "$logfile" ]]; then
      tail -n 120 "$logfile" || true
    fi
    return 1
  fi
  return 0
}

wait_for_log() {
  local pattern="$1"
  local file="$2"
  local i
  for i in {1..80}; do
    if [[ -f "$file" ]] && grep -q "$pattern" "$file"; then
      return 0
    fi
    sleep 0.25
  done
  return 1
}

echo "Starting local dependencies..."
make up >/dev/null
make smoke

stop_stale_local_services

assert_port_free 8080
assert_port_free 8081
assert_port_free 8082
assert_port_free 9091

echo "Starting services..."
GOCACHE="${ROOT_DIR}/.gocache" go run ./services/notifications/cmd/notifications >"$NOTIFY_LOG" 2>&1 &
PID_NOTIFY=$!
GOCACHE="${ROOT_DIR}/.gocache" go run ./services/orders/cmd/orders >"$ORDERS_LOG" 2>&1 &
PID_ORDERS=$!
GOCACHE="${ROOT_DIR}/.gocache" go run ./services/api-gateway/cmd/api-gateway >"$API_LOG" 2>&1 &
PID_API=$!
GOCACHE="${ROOT_DIR}/.gocache" go run ./services/worker/cmd/worker >"$WORKER_LOG" 2>&1 &
PID_WORKER=$!

sleep 0.5
assert_process_alive "$PID_NOTIFY" "notifications" "$NOTIFY_LOG"
assert_process_alive "$PID_ORDERS" "orders" "$ORDERS_LOG"
assert_process_alive "$PID_API" "api-gateway" "$API_LOG"
assert_process_alive "$PID_WORKER" "worker" "$WORKER_LOG"

wait_for_http "notifications" "http://localhost:8082/healthz"
wait_for_http "orders" "http://localhost:8081/healthz"
wait_for_http "api-gateway" "http://localhost:8080/healthz"
wait_for_http "orders metrics" "http://localhost:8081/metrics"
wait_for_http "api-gateway metrics" "http://localhost:8080/metrics"
wait_for_http "worker metrics" "http://localhost:9091/metrics"

request_body='{"user_id":"u_e2e","items":[{"sku":"sku_e2e","qty":2,"price_cents":1250}],"currency":"USD"}'
run_id="$(date +%s)-$$"
idem_key="idem-e2e-${run_id}"
request_id="req-e2e-${run_id}"

echo "Sending create order request via gateway..."
status_one="$(curl -s -o /tmp/triad_e2e_resp1.json -w '%{http_code}' \
  -X POST http://localhost:8080/v1/orders \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: ${idem_key}" \
  -H "X-Request-Id: ${request_id}" \
  -d "$request_body")"

if [[ "$status_one" != "201" ]]; then
  echo "expected first request status 201, got ${status_one}"
  cat /tmp/triad_e2e_resp1.json || true
  exit 1
fi

order_id="$(sed -n 's/.*"order_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' /tmp/triad_e2e_resp1.json | head -n1)"
if [[ -z "$order_id" ]]; then
  echo "failed to parse order_id from first response"
  cat /tmp/triad_e2e_resp1.json || true
  exit 1
fi

echo "Sending duplicate request to validate idempotency..."
status_two="$(curl -s -o /tmp/triad_e2e_resp2.txt -w '%{http_code}' \
  -X POST http://localhost:8080/v1/orders \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: ${idem_key}" \
  -H "X-Request-Id: ${request_id}" \
  -d "$request_body")"

if [[ "$status_two" != "409" ]]; then
  echo "expected duplicate request status 409, got ${status_two}"
  cat /tmp/triad_e2e_resp2.txt || true
  exit 1
fi

echo "Validating Postgres persistence..."
stored_order_count="$(docker exec pulsecart-postgres psql -U pulsecart -d pulsecart -tAc "SELECT count(*) FROM orders WHERE id='${order_id}';" | tr -d '[:space:]')"
if [[ "$stored_order_count" != "1" ]]; then
  echo "expected one persisted order row for ${order_id}, got ${stored_order_count}"
  exit 1
fi

stored_item_count="$(docker exec pulsecart-postgres psql -U pulsecart -d pulsecart -tAc "SELECT count(*) FROM order_items WHERE order_id='${order_id}';" | tr -d '[:space:]')"
if [[ "$stored_item_count" != "1" ]]; then
  echo "expected one persisted order_items row for ${order_id}, got ${stored_item_count}"
  exit 1
fi

echo "Waiting for async worker -> notifications chain..."
if ! wait_for_log "notification accepted" "$NOTIFY_LOG"; then
  echo "did not observe notification acceptance log"
  exit 1
fi

notify_count="$(grep -c "notification accepted" "$NOTIFY_LOG" || true)"
if [[ "$notify_count" != "1" ]]; then
  echo "expected exactly one notification acceptance, got ${notify_count}"
  exit 1
fi

if wait_for_log "message processed" "$WORKER_LOG"; then
  worker_signal="message processed"
elif wait_for_log "worker started and subscribed to NATS" "$WORKER_LOG"; then
  worker_signal="subscribed log observed"
else
  worker_signal="worker log signal not observed (non-blocking)"
fi

echo ""
echo "E2E PASS"
echo "  First request: 201"
echo "  Duplicate request: 409"
echo "  Persisted order row: 1"
echo "  Persisted order_items row: 1"
echo "  Worker processed event and notifications accepted exactly once"
echo "  Worker log signal: ${worker_signal}"
