# beef-briefing Makefile
# Development and infrastructure management automation

# Variables
GO_VERSION := 1.25
PROJECT_NAME := beef-briefing
DOCKER_COMPOSE := docker-compose -f infrastructure/docker-compose.dev.yml
SERVICES := api-service

# Service paths
API_SERVICE_PATH := apps/api-service

# Build variables
CGO_ENABLED := 0
GOOS := linux
BUILD_FLAGS := -a -installsuffix cgo

# Colors for output
COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_BLUE := \033[34m

.DEFAULT_GOAL := help

##@ General

.PHONY: help
help: ## Display this help message
	@echo "$(COLOR_BOLD)$(PROJECT_NAME) - Available targets:$(COLOR_RESET)"
	@awk 'BEGIN {FS = ":.*##"; printf "\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  $(COLOR_BLUE)%-20s$(COLOR_RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(COLOR_BOLD)%s$(COLOR_RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Infrastructure

.PHONY: up
up: env-validate ## Start all services with docker-compose
	@echo "$(COLOR_GREEN)Starting all services...$(COLOR_RESET)"
	$(DOCKER_COMPOSE) up -d
	@echo "$(COLOR_GREEN)Services started successfully$(COLOR_RESET)"

.PHONY: down
down: ## Stop and remove all containers
	@echo "$(COLOR_YELLOW)Stopping all services...$(COLOR_RESET)"
	$(DOCKER_COMPOSE) down
	@echo "$(COLOR_GREEN)Services stopped$(COLOR_RESET)"

.PHONY: restart
restart: ## Restart all services
	@echo "$(COLOR_YELLOW)Restarting all services...$(COLOR_RESET)"
	$(DOCKER_COMPOSE) restart
	@echo "$(COLOR_GREEN)Services restarted$(COLOR_RESET)"

.PHONY: logs
logs: ## Show logs for all services (use SVC=service-name for specific service)
	@if [ -n "$(SVC)" ]; then \
		echo "$(COLOR_BLUE)Showing logs for $(SVC)...$(COLOR_RESET)"; \
		$(DOCKER_COMPOSE) logs -f $(SVC); \
	else \
		echo "$(COLOR_BLUE)Showing logs for all services...$(COLOR_RESET)"; \
		$(DOCKER_COMPOSE) logs -f; \
	fi

.PHONY: ps
ps: ## Show status of all services
	@echo "$(COLOR_BLUE)Service status:$(COLOR_RESET)"
	$(DOCKER_COMPOSE) ps

##@ Build

.PHONY: build
build: build-api-service ## Build all Go service binaries

.PHONY: build-api-service
build-api-service: ## Build api-service binary
	@echo "$(COLOR_GREEN)Building api-service...$(COLOR_RESET)"
	cd $(API_SERVICE_PATH) && \
		CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) go build $(BUILD_FLAGS) -o bin/api-service ./cmd
	@echo "$(COLOR_GREEN)api-service binary built successfully$(COLOR_RESET)"

.PHONY: docker-build
docker-build: docker-build-api-service ## Build all Docker images

.PHONY: docker-build-api-service
docker-build-api-service: ## Build api-service Docker image
	@echo "$(COLOR_GREEN)Building api-service Docker image...$(COLOR_RESET)"
	docker build -t $(PROJECT_NAME)/api-service:latest -f $(API_SERVICE_PATH)/Dockerfile $(API_SERVICE_PATH)
	@echo "$(COLOR_GREEN)api-service Docker image built successfully$(COLOR_RESET)"

.PHONY: docker-rebuild
docker-rebuild: docker-rebuild-api-service ## Rebuild and restart all services

.PHONY: docker-rebuild-api-service
docker-rebuild-api-service: docker-build-api-service ## Rebuild and restart api-service
	@echo "$(COLOR_YELLOW)Restarting api-service...$(COLOR_RESET)"
	$(DOCKER_COMPOSE) up -d --no-deps --build api-service
	@echo "$(COLOR_GREEN)api-service rebuilt and restarted$(COLOR_RESET)"

##@ Environment

.PHONY: env-validate
env-validate: ## Validate .env file exists and contains required variables
	@echo "$(COLOR_BLUE)Validating environment configuration...$(COLOR_RESET)"
	@if [ ! -f .env ]; then \
		echo "$(COLOR_YELLOW)Warning: .env file not found$(COLOR_RESET)"; \
		if [ -f .env.example ]; then \
			echo "$(COLOR_YELLOW)Please copy .env.example to .env and configure it$(COLOR_RESET)"; \
		fi; \
		exit 1; \
	fi
	@echo "$(COLOR_GREEN)Environment file validated$(COLOR_RESET)"

##@ Cleanup

.PHONY: clean
clean: ## Remove containers, volumes, and build artifacts
	@echo "$(COLOR_YELLOW)Cleaning up...$(COLOR_RESET)"
	$(DOCKER_COMPOSE) down -v
	@if [ -d $(API_SERVICE_PATH)/bin ]; then \
		rm -rf $(API_SERVICE_PATH)/bin; \
		echo "$(COLOR_GREEN)Removed api-service build artifacts$(COLOR_RESET)"; \
	fi
	@echo "$(COLOR_GREEN)Cleanup complete$(COLOR_RESET)"

.PHONY: clean-volumes
clean-volumes: ## Remove all Docker volumes (WARNING: deletes all data)
	@echo "$(COLOR_YELLOW)WARNING: This will delete all database and MinIO data$(COLOR_RESET)"
	@read -p "Are you sure? [y/N] " -n 1 -r; \
	echo; \
	if [[ $$REPLY =~ ^[Yy]$$ ]]; then \
		$(DOCKER_COMPOSE) down -v; \
		echo "$(COLOR_GREEN)Volumes removed$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_BLUE)Cancelled$(COLOR_RESET)"; \
	fi
