# Backend-domain targets. The root Makefile (../Makefile) is the one-stop shop for
# the dev loop and delegates here; these also work standalone from backend/.

.DEFAULT_GOAL := help
.PHONY: help build test coverage deps fmt fmt-check clean

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

fmt: ## Format code (gofmt + go fix modernizers)
	go fmt ./...
	go fix ./...

# gofmt comes from the go.mod toolchain's GOROOT, not PATH: gofmt alignment rules
# differ across Go versions. `gofmt -l` exits 0 even when it lists files, so test
# for output; `go fix -diff` exits non-zero on drift and prints the diff.
fmt-check: ## Fail if gofmt or go fix would change files (CI gate)
	@out=$$("$$(go env GOROOT)/bin/gofmt" -l .); \
	if [ -n "$$out" ]; then \
		echo "gofmt needed (run 'make fmt'):"; \
		echo "$$out"; \
		exit 1; \
	fi
	go fix -diff ./...

clean: ## Clean build artifacts
	rm -rf bin/
