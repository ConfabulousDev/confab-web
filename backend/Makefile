.PHONY: help dev build test clean docker-up docker-down docker-logs

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

dev: ## Run development server
	go run cmd/server/main.go

build: ## Build binary
	go build -o bin/confab-backend cmd/server/main.go

test: ## Run tests
	go test -v ./...

clean: ## Clean build artifacts
	rm -rf bin/

docker-up: ## Start PostgreSQL and MinIO
	docker-compose up -d
	@echo "Waiting for services to be healthy..."
	@sleep 5
	@echo "PostgreSQL: http://localhost:5432"
	@echo "MinIO Console: http://localhost:9001 (minioadmin/minioadmin)"

docker-down: ## Stop PostgreSQL and MinIO
	docker-compose down

docker-logs: ## Show docker logs
	docker-compose logs -f

deps: ## Download dependencies
	go mod download
	go mod tidy
