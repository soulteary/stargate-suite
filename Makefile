# 默认使用 build/image（需先执行 make gen 或 ./bin/suite gen all）
COMPOSE_FILE ?= build/image/docker-compose.yml
BUILD_DIR ?= build

.PHONY: help gen up up-build up-image up-traefik down down-build down-image down-traefik logs test clean suite suite-build serve

help: ## Show help information
	@echo "the-gate End-to-End Integration Test Project"
	@echo ""
	@echo "Compose 生成到 $(BUILD_DIR)/，默认使用: $(COMPOSE_FILE)"
	@echo "首次使用请执行: make gen 或 ./bin/suite gen all"
	@echo ""
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-18s %s\n", $$1, $$2}'

gen: ## Generate docker-compose and .env into build/ (run before up)
	@go run ./cmd/suite gen all

up: ## Start all services（默认 build/image）
	docker compose -f $(COMPOSE_FILE) up -d

up-build: ## Start all services（从源码构建，build/build）
	docker compose -f $(BUILD_DIR)/build/docker-compose.yml up -d --build

up-image: ## Start all services（预构建镜像，build/image）
	docker compose -f $(BUILD_DIR)/image/docker-compose.yml up -d

up-traefik: ## Start all services（接入 Traefik，三合一，build/traefik）
	docker compose -f $(BUILD_DIR)/traefik/docker-compose.yml up -d

net-traefik-split: ## Create networks for split Traefik compose（三分开前执行一次）
	docker network create the-gate-network 2>/dev/null || true
	docker network create traefik 2>/dev/null || true

up-traefik-herald: ## Start Herald only（三分开，build/traefik-herald）
	docker compose -f $(BUILD_DIR)/traefik-herald/docker-compose.yml up -d

up-traefik-warden: ## Start Warden only（三分开，build/traefik-warden）
	docker compose -f $(BUILD_DIR)/traefik-warden/docker-compose.yml up -d

up-traefik-stargate: ## Start Stargate + protected-service only（三分开，build/traefik-stargate）
	docker compose -f $(BUILD_DIR)/traefik-stargate/docker-compose.yml up -d

down: ## Stop all services（默认与 up 一致，使用 COMPOSE_FILE）
	docker compose -f $(COMPOSE_FILE) down

down-build: ## Stop build/build 启动的服务
	docker compose -f $(BUILD_DIR)/build/docker-compose.yml down

down-image: ## Stop build/image 启动的服务
	docker compose -f $(BUILD_DIR)/image/docker-compose.yml down

down-traefik: ## Stop build/traefik 三合一启动的服务
	docker compose -f $(BUILD_DIR)/traefik/docker-compose.yml down

down-traefik-herald: ## Stop Herald（三分开）
	docker compose -f $(BUILD_DIR)/traefik-herald/docker-compose.yml down

down-traefik-warden: ## Stop Warden（三分开）
	docker compose -f $(BUILD_DIR)/traefik-warden/docker-compose.yml down

down-traefik-stargate: ## Stop Stargate（三分开）
	docker compose -f $(BUILD_DIR)/traefik-stargate/docker-compose.yml down

logs: ## View service logs
	docker compose -f $(COMPOSE_FILE) logs -f

ps: ## View service status
	docker compose -f $(COMPOSE_FILE) ps

test: ## Run end-to-end tests
	go test -v ./e2e/...

test-wait: ## Wait for services to be ready then run tests (recommended)
	@go run ./cmd/suite test-wait

clean: ## Clean services and data volumes
	docker compose -f $(COMPOSE_FILE) down -v

restart: ## Restart all services
	docker compose -f $(COMPOSE_FILE) restart

restart-warden: ## Restart Warden service
	docker compose -f $(COMPOSE_FILE) restart warden

restart-herald: ## Restart Herald service
	docker compose -f $(COMPOSE_FILE) restart herald

restart-stargate: ## Restart Stargate service
	docker compose -f $(COMPOSE_FILE) restart stargate

health: ## Check service health status
	@echo "Checking Stargate..."
	@curl -sf http://localhost:8080/_auth > /dev/null && echo "✓ Stargate Healthy" || echo "✗ Stargate Unhealthy"
	@echo "Checking Warden..."
	@curl -sf http://localhost:8081/health > /dev/null && echo "✓ Warden Healthy" || echo "✗ Warden Unhealthy"
	@echo "Checking Herald..."
	@curl -sf http://localhost:8082/healthz > /dev/null && echo "✓ Herald Healthy" || echo "✗ Herald Unhealthy"

# Go CLI（与 Makefile 等效，可替代 make 使用）
suite: ## Run suite CLI (e.g. make suite ARGS="up" or go run ./cmd/suite help)
	@go run ./cmd/suite $(ARGS)

suite-build: ## Build suite binary to bin/suite
	@mkdir -p bin && go build -o bin/suite ./cmd/suite

serve: ## Start web UI for compose generation (default :8085)
	@go run ./cmd/suite serve
