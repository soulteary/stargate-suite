#!/bin/bash

# stargate-suite End-to-End Test Runner Script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_DIR"

# Use build/image by default; override with COMPOSE_FILE if set
COMPOSE_FILE="${COMPOSE_FILE:-$PROJECT_DIR/build/image/docker-compose.yml}"
export COMPOSE_FILE

echo "=========================================="
echo "stargate-suite End-to-End Integration Tests"
echo "=========================================="
echo "Using compose: $COMPOSE_FILE"
echo ""

# Herald v1.1.0 serves the test-code endpoint only on a dedicated loopback-only
# listener inside the container (127.0.0.1:8092), never on the main :8082
# listener. getTestCode therefore fetches it via `docker compose exec herald curl`.
# Export the compose dir + test listener wiring so the E2E helper takes that path.
# COMPOSE_FILE is already exported, so the exec'd `docker compose` honors it.
export HERALD_COMPOSE_DIR="${HERALD_COMPOSE_DIR:-$PROJECT_DIR}"
export HERALD_TEST_LISTENER_ADDR="${HERALD_TEST_LISTENER_ADDR:-127.0.0.1:8092}"
export HERALD_TEST_API_KEY="${HERALD_TEST_API_KEY:-test-herald-test-code-key}"

# Ensure compose config is applied (recreates services if env/volumes changed, e.g. Warden DATA_FILE)
docker compose -f "$COMPOSE_FILE" up -d 2>/dev/null || true

# CI 环境下无 TTY，自动启动服务并等待；本地可交互时提示
AUTO_START=
if [ -n "${CI:-}" ] || [ -n "${GITHUB_ACTIONS:-}" ]; then
  AUTO_START=1
fi

# Check Docker Compose service status
echo "Checking service status..."
if ! docker compose -f "$COMPOSE_FILE" ps 2>/dev/null | grep -q "Up"; then
    if [ -n "$AUTO_START" ]; then
        echo "CI mode: starting services and waiting for readiness..."
    else
        echo "Warning: Services may not be started. Please run first: make up  or  docker compose -f $COMPOSE_FILE up -d"
        echo ""
        read -p "Start services now? (y/n) " -n 1 -r
        echo ""
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            echo "Please start services first: make up  or  docker compose -f $COMPOSE_FILE up -d"
            exit 1
        fi
    fi
    echo "Starting services..."
    docker compose -f "$COMPOSE_FILE" up -d --build
    TEST_WAIT_TIMEOUT="${TEST_WAIT_TIMEOUT:-60}"
    echo "Waiting for services to be ready (timeout ${TEST_WAIT_TIMEOUT}s)..."
    start=$(date +%s)
    while true; do
        all_ok=true
        for service in "stargate:8080/health" "warden:8081/health" "herald:8082/healthz"; do
            port=$(echo "$service" | cut -d: -f2 | cut -d/ -f1)
            path=$(echo "$service" | cut -d/ -f2-)
            if ! curl -sf "http://localhost:$port/$path" > /dev/null 2>&1; then
                all_ok=false
                break
            fi
        done
        if [ "$all_ok" = true ]; then
            echo "All services ready."
            break
        fi
        now=$(date +%s)
        if [ $(( now - start )) -ge "$TEST_WAIT_TIMEOUT" ]; then
            echo "Services did not become ready within ${TEST_WAIT_TIMEOUT}s. Run: docker compose -f $COMPOSE_FILE logs"
            exit 1
        fi
        sleep 1
    done
    # CI 下直接跑测并退出；本地交互已启动后继续走下方统一健康检查与 go test
    if [ -n "$AUTO_START" ]; then
        echo "Running End-to-End Tests..."
        echo ""
        go test -v ./e2e/...
        exit $?
    fi
fi

# 无论是否刚启动，在健康检查前都等待所有服务就绪（避免部分容器已 Up 但 Stargate 尚未监听）
TEST_WAIT_TIMEOUT="${TEST_WAIT_TIMEOUT:-60}"
echo "Waiting for services to be ready (timeout ${TEST_WAIT_TIMEOUT}s)..."
start=$(date +%s)
while true; do
    all_ok=true
    for service in "stargate:8080/health" "warden:8081/health" "herald:8082/healthz"; do
        port=$(echo "$service" | cut -d: -f2 | cut -d/ -f1)
        path=$(echo "$service" | cut -d/ -f2-)
        if ! curl -sf "http://localhost:$port/$path" > /dev/null 2>&1; then
            all_ok=false
            break
        fi
    done
    if [ "$all_ok" = true ]; then
        echo "All services ready."
        break
    fi
    now=$(date +%s)
    if [ $(( now - start )) -ge "$TEST_WAIT_TIMEOUT" ]; then
        echo "Services did not become ready within ${TEST_WAIT_TIMEOUT}s. Run: docker compose -f $COMPOSE_FILE logs"
        docker compose -f "$COMPOSE_FILE" logs 2>/dev/null || true
        exit 1
    fi
    sleep 1
done

# Services ready: check health then run tests
echo "Checking service health status..."
services=("stargate:8080/health" "warden:8081/health" "herald:8082/healthz")
all_healthy=true

for service in "${services[@]}"; do
    name=$(echo $service | cut -d: -f1)
    port=$(echo $service | cut -d: -f2 | cut -d/ -f1)
    path=$(echo $service | cut -d/ -f2-)
    
    if curl -sf "http://localhost:$port/$path" > /dev/null 2>&1; then
        echo "✓ $name Healthy"
    else
        echo "✗ $name Unhealthy"
        all_healthy=false
    fi
done

if [ "$all_healthy" = false ]; then
    echo ""
    echo "Some services are unhealthy. Please check logs: docker compose -f $COMPOSE_FILE logs"
    exit 1
fi

echo ""
echo "Running End-to-End Tests..."
echo ""

go test -v ./e2e/...

test_exit_code=$?

echo ""
if [ $test_exit_code -eq 0 ]; then
    echo "=========================================="
    echo "✓ All tests passed"
    echo "=========================================="
else
    echo "=========================================="
    echo "✗ Tests failed (Exit Code: $test_exit_code)"
    echo "=========================================="
fi

exit $test_exit_code
