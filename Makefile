.PHONY: help build run-api run-worker test lint clean docker-up docker-down docker-logs dev

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build API and worker binaries
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

run-api: build ## Run the API server
	./bin/api

run-worker: build ## Run the Temporal worker
	./bin/worker

test: ## Run tests
	go test ./...

lint: ## Run linting
	golangci-lint run

clean: ## Clean build artifacts
	rm -rf bin/

docker-up: ## Start Temporal server with Docker Compose
	docker-compose up -d

docker-down: ## Stop Temporal server
	docker-compose down

docker-logs: ## Show Temporal server logs
	docker-compose logs -f temporal

dev: docker-up ## Start full development environment
	@echo "Temporal UI: http://localhost:8080"
	@echo "Starting worker in background..."
	@go run ./cmd/worker &
	@echo "Starting API server..."
	@go run ./cmd/api
