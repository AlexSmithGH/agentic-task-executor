.PHONY: help install dev-install test lint format clean docker-up docker-down run-worker run-api

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install production dependencies
	pip install -r requirements.txt

dev-install: ## Install development dependencies
	pip install -e ".[dev]"
	pre-commit install

test: ## Run tests
	pytest tests/ -v

lint: ## Run linting
	ruff check src/ tests/
	mypy src/

format: ## Format code
	ruff format src/ tests/

clean: ## Clean temporary files
	find . -type d -name __pycache__ -exec rm -rf {} +
	find . -type f -name "*.pyc" -delete
	find . -type d -name "*.egg-info" -exec rm -rf {} +
	rm -rf build/ dist/ .pytest_cache/ .coverage htmlcov/

docker-up: ## Start Temporal server with Docker Compose
	docker-compose up -d

docker-down: ## Stop Temporal server
	docker-compose down

docker-logs: ## Show Temporal server logs
	docker-compose logs -f temporal

run-worker: ## Run the Temporal worker
	python -m src.worker

run-api: ## Run the API server
	uvicorn src.api:app --reload --host 0.0.0.0 --port 8000

dev: docker-up ## Start full development environment
	@echo "Temporal UI: http://localhost:8080"
	@echo "Starting worker in background..."
	@python -m src.worker &
	@echo "Starting API server..."
	@uvicorn src.api:app --reload
