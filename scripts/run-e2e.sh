#!/usr/bin/env bash

# stargate-suite deterministic end-to-end test runner.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

COMPOSE_FILE="${COMPOSE_FILE:-$PROJECT_DIR/build/image/docker-compose.yml}"
export COMPOSE_FILE

# Herald v1.1.0 serves test codes only on its dedicated container-loopback
# listener. The Go helper reaches it through `docker compose exec herald`.
export HERALD_COMPOSE_DIR="${HERALD_COMPOSE_DIR:-$PROJECT_DIR}"
export HERALD_TEST_LISTENER_ADDR="${HERALD_TEST_LISTENER_ADDR:-127.0.0.1:8092}"
export HERALD_TEST_API_KEY="${HERALD_TEST_API_KEY:-test-herald-test-code-key}"

compose=(docker compose -f "$COMPOSE_FILE")
readiness_endpoints=(
  "stargate:8080/readyz"
  "warden:8081/healthcheck"
  "herald:8082/readyz"
)

echo "=========================================="
echo "stargate-suite End-to-End Integration Tests"
echo "=========================================="
echo "Using compose: $COMPOSE_FILE"

# Fail immediately on an invalid configuration or a failed container start.
# The previous runner suppressed this error, allowing tests to hit stale or
# partially started services.
"${compose[@]}" config >/dev/null
"${compose[@]}" up -d

TEST_WAIT_TIMEOUT="${TEST_WAIT_TIMEOUT:-180}"
echo "Waiting for readiness endpoints (timeout ${TEST_WAIT_TIMEOUT}s)..."
start=$(date +%s)
while true; do
  all_ready=true
  for endpoint in "${readiness_endpoints[@]}"; do
    name="${endpoint%%:*}"
    target="${endpoint#*:}"
    port="${target%%/*}"
    path="${target#*/}"
    if ! curl --fail --silent --show-error --connect-timeout 3 --max-time 5 \
      "http://127.0.0.1:${port}/${path}" >/dev/null; then
      echo "  waiting for ${name} at /${path}..."
      all_ready=false
      break
    fi
  done

  if [ "$all_ready" = true ]; then
    echo "All services ready."
    break
  fi
  now=$(date +%s)
  if [ $((now - start)) -ge "$TEST_WAIT_TIMEOUT" ]; then
    echo "Services did not become ready within ${TEST_WAIT_TIMEOUT}s."
    "${compose[@]}" ps -a || :
    "${compose[@]}" logs --tail=200 || :
    exit 1
  fi
  sleep 2
done

echo "Running End-to-End Tests..."
go test -v -timeout 15m ./e2e/...
