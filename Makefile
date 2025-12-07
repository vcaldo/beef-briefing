# Variables
COMPOSE_FILE := infrastructure/docker-compose.dev.yml
ENV_FILE := infrastructure/.env.dev

# Service names
API_SERVICE := api-service
TELEGRAM_BOT := telegram-bot
POSTGRES_SERVICE := postgres
MINIO_SERVICE := minio

# Go directories
API_DIR := apps/api-service
BOT_DIR := apps/telegram-bot
PKG_DIR := pkg/config

# Default target
.DEFAULT_GOAL := help

# Help target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

# Docker lifecycle targets
up: ## Start all services
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) up -d

down: ## Stop all services
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) down

restart: ## Restart all services
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) restart

ps: ## Show running containers
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) ps

clean: ## Stop services and remove volumes
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) down -v

prune: ## Remove all project containers, images, volumes, and networks
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) down -v --rmi all --remove-orphans

# Docker build targets
build: ## Rebuild all images
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) build

build-api: ## Rebuild api-service image
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) build $(API_SERVICE)

build-bot: ## Rebuild telegram-bot image
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) build $(TELEGRAM_BOT)

build-postgres: ## Rebuild postgres image
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) build $(POSTGRES_SERVICE)

# Logging targets
logs: ## Tail logs from all services
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) logs -f

logs-api: ## Tail logs from api-service
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) logs -f $(API_SERVICE)

logs-bot: ## Tail logs from telegram-bot
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) logs -f $(TELEGRAM_BOT)

logs-postgres: ## Tail logs from postgres
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) logs -f $(POSTGRES_SERVICE)

logs-minio: ## Tail logs from minio
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) logs -f $(MINIO_SERVICE)

# Shell targets
shell-api: ## Open shell in api-service container
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) exec $(API_SERVICE) /bin/bash

shell-bot: ## Open shell in telegram-bot container
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) exec $(TELEGRAM_BOT) /bin/bash

shell-postgres: ## Open shell in postgres container
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) exec $(POSTGRES_SERVICE) /bin/bash

shell-minio: ## Open shell in minio container
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) exec $(MINIO_SERVICE) /bin/sh

# Go build targets
go-build-api: ## Build api-service binary locally
	@echo "Building api-service..."
	cd $(API_DIR) && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/api-service ./cmd
	@echo "Binary created at $(API_DIR)/bin/api-service"

go-build-bot: ## Build telegram-bot binary locally
	@echo "Building telegram-bot..."
	cd $(BOT_DIR) && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/telegram-bot ./cmd
	@echo "Binary created at $(BOT_DIR)/bin/telegram-bot"

go-build: go-build-api go-build-bot ## Build all Go binaries locally

go-clean: ## Remove Go build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(API_DIR)/bin
	rm -rf $(BOT_DIR)/bin
	@echo "Done!"

# Go quality targets
fmt: ## Format Go code
	@echo "Formatting api-service..."
	cd $(API_DIR) && gofmt -w -s .
	@echo "Formatting telegram-bot..."
	cd $(BOT_DIR) && gofmt -w -s .
	@echo "Formatting pkg/config..."
	cd $(PKG_DIR) && gofmt -w -s .
	@echo "Done!"

fmt-check: ## Check if Go code is formatted
	@echo "Checking api-service formatting..."
	@cd $(API_DIR) && test -z "$$(gofmt -l .)" || (echo "Files need formatting in $(API_DIR):" && gofmt -l . && exit 1)
	@echo "Checking telegram-bot formatting..."
	@cd $(BOT_DIR) && test -z "$$(gofmt -l .)" || (echo "Files need formatting in $(BOT_DIR):" && gofmt -l . && exit 1)
	@echo "Checking pkg/config formatting..."
	@cd $(PKG_DIR) && test -z "$$(gofmt -l .)" || (echo "Files need formatting in $(PKG_DIR):" && gofmt -l . && exit 1)
	@echo "All files properly formatted!"

# Phony targets
.PHONY: help up down restart ps clean prune build build-api build-bot build-postgres \
	logs logs-api logs-bot logs-postgres logs-minio shell-api shell-bot shell-postgres shell-minio \
	go-build-api go-build-bot go-build go-clean fmt fmt-check
