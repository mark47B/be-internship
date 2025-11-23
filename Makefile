.PHONY: help build test test-e2e lint generate-models generate-server compose-up

# Variables
OAPI_CODEGEN := oapi-codegen
OPENAPI_FILE := api/openapi.yml
GEN_DIR := internal/infra/transport/rest/gen
GO_VERSION := 1.24.10
BINARY_NAME := app
DOCKER_COMPOSE := docker compose
BINARY_DIR := bin

# Code generation
generate-models: ## Генерация моделей из OpenAPI спецификации
	@echo "Generating models from OpenAPI spec..."
	@mkdir -p $(GEN_DIR)
	$(OAPI_CODEGEN) -config configs/oapi/models.yaml $(OPENAPI_FILE)
	@echo "Models generated"

generate-server: ## Генерация серверного кода из OpenAPI спецификации
	@echo "Generating chi-server from OpenAPI spec..."
	@mkdir -p $(GEN_DIR)
	$(OAPI_CODEGEN) -config configs/oapi/server.yaml $(OPENAPI_FILE)
	@echo "Server code generated"

generate: generate-models generate-server ## Генерация всего кода из OpenAPI

# Build
build: generate
	@echo "Building application..."
	@mkdir -p $(BINARY_DIR)
	go build -o $(BINARY_DIR)/$(BINARY_NAME) ./cmd
	@echo "Build completed: $(BINARY_NAME)"

# Linting
lint: ## Запуск линтера
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi
	@echo "Linting completed"

# Testing
test-e2e: ## Запуск E2E тестов (требует Docker)
	@echo "Running E2E tests..."
	@echo "Note: E2E tests require Docker to be running"
	go test -tags=e2e -v ./test/e2e
	@echo "E2E tests completed"

# Docker Compose
compose-up: ## Запуск docker-compose
	@echo "Starting services..."
	docker compose up --build
	@echo "Services started"

compose-restart:
	@echo "Stopping services..."
	docker compose down
	docker rm -f $$(docker ps -aq) || true
	docker volume rm $$(docker volume ls -q) || true
	docker compose up --build
	@echo "Services stopped"

j-re:
	docker rm -f $$(docker ps -aq) || true
	docker volume rm $$(docker volume ls -q) || true
	docker compose -f docker-compose.jmeter.yml up --build


# Development
dev: generate ## Запуск в режиме разработки (требует локальный PostgreSQL)
	@echo "Starting development server..."
	@echo "Make sure PostgreSQL is running and migrations are applied"
	go run cmd/main.go
