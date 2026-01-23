#!/bin/bash
# 通过 Web API 生成 compose 到 build/（供 Makefile/CI 使用，无 CLI gen 子命令）
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

PORT="${SERVE_PORT:-8085}"
BUILD_DIR="${BUILD_DIR:-build}"
# 默认生成所有模式；传入参数时可覆盖，如 "image" 仅生成 build/image
MODES="${1:-image build traefik traefik-herald traefik-warden traefik-stargate}"

if ! command -v jq >/dev/null 2>&1; then
  echo "gen-via-api: jq is required. Install with: apt-get install jq / brew install jq"
  exit 1
fi

# 构建 JSON modes 数组
MODES_JSON=$(echo "$MODES" | tr ' ' '\n' | jq -R . | jq -s .)

BODY=$(jq -n \
  --argjson modes "$MODES_JSON" \
  '{ modes: $modes, envOverride: "", options: null }')

echo "Starting suite serve on port $PORT..."
go run ./cmd/suite serve -port "$PORT" &
SRV_PID=$!
trap 'kill $SRV_PID 2>/dev/null || true' EXIT

for i in $(seq 1 30); do
  if curl -sf "http://127.0.0.1:$PORT/" -o /dev/null 2>/dev/null; then
    break
  fi
  if [ "$i" -eq 30 ]; then
    echo "gen-via-api: server did not become ready in time"
    exit 1
  fi
  sleep 0.5
done

echo "POST /api/generate (modes: $MODES)"
RESP=$(curl -sf -X POST "http://127.0.0.1:$PORT/api/generate" \
  -H "Content-Type: application/json" \
  -d "$BODY")

ENV_BODY=$(echo "$RESP" | jq -r '.env')
mkdir -p "$BUILD_DIR"
for mode in $MODES; do
  dir="$BUILD_DIR/$mode"
  mkdir -p "$dir"
  echo "$RESP" | jq -r --arg m "$mode" '.composes[$m]' > "$dir/docker-compose.yml"
  echo "$ENV_BODY" > "$dir/.env"
  echo "  $dir/docker-compose.yml, $dir/.env"
done
echo "Generated into $BUILD_DIR/ for mode(s): $MODES"
