# =============================================================================
# VARIABLES
# =============================================================================

# Environment files
COMPOSE_FILE ?= infrastructure/docker-compose.dev.yml
ENV_FILE ?= infrastructure/.env.dev
PROD_COMPOSE_FILE := infrastructure/docker-compose.prod.yml
PROD_ENV_FILE := infrastructure/.env.prod

# Directories
TERRAFORM_DIR := infrastructure/terraform
SCRIPTS_DIR := scripts
SECRETS_DIR := infrastructure/secrets

# Docker compose shortcuts (DRY)
DC := docker compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE)
DC_PROD := docker compose -f $(PROD_COMPOSE_FILE) --env-file $(PROD_ENV_FILE)

# Service names
API_SERVICE := api-service
TELEGRAM_BOT := telegram-bot
POSTGRES_SERVICE := postgres
MINIO_SERVICE := minio
NEWRELIC_INFRA := newrelic-infra
TRAEFIK := traefik

# Go directories
API_DIR := apps/api-service
BOT_DIR := apps/telegram-bot
IMPORT_CLI_DIR := apps/import-cli
PKG_DIR := pkg/config

# Python directories
ML_PROCESSOR_DIR := apps/ml-processor

# Git
COMMIT_HASH ?= $(shell git rev-parse --short HEAD)

# =============================================================================
# DEFAULT TARGET
# =============================================================================
.DEFAULT_GOAL := help

# =============================================================================
# HELP
# =============================================================================
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Popular aliases:'
	@echo '  up       -> dev-up'
	@echo '  build    -> docker-build'
	@echo '  deploy   -> prod-deploy'
	@echo ''
	@echo 'Available targets (grouped by domain):'
	@echo ''
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-30s\033[0m %s\n", $$1, $$2}' | \
		sort

# =============================================================================
# POPULAR ALIASES
# =============================================================================
up: dev-up ## Alias for dev-up
build: docker-build ## Alias for docker-build
deploy: prod-deploy ## Alias for prod-deploy
down: dev-down ## Alias for dev-down

# =============================================================================
# DEVELOPMENT (dev-*)
# =============================================================================
dev-up: ## Start all dev services
	@$(DC) up -d
	@$(SCRIPTS_DIR)/show-summary.sh dev

dev-up-build: ## Rebuild images and start all dev services
	@$(DC) up -d --build
	@$(SCRIPTS_DIR)/show-summary.sh dev

dev-up-logs: ## Start all dev services with logs in foreground
	$(DC) up

dev-down: ## Stop all dev services
	$(DC) down

dev-restart: ## Restart all dev services
	$(DC) restart

dev-ps: ## Show running dev containers
	$(DC) ps

dev-clean: ## Stop dev services and remove volumes
	$(DC) down -v

dev-prune: ## Remove all dev containers, images, volumes, and networks
	$(DC) down -v --rmi all --remove-orphans

# =============================================================================
# PRODUCTION (prod-*)
# =============================================================================
prod-deploy: tf-sync-object-storage ## Deploy to production server
	@$(SCRIPTS_DIR)/deploy.sh

prod-deploy-skip-build: tf-sync-object-storage ## Deploy using existing images (skip build)
	@$(SCRIPTS_DIR)/deploy.sh --skip-build

prod-deploy-skip-cleanup: tf-sync-object-storage ## Deploy without cleaning up old images
	@$(SCRIPTS_DIR)/deploy.sh --skip-cleanup

prod-deploy-regenerate-certs: ## Deploy with fresh Let's Encrypt certificates
	@echo "Removing Let's Encrypt certificates on remote server..."
	@ssh $$($(MAKE) -s tf-ssh-user-host) 'rm -f ~/beef-briefing/infrastructure/letsencrypt/acme.json'
	@echo "Certificates removed, deploying with fresh certificates..."
	@$(MAKE) prod-deploy

prod-rollback: ## Rollback to previous deployment
	@$(SCRIPTS_DIR)/rollback.sh

prod-rollback-force: ## Rollback to previous deployment (skip confirmation)
	@$(SCRIPTS_DIR)/rollback.sh --force

prod-backup-db: ## Backup production database to local_backups/db/
	@$(SCRIPTS_DIR)/backup-prod-db.sh

prod-clean-certs: ## Remove Let's Encrypt certificates on remote server
	@echo "Removing Let's Encrypt certificates on remote server..."
	@ssh $$($(MAKE) -s tf-ssh-user-host) 'rm -f ~/beef-briefing/infrastructure/letsencrypt/acme.json'
	@echo "Certificates removed"

prod-logs-traefik: ## Tail logs from traefik (production)
	@SSH_HOST=$$($(MAKE) -s tf-ssh-user-host); \
	ssh $$SSH_HOST 'cd ~/beef-briefing && docker compose logs -f traefik'

prod-update-ip: ## Update IP allowlist for api-service and card-renderer
	@echo "Fetching current IP address..."
	@ALLOWED_IP=$$(curl -s whatismyip.akamai.com); \
	if [ -z "$$ALLOWED_IP" ]; then \
		echo "Error: Failed to fetch IP"; \
		exit 1; \
	fi; \
	echo "Current IP: $$ALLOWED_IP"; \
	if grep -q "^ALLOWED_IP=" $(PROD_ENV_FILE); then \
		sed -i "s/^ALLOWED_IP=.*/ALLOWED_IP=$$ALLOWED_IP/" $(PROD_ENV_FILE); \
	else \
		echo "ALLOWED_IP=$$ALLOWED_IP" >> $(PROD_ENV_FILE); \
	fi; \
	echo "Updated $(PROD_ENV_FILE)"; \
	echo "Syncing to remote and recreating containers with IP allowlist..."; \
	scp $(PROD_ENV_FILE) $$($(MAKE) -s tf-ssh-user-host):~/beef-briefing/.env; \
	ssh $$($(MAKE) -s tf-ssh-user-host) 'cd ~/beef-briefing && docker compose up -d --force-recreate --no-deps api-service card-renderer'

pg-tunnel: ## Open SSH tunnel to production PostgreSQL (localhost:5433 -> prod postgres)
	@echo "Opening SSH tunnel to production PostgreSQL..."
	@echo "Connect locally using: psql -h localhost -p 5433 -U postgres -d beef_briefing"
	@echo "Press Ctrl+C to close the tunnel"
	@ssh -L 5433:localhost:5432 $$($(MAKE) -s tf-ssh-user-host) -N

# =============================================================================
# DOCKER BUILD (docker-build-*)
# =============================================================================
docker-build: ## Rebuild all Docker images
	@START_TIME=$$(date +%s); \
	$(DC) build; \
	END_TIME=$$(date +%s); \
	IMAGE_TAG=$(COMMIT_HASH) START_TIME=$$START_TIME END_TIME=$$END_TIME $(SCRIPTS_DIR)/show-summary.sh build

docker-build-api: ## Rebuild api-service image
	$(DC) build $(API_SERVICE)

docker-build-bot: ## Rebuild telegram-bot image
	$(DC) build $(TELEGRAM_BOT)

# =============================================================================
# DOCKER LOGS (docker-logs-*)
# =============================================================================
docker-logs: ## Tail logs from all services
	$(DC) logs -f

docker-logs-api: ## Tail logs from api-service
	$(DC) logs -f $(API_SERVICE)

docker-logs-bot: ## Tail logs from telegram-bot
	$(DC) logs -f $(TELEGRAM_BOT)

docker-logs-postgres: ## Tail logs from postgres
	$(DC) logs -f $(POSTGRES_SERVICE)

docker-logs-minio: ## Tail logs from minio
	$(DC) logs -f $(MINIO_SERVICE)

docker-logs-newrelic: ## Tail logs from newrelic-infra
	$(DC) logs -f $(NEWRELIC_INFRA)

# =============================================================================
# DOCKER SHELL (docker-shell-*)
# =============================================================================
docker-shell-api: ## Open shell in api-service container
	$(DC) exec $(API_SERVICE) /bin/bash

docker-shell-bot: ## Open shell in telegram-bot container
	$(DC) exec $(TELEGRAM_BOT) /bin/bash

docker-shell-postgres: ## Open shell in postgres container
	$(DC) exec $(POSTGRES_SERVICE) /bin/bash

docker-shell-minio: ## Open shell in minio container
	$(DC) exec $(MINIO_SERVICE) /bin/sh

docker-shell-newrelic: ## Open shell in newrelic-infra container
	$(DC) exec $(NEWRELIC_INFRA) /bin/sh

# =============================================================================
# GO BUILD (go-build-*)
# =============================================================================
go-build: go-build-api go-build-bot go-build-import-cli ## Build all Go binaries locally

go-build-api: ## Build api-service binary locally
	@echo "Building api-service..."
	cd $(API_DIR) && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/api-service ./cmd
	@echo "Binary created at $(API_DIR)/bin/api-service"

go-build-bot: ## Build telegram-bot binary locally
	@echo "Building telegram-bot..."
	cd $(BOT_DIR) && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/telegram-bot ./cmd
	@echo "Binary created at $(BOT_DIR)/bin/telegram-bot"

go-build-import-cli: ## Build import-cli binary locally
	@echo "Building import-cli..."
	cd $(IMPORT_CLI_DIR) && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o bin/import-cli ./cmd
	@echo "Binary created at $(IMPORT_CLI_DIR)/bin/import-cli"

go-build-import-cli-prod: ## Build import-cli for production server and deploy
	@$(SCRIPTS_DIR)/go-build-import-cli-prod.sh

go-clean: ## Remove Go build artifacts
	@echo "Cleaning build artifacts..."
	rm -rf $(API_DIR)/bin
	rm -rf $(BOT_DIR)/bin
	rm -rf $(IMPORT_CLI_DIR)/bin
	@echo "Done!"

# =============================================================================
# GO FORMAT (go-fmt-*)
# =============================================================================
go-fmt: ## Format all Go code
	@echo "Formatting api-service..."
	cd $(API_DIR) && gofmt -w -s .
	@echo "Formatting telegram-bot..."
	cd $(BOT_DIR) && gofmt -w -s .
	@echo "Formatting import-cli..."
	cd $(IMPORT_CLI_DIR) && gofmt -w -s .
	@echo "Formatting pkg/config..."
	cd $(PKG_DIR) && gofmt -w -s .
	@echo "Done!"

go-fmt-check: ## Check if Go code is formatted
	@echo "Checking api-service formatting..."
	@cd $(API_DIR) && test -z "$$(gofmt -l .)" || (echo "Files need formatting in $(API_DIR):" && gofmt -l . && exit 1)
	@echo "Checking telegram-bot formatting..."
	@cd $(BOT_DIR) && test -z "$$(gofmt -l .)" || (echo "Files need formatting in $(BOT_DIR):" && gofmt -l . && exit 1)
	@echo "Checking import-cli formatting..."
	@cd $(IMPORT_CLI_DIR) && test -z "$$(gofmt -l .)" || (echo "Files need formatting in $(IMPORT_CLI_DIR):" && gofmt -l . && exit 1)
	@echo "Checking pkg/config formatting..."
	@cd $(PKG_DIR) && test -z "$$(gofmt -l .)" || (echo "Files need formatting in $(PKG_DIR):" && gofmt -l . && exit 1)
	@echo "All files properly formatted!"

# =============================================================================
# SECRETS (secrets-*)
# =============================================================================
secrets-traefik-password: ## Generate Traefik dashboard password
	@chmod +x $(SCRIPTS_DIR)/generate-traefik-password.sh
	@$(SCRIPTS_DIR)/generate-traefik-password.sh

secrets-service-api: ## Generate API key for an app (APP=telegram-bot required)
	@if [ -z "$(APP)" ]; then \
		echo "Error: APP variable is required. Usage: make secrets-service-api APP=telegram-bot"; \
		exit 1; \
	fi
	@chmod +x $(SCRIPTS_DIR)/generate-api-service-key.sh
	@$(SCRIPTS_DIR)/generate-api-service-key.sh "$(APP)" "$(SECRETS_DIR)"

secrets-card-renderer: ## Generate card-renderer API key for an app (APP=ml-processor required)
	@if [ -z "$(APP)" ]; then \
		echo "Error: APP variable is required. Usage: make secrets-card-renderer APP=ml-processor"; \
		exit 1; \
	fi
	@chmod +x $(SCRIPTS_DIR)/generate-card-renderer-key.sh
	@$(SCRIPTS_DIR)/generate-card-renderer-key.sh "$(APP)" "$(SECRETS_DIR)"

secrets-jwt: ## Generate JWT secret key for Mini App authentication
	@chmod +x $(SCRIPTS_DIR)/generate-jwt-secret.sh
	@$(SCRIPTS_DIR)/generate-jwt-secret.sh

# =============================================================================
# TERRAFORM (tf-*)
# =============================================================================

# Terraform lifecycle
tf-init: ## Initialize Terraform working directory
	cd $(TERRAFORM_DIR) && terraform init

tf-plan: ## Show Terraform execution plan
	cd $(TERRAFORM_DIR) && terraform plan

tf-apply: ## Apply Terraform configuration (updates SSH config after)
	cd $(TERRAFORM_DIR) && terraform apply
	@$(SCRIPTS_DIR)/update-ssh-config.sh

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

# Terraform formatting
tf-fmt: ## Format Terraform files
	cd $(TERRAFORM_DIR) && terraform fmt -recursive

tf-fmt-check: ## Check if Terraform files are formatted
	cd $(TERRAFORM_DIR) && terraform fmt -check -recursive

# Terraform state
tf-state-list: ## List resources in Terraform state
	cd $(TERRAFORM_DIR) && terraform state list

tf-state-show: ## Show detailed state for a resource (RESOURCE=<name>)
	@if [ -z "$(RESOURCE)" ]; then \
		echo "Error: RESOURCE variable is required. Usage: make tf-state-show RESOURCE=<resource_name>"; \
		exit 1; \
	fi
	cd $(TERRAFORM_DIR) && terraform state show $(RESOURCE)

tf-unlock: ## Force unlock Terraform state (DANGEROUS!)
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

tf-arch: ## Show instance CPU architecture
	@REMOTE_ARCH_RAW=$$(ssh $$($(MAKE) -s tf-ssh-user-host) 'uname -m' 2>/dev/null || echo "x86_64"); \
	case "$$REMOTE_ARCH_RAW" in \
		x86_64) REMOTE_ARCH="amd64" ;; \
		aarch64) REMOTE_ARCH="arm64" ;; \
		armv7l) REMOTE_ARCH="arm" ;; \
		*) REMOTE_ARCH="amd64" ;; \
	esac; \
	echo "$$REMOTE_ARCH_RAW (GOARCH: $$REMOTE_ARCH)"

tf-root-pass: ## Show root password (SENSITIVE)
	@cd $(TERRAFORM_DIR) && terraform output -raw root_password

tf-object-storage-endpoint: ## Show Object Storage endpoint
	@cd $(TERRAFORM_DIR) && terraform output -raw object_storage_endpoint

tf-object-storage-access-key: ## Show Object Storage access key ID (SENSITIVE)
	@cd $(TERRAFORM_DIR) && terraform output -raw object_storage_access_key_id

tf-object-storage-secret-key: ## Show Object Storage secret key (SENSITIVE)
	@cd $(TERRAFORM_DIR) && terraform output -raw object_storage_secret_access_key

tf-object-storage-bucket: ## Show Object Storage bucket name
	@cd $(TERRAFORM_DIR) && terraform output -raw object_storage_bucket_name

# Terraform utilities
tf-connect: ## SSH to the Linode instance
	@ssh admin@$$(cd $(TERRAFORM_DIR) && terraform output -raw instance_ip)

tf-setup: ## Setup Terraform configuration from .env file
	@$(SCRIPTS_DIR)/tf-setup.sh

tf-sync-object-storage: ## Sync Object Storage credentials to .env.prod
	@$(SCRIPTS_DIR)/tf-sync-object-storage.sh

tf-update-ssh-config: ## Update SSH config with server details
	@$(SCRIPTS_DIR)/update-ssh-config.sh

tf-docs: ## Show Terraform documentation
	@cat $(TERRAFORM_DIR)/README.md

tf-deploy-check: tf-validate tf-fmt-check tf-plan ## Full pre-deployment check

# =============================================================================
# ML PROCESSOR (ml-*)
# =============================================================================
# Run ml-processor commands inside Docker container
# Container must be running: make up-build
# Pass additional args with ML_ARGS: make ml-run ML_ARGS="--limit 100"
# Use -prod suffix for production: make ml-run-prod (requires: make pg-tunnel)

# Development (local postgres) - DEFAULT
ml-run: ## Run ml-processor batch processing (dev)
	./scripts/ml-processor.sh process $(ML_ARGS)

ml-run-status: ## Show ml-processor status (dev)
	./scripts/ml-processor.sh status $(ML_ARGS)

ml-run-once: ## Run single batch (dev)
	./scripts/ml-processor.sh process --limit 1 $(ML_ARGS)

ml-run-continuous: ## Run continuous processing (dev)
	./scripts/ml-processor.sh continuous $(ML_ARGS)

ml-run-cards: ## Generate weekly user cards (dev)
	./scripts/ml-processor.sh cards $(ML_ARGS)

ml-run-render: ## Render card images (dev)
	./scripts/ml-processor.sh render $(ML_ARGS)

# Production (requires: make pg-tunnel in another terminal)
ml-run-prod: ## Run ml-processor batch processing (prod)
	./scripts/ml-processor.sh --prod process $(ML_ARGS)

ml-run-status-prod: ## Show ml-processor status (prod)
	./scripts/ml-processor.sh --prod status $(ML_ARGS)

ml-run-once-prod: ## Run single batch (prod)
	./scripts/ml-processor.sh --prod process --limit 1 $(ML_ARGS)

ml-run-continuous-prod: ## Run continuous processing (prod)
	./scripts/ml-processor.sh --prod continuous $(ML_ARGS)

ml-run-cards-prod: ## Generate weekly user cards (prod)
	./scripts/ml-processor.sh --prod cards $(ML_ARGS)

ml-run-render-prod: ## Render card images (prod)
	./scripts/ml-processor.sh --prod render $(ML_ARGS)

# Utility
ml-shell: ## Open shell in ml-processor container
	./scripts/ml-processor.sh shell

ml-clean-dev: ## Clean all ML data (dev - PostgreSQL + Qdrant)
	@echo "Cleaning ML data from dev PostgreSQL..."
	@$(DC) exec -T postgres psql -U $${DB_USER:-postgres} -d $${DB_NAME:-beef_briefing} -c "\
		TRUNCATE ml_user_profiles, ml_user_cards, ml_message_topics, ml_topics, ml_ner, ml_humor, ml_questions, ml_toxicity, ml_sentiment, ml_processing_state CASCADE;"
	@echo "Cleaning ML data from dev Qdrant..."
	@curl -s -X DELETE "http://localhost:6333/collections/message_embeddings" || true
	@echo "ML data cleaned (dev)"

ml-clean-prod: ## Clean all ML data (prod - PostgreSQL + Qdrant)
	@echo "WARNING: This will delete ALL ML data from production!"
	@read -p "Are you sure? (yes/no): " confirm && [ "$$confirm" = "yes" ] || exit 1
	@echo "Cleaning ML data from prod PostgreSQL..."
	@ssh $$($(MAKE) -s tf-ssh-user-host) 'cd ~/beef-briefing && docker compose exec -T postgres psql -U postgres -d beef_briefing -c "\
		TRUNCATE ml_user_profiles, ml_user_cards, ml_message_topics, ml_topics, ml_ner, ml_humor, ml_questions, ml_toxicity, ml_sentiment, ml_processing_state CASCADE;"'
	@echo "Cleaning ML data from prod Qdrant (local)..."
	@curl -s -X DELETE "http://localhost:6333/collections/message_embeddings" || true
	@echo "ML data cleaned (prod)"

# Card cleanup targets
ml-clean-cards-dev: ## Clean cards for a chat (dev). Usage: make ml-clean-cards-dev ML_ARGS="--chat-id -1003280306634 [--week 2024-12-16]"
	./scripts/ml-processor.sh clean-cards $(ML_ARGS)

ml-clean-cards-prod: ## Clean cards for a chat (prod). Usage: make ml-clean-cards-prod ML_ARGS="--chat-id -1003280306634 [--week 2024-12-16] [--force]"
	./scripts/ml-processor.sh --prod clean-cards $(ML_ARGS)

# =============================================================================
# ML DASHBOARD (ml-dashboard-*)
# =============================================================================
# Dev-only tool for exploring ML-processed data
# Backend: FastAPI (port 8052) | Frontend: React/Vite (port 5175)

ml-dashboard-up: ## Start ML Dashboard (backend + frontend)
	@$(DC) up -d ml-dashboard-backend ml-dashboard-frontend
	@echo ""
	@echo "ML Dashboard started:"
	@echo "  Frontend: http://localhost:5175"
	@echo "  Backend:  http://localhost:8052"
	@echo ""

ml-dashboard-up-build: ## Rebuild and start ML Dashboard
	@$(DC) up -d --build ml-dashboard-backend ml-dashboard-frontend
	@echo ""
	@echo "ML Dashboard started:"
	@echo "  Frontend: http://localhost:5175"
	@echo "  Backend:  http://localhost:8052"
	@echo ""

ml-dashboard-down: ## Stop ML Dashboard services
	@$(DC) stop ml-dashboard-backend ml-dashboard-frontend

ml-dashboard-logs: ## Tail logs from ML Dashboard services
	$(DC) logs -f ml-dashboard-backend ml-dashboard-frontend

ml-dashboard-logs-backend: ## Tail logs from ML Dashboard backend
	$(DC) logs -f ml-dashboard-backend

ml-dashboard-logs-frontend: ## Tail logs from ML Dashboard frontend
	$(DC) logs -f ml-dashboard-frontend

ml-dashboard-shell: ## Open shell in ML Dashboard backend container
	$(DC) exec ml-dashboard-backend /bin/bash

# =============================================================================
# MINIO CLIENT (mc-*)
# =============================================================================
mc-setup-prod: ## Configure MinIO Client alias for production
	@if [ ! -f $(TERRAFORM_DIR)/terraform.tfstate ]; then \
		echo "Error: No Terraform state found. Run 'make tf-apply' first."; \
		exit 1; \
	fi
	@echo "Configuring mc alias 'beef-briefing-prod'..."
	@ENDPOINT=$$($(MAKE) -s tf-object-storage-endpoint); \
	ACCESS_KEY=$$($(MAKE) -s tf-object-storage-access-key); \
	SECRET_KEY=$$($(MAKE) -s tf-object-storage-secret-key); \
	mc alias set beef-briefing-prod https://$$ENDPOINT $$ACCESS_KEY $$SECRET_KEY
	@echo "Alias 'beef-briefing-prod' configured successfully"
	@echo "Test with: mc ls beef-briefing-prod"

# =============================================================================
# PHONY TARGETS
# =============================================================================
.PHONY: help \
	up build deploy \
	dev-up dev-up-build dev-up-logs dev-down dev-restart dev-ps dev-clean dev-prune \
	prod-deploy prod-deploy-skip-build prod-deploy-skip-cleanup prod-deploy-regenerate-certs \
	prod-rollback prod-rollback-force prod-backup-db prod-clean-certs prod-logs-traefik prod-update-ip pg-tunnel \
	docker-build docker-build-api docker-build-bot \
	docker-logs docker-logs-api docker-logs-bot docker-logs-postgres docker-logs-minio \
	docker-logs-newrelic \
	docker-shell-api docker-shell-bot docker-shell-postgres docker-shell-minio \
	docker-shell-newrelic \
	go-build go-build-api go-build-bot go-build-import-cli go-build-import-cli-prod go-clean \
	go-fmt go-fmt-check \
	secrets-traefik-password secrets-service-api secrets-card-renderer secrets-jwt \
	tf-init tf-plan tf-apply tf-destroy tf-output tf-show tf-validate tf-refresh \
	tf-fmt tf-fmt-check tf-state-list tf-state-show tf-unlock \
	tf-ip tf-ssh tf-ssh-user-host tf-arch tf-root-pass \
	tf-object-storage-endpoint tf-object-storage-access-key tf-object-storage-secret-key tf-object-storage-bucket \
	tf-connect tf-setup tf-sync-object-storage tf-update-ssh-config tf-docs tf-deploy-check \
	ml-run ml-run-status ml-run-once ml-run-continuous ml-run-cards ml-run-render \
	ml-run-prod ml-run-status-prod ml-run-once-prod ml-run-continuous-prod ml-run-cards-prod ml-run-render-prod \
	ml-shell ml-clean-dev ml-clean-prod ml-clean-cards-dev ml-clean-cards-prod \
	ml-dashboard-up ml-dashboard-up-build ml-dashboard-down ml-dashboard-logs \
	ml-dashboard-logs-backend ml-dashboard-logs-frontend ml-dashboard-shell \
	mc-setup-prod
