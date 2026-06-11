# TPT Online Video — Root Makefile
#
# Common development tasks for the monorepo.

.PHONY: help infra up down api worker live web test lint tidy clean \
        web-install web-build web-lint web-typecheck web-format \
        lint-go format-go lint-all \
        docker-build docker-up-prod docker-down-prod docker-logs-prod

help: ## Show this help
	@echo "TPT Online Video — Development Makefile"
	@echo ""
	@echo "--- Development Commands ---"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

## Infrastructure

infra: ## Start all infrastructure services (Postgres, Redis, MinIO, MediaMTX)
	docker compose up -d postgres redis minio mediamtx

up: ## Start all services via Docker Compose
	docker compose up -d

down: ## Stop all Docker Compose services
	docker compose down

logs: ## Tail logs from all Docker Compose services
	docker compose logs -f

## Production Docker (fully containerized)

docker-build: ## Build all Docker images for production
	docker compose -f docker-compose.prod.yml build

docker-up-prod: ## Start production stack (builds images first if needed)
	docker compose -f docker-compose.prod.yml up -d

docker-down-prod: ## Stop production stack
	docker compose -f docker-compose.prod.yml down

docker-logs-prod: ## Tail logs from production stack
	docker compose -f docker-compose.prod.yml logs -f

docker-restart-prod: ## Restart a specific service in production stack (usage: make docker-restart-prod SVC=tpt-api)
	docker compose -f docker-compose.prod.yml restart $(SVC)

## Go services

api: ## Run the API service locally
	cd services/api && go run ./cmd/tpt-api

worker: ## Run the worker service locally
	cd services/worker && go run ./cmd/tpt-worker

live: ## Run the live helper service locally
	cd services/live && go run ./cmd/tpt-live

test-go: ## Run all Go tests
	go test ./...

tidy-go: ## Tidy all Go modules
	cd packages/shared && go mod tidy
	cd packages/storage && go mod tidy
	cd packages/search && go mod tidy
	cd packages/auth && go mod tidy
	cd packages/media && go mod tidy
	cd packages/moderation && go mod tidy
	cd services/api && go mod tidy
	cd services/worker && go mod tidy
	cd services/live && go mod tidy
	go work sync

lint-go: ## Lint all Go code
	gofmt -l -s -w packages/ services/

## Frontend (via pnpm)

web: ## Run the frontend dev server
	cd apps/web && npm run dev

web-install: ## Install frontend dependencies (npm fallback)
	cd apps/web && npm install

web-build: ## Build frontend for production
	cd apps/web && npm run build

web-lint: ## Lint frontend with ESLint
	cd apps/web && npx eslint src/

web-typecheck: ## Run TypeScript type checking on frontend
	cd apps/web && npx tsc --noEmit

web-format: ## Format frontend with Prettier
	npx prettier --write "apps/web/src/**/*.{ts,tsx,css}"

## Linting

lint-all: lint-go web-lint ## Lint all code (Go + frontend)

## Utility

clean: ## Clean up local data directories
	rm -rf ./data/storage
	rm -rf ./data/redis
	rm -rf ./data/postgres

format-all: ## Format all code (Go + frontend)
	gofmt -l -s -w packages/ services/
	npx prettier --write "apps/web/src/**/*.{ts,tsx,css}"