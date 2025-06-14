# Lodestone Makefile

.PHONY: build clean test run dev deploy docker help

# Build variables
BINARY_DIR := bin
SERVICES := api-gateway
GO_VERSION := 1.24.3
DOCKER_REGISTRY := lodestone

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build all services
build: ## Build all services
	@echo "Building all services..."
	@mkdir -p $(BINARY_DIR)
	@for service in $(SERVICES); do \
		echo "Building $$service..."; \
		CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(BINARY_DIR)/$$service ./cmd/$$service; \
	done
	@echo "Build complete!"

# Build a specific service
build-%: ## Build a specific service (e.g., make build-api-gateway)
	@echo "Building $*..."
	@mkdir -p $(BINARY_DIR)
	@CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o $(BINARY_DIR)/$* ./cmd/$*
	@echo "Build complete for $*!"

# Clean build artifacts
clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BINARY_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete!"

# Run tests
test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Test complete! Coverage report: coverage.html"

# Run tests without coverage
test-quick: ## Run tests without coverage
	@echo "Running quick tests..."
	@go test -v ./...

# Database operations
migrate-up: ## Run database migrations (standalone)
	@echo "Running database migrations..."
	@go run ./cmd/migrate -up
	@echo "Migrations complete!"

migrate-down: ## Roll back the last migration (standalone)
	@echo "Rolling back last migration..."
	@go run ./cmd/migrate -down
	@echo "Rollback complete!"

# Database operations with explicit connection
migrate-dev-up: ## Run database migrations with dev settings
	@echo "Running database migrations with dev settings..."
	@export DB_HOST="localhost" && \
	export DB_PORT="5432" && \
	export DB_USER="lodestone" && \
	export DB_PASSWORD="lodestone" && \
	export DB_NAME="lodestone" && \
	export DB_SSLMODE="disable" && \
	go run ./cmd/migrate -up
	@echo "Migrations complete!"

migrate-dev-down: ## Roll back migrations with dev settings  
	@echo "Rolling back last migration with dev settings..."
	@export DB_HOST="localhost" && \
	export DB_PORT="5432" && \
	export DB_USER="lodestone" && \
	export DB_PASSWORD="lodestone" && \
	export DB_NAME="lodestone" && \
	export DB_SSLMODE="disable" && \
	go run ./cmd/migrate -down
	@echo "Rollback complete!"

migrate-build: ## Build migration tool
	@echo "Building migration tool..."
	@mkdir -p $(BINARY_DIR)
	@go build -o $(BINARY_DIR)/migrate ./cmd/migrate
	@echo "Migration tool built!"

# Deployment with migrations
deploy-migrate-local: ## Deploy local environment with migrations
	@echo "Deploying local environment with migrations..."
	@./deploy/scripts/deploy.sh migrate-up local
	@./deploy/scripts/deploy.sh up local

deploy-migrate-dev: ## Deploy dev environment with migrations
	@echo "Deploying dev environment with migrations..."
	@./deploy/scripts/deploy.sh migrate-up dev
	@./deploy/scripts/deploy.sh up dev

deploy-migrate-prod: ## Deploy production environment with migrations
	@echo "Deploying production environment with migrations..."
	@./deploy/scripts/deploy.sh migrate-up prod
	@./deploy/scripts/deploy.sh up prod

# Migration commands for deployment environments
migrate-prod-up: ## Run migrations in production environment
	@echo "Running migrations in production environment..."
	@./deploy/scripts/deploy.sh migrate-up prod

migrate-prod-down: ## Roll back migrations in production environment
	@echo "Rolling back migrations in production environment..."
	@./deploy/scripts/deploy.sh migrate-down prod

# Format code
fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Format complete!"

# Lint code
lint: ## Lint code
	@echo "Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Download dependencies
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "Dependencies updated!"

# Run the API gateway locally
run: build-api-gateway ## Run API gateway locally
	@echo "Starting API gateway..."
	@./$(BINARY_DIR)/api-gateway

# Run the API gateway locally with development settings
run-dev: build-api-gateway ## Run API gateway locally with dev environment
	@echo "Starting API gateway with development settings..."
	@export DATABASE_URL="postgres://lodestone:lodestone@localhost:5432/lodestone?sslmode=disable" && \
	export REDIS_URL="redis://localhost:6379" && \
	export JWT_SECRET="dev-secret-key-change-in-production" && \
	export PORT="8080" && \
	export LOG_LEVEL="debug" && \
	export STORAGE_TYPE="local" && \
	export STORAGE_LOCAL_PATH="./storage" && \
	./$(BINARY_DIR)/api-gateway

# Run with Swagger docs generation
run-swagger: swagger run-dev ## Generate Swagger docs and run API gateway
	@echo "API running with Swagger UI available at http://localhost:8080/swagger/index.html"

# Development setup with Docker Compose
dev: ## Start development environment with Docker Compose
	@echo "Starting development environment..."
	@./deploy/scripts/deploy.sh up dev

# Development with migrations
dev-migrate: ## Start development environment with automatic migrations
	@echo "Starting development environment with migrations..."
	@./deploy/scripts/deploy.sh up dev --migrate

# Stop development environment
dev-down: ## Stop development environment
	@echo "Stopping development environment..."
	@./deploy/scripts/deploy.sh down dev

# Build Docker images
docker: ## Build Docker images for all services
	@echo "Building Docker images..."
	@for service in $(SERVICES); do \
		echo "Building Docker image for $$service..."; \
		docker build -f deployments/docker/Dockerfile.$$service -t $(DOCKER_REGISTRY)/$$service:latest .; \
	done
	@echo "Docker build complete!"

# Build Docker image for specific service
docker-%: ## Build Docker image for specific service
	@echo "Building Docker image for $*..."
	@docker build -f deploy/configs/docker/Dockerfile.$* -t $(DOCKER_REGISTRY)/$*:latest .
	@echo "Docker build complete for $*!"

# Deploy to Kubernetes
deploy: ## Deploy to Kubernetes
	@echo "Deploying to Kubernetes..."
	@kubectl apply -f deploy/configs/k8s/
	@echo "Deployment complete!"

# Undeploy from Kubernetes
undeploy: ## Remove from Kubernetes
	@echo "Removing from Kubernetes..."
	@kubectl delete -f deploy/configs/k8s/
	@echo "Undeployment complete!"

# Generate Swagger documentation
swagger: ## Generate Swagger documentation
	@echo "Generating Swagger docs..."
	@go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api-gateway/main.go -o docs --parseDependency --parseInternal
	@echo "Swagger documentation generated in docs/"

# Security scan
security: ## Run security scan
	@echo "Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not installed. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

# Performance benchmarks
bench: ## Run performance benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

# Check for outdated dependencies
deps-check: ## Check for outdated dependencies
	@echo "Checking for outdated dependencies..."
	@go list -u -m all

# Initialize project (run once)
init: ## Initialize project dependencies
	@echo "Initializing project..."
	@go mod init github.com/lgulliver/lodestone || true
	@go mod tidy
	@echo "Project initialized!"

# Full pipeline (format, lint, test, build)
ci: fmt lint test build ## Run full CI pipeline

# Show project info
info: ## Show project information
	@echo "Project: Lodestone Artifact Feed"
	@echo "Go Version: $(GO_VERSION)"
	@echo "Services: $(SERVICES)"
	@echo "Binary Directory: $(BINARY_DIR)"

# Docker Compose Management
.PHONY: docker-build docker-up docker-down docker-logs docker-clean docker-dev docker-prod

docker-build: ## Build Docker images
	@echo "Building Docker images..."
	@cd deploy/compose && docker-compose build

docker-up: ## Start all services with Docker Compose
	@echo "Starting Lodestone services..."
	@./deploy/scripts/deploy.sh up dev

docker-down: ## Stop all services
	@echo "Stopping Lodestone services..."
	@./deploy/scripts/deploy.sh down dev

docker-logs: ## View logs from all services
	@./deploy/scripts/deploy.sh logs dev

docker-clean: ## Clean up Docker resources
	@echo "Cleaning up Docker resources..."
	@./deploy/scripts/deploy.sh clean dev

docker-dev: ## Start development environment
	@echo "Starting development environment..."
	@./deploy/scripts/deploy.sh up dev

docker-prod: ## Start production environment with Nginx
	@echo "Starting production environment..."
	@./deploy/scripts/deploy.sh up prod

# Backup and Restore
backup: ## Create backup of data volumes
	@echo "Creating backup..."
	@mkdir -p backups/$(shell date +%Y%m%d_%H%M%S)
	@docker run --rm -v lodestone_postgres_data:/data -v $(PWD)/backups/$(shell date +%Y%m%d_%H%M%S):/backup alpine tar czf /backup/postgres.tar.gz /data
	@docker run --rm -v lodestone_artifacts_data:/data -v $(PWD)/backups/$(shell date +%Y%m%d_%H%M%S):/backup alpine tar czf /backup/artifacts.tar.gz /data
	@echo "Backup completed in backups/$(shell date +%Y%m%d_%H%M%S)"

# Deployment helpers
deploy-check: ## Check if deployment is ready
	@echo "Checking deployment health..."
	@./deploy/scripts/health-check.sh

deploy-status: ## Show status of all services
	@./deploy/scripts/deploy.sh ps dev

# Environment setup
env-setup: ## Copy environment template
	@echo "Setting up environment file..."
	@./deploy/scripts/setup.sh dev
	@echo "Please edit .env file with your configuration"

# Security
security-scan: ## Run security scan on Docker images
	@echo "Running security scan..."
	@docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
		-v $(PWD):/path \
		aquasec/trivy image lodestone_api-gateway:latest
