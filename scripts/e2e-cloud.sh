#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BASE_URL="${BASE_URL:-https://pulsecart-dev.cloudevopsguru.com}"
MAX_ATTEMPTS="${MAX_ATTEMPTS:-18}"
SLEEP_SECONDS="${SLEEP_SECONDS:-10}"

wait_for_http() {
  local url="$1"
  local i
  for ((i = 1; i <= MAX_ATTEMPTS; i++)); do
    if curl -fsS "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$SLEEP_SECONDS"
  done
  return 1
}

echo "Checking public health endpoint..."
if ! wait_for_http "${BASE_URL}/healthz"; then
  echo "timed out waiting for ${BASE_URL}/healthz"
  exit 1
fi

run_id="$(date +%s)-$$"
idem_key="cloud-smoke-${run_id}"
request_id="cloud-smoke-${run_id}"

request_body='{"user_id":"cloud-smoke-user","currency":"USD","items":[{"sku":"sku-cloud-smoke","quantity":1,"unit_price":1999}]}'

echo "Sending create order request..."
resp_one="$(mktemp)"
status_one="$(curl -sS -o "$resp_one" -w '%{http_code}' \
  -X POST "${BASE_URL}/v1/orders" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: ${idem_key}" \
  -H "X-Request-Id: ${request_id}" \
  -d "$request_body")"

if [[ "$status_one" != "201" ]]; then
  echo "expected first request status 201, got ${status_one}"
  cat "$resp_one" || true
  rm -f "$resp_one"
  exit 1
fi

order_id="$(sed -n 's/.*"order_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$resp_one" | head -n1)"
if [[ -z "$order_id" ]]; then
  echo "failed to parse order_id from first response"
  cat "$resp_one" || true
  rm -f "$resp_one"
  exit 1
fi
rm -f "$resp_one"

echo "Sending duplicate request..."
resp_two="$(mktemp)"
status_two="$(curl -sS -o "$resp_two" -w '%{http_code}' \
  -X POST "${BASE_URL}/v1/orders" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: ${idem_key}" \
  -H "X-Request-Id: ${request_id}-dup" \
  -d "$request_body")"

if [[ "$status_two" != "409" ]]; then
  echo "expected duplicate request status 409, got ${status_two}"
  cat "$resp_two" || true
  rm -f "$resp_two"
  exit 1
fi
rm -f "$resp_two"

echo ""
echo "CLOUD SMOKE PASS"
echo "  Base URL: ${BASE_URL}"
echo "  First request: 201"
echo "  Duplicate request: 409"
echo "  Order ID: ${order_id}"
