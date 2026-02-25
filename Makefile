# Job Queue Makefile
# Provides common development tasks and quality checks

.PHONY: all help build test race vet lint fmt tidy clean check

# Default target
all: check

# Help target
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

# Build targets
build: ## Build all packages
	go build ./...

build-examples: ## Build example applications
	go build ./examples/...

# Test targets
test: ## Run all tests
	go test -v ./...

race: ## Run tests with race detector
	go test -race ./...

# Quality checks
vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

fmt: ## Format all Go files
	gofmt -w .

check-fmt: ## Check if all Go files are formatted
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "The following files need formatting:"; \
		gofmt -l .; \
		exit 1; \
	else \
		echo "All files are properly formatted"; \
	fi

# Dependency management
tidy: ## Tidy and verify go modules
	go mod tidy
	go mod verify

download: ## Download all dependencies
	go mod download

# Coverage
coverage: ## Run tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

coverage-html: ## Open coverage report in browser
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report written to coverage.html"

# Combined quality check
check: check-fmt vet test lint ## Run all quality checks (format, vet, test, lint)

# Clean
clean: ## Clean build artifacts and test cache
	go clean -testcache
	rm -f coverage.out coverage.html

# Development workflow
dev: build test ## Quick development check (build + test)

# CI simulation (runs checks that CI would run)
ci: check-fmt tidy vet race ## Run CI checks locally
	@echo "All CI checks passed!"

# Install dependencies for development
setup: ## Install development dependencies
	@echo "Installing golangci-lint..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Setup complete!"
