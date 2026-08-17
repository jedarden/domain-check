.PHONY: help build test check-health clean lint fmt setup-repo-health pre-commit-check

help: ## Show this help message
	@echo 'Domain Check - Makefile commands'
	@echo ''
	@echo 'Usage:'
	@echo '  make <target>'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: ## Build the Go binary
	@echo "Running pre-build repository health check..."
	@$(MAKE) check-health-quick
	go build -o domain-check ./cmd/domain-check

test: ## Run tests
	go test ./...

test-race: ## Run tests with race detector
	go test -race ./...

test-fuzz: ## Run fuzz tests for 30 seconds
	go test -fuzz=. -fuzztime=30s ./internal/domain/

check-health: ## Check repository health and size (quick check)
	@echo "Checking repository health..."
	@bash ./scripts/check-repo-size.sh

check-health-quick: ## Quick repository health check for pre-commit
	@echo "🔍 Quick repository health check..."
	@if [ -f "./scripts/check-repo-size.sh" ]; then \
		bash ./scripts/check-repo-size.sh || echo "⚠️  Repository health check failed"; \
	else \
		echo "⚠️  check-repo-size.sh not found"; \
	fi

check-health-full: ## Run comprehensive repository health check
	@echo "Running comprehensive repository health check..."
	@bash ./scripts/check-repo-health.sh

lint: ## Run golangci-lint
	golangci-lint run

fmt: ## Format Go code
	go fmt ./...

clean: ## Clean build artifacts
	rm -f domain-check
	go clean

setup-repo-health: ## Setup repository health monitoring and git gc configuration
	@echo "Setting up repository health monitoring..."
	@bash ./scripts/setup-git-gc-config.sh
	@echo "✅ Repository health monitoring configured"

pre-commit-check: ## Run pre-commit checks (health + lint + test)
	@echo "Running pre-commit checks..."
	@$(MAKE) check-health-quick
	@$(MAKE) lint
	@$(MAKE) test
	@echo "✅ Pre-commit checks passed"

verify: ## Run all checks (lint, test, health)
	@echo "Running all verification checks..."
	@$(MAKE) check-health-full
	@$(MAKE) lint
	@$(MAKE) test
	@echo "✅ All checks passed"

.DEFAULT_GOAL := help
