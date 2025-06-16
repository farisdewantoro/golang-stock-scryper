# Makefile for Golang Job Scheduler

.PHONY: all build clean run test help swag-install docs migrate docker-up docker-down docker-deps-up docker-deps-down docker-logs

# Variables
APP_NAME_SCHEDULER := scheduling-service
APP_NAME_EXECUTOR := execution-service
CMD_SCHEDULER_PATH := ./cmd/$(APP_NAME_SCHEDULER)
CMD_EXECUTOR_PATH := ./cmd/$(APP_NAME_EXECUTOR)
CMD_MIGRATE_PATH := ./cmd/migrate

# Default target
all: build

# Build the application binaries
build:
	@echo "Building $(APP_NAME_SCHEDULER)..."
	go build -o bin/$(APP_NAME_SCHEDULER) $(CMD_SCHEDULER_PATH)/main.go
	@echo "Building $(APP_NAME_EXECUTOR)..."
	go build -o bin/$(APP_NAME_EXECUTOR) $(CMD_EXECUTOR_PATH)/main.go
	@echo "Build complete."

# Run the services (example, will need refinement)
run-scheduler:
	@echo "Starting $(APP_NAME_SCHEDULER)..."
	go run $(CMD_SCHEDULER_PATH)/main.go serve

run-executor:
	@echo "Starting $(APP_NAME_EXECUTOR)..."
	go run $(CMD_EXECUTOR_PATH)/main.go serve

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	rm -rf bin/
	@echo "Clean complete."

# Run tests (to be implemented)
test:
	@echo "Running tests... (not implemented)"
	go test ./...

# Documentation commands
swag-install:
	@echo "Installing swag CLI..."
	go install github.com/swaggo/swag/cmd/swag@latest

docs:
	@echo "Generating Swagger docs..."
	go run github.com/swaggo/swag/cmd/swag init -g cmd/scheduling-service/main.go --output internal/scheduler/docs


# Database migration command
migrate:
	@echo "Running database migrations..."
	go run $(CMD_MIGRATE_PATH)/main.go up
	@echo "Migrations complete."

# Docker compose commands
docker-up:
	@echo "Starting services with Docker Compose..."
	docker-compose -f deployments/docker-compose.yaml -f deployments/docker-compose.deps.yaml up -d

docker-down:
	@echo "Stopping services with Docker Compose..."
	docker-compose -f deployments/docker-compose.yaml -f deployments/docker-compose.deps.yaml down

docker-deps-up:
	@echo "Starting dependencies with Docker Compose..."
	docker-compose -f deployments/docker-compose.deps.yaml up

docker-deps-down:
	@echo "Stopping dependencies with Docker Compose..."
	docker-compose -f deployments/docker-compose.deps.yaml down

docker-logs:
	@echo "Showing Docker logs..."
	docker-compose -f deployments/docker-compose.yaml -f deployments/docker-compose.deps.yaml logs -f

# Help target
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all                Build all services (default)"
	@echo "  build              Build all services"
	@echo "  run-scheduler      Run the scheduling service"
	@echo "  run-executor       Run the execution service"
	@echo "  clean              Remove build artifacts"
	@echo "  test               Run tests (not yet implemented)"
	@echo "  swag-install       Install the swag CLI tool"
	@echo "  docs               Generate Swagger API documentation"
	@echo "  docker-up          Start services with Docker Compose"
	@echo "  docker-down        Stop services with Docker Compose"
	@echo "  docker-deps-up     Start dependencies with Docker Compose"
	@echo "  docker-deps-down   Stop dependencies with Docker Compose"
	@echo "  docker-logs        Follow Docker logs"
	@echo "  migrate            Run database migrations"
	@echo "  help               Show this help message"

