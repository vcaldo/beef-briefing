# Variables
COMPOSE_FILE ?= infrastructure/docker-compose.dev.yml
ENV_FILE ?= infrastructure/.env.dev
TERRAFORM_DIR := infrastructure/terraform

# Service names
API_SERVICE := api-service
TELEGRAM_BOT := telegram-bot
POSTGRES_SERVICE := postgres
MINIO_SERVICE := minio
ADMIN_PANEL := admin-panel
NEWRELIC_INFRA := newrelic-infra

# Go directories
API_DIR := apps/api-service
BOT_DIR := apps/telegram-bot
ADMIN_PANEL_DIR := apps/admin-panel
IMPORT_CLI_DIR := apps/import-cli
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

up-build: ## Rebuild images and start all services
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) up -d --build

up-logs: ## Start all services and show logs in foreground
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) up

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

build-admin-panel: ## Rebuild admin-panel image
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) build $(ADMIN_PANEL)

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

logs-admin-panel: ## Tail logs from admin-panel
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) logs -f $(ADMIN_PANEL)

logs-newrelic: ## Tail logs from newrelic-infra
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) logs -f $(NEWRELIC_INFRA)

# Shell targets
shell-api: ## Open shell in api-service container
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) exec $(API_SERVICE) /bin/bash

shell-bot: ## Open shell in telegram-bot container
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) exec $(TELEGRAM_BOT) /bin/bash

shell-postgres: ## Open shell in postgres container
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) exec $(POSTGRES_SERVICE) /bin/bash

shell-minio: ## Open shell in minio container
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) exec $(MINIO_SERVICE) /bin/sh

shell-admin-panel: ## Open shell in admin-panel container
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) exec $(ADMIN_PANEL) /bin/bash

shell-newrelic: ## Open shell in newrelic-infra container
	docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) exec $(NEWRELIC_INFRA) /bin/sh

# Go build targets
go-build-api: ## Build api-service binary locally
	@echo "Building api-service..."
	cd $(API_DIR) && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/api-service ./cmd
	@echo "Binary created at $(API_DIR)/bin/api-service"

go-build-bot: ## Build telegram-bot binary locally
	@echo "Building telegram-bot..."
	cd $(BOT_DIR) && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/telegram-bot ./cmd
	@echo "Binary created at $(BOT_DIR)/bin/telegram-bot"

go-build-admin-panel: ## Build admin-panel binary locally
	@echo "Building admin-panel..."
	cd $(ADMIN_PANEL_DIR) && templ generate && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/admin-panel ./cmd
	@echo "Binary created at $(ADMIN_PANEL_DIR)/bin/admin-panel"

go-build-import-cli: ## Build import-cli binary locally
	@echo "Building import-cli..."
	cd $(IMPORT_CLI_DIR) && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/import-cli ./cmd
	@echo "Binary created at $(IMPORT_CLI_DIR)/bin/import-cli"

go-build: go-build-api go-build-bot go-build-admin-panel go-build-import-cli ## Build all Go binaries locally

go-clean: ## Remove Go build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(API_DIR)/bin
	rm -rf $(BOT_DIR)/bin
	rm -rf $(ADMIN_PANEL_DIR)/bin
	rm -rf $(IMPORT_CLI_DIR)/bin
	@echo "Done!"

# Go quality targets
fmt: ## Format Go code
	@echo "Formatting api-service..."
	cd $(API_DIR) && gofmt -w -s .
	@echo "Formatting telegram-bot..."
	cd $(BOT_DIR) && gofmt -w -s .
	@echo "Formatting admin-panel..."
	cd $(ADMIN_PANEL_DIR) && gofmt -w -s .
	@echo "Formatting import-cli..."
	cd $(IMPORT_CLI_DIR) && gofmt -w -s .
	@echo "Formatting pkg/config..."
	cd $(PKG_DIR) && gofmt -w -s .
	@echo "Done!"

fmt-check: ## Check if Go code is formatted
	@echo "Checking api-service formatting..."
	@cd $(API_DIR) && test -z "$$(gofmt -l .)" || (echo "Files need formatting in $(API_DIR):" && gofmt -l . && exit 1)
	@echo "Checking telegram-bot formatting..."
	@cd $(BOT_DIR) && test -z "$$(gofmt -l .)" || (echo "Files need formatting in $(BOT_DIR):" && gofmt -l . && exit 1)
	@echo "Checking admin-panel formatting..."
	@cd $(ADMIN_PANEL_DIR) && test -z "$$(gofmt -l .)" || (echo "Files need formatting in $(ADMIN_PANEL_DIR):" && gofmt -l . && exit 1)
	@echo "Checking import-cli formatting..."
	@cd $(IMPORT_CLI_DIR) && test -z "$$(gofmt -l .)" || (echo "Files need formatting in $(IMPORT_CLI_DIR):" && gofmt -l . && exit 1)
	@echo "Checking pkg/config formatting..."
	@cd $(PKG_DIR) && test -z "$$(gofmt -l .)" || (echo "Files need formatting in $(PKG_DIR):" && gofmt -l . && exit 1)
	@echo "All files properly formatted!"

# Admin Panel secret generation
admin-panel-set-secrets: ## Generate password hash and session secret for admin panel
	@cd $(ADMIN_PANEL_DIR)/tools && go run update_secrets.go -file ../../../$(ENV_FILE)

admin-panel-set-password: ## Generate and update only password hash
	@cd $(ADMIN_PANEL_DIR)/tools && go run update_secrets.go -file ../../../$(ENV_FILE) -password-only

admin-panel-set-session: ## Generate and update only session secret
	@cd $(ADMIN_PANEL_DIR)/tools && go run update_secrets.go -file ../../../$(ENV_FILE) -session-only

admin-panel-set-secrets-files: ## Generate secrets and write to separate files (recommended)
	@cd $(ADMIN_PANEL_DIR)/tools && go run update_secrets.go -mode=files -secrets-dir ../../../infrastructure/secrets/admin-panel

admin-panel-set-password-file: ## Generate password hash and write to file
	@cd $(ADMIN_PANEL_DIR)/tools && go run update_secrets.go -mode=files -secrets-dir ../../../infrastructure/secrets/admin-panel -password-only

admin-panel-set-session-file: ## Generate session secret and write to file
	@cd $(ADMIN_PANEL_DIR)/tools && go run update_secrets.go -mode=files -secrets-dir ../../../infrastructure/secrets/admin-panel -session-only

# Terraform targets
tf-init: ## Initialize Terraform working directory
	cd $(TERRAFORM_DIR) && terraform init

tf-plan: ## Show Terraform execution plan
	cd $(TERRAFORM_DIR) && terraform plan

tf-apply: ## Apply Terraform configuration
	cd $(TERRAFORM_DIR) && terraform apply

tf-destroy: ## Destroy Terraform-managed infrastructure (DANGEROUS!)
	cd $(TERRAFORM_DIR) && terraform destroy

tf-output: ## Show all Terraform outputs
	cd $(TERRAFORM_DIR) && terraform output

tf-show: ## Show current Terraform state
	cd $(TERRAFORM_DIR) && terraform show

tf-validate: ## Validate Terraform configuration
	cd $(TERRAFORM_DIR) && terraform validate

tf-refresh: ## Refresh Terraform state
	cd $(TERRAFORM_DIR) && terraform refresh

# Terraform formatting and state
tf-fmt: ## Format Terraform files
	cd $(TERRAFORM_DIR) && terraform fmt -recursive

tf-fmt-check: ## Check if Terraform files are formatted
	cd $(TERRAFORM_DIR) && terraform fmt -check -recursive

tf-state-list: ## List resources in Terraform state
	cd $(TERRAFORM_DIR) && terraform state list

tf-state-show: ## Show detailed state for a resource (set RESOURCE=<name>)
	@if [ -z "$(RESOURCE)" ]; then \
		echo "Error: RESOURCE variable is required. Usage: make tf-state-show RESOURCE=<resource_name>"; \
		exit 1; \
	fi
	cd $(TERRAFORM_DIR) && terraform state show $(RESOURCE)

tf-unlock: ## Force unlock Terraform state (DANGEROUS! Prompts for LOCK_ID)
	@echo "WARNING: This will force unlock the Terraform state."
	@echo "Only use this if you are certain there is no other Terraform process running."
	@read -p "Enter the Lock ID to unlock: " lock_id; \
	if [ -z "$$lock_id" ]; then \
		echo "Error: Lock ID cannot be empty"; \
		exit 1; \
	fi; \
	cd $(TERRAFORM_DIR) && terraform force-unlock -force $$lock_id

# Terraform outputs
tf-ip: ## Show instance IP address
	@cd $(TERRAFORM_DIR) && terraform output -raw instance_ip

tf-ssh: ## Show SSH connection command
	@cd $(TERRAFORM_DIR) && terraform output -raw ssh_connection

tf-ssh-user-host: ## Show SSH user and host
	@cd $(TERRAFORM_DIR) && terraform output -raw ssh_user_host

tf-root-pass: ## Show root password (SENSITIVE)
	@cd $(TERRAFORM_DIR) && terraform output -raw root_password

# Terraform utilities
tf-connect: ## SSH to the Linode instance
	@ssh admin@$$(cd $(TERRAFORM_DIR) && terraform output -raw instance_ip)

tf-setup: ## Setup Terraform configuration (copy tfvars example and populate from .env)
	@if [ ! -f $(TERRAFORM_DIR)/terraform.tfvars ]; then \
		cp $(TERRAFORM_DIR)/terraform.tfvars.example $(TERRAFORM_DIR)/terraform.tfvars; \
		echo "Created terraform.tfvars from example."; \
		if [ -f infrastructure/.env.prod ]; then \
			ENV_FILE=infrastructure/.env.prod; \
		elif [ -f infrastructure/.env.dev ]; then \
			ENV_FILE=infrastructure/.env.dev; \
		else \
			echo "Warning: No .env.prod or .env.dev found. Please edit terraform.tfvars manually."; \
			exit 0; \
		fi; \
		LINODE_TOKEN=$$(grep '^LINODE_TOKEN=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		DOMAIN_NAME=$$(grep '^DOMAIN_NAME=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		NEW_RELIC_KEY=$$(grep '^NEW_RELIC_LICENSE_KEY=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		NEW_RELIC_ACCOUNT=$$(grep '^NEW_RELIC_ACCOUNT_ID=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		SSH_KEY_PATH=$$(grep '^SSH_PUBLIC_KEY_PATH=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		if [ -n "$$LINODE_TOKEN" ]; then \
			sed -i "s|linode_token = \".*\"|linode_token = \"$$LINODE_TOKEN\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated linode_token from $$ENV_FILE"; \
		fi; \
		if [ -n "$$DOMAIN_NAME" ]; then \
			sed -i "s|# domain_name = \".*\"|domain_name = \"$$DOMAIN_NAME\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated domain_name from $$ENV_FILE"; \
		fi; \
		if [ -n "$$NEW_RELIC_KEY" ]; then \
			echo "new_relic_license_key = \"$$NEW_RELIC_KEY\"" >> $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated new_relic_license_key from $$ENV_FILE"; \
		fi; \
		if [ -n "$$NEW_RELIC_ACCOUNT" ]; then \
			echo "new_relic_account_id = \"$$NEW_RELIC_ACCOUNT\"" >> $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated new_relic_account_id from $$ENV_FILE"; \
		fi; \
		if [ -n "$$SSH_KEY_PATH" ]; then \
			sed -i "s|# ssh_public_key_path = \".*\"|ssh_public_key_path = \"$$SSH_KEY_PATH\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated ssh_public_key_path from $$ENV_FILE"; \
		fi; \
		BLOCK_STORAGE_SIZE=$$(grep '^BLOCK_STORAGE_SIZE=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		if [ -n "$$BLOCK_STORAGE_SIZE" ]; then \
			echo "block_storage_size = $$BLOCK_STORAGE_SIZE" >> $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated block_storage_size from $$ENV_FILE"; \
		fi; \
		echo "Setup complete! Review $(TERRAFORM_DIR)/terraform.tfvars before applying."; \
	else \
		echo "terraform.tfvars already exists. Delete it first to recreate."; \
	fi

tf-docs: ## Show Terraform documentation
	@cat $(TERRAFORM_DIR)/README.md

tf-deploy-check: tf-validate tf-fmt-check tf-plan ## Full pre-deployment check (validate, format check, plan)

# Production deployment targets
PROD_COMPOSE_FILE := infrastructure/docker-compose.prod.yml
PROD_ENV_FILE := infrastructure/.env.prod
COMMIT_HASH ?= $(shell git rev-parse --short HEAD)

deploy: ## Deploy to production server with commit-tagged images
	@echo "🚀 Starting deployment with commit hash: $(COMMIT_HASH)"
	@echo "📝 Updating IMAGE_TAG in $(PROD_ENV_FILE)..."
	@sed -i 's/^IMAGE_TAG=.*/IMAGE_TAG=$(COMMIT_HASH)/' $(PROD_ENV_FILE)
	@echo "🔨 Building Docker images..."
	@docker compose -f $(PROD_COMPOSE_FILE) --env-file $(PROD_ENV_FILE) build
	@echo "📋 Verifying built images..."
	@docker images | grep "beef-briefing.*$(COMMIT_HASH)" || (echo "❌ Error: Images not built with tag $(COMMIT_HASH)" && exit 1)
	@echo "💾 Saving images to /tmp/images-$(COMMIT_HASH).tar.gz..."
	@docker save \
		beef-briefing/postgres:$(COMMIT_HASH) \
		beef-briefing/api-service:$(COMMIT_HASH) \
		beef-briefing/telegram-bot:$(COMMIT_HASH) \
		beef-briefing/admin-panel:$(COMMIT_HASH) \
		| gzip > /tmp/images-$(COMMIT_HASH).tar.gz
	@echo "✓ Image archive created: $$(du -h /tmp/images-$(COMMIT_HASH).tar.gz | cut -f1)"
	@echo "📤 Transferring files to server..."
	@scp $(PROD_COMPOSE_FILE) $$($(MAKE) -s tf-ssh-user-host):/tmp/docker-compose.yml
	@scp $(PROD_ENV_FILE) $$($(MAKE) -s tf-ssh-user-host):/tmp/.env
	@scp -r infrastructure/secrets $$($(MAKE) -s tf-ssh-user-host):/tmp/
	@scp /tmp/images-$(COMMIT_HASH).tar.gz $$($(MAKE) -s tf-ssh-user-host):/tmp/
	@echo "🚢 Deploying on server..."
	@ssh $$($(MAKE) -s tf-ssh-user-host) '\
		mkdir -p ~/beef-briefing && \
		mv /tmp/docker-compose.yml ~/beef-briefing/ && \
		mv /tmp/.env ~/beef-briefing/ && \
		rm -rf ~/beef-briefing/secrets && mv /tmp/secrets ~/beef-briefing/ && \
		gunzip -c /tmp/images-$(COMMIT_HASH).tar.gz | docker load && \
		cd ~/beef-briefing && docker compose up -d && \
		rm /tmp/images-$(COMMIT_HASH).tar.gz'
	@rm /tmp/images-$(COMMIT_HASH).tar.gz
	@echo "✅ Deployment complete! Services running with image tag: $(COMMIT_HASH)"

# Phony targets
.PHONY: help up down restart ps clean prune build build-api build-bot build-postgres build-admin-panel \
	logs logs-api logs-bot logs-postgres logs-minio logs-admin-panel logs-newrelic \
	shell-api shell-bot shell-postgres shell-minio shell-admin-panel shell-newrelic \
	go-build-api go-build-bot go-build-admin-panel go-build-import-cli go-build go-clean fmt fmt-check \
	admin-panel-set-secrets admin-panel-set-password admin-panel-set-session \
	admin-panel-set-secrets-files admin-panel-set-password-file admin-panel-set-session-file \
	tf-init tf-plan tf-apply tf-destroy tf-output tf-show tf-validate tf-refresh \
	tf-fmt tf-fmt-check tf-state-list tf-state-show tf-unlock \
	tf-ip tf-ssh tf-root-pass tf-connect tf-setup tf-docs tf-deploy-check \
	deploy
