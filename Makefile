.PHONY: help up down logs test clean

help: ## Show help information
	@echo "the-gate End-to-End Integration Test Project"
	@echo ""
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

up: ## Start all services
	docker compose up -d --build

down: ## Stop all services
	docker compose down

logs: ## View service logs
	docker compose logs -f

ps: ## View service status
	docker compose ps

test: ## Run end-to-end tests
	go test -v ./e2e/...

test-wait: ## Wait for services to be ready then run tests (recommended)
	@echo "Waiting for services to be ready (3s)..."
	@sleep 3
	@go test -v ./e2e/...

clean: ## Clean services and data volumes
	docker compose down -v

restart: ## Restart all services
	docker compose restart

restart-warden: ## Restart Warden service
	docker compose restart warden

restart-herald: ## Restart Herald service
	docker compose restart herald

restart-stargate: ## Restart Stargate service
	docker compose restart stargate

health: ## Check service health status
	@echo "Checking Stargate..."
	@curl -sf http://localhost:8080/_auth > /dev/null && echo "✓ Stargate Healthy" || echo "✗ Stargate Unhealthy"
	@echo "Checking Warden..."
	@curl -sf http://localhost:8081/health > /dev/null && echo "✓ Warden Healthy" || echo "✗ Warden Unhealthy"
	@echo "Checking Herald..."
	@curl -sf http://localhost:8082/healthz > /dev/null && echo "✓ Herald Healthy" || echo "✗ Herald Unhealthy"
