# Confabulous — one-stop shop for local development.
#
# Quick start:  make setup && make dev
#
# `make dev` runs the whole stack in one terminal; or run the pieces (make up,
# make backend, make frontend) in separate terminals. `make help` lists targets.

.DEFAULT_GOAL := help
.PHONY: help setup dev up down logs backend frontend test coverage build clean

help: ## List available targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

setup: ## First-time setup: create backend/.env and install frontend deps
	@cp -n backend/.env.example backend/.env 2>/dev/null && echo "created backend/.env" || echo "backend/.env already exists"
	cd frontend && npm install

dev: up ## Run the full stack (infra + backend + frontend) in one terminal
	@cp -n backend/.env.example backend/.env 2>/dev/null || true
	@echo "Backend → http://localhost:8080   Frontend → http://localhost:5173   ·   Ctrl-C stops both"
	@echo "(Postgres + MinIO keep running in Docker — 'make down' stops them)"
	@trap 'kill 0' EXIT; \
		( cd backend && set -a && . ./.env && set +a && go run ./cmd/server ) & \
		( cd frontend && npm install && npm run dev ) & \
		wait

up: ## Start local infra (Postgres + MinIO) and apply migrations
	docker compose -f docker-compose.infra.yml up -d --build

down: ## Stop and remove infra containers
	docker compose -f docker-compose.infra.yml down

logs: ## Tail infra logs
	docker compose -f docker-compose.infra.yml logs -f

backend: ## Run the backend, hot-compiled (http://localhost:8080)
	@cp -n backend/.env.example backend/.env 2>/dev/null || true
	cd backend && set -a && . ./.env && set +a && go run ./cmd/server

frontend: ## Run the frontend with hot reload (http://localhost:5173)
	cd frontend && npm install && npm run dev

test: ## Run backend + frontend tests
	$(MAKE) -C backend test
	cd frontend && npm test

coverage: ## Backend test coverage (sharded)
	$(MAKE) -C backend coverage

build: ## Build the backend binary
	$(MAKE) -C backend build

clean: ## Remove build artifacts
	$(MAKE) -C backend clean
