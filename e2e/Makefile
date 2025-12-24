# E2E Test Makefile
#
# Prerequisites:
#   - Docker and docker-compose
#   - Go toolchain (for building confab CLI)
#   - ANTHROPIC_API_KEY environment variable set
#
# Usage:
#   make build    # Build all containers
#   make test     # Run all E2E tests
#   make clean    # Teardown and cleanup

.PHONY: build build-cli build-containers up down test clean check-api-key lint setup-dev

# Path to confab CLI source (sibling repo)
CONFAB_CLI_DIR := ../../confab

# Check for required env var
check-api-key:
	@if [ -z "$(E2E_ANTHROPIC_API_KEY)" ]; then \
		echo "Error: E2E_ANTHROPIC_API_KEY environment variable is required"; \
		echo "Get an API key from https://console.anthropic.com/"; \
		exit 1; \
	fi

# Detect architecture (arm64 for Apple Silicon, amd64 otherwise)
ARCH := $(shell uname -m)
ifeq ($(ARCH),arm64)
    GOARCH := arm64
else
    GOARCH := amd64
endif

# Build confab CLI for linux
build-cli:
	@echo "Building confab CLI for linux/$(GOARCH)..."
	cd $(CONFAB_CLI_DIR) && GOOS=linux GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o $(CURDIR)/containers/workstation/confab .
	@echo "CLI built: containers/workstation/confab"

# Build all Docker containers
build-containers:
	@echo "Building Docker containers..."
	docker-compose build

# Build everything
build: build-cli build-containers
	@echo "Build complete"

# Start infrastructure services (postgres, minio, backend)
# Health checks are handled by docker-compose depends_on conditions
up:
	@echo "Starting infrastructure..."
	docker-compose up -d postgres minio backend
	@echo "Infrastructure started (workstation will wait for health checks)"

# Stop all services
down:
	docker-compose down -v

# Run all E2E tests
test: check-api-key build up
	@echo "Running E2E tests..."
	docker-compose run --rm workstation pytest /e2e/tests -v
	docker-compose down -v

# Run a specific test
test-%: check-api-key build up
	@echo "Running test: $*"
	docker-compose run --rm workstation pytest /e2e/tests -v -k "$*"
	docker-compose down -v

# Cleanup everything
clean:
	docker-compose down -v --rmi local
	rm -f containers/workstation/confab

# Shell into workstation for debugging
shell: up
	docker-compose run --rm workstation bash

# View logs
logs:
	docker-compose logs -f

# View backend logs only
logs-backend:
	docker-compose logs -f backend

# Lint Python tests (run from host, requires: make setup-dev)
lint:
	.venv/bin/ruff check tests/ && .venv/bin/ruff format --check tests/
	.venv/bin/pyright tests/

# Set up local dev environment for linting
setup-dev:
	python3 -m venv .venv
	.venv/bin/pip install -q ruff pyright pytest requests types-requests
