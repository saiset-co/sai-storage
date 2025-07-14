# SAI Storage Microservice Makefile

# Build configuration
BINARY_NAME=sai-storage
VERSION?=1.0.0
BUILD_DIR=build

# Go configuration
GO_VERSION=1.21
GOOS?=linux
GOARCH?=amd64
CGO_ENABLED?=0

# Docker configuration
DOCKER_IMAGE=sai-storage
DOCKER_TAG?=latest

# Environment configuration
ENV_FILE?=.env

# Default target
.DEFAULT_GOAL := help

# Colors for output
GREEN=\033[0;32m
YELLOW=\033[1;33m
RED=\033[0;31m
NC=\033[0m # No Color

## Help
.PHONY: help
help: ## Show this help message
	@echo "$(GREEN)SAI Storage Microservice$(NC)"
	@echo "$(YELLOW)Available commands:$(NC)"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(GREEN)%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## Development

.PHONY: deps
deps: ## Download Go dependencies
	@echo "$(YELLOW)Downloading Go dependencies...$(NC)"
	@go mod download
	@go mod tidy
	@echo "$(GREEN)Dependencies downloaded!$(NC)"

.PHONY: setup
setup: ## Create .env file from template if it doesn't exist
	@if [ ! -f "$(ENV_FILE)" ]; then \
		echo "$(YELLOW)Creating .env file from template...$(NC)"; \
		cp .env.example $(ENV_FILE) 2>/dev/null || \
		echo "$(RED)No .env.example found. Please create $(ENV_FILE) manually.$(NC)"; \
	else \
		echo "$(GREEN).env file already exists!$(NC)"; \
	fi


.PHONY: config
config: ## Generate config.yaml from template using environment variables
	@echo "$(YELLOW)Generating configuration from template...$(NC)"
	@if [ ! -f "$(ENV_FILE)" ]; then \
		echo "$(RED)Error: $(ENV_FILE) file not found! Run 'make setup' first.$(NC)"; \
		exit 1; \
	fi
	@if [ ! -f "config.yaml.template" ]; then \
		echo "$(RED)Error: config.yaml.template not found!$(NC)"; \
		exit 1; \
	fi
	@echo "$(YELLOW)Loading environment variables from $(ENV_FILE)...$(NC)"
	@set -a; . ./$(ENV_FILE); set +a; envsubst < ./config.yaml.template > ./config.yaml
	@echo "$(GREEN)Configuration generated at ./config.yaml$(NC)"

.PHONY: config-debug
config-debug: ## Debug config generation with verbose output
	@echo "$(YELLOW)=== CONFIG GENERATION DEBUG ===$(NC)"
	@echo "$(YELLOW)1. Checking files...$(NC)"
	@ls -la .env config.yaml.template 2>/dev/null || echo "$(RED)Some files missing$(NC)"
	@echo "$(YELLOW)2. Environment variables from .env:$(NC)"
	@grep -v '^#' .env | grep -v '^$$' | head -10
	@echo "$(YELLOW)3. Testing variable export...$(NC)"
	@set -a; . ./.env; set +a; echo "SERVICE_NAME=$$SERVICE_NAME, SERVER_PORT=$$SERVER_PORT"
	@echo "$(YELLOW)4. Generating config...$(NC)"
	@set -a; . ./.env; set +a; envsubst < ./config.yaml.template > ./config.yaml
	@echo "$(GREEN)5. Done! First few lines of generated config:$(NC)"
	@head -10 config.yaml

## Build
.PHONY: build
build: config ## Build the application binary
	@echo "$(YELLOW)Building $(BINARY_NAME)...$(NC)"
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags="-w -s -X main.version=$(VERSION) -extldflags '-static'" \
		-a -installsuffix cgo \
		-o $(BINARY_NAME) \
		./cmd/main.go
	@echo "$(GREEN)Build complete: $(BINARY_NAME)$(NC)"

## Run
.PHONY: run
run: config ## Run the application locally
	@echo "$(YELLOW)Starting SAI Storage locally...$(NC)"
	@if [ ! -f "./config.yaml" ]; then \
		echo "$(RED)Configuration not found. Generating...$(NC)"; \
		$(MAKE) config; \
	fi
	@go run ./cmd/main.go

## Docker
.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "$(YELLOW)Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)...$(NC)"
	@docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "$(GREEN)Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)$(NC)"

.PHONY: docker-run
docker-run: ## Run Docker container
	@echo "$(YELLOW)Running Docker container...$(NC)"
	@if [ ! -f ".env" ]; then \
		echo "$(RED)Error: .env file not found! Run 'make setup' first.$(NC)"; \
		exit 1; \
	fi
	@docker run --rm --env-file .env -p 8080:8080 $(DOCKER_IMAGE):$(DOCKER_TAG)

## Docker Compose
.PHONY: up
up: ## Start all services with docker-compose
	@echo "$(YELLOW)Starting all services...$(NC)"
	@if [ ! -f ".env" ]; then \
		echo "$(RED)Error: .env file not found! Run 'make setup' first.$(NC)"; \
		exit 1; \
	fi
	@docker-compose up -d
	@echo "$(GREEN)Services started!$(NC)"

.PHONY: down
down: ## Stop all services
	@echo "$(YELLOW)Stopping all services...$(NC)"
	@docker-compose down
	@echo "$(GREEN)Services stopped!$(NC)"

.PHONY: logs
logs: ## Show logs from all services
	@docker-compose logs -f

.PHONY: logs-app
logs-app: ## Show logs from application only
	@docker-compose logs -f sai-storage

.PHONY: logs-mongo
logs-mongo: ## Show logs from MongoDB only
	@docker-compose logs -f mongodb

.PHONY: restart
restart: down up ## Restart all services

.PHONY: rebuild
rebuild: ## Rebuild and restart all services
	@echo "$(YELLOW)Rebuilding and restarting services...$(NC)"
	@docker-compose down
	@docker-compose build --no-cache
	@docker-compose up -d
	@echo "$(GREEN)Services rebuilt and restarted!$(NC)"

## Database
.PHONY: mongo-shell
mongo-shell: ## Connect to MongoDB shell
	@echo "$(YELLOW)Connecting to MongoDB shell...$(NC)"
	@if [ ! -f ".env" ]; then \
		echo "$(RED)Error: .env file not found!$(NC)"; \
		exit 1; \
	fi
	@set -a && . ./.env && set +a && \
		docker exec -it sai-storage-mongodb mongosh \
		--authenticationDatabase admin \
		-u "$$MONGO_ROOT_USERNAME" \
		-p "$$MONGO_ROOT_PASSWORD"

.PHONY: mongo-express
mongo-express: ## Open MongoDB Express in browser
	@echo "$(YELLOW)Opening MongoDB Express...$(NC)"
	@echo "$(GREEN)MongoDB Express should be available at: http://localhost:8081$(NC)"
	@if command -v open >/dev/null 2>&1; then \
		open http://localhost:8081; \
	elif command -v xdg-open >/dev/null 2>&1; then \
		xdg-open http://localhost:8081; \
	fi

.PHONY: mongo-reset
mongo-reset: ## Reset MongoDB data (WARNING: This will delete all data!)
	@echo "$(RED)WARNING: This will delete all MongoDB data!$(NC)"
	@read -p "Are you sure? (y/N): " confirm && [ "$$confirm" = "y" ]
	@echo "$(YELLOW)Stopping services and removing MongoDB data...$(NC)"
	@docker-compose down
	@docker volume rm sai-storage_mongodb_data || true
	@echo "$(GREEN)MongoDB data reset complete!$(NC)"

## Code Quality
.PHONY: lint
lint: ## Run linter
	@echo "$(YELLOW)Running linter...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "$(RED)golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)"; \
	fi

.PHONY: fmt
fmt: ## Format Go code
	@echo "$(YELLOW)Formatting Go code...$(NC)"
	@go fmt ./...
	@echo "$(GREEN)Code formatted!$(NC)"

.PHONY: vet
vet: ## Run go vet
	@echo "$(YELLOW)Running go vet...$(NC)"
	@go vet ./...

.PHONY: mod-tidy
mod-tidy: ## Tidy Go modules
	@echo "$(YELLOW)Tidying Go modules...$(NC)"
	@go mod tidy
	@echo "$(GREEN)Modules tidied!$(NC)"

## Testing
.PHONY: test
test: ## Run tests
	@echo "$(YELLOW)Running tests...$(NC)"
	@go test -v ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage
	@echo "$(YELLOW)Running tests with coverage...$(NC)"
	@go test -v -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report generated: coverage.html$(NC)"

## Cleanup
.PHONY: clean
clean: ## Clean build artifacts and generated files
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -rf $(BUILD_DIR)
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@rm -f config.yaml
	@echo "$(GREEN)Cleanup complete!$(NC)"

.PHONY: clean-docker
clean-docker: ## Clean Docker images and volumes
	@echo "$(YELLOW)Cleaning Docker resources...$(NC)"
	@docker-compose down -v --remove-orphans
	@docker system prune -f
	@echo "$(GREEN)Docker cleanup complete!$(NC)"

.PHONY: clean-all
clean-all: clean clean-docker ## Clean everything

## Quick Commands
.PHONY: status
status: ## Show status of all services
	@echo "$(YELLOW)Service status:$(NC)"
	@docker-compose ps

.PHONY: health
health: ## Check health of the application
	@echo "$(YELLOW)Checking application health...$(NC)"
	@curl -s http://localhost:8080/health | jq . || echo "$(RED)Service not responding$(NC)"

.PHONY: version
version: ## Show version information
	@echo "$(GREEN)SAI Storage Microservice$(NC)"
	@echo "Version: $(VERSION)"
	@echo "Go Version: $(GO_VERSION)"

# Ensure required tools are available
.PHONY: check-tools
check-tools: ## Check if required tools are available
	@echo "$(YELLOW)Checking required tools...$(NC)"
	@command -v go >/dev/null 2>&1 || (echo "$(RED)Go is not installed$(NC)" && exit 1)
	@command -v docker >/dev/null 2>&1 || (echo "$(RED)Docker is not installed$(NC)" && exit 1)
	@command -v docker-compose >/dev/null 2>&1 || (echo "$(RED)Docker Compose is not installed$(NC)" && exit 1)
	@command -v envsubst >/dev/null 2>&1 || (echo "$(RED)envsubst is not installed$(NC)" && exit 1)
	@echo "$(GREEN)All required tools are available!$(NC)"