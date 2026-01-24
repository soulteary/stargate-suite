#!/bin/bash

# the-gate 端到端测试运行脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_DIR"

echo "=========================================="
echo "the-gate 端到端集成测试"
echo "=========================================="
echo ""

# 检查 Docker Compose 服务状态
echo "检查服务状态..."
if ! docker compose ps | grep -q "Up"; then
    echo "警告: 服务可能未启动，请先运行: docker compose up -d --build"
    echo ""
    read -p "是否现在启动服务? (y/n) " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "启动服务..."
        docker compose up -d --build
        echo "等待服务就绪（30秒）..."
        sleep 30
    else
        echo "请先启动服务: docker compose up -d --build"
        exit 1
    fi
fi

# 检查服务健康状态
echo "检查服务健康状态..."
services=("stargate:8080/_auth" "warden:8081/health" "herald:8082/healthz")
all_healthy=true

for service in "${services[@]}"; do
    name=$(echo $service | cut -d: -f1)
    port=$(echo $service | cut -d: -f2 | cut -d/ -f1)
    path=$(echo $service | cut -d/ -f2-)
    
    if curl -sf "http://localhost:$port/$path" > /dev/null 2>&1; then
        echo "✓ $name 健康"
    else
        echo "✗ $name 不健康"
        all_healthy=false
    fi
done

if [ "$all_healthy" = false ]; then
    echo ""
    echo "部分服务不健康，请检查日志: docker compose logs"
    exit 1
fi

echo ""
echo "运行端到端测试..."
echo ""

# 运行测试
go test -v ./e2e/...

test_exit_code=$?

echo ""
if [ $test_exit_code -eq 0 ]; then
    echo "=========================================="
    echo "✓ 所有测试通过"
    echo "=========================================="
else
    echo "=========================================="
    echo "✗ 测试失败 (退出码: $test_exit_code)"
    echo "=========================================="
fi

exit $test_exit_code
