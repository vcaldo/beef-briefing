# Variables
COMPOSE_FILE ?= infrastructure/docker-compose.dev.yml
ENV_FILE ?= infrastructure/.env.dev
PROD_COMPOSE_FILE := infrastructure/docker-compose.prod.yml
PROD_ENV_FILE := infrastructure/.env.prod
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

logs-traefik: ## Tail logs from traefik (production only)
	@if [ "$(COMPOSE_FILE)" = "$(PROD_COMPOSE_FILE)" ]; then \
		SSH_HOST=$$($(MAKE) -s tf-ssh-user-host); \
		ssh $$SSH_HOST 'cd ~/beef-briefing && docker compose logs -f traefik'; \
	else \
		echo "Error: logs-traefik only available for production environment"; \
		echo "Use: make logs-traefik COMPOSE_FILE=$(PROD_COMPOSE_FILE)"; \
	fi

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

go-build-import-cli-prod: ## Build import-cli for remote architecture and deploy to production server
	@echo "Building import-cli for remote server..."
	@SSH_HOST=$$($(MAKE) -s tf-ssh-user-host); \
	REMOTE_ARCH_RAW=$$(ssh $$SSH_HOST 'uname -m' 2>/dev/null || echo "x86_64"); \
	case "$$REMOTE_ARCH_RAW" in \
		x86_64) REMOTE_ARCH="amd64" ;; \
		aarch64) REMOTE_ARCH="arm64" ;; \
		armv7l) REMOTE_ARCH="arm" ;; \
		*) REMOTE_ARCH="amd64" ;; \
	esac; \
	HOST_ARCH=$$(go env GOHOSTARCH); \
	echo "Remote architecture: $$REMOTE_ARCH_RAW (GOARCH: $$REMOTE_ARCH)"; \
	if [ "$$HOST_ARCH" != "$$REMOTE_ARCH" ]; then \
		echo "Cross-compiling from $$HOST_ARCH to $$REMOTE_ARCH"; \
	fi; \
	cd $(IMPORT_CLI_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=$$REMOTE_ARCH go build -a -installsuffix cgo -o bin/import-cli ./cmd
	@echo "Creating remote directory..."
	@ssh $$($(MAKE) -s tf-ssh-user-host) 'mkdir -p ~/beef-briefing/apps/import-cli/bin'
	@echo "Copying binary to production server..."
	@scp $(IMPORT_CLI_DIR)/bin/import-cli $$($(MAKE) -s tf-ssh-user-host):~/beef-briefing/apps/import-cli/bin/
	@ssh $$($(MAKE) -s tf-ssh-user-host) 'chmod +x ~/beef-briefing/apps/import-cli/bin/import-cli'
	@echo "✓ Binary deployed to $$($(MAKE) -s tf-ssh-user-host):~/beef-briefing/apps/import-cli/bin/import-cli"

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
	@cd $(ADMIN_PANEL_DIR)/tools && go run update_secrets.go -mode=files -secrets-dir ../../../infrastructure/secrets/apps/admin-panel

admin-panel-set-password-file: ## Generate password hash and write to file
	@cd $(ADMIN_PANEL_DIR)/tools && go run update_secrets.go -mode=files -secrets-dir ../../../infrastructure/secrets/apps/admin-panel -password-only

admin-panel-set-session-file: ## Generate session secret and write to file
	@cd $(ADMIN_PANEL_DIR)/tools && go run update_secrets.go -mode=files -secrets-dir ../../../infrastructure/secrets/apps/admin-panel -session-only

# Traefik password generation
generate-traefik-password: ## Generate Traefik dashboard password and update .env.prod
	@chmod +x scripts/generate-traefik-password.sh
	@scripts/generate-traefik-password.sh

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

tf-arch: ## Show instance CPU architecture for cross-compilation
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

tf-object-storage-secret-key: ## Show Object Storage secret access key (SENSITIVE)
	@cd $(TERRAFORM_DIR) && terraform output -raw object_storage_secret_access_key

tf-object-storage-bucket: ## Show Object Storage bucket name
	@cd $(TERRAFORM_DIR) && terraform output -raw object_storage_bucket_name

# Terraform utilities
tf-connect: ## SSH to the Linode instance
	@ssh admin@$$(cd $(TERRAFORM_DIR) && terraform output -raw instance_ip)

tf-setup: ## Setup Terraform configuration (copy tfvars example and populate from .env)
	@if [ ! -f $(TERRAFORM_DIR)/terraform.tfvars ]; then \
		cp $(TERRAFORM_DIR)/terraform.tfvars.example $(TERRAFORM_DIR)/terraform.tfvars; \
		echo "Created terraform.tfvars from example."; \
		if [ -f $(PROD_ENV_FILE) ]; then \
			ENV_FILE=$(PROD_ENV_FILE); \
		elif [ -f $(ENV_FILE) ]; then \
			ENV_FILE=$(ENV_FILE); \
		else \
			echo "Warning: No .env.prod or .env.dev found. Please edit terraform.tfvars manually."; \
			exit 0; \
		fi; \
		LINODE_TOKEN=$$(grep '^LINODE_TOKEN=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		LINODE_REGION=$$(grep '^LINODE_REGION=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		LINODE_INSTANCE_TYPE=$$(grep '^LINODE_INSTANCE_TYPE=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		LINODE_HOSTNAME=$$(grep '^LINODE_HOSTNAME=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		DOMAIN_NAME=$$(grep '^DOMAIN_NAME=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		ADMIN_PANEL_DOMAIN_SUFFIX=$$(grep '^ADMIN_PANEL_DOMAIN_SUFFIX=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		NEW_RELIC_KEY=$$(grep '^NEW_RELIC_LICENSE_KEY=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		NEW_RELIC_ACCOUNT=$$(grep '^NEW_RELIC_ACCOUNT_ID=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		SSH_KEY_PATH=$$(grep '^SSH_PUBLIC_KEY_PATH=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		if [ -n "$$LINODE_TOKEN" ]; then \
			sed -i "s|linode_token = \".*\"|linode_token = \"$$LINODE_TOKEN\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated linode_token from $$ENV_FILE"; \
		fi; \
		if [ -n "$$LINODE_REGION" ]; then \
			sed -i "s|# region = \".*\"|region = \"$$LINODE_REGION\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated region from $$ENV_FILE"; \
		fi; \
		if [ -n "$$LINODE_INSTANCE_TYPE" ]; then \
			sed -i "s|# instance_type = \".*\"|instance_type = \"$$LINODE_INSTANCE_TYPE\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated instance_type from $$ENV_FILE"; \
		fi; \
		if [ -n "$$LINODE_HOSTNAME" ]; then \
			sed -i "s|# hostname = \".*\"|hostname = \"$$LINODE_HOSTNAME\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated hostname from $$ENV_FILE"; \
		fi; \
		if [ -n "$$DOMAIN_NAME" ]; then \
			sed -i "s|# domain_name = \".*\"|domain_name = \"$$DOMAIN_NAME\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated domain_name from $$ENV_FILE"; \
			sed -i "s|# domain_email = \".*\"|domain_email = \"admin@$$DOMAIN_NAME\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated domain_email as admin@$$DOMAIN_NAME"; \
		fi; \
		if [ -n "$$ADMIN_PANEL_DOMAIN_SUFFIX" ]; then \
			sed -i "s|# admin_panel_domain_suffix = \".*\"|admin_panel_domain_suffix = \"$$ADMIN_PANEL_DOMAIN_SUFFIX\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated admin_panel_domain_suffix from $$ENV_FILE"; \
		fi; \
		if [ -n "$$NEW_RELIC_KEY" ]; then \
			sed -i "s|# new_relic_license_key = \".*\"|new_relic_license_key = \"$$NEW_RELIC_KEY\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated new_relic_license_key from $$ENV_FILE"; \
		fi; \
		if [ -n "$$NEW_RELIC_ACCOUNT" ]; then \
			sed -i "s|# new_relic_account_id = \".*\"|new_relic_account_id = \"$$NEW_RELIC_ACCOUNT\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated new_relic_account_id from $$ENV_FILE"; \
		fi; \
		NEW_RELIC_REGION=$$(grep '^NEW_RELIC_REGION=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		if [ -n "$$NEW_RELIC_REGION" ]; then \
			sed -i "s|# new_relic_region = \".*\"|new_relic_region = \"$$NEW_RELIC_REGION\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated new_relic_region from $$ENV_FILE"; \
		fi; \
		if [ -n "$$SSH_KEY_PATH" ]; then \
			sed -i "s|# ssh_public_key_path = \".*\"|ssh_public_key_path = \"$$SSH_KEY_PATH\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated ssh_public_key_path from $$ENV_FILE"; \
		fi; \
		POSTGRES_VOLUME_SIZE=$$(grep '^POSTGRES_VOLUME_SIZE=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		if [ -n "$$POSTGRES_VOLUME_SIZE" ]; then \
			sed -i "s|# postgres_volume_size = .*|postgres_volume_size = $$POSTGRES_VOLUME_SIZE|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated postgres_volume_size from $$ENV_FILE"; \
		fi; \
		POSTGRES_DATA_PATH=$$(grep '^POSTGRES_DATA_PATH=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		if [ -n "$$POSTGRES_DATA_PATH" ]; then \
			sed -i "s|# postgres_volume_mount_path = \".*\"|postgres_volume_mount_path = \"$$POSTGRES_DATA_PATH\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated postgres_volume_mount_path from $$ENV_FILE"; \
		fi; \
		OBJECT_STORAGE_REGION=$$(grep '^OBJECT_STORAGE_REGION=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		if [ -n "$$OBJECT_STORAGE_REGION" ]; then \
			sed -i "s|# object_storage_region = \".*\"|object_storage_region = \"$$OBJECT_STORAGE_REGION\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated object_storage_region from $$ENV_FILE"; \
		fi; \
		OBJECT_STORAGE_BUCKET_SUFFIX=$$(grep '^OBJECT_STORAGE_BUCKET_SUFFIX=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		if [ -n "$$OBJECT_STORAGE_BUCKET_SUFFIX" ]; then \
			sed -i "s|# object_storage_bucket_suffix = \".*\"|object_storage_bucket_suffix = \"$$OBJECT_STORAGE_BUCKET_SUFFIX\"|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated object_storage_bucket_suffix from $$ENV_FILE"; \
		fi; \
		OBJECT_STORAGE_VERSIONING=$$(grep '^OBJECT_STORAGE_VERSIONING=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		if [ -n "$$OBJECT_STORAGE_VERSIONING" ]; then \
			sed -i "s|# object_storage_versioning = .*|object_storage_versioning = $$OBJECT_STORAGE_VERSIONING|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated object_storage_versioning from $$ENV_FILE"; \
		fi; \
		OBJECT_STORAGE_LIFECYCLE_DAYS=$$(grep '^OBJECT_STORAGE_LIFECYCLE_DAYS=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		if [ -n "$$OBJECT_STORAGE_LIFECYCLE_DAYS" ]; then \
			sed -i "s|# object_storage_lifecycle_expiration_days = .*|object_storage_lifecycle_expiration_days = $$OBJECT_STORAGE_LIFECYCLE_DAYS|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated object_storage_lifecycle_expiration_days from $$ENV_FILE"; \
		fi; \
		OBJECT_STORAGE_VERSION_RETENTION_DAYS=$$(grep '^OBJECT_STORAGE_VERSION_RETENTION_DAYS=' $$ENV_FILE | cut -d'=' -f2 | tr -d '\n\r'); \
		if [ -n "$$OBJECT_STORAGE_VERSION_RETENTION_DAYS" ]; then \
			sed -i "s|# object_storage_noncurrent_version_expiration_days = .*|object_storage_noncurrent_version_expiration_days = $$OBJECT_STORAGE_VERSION_RETENTION_DAYS|" $(TERRAFORM_DIR)/terraform.tfvars; \
			echo "✓ Populated object_storage_noncurrent_version_expiration_days from $$ENV_FILE"; \
		fi; \
		echo "Setup complete! Review $(TERRAFORM_DIR)/terraform.tfvars before applying."; \
	else \
		echo "terraform.tfvars already exists. Delete it first to recreate."; \
	fi

tf-sync-object-storage-env: ## Sync Object Storage credentials from Terraform to .env.prod
	@if [ ! -f $(TERRAFORM_DIR)/terraform.tfstate ]; then \
		echo "Error: No Terraform state found. Run 'make tf-apply' first."; \
		exit 1; \
	fi
	@echo "Syncing Object Storage credentials to $(PROD_ENV_FILE)..."
	@ENDPOINT=$$($(MAKE) -s tf-object-storage-endpoint); \
	ACCESS_KEY=$$($(MAKE) -s tf-object-storage-access-key); \
	SECRET_KEY=$$($(MAKE) -s tf-object-storage-secret-key); \
	BUCKET_NAME=$$($(MAKE) -s tf-object-storage-bucket); \
	ENDPOINT_ESCAPED=$$(printf '%s\n' "$$ENDPOINT" | sed 's/[&/\]/\\&/g'); \
	ACCESS_KEY_ESCAPED=$$(printf '%s\n' "$$ACCESS_KEY" | sed 's/[&/\]/\\&/g'); \
	SECRET_KEY_ESCAPED=$$(printf '%s\n' "$$SECRET_KEY" | sed 's/[&/\]/\\&/g'); \
	BUCKET_NAME_ESCAPED=$$(printf '%s\n' "$$BUCKET_NAME" | sed 's/[&/\]/\\&/g'); \
	if grep -q '^MINIO_ENDPOINT=' $(PROD_ENV_FILE); then \
		sed -i "s|^MINIO_ENDPOINT=.*|MINIO_ENDPOINT=$$ENDPOINT_ESCAPED|" $(PROD_ENV_FILE); \
	else \
		echo "MINIO_ENDPOINT=$$ENDPOINT" >> $(PROD_ENV_FILE); \
	fi; \
	if grep -q '^MINIO_ACCESS_KEY=' $(PROD_ENV_FILE); then \
		sed -i "s|^MINIO_ACCESS_KEY=.*|MINIO_ACCESS_KEY=$$ACCESS_KEY_ESCAPED|" $(PROD_ENV_FILE); \
	else \
		echo "MINIO_ACCESS_KEY=$$ACCESS_KEY" >> $(PROD_ENV_FILE); \
	fi; \
	if grep -q '^MINIO_SECRET_KEY=' $(PROD_ENV_FILE); then \
		sed -i "s|^MINIO_SECRET_KEY=.*|MINIO_SECRET_KEY=$$SECRET_KEY_ESCAPED|" $(PROD_ENV_FILE); \
	else \
		echo "MINIO_SECRET_KEY=$$SECRET_KEY" >> $(PROD_ENV_FILE); \
	fi; \
	if grep -q '^MINIO_USE_SSL=' $(PROD_ENV_FILE); then \
		sed -i "s|^MINIO_USE_SSL=.*|MINIO_USE_SSL=true|" $(PROD_ENV_FILE); \
	else \
		echo "MINIO_USE_SSL=true" >> $(PROD_ENV_FILE); \
	fi; \
	if grep -q '^MINIO_BUCKET=' $(PROD_ENV_FILE); then \
		sed -i "s|^MINIO_BUCKET=.*|MINIO_BUCKET=$$BUCKET_NAME_ESCAPED|" $(PROD_ENV_FILE); \
	else \
		echo "MINIO_BUCKET=$$BUCKET_NAME" >> $(PROD_ENV_FILE); \
	fi; \
	echo "✓ Updated MINIO_ENDPOINT=$$ENDPOINT"; \
	echo "✓ Updated MINIO_ACCESS_KEY"; \
	echo "✓ Updated MINIO_SECRET_KEY"; \
	echo "✓ Updated MINIO_USE_SSL=true"; \
	echo "✓ Updated MINIO_BUCKET=$$BUCKET_NAME"; \
	echo "Sync complete!"

mc-setup-prod: ## Configure MinIO Client (mc) alias 'beef-briefing-prod' from Terraform outputs
	@if [ ! -f $(TERRAFORM_DIR)/terraform.tfstate ]; then \
		echo "Error: No Terraform state found. Run 'make tf-apply' first."; \
		exit 1; \
	fi
	@echo "Configuring mc alias 'beef-briefing-prod'..."
	@ENDPOINT=$$($(MAKE) -s tf-object-storage-endpoint); \
	ACCESS_KEY=$$($(MAKE) -s tf-object-storage-access-key); \
	SECRET_KEY=$$($(MAKE) -s tf-object-storage-secret-key); \
	mc alias set beef-briefing-prod https://$$ENDPOINT $$ACCESS_KEY $$SECRET_KEY
	@echo "✓ Alias 'beef-briefing-prod' configured successfully"
	@echo "Test with: mc ls beef-briefing-prod"

tf-docs: ## Show Terraform documentation
	@cat $(TERRAFORM_DIR)/README.md

tf-deploy-check: tf-validate tf-fmt-check tf-plan ## Full pre-deployment check (validate, format check, plan)

# Production deployment targets
COMMIT_HASH ?= $(shell git rev-parse --short HEAD)

deploy: tf-sync-object-storage-env ## Deploy to production server with commit-tagged images
	@./scripts/deploy.sh

deploy-skip-build: tf-sync-object-storage-env ## Deploy using existing images (skip build step)
	@./scripts/deploy.sh --skip-build

deploy-skip-cleanup: tf-sync-object-storage-env ## Deploy without cleaning up old images
	@./scripts/deploy.sh --skip-cleanup

deploy-regenerate-certs: ## Deploy with fresh Let's Encrypt certificates (removes acme.json first)
	@echo "Removing Let's Encrypt certificates on remote server..."
	@ssh $(shell $(MAKE) -s tf-ssh-user-host) 'rm -f ~/beef-briefing/infrastructure/letsencrypt/acme.json'
	@echo "✓ Certificates removed, deploying with fresh certificates..."
	@$(MAKE) deploy

clean-letsencrypt-certs: ## Remove Let's Encrypt certificates on remote server (without deploying)
	@echo "Removing Let's Encrypt certificates on remote server..."
	@ssh $(shell $(MAKE) -s tf-ssh-user-host) 'rm -f ~/beef-briefing/infrastructure/letsencrypt/acme.json'
	@echo "✓ Certificates removed"

rollback: ## Rollback to previous deployment
	@./scripts/rollback.sh

rollback-force: ## Rollback to previous deployment (skip confirmation)
	@./scripts/rollback.sh --force

# Phony targets
.PHONY: help up down restart ps clean prune build build-api build-bot build-postgres build-admin-panel \
	logs logs-api logs-bot logs-postgres logs-minio logs-admin-panel logs-newrelic logs-traefik \
	shell-api shell-bot shell-postgres shell-minio shell-admin-panel shell-newrelic \
	go-build-api go-build-bot go-build-admin-panel go-build-import-cli go-build go-clean fmt fmt-check \
	admin-panel-set-secrets admin-panel-set-password admin-panel-set-session \
	admin-panel-set-secrets-files admin-panel-set-password-file admin-panel-set-session-file \
	generate-traefik-password \
	tf-init tf-plan tf-apply tf-destroy tf-output tf-show tf-validate tf-refresh \
	tf-fmt tf-fmt-check tf-state-list tf-state-show tf-unlock \
	tf-ip tf-ssh tf-ssh-user-host tf-root-pass tf-object-storage-endpoint tf-object-storage-access-key tf-object-storage-secret-key tf-object-storage-bucket \
	tf-connect tf-setup tf-sync-object-storage-env mc-setup-prod tf-docs tf-deploy-check \
	deploy deploy-skip-build deploy-skip-cleanup deploy-regenerate-certs clean-letsencrypt-certs rollback rollback-force
