# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go-based Telegram bot system for managing beef briefing subscriptions with REST API and PostgreSQL backend. Deployed on Linode with automatic SSL certificates via Traefik and Let's Encrypt.

**Technology Stack:**
- **Backend**: Go 1.25+
- **Database**: PostgreSQL 17 with PostGIS 3.4
- **Storage**: MinIO (dev) / Linode Object Storage (prod)
- **Reverse Proxy**: Traefik v3 with Let's Encrypt SSL
- **Frontend**: Flask, Jinja2, Tailwind CSS, ECharts (Dashboard)
- **Infrastructure**: Terraform (Linode), Docker Compose

## Architecture

### Services

The system consists of 3 main Go services:

1. **api-service** (port 8080): REST API for ingesting Telegram updates with media uploads
   - Handles multipart uploads with JSON metadata + binary files
   - Content-addressable storage using SHA256 hashing
   - Cross-table deduplication for media files

2. **telegram-bot**: Telegram bot that listens to group messages and forwards to API
   - Concurrent media downloads (max 5 simultaneous)
   - Smart photo handling (largest size only)
   - Exponential backoff retry logic
   - 100MB file size limit

3. **import-cli**: CLI tool to import Telegram Desktop exports
   - Streaming parser for large datasets (1M+ messages)
   - Resume support with state tracking
   - Handles group→supergroup migration

### Database Architecture

**22 tables modeling complete Telegram data structure:**

- **Core**: `chats`, `users`, `updates`
- **Messages**: `messages`, `message_entities`, `message_edits`
- **Reactions**: `message_reactions`, `reaction_counts` (denormalized - stores Telegram message ID, not FK)
- **Media**: `media_files`, `photos`, `stickers`, `games`, `game_photos`
- **Other**: `polls`, `poll_options`, `contacts`, `locations`, `venues`, `dice`

**Key Design Patterns:**
- Content-addressable storage: `file_hash` column enables deduplication across `media_files`, `photos`, `game_photos`
- MinIO path structure: `{mediaType}/{hash[:2]}/{hash}`
- Group migration tracking: `chats.migrated_from_chat_id` links old group ID to new supergroup ID
- Denormalized reactions: Allow storing reactions for messages not yet captured

### Network Architecture (Production)

```
Internet (443/80) → Traefik (SSL termination)
                         ├─→ Dashboard (/beef-dashboard)
                         └─→ Traefik Dashboard (/dashboard)

Internal Docker Network:
  ├─ API Service (8080) ←→ Telegram Bot
  ├─ Dashboard (8050)
  └─ PostgreSQL (5432)
```

Only Traefik exposes ports externally. All services communicate via `beef-prod-network`.

## Development Commands

### Docker Lifecycle

```bash
make up              # Start all services (dev environment)
make up-build        # Rebuild images and start
make down            # Stop all services
make logs            # Tail logs from all services
make logs-api        # Tail specific service logs
make logs-bot
make logs-dashboard
make clean           # Stop and remove volumes
```

### Building Go Services

```bash
# Build all Go binaries locally
make go-build

# Build specific service
make go-build-api
make go-build-bot
make go-build-import-cli

# Clean build artifacts
make go-clean
```

### Code Quality

```bash
make fmt              # Format all Go code with gofmt
make fmt-check        # Check if code is formatted
```

## Production Deployment

### Initial Setup

```bash
# 1. Configure environment
cp infrastructure/.env.prod.example infrastructure/.env.prod
# Edit .env.prod with your settings

# 2. Setup Terraform
make tf-setup          # Populates terraform.tfvars from .env.prod
make tf-init
make tf-plan
make tf-apply

# 3. Get server IP and configure DNS
make tf-ip            # Point domain A record to this IP

# 4. Generate secrets
make secrets-traefik-password
make secrets-dashboard
make secrets-analytics-api-key

# 5. Deploy
make deploy           # Full deployment
```

### Subsequent Deployments

```bash
make deploy                    # Standard deployment (rebuilds images)
make deploy-skip-build         # Use existing images (faster)
make deploy-skip-cleanup       # Keep old images for rollback
make rollback                  # Rollback to previous version
```

### SSL Certificates

Traefik automatically handles Let's Encrypt certificates:

- **Storage**: `infrastructure/letsencrypt/acme.json` (gitignored, 600 permissions)
- **Auto-renewal**: 60 days before expiration
- **Staging mode**: For testing without rate limits (see README.md)

```bash
# View Traefik logs
make logs-traefik COMPOSE_FILE=infrastructure/docker-compose.prod.yml

# Regenerate certificates (removes acme.json first)
make deploy-regenerate-certs

# Just remove certificates without deploying
make clean-letsencrypt-certs
```

### Terraform Commands

```bash
make tf-init              # Initialize Terraform
make tf-plan              # Show execution plan
make tf-apply             # Apply changes
make tf-ip                # Get server IP
make tf-ssh               # SSH to server
make tf-connect           # Alternative SSH command
make tf-deploy-check      # Pre-deployment validation
```

## Environment Configuration

### Development vs Production

- **Development**: `infrastructure/.env.dev` + `docker-compose.dev.yml`
  - Uses MinIO for object storage
  - No Traefik/SSL
  - Text logging

- **Production**: `infrastructure/.env.prod` + `docker-compose.prod.yml`
  - Uses Linode Object Storage
  - Traefik with Let's Encrypt SSL
  - JSON logging
  - Secure cookies enabled

### Secrets Management

**Traefik Dashboard**:
```bash
make secrets-traefik-password
```

Updates `TRAEFIK_DASHBOARD_USERS` in `.env.prod` with bcrypt hash ($$2y$$ escaping for docker-compose).

**Dashboard** (Flask secret key):
```bash
make secrets-dashboard
```

Generates Flask secret key at `infrastructure/secrets/apps/dashboard/flask_secret_key`.

**Analytics API Key**:
```bash
make secrets-analytics-api-key
```

Generates a secure random API key and saves it to `infrastructure/secrets/apps/api-service/analytics_api_key`. This key is automatically deployed to production when you run `make deploy`. The key is used for authenticating requests to the analytics endpoints.

## Import CLI Usage

Import Telegram Desktop exports into the system:

```bash
# Build and deploy to production server
make go-build-import-cli-prod

# SSH to server and run
ssh $(make tf-ssh-user-host)
cd ~/beef-briefing/apps/import-cli

# Basic import (requires chat ID)
./bin/import-cli import --chat-id -1003280306634 --export-path /path/to/export

# With media files
./bin/import-cli import --chat-id -1003280306634 --export-path /path/to/export --include-media

# With bot detection (skips bot messages by default)
./bin/import-cli import --chat-id -1003280306634 --export-path /path/to/export --telegram-token YOUR_BOT_TOKEN

# Include bot messages (disable bot filtering)
./bin/import-cli import --chat-id -1003280306634 --export-path /path/to/export --telegram-token YOUR_BOT_TOKEN --skip-bots=false

# Check status
./bin/import-cli status --export-path /path/to/export
```

**Bot Detection**: The import-cli can query the Telegram API to detect bot users and skip their messages during import. This is enabled by default (`--skip-bots=true`) but requires a Telegram bot token. The token can be provided via `--telegram-token` flag or `TELEGRAM_BOT_TOKEN` environment variable. User lookups are cached to minimize API calls. Detected bots and skipped message counts are tracked in the import state and displayed in status output.

**Group Migration**: When importing supergroup exports, the `result.json` contains the old group ID. You must provide the actual supergroup ID using `--chat-id`. The CLI validates the conversion formula: `supergroup_id = -1000000000000 - old_group_id`.

## Common Development Workflows

### Adding a New Telegram Message Type

1. Update database schema in `apps/api-service/internal/migrations/sql/`
2. Add corresponding structs in `apps/api-service/internal/models/telegram.go`
3. Update repository layer in `apps/api-service/internal/repository/`
4. Update service layer in `apps/api-service/internal/services/ingest_service.go`
5. Test with curl using multipart form data

### Adding New Environment Variables

1. Add to both `.env.dev.example` and `.env.prod.example`
2. Update docker-compose files if needed
3. Update Terraform variables in `infrastructure/terraform/variables.tf` if infrastructure-related
4. Update relevant Go service config structs
5. Document in service README.md

## Testing the System

### Local Development Testing

```bash
# Start all services
make up-build

# Test API health
curl http://localhost:8080/health

# Test message ingestion
curl -X POST http://localhost:8080/api/v1/ingest \
  -F 'update={"update_id":1,"message":{"message_id":1,"chat":{"id":-100123,"type":"supergroup"},"from":{"id":456,"first_name":"User"},"date":1733611200,"text":"Test"}}'

# Access dashboard
# Visit http://localhost:8050

# Test Telegram bot
# Send a message to your bot in Telegram
# Check logs: make logs-bot
```

## Project Structure

```
beef-briefing/
├── apps/
│   ├── api-service/       # REST API for Telegram data ingestion (includes embedded migrations)
│   ├── dashboard/         # Flask analytics dashboard
│   ├── telegram-bot/      # Telegram bot client
│   └── import-cli/        # CLI for importing Telegram exports
├── infrastructure/
│   ├── docker-compose.dev.yml     # Development environment
│   ├── docker-compose.prod.yml    # Production with Traefik
│   ├── terraform/                 # Linode infrastructure as code
│   ├── secrets/                   # Secrets directory (gitignored)
│   └── letsencrypt/               # SSL certificates (gitignored)
├── pkg/config/            # Shared configuration package
├── scripts/               # Deployment and utility scripts
├── Makefile              # Build automation
└── CLAUDE.md             # This file
```

## Important Notes

### Media Storage

- **Content-addressable**: Files stored by SHA256 hash
- **Deduplication**: Hash checked across multiple tables before upload
- **Path format**: `{mediaType}/{hash[:2]}/{hash}`
- **Development**: MinIO at localhost:9000
- **Production**: Linode Object Storage (credentials from Terraform)

### Telegram Bot Permissions

The bot must be **admin** in groups to receive:
- All message types
- Message reactions (`message_reaction`)
- Reaction counts (`message_reaction_count`)
- Chat member updates (`my_chat_member` for group migration detection)

### Database Migrations

Migrations are embedded in `apps/api-service/internal/migrations/sql/`. They run automatically when the API service starts, tracked via a `schema_migrations` table.

### Logging Standards

All Go services use `log/slog`:
- **Development**: Text format with DEBUG level
- **Production**: JSON format with INFO level
- Set via `ENVIRONMENT` and `LOG_LEVEL` environment variables

### Traefik Configuration

Production routing rules in `docker-compose.prod.yml`:
- Dashboard: `https://yourdomain.com/beef-dashboard`
- Traefik dashboard: `https://yourdomain.com/dashboard` (basic auth)
- All HTTP traffic redirects to HTTPS
- Let's Encrypt certificates stored in `infrastructure/letsencrypt/acme.json`

### Cross-Compilation

When building binaries for production server (may differ from dev architecture):
```bash
make go-build-import-cli-prod  # Auto-detects remote arch and cross-compiles
make tf-arch                   # Show remote architecture
```
