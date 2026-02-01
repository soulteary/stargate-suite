#!/bin/bash

# the-gate End-to-End Test Runner Script

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_DIR"

# Use build/image by default; override with COMPOSE_FILE if set
COMPOSE_FILE="${COMPOSE_FILE:-$PROJECT_DIR/build/image/docker-compose.yml}"
export COMPOSE_FILE

echo "=========================================="
echo "the-gate End-to-End Integration Tests"
echo "=========================================="
echo "Using compose: $COMPOSE_FILE"
echo ""

# Ensure compose config is applied (recreates services if env/volumes changed, e.g. Warden DATA_FILE)
docker compose -f "$COMPOSE_FILE" up -d 2>/dev/null || true

# Check Docker Compose service status
echo "Checking service status..."
if ! docker compose -f "$COMPOSE_FILE" ps 2>/dev/null | grep -q "Up"; then
    echo "Warning: Services may not be started. Please run first: make up  or  docker compose -f $COMPOSE_FILE up -d"
    echo ""
    read -p "Start services now? (y/n) " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Starting services..."
        docker compose -f "$COMPOSE_FILE" up -d --build
        echo "Waiting for services to be ready (uses TEST_WAIT_TIMEOUT, default 60s), then running tests..."
        go run ./cmd/suite test-wait
        exit $?
    else
        echo "Please start services first: make up  or  docker compose -f $COMPOSE_FILE up -d"
        exit 1
    fi
fi

# Services already up: check health then run tests
echo "Checking service health status..."
services=("stargate:8080/_auth" "warden:8081/health" "herald:8082/healthz")
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
