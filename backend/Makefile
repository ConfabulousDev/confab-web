# Backend-domain targets. The root Makefile (../Makefile) is the one-stop shop for
# the dev loop and delegates here; these also work standalone from backend/.

.DEFAULT_GOAL := help
.PHONY: help build test coverage deps fmt clean

help: ## List available targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-10s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the backend binary
	go build -o bin/confab-backend ./cmd/server

test: ## Run tests
	go test ./...

coverage: ## Run full backend test coverage (sharded, reliable)
	./scripts/coverage.sh

deps: ## Download and tidy dependencies
	go mod download
	go mod tidy

fmt: ## Format code
	go fmt ./...

clean: ## Clean build artifacts
	rm -rf bin/
