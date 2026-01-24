.PHONY: help up down logs test clean

help: ## 显示帮助信息
	@echo "the-gate 端到端集成测试项目"
	@echo ""
	@echo "可用命令:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

up: ## 启动所有服务
	docker compose up -d --build

down: ## 停止所有服务
	docker compose down

logs: ## 查看服务日志
	docker compose logs -f

ps: ## 查看服务状态
	docker compose ps

test: ## 运行端到端测试
	go test -v ./e2e/...

test-wait: ## 等待服务就绪后运行测试（推荐）
	@echo "等待服务就绪（3秒）..."
	@sleep 3
	@go test -v ./e2e/...

clean: ## 清理服务和数据卷
	docker compose down -v

restart: ## 重启所有服务
	docker compose restart

restart-warden: ## 重启 Warden 服务
	docker compose restart warden

restart-herald: ## 重启 Herald 服务
	docker compose restart herald

restart-stargate: ## 重启 Stargate 服务
	docker compose restart stargate

health: ## 检查服务健康状态
	@echo "检查 Stargate..."
	@curl -sf http://localhost:8080/_auth > /dev/null && echo "✓ Stargate 健康" || echo "✗ Stargate 不健康"
	@echo "检查 Warden..."
	@curl -sf http://localhost:8081/health > /dev/null && echo "✓ Warden 健康" || echo "✗ Warden 不健康"
	@echo "检查 Herald..."
	@curl -sf http://localhost:8082/healthz > /dev/null && echo "✓ Herald 健康" || echo "✗ Herald 不健康"
