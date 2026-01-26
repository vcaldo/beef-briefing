# Infrastructure

This directory contains all infrastructure configuration for the Beef Briefing system, including Docker Compose environments, Terraform provisioning, and secrets management.

## Directory Structure

```
infrastructure/
├── docker-compose.dev.yml    # Development environment (with MinIO)
├── docker-compose.prod.yml   # Production environment (with Traefik SSL)
├── .env.dev.example          # Development environment template
├── .env.prod.example         # Production environment template
├── secrets/                  # Secret files (gitignored)
│   └── apps/
│       ├── api-service/      # Analytics API key
│       └── new-relic/        # New Relic configuration
├── terraform/                # Linode infrastructure as code
│   └── README.md             # Detailed Terraform documentation
└── apps/
    └── postgres/
        └── migrations/       # (Symlink/duplicate - use apps/postgres/migrations)
```

## Quick Start

### Development

```bash
# 1. Copy environment template
cp .env.dev.example .env.dev

# 2. Edit .env.dev and set required variables:
#    - TELEGRAM_BOT_TOKEN (required)
#    - DB_PASSWORD

# 3. Generate secrets (from repo root)
make secrets-service-api APP=telegram-bot
make secrets-card-renderer APP=ml-processor

# 4. Start services
make up-build
```

### Production

```bash
# 1. Copy environment template
cp .env.prod.example .env.prod

# 2. Edit .env.prod and set all required variables
#    See "Required Environment Variables" below

# 3. Setup Terraform
make tf-setup
make tf-init
make tf-apply

# 4. Configure DNS (point domain to server IP)
make tf-ip

# 5. Generate secrets
make secrets-traefik-password
make secrets-analytics-api-key

# 6. Deploy
make deploy
```

## Environment Files

| File | Purpose |
|------|---------|
| `.env.dev.example` | Template for development configuration |
| `.env.dev` | Actual development config (gitignored) |
| `.env.prod.example` | Template for production configuration |
| `.env.prod` | Actual production config (gitignored) |

## Required Environment Variables

### Development

| Variable | Description |
|----------|-------------|
| `TELEGRAM_BOT_TOKEN` | Telegram Bot API token from @BotFather |
| `DB_PASSWORD` | PostgreSQL password |

Secrets are loaded from files in `secrets/apps/`:
- `api-service/analytics_api_key`

### Production

All development variables plus:

| Variable | Description |
|----------|-------------|
| `DOMAIN_NAME` | Domain for SSL certificates |
| `LETSENCRYPT_EMAIL` | Email for Let's Encrypt notifications |
| `TRAEFIK_DASHBOARD_USERS` | Htpasswd credentials (bcrypt, `$$` escaped) |
| `LINODE_TOKEN` | Linode API token for Terraform |
| `MINIO_ENDPOINT` | Linode Object Storage endpoint |
| `MINIO_ACCESS_KEY` | Object Storage access key (from Terraform) |
| `MINIO_SECRET_KEY` | Object Storage secret key (from Terraform) |

## Docker Compose Services

### Development (`docker-compose.dev.yml`)

| Service | Port | Language | Description |
|---------|------|----------|-------------|
| postgres | 5432 | - | PostgreSQL 17 with PostGIS 3.4 |
| minio | 9000/9001 | - | S3-compatible object storage |
| qdrant | 6333/6334 | - | Vector database for ML embeddings |
| api-service | 8080 | Go | REST API for Telegram data ingestion |
| telegram-bot | - | Go | Telegram bot client |
| card-renderer | 8051 | Python | Card image renderer (Playwright) |
| ml-processor | - | Python | ML pipeline for message analysis |
| ml-dashboard-backend | 8052 | Python | ML Dashboard API |
| ml-dashboard-frontend | 3000 | React | ML Dashboard UI |
| deck-mini-app-dev | 5173 | React | Telegram Mini App for deck viewing |
| leaderboard-mini-app-dev | 5174 | React | Telegram Mini App for leaderboard |
| arena-mini-app-dev | 5175 | React | Telegram Mini App for card battles |

### Production (`docker-compose.prod.yml`)

| Service | Port | Language | Description |
|---------|------|----------|-------------|
| traefik | 80/443 | - | Reverse proxy with Let's Encrypt SSL |
| postgres | 5432 (internal) | - | PostgreSQL with PostGIS |
| api-service | 8080 (internal) | Go | REST API |
| telegram-bot | - | Go | Telegram bot client |
| card-renderer | 8051 (internal) | Python | Card image renderer |
| deck-mini-app | - | React | Telegram Mini App (deck viewing) |
| leaderboard-mini-app | - | React | Telegram Mini App (leaderboard) |
| arena-mini-app | - | React | Telegram Mini App (card battles) |

## Secrets Management

See [secrets/README.md](secrets/README.md) for detailed secrets documentation.

**Quick reference:**

```bash
# Generate Traefik dashboard password (production)
make secrets-traefik-password

# Generate API keys for services
make secrets-service-api APP=telegram-bot
make secrets-service-api APP=ml-processor
make secrets-card-renderer APP=ml-processor

# Generate JWT secret for Mini Apps
make secrets-jwt
```

## Terraform (Linode)

See [terraform/README.md](terraform/README.md) for detailed infrastructure documentation.

**Quick reference:**

```bash
make tf-setup     # Setup terraform.tfvars from .env.prod
make tf-init      # Initialize Terraform
make tf-plan      # Preview changes
make tf-apply     # Apply changes
make tf-ip        # Get server IP
make tf-ssh       # SSH connection command
```

## SSL Certificates

Production uses Traefik with automatic Let's Encrypt certificates.

- **Storage**: `letsencrypt/acme.json` (gitignored)
- **Auto-renewal**: 60 days before expiration
- **Challenge**: HTTP-01 (requires port 80 open)

### Endpoints

| URL | Service |
|-----|---------|
| `https://{domain}/dashboard` | Traefik Dashboard (basic auth) |

### Troubleshooting SSL

```bash
# View Traefik logs
make logs-traefik COMPOSE_FILE=infrastructure/docker-compose.prod.yml

# Check certificate expiration
openssl s_client -connect yourdomain.com:443 </dev/null 2>/dev/null | openssl x509 -noout -dates

# Force certificate regeneration
make deploy-regenerate-certs
```

## Network Architecture

```
Internet (443/80) --> Traefik (SSL termination)
                           |
                           +--> api.{domain} --> API Service (8080)
                           |       +--> /api/v1/mini-app/* (public, JWT auth)
                           |       +--> /api/v1/* (IP restricted)
                           +--> cards-api.{domain} --> Card Renderer (8051)
                           +--> deck.{domain} --> Deck Mini App
                           +--> leaderboard.{domain} --> Leaderboard Mini App
                           +--> arena.{domain} --> Arena Mini App
                           +--> {domain}/dashboard --> Traefik Dashboard

Internal Docker Network (beef-prod-network):
  +-- API Service (8080) <--> Telegram Bot
  +-- Card Renderer (8051)
  +-- PostgreSQL (5432) - NOT exposed externally
```

## Commands Reference

### Docker Lifecycle

| Command | Description |
|---------|-------------|
| `make up` | Start all dev services |
| `make up-build` | Rebuild images and start |
| `make down` | Stop all services |
| `make logs` | Tail logs from all services |
| `make logs-api` | Tail API service logs |
| `make clean` | Stop and remove volumes |

### Production Deployment

| Command | Description |
|---------|-------------|
| `make deploy` | Full production deployment |
| `make deploy-skip-build` | Deploy with existing images |
| `make rollback` | Rollback to previous version |
| `make prod-logs-traefik` | View Traefik logs |
| `make prod-update-ip` | Update IP allowlist |

### Database

| Command | Description |
|---------|-------------|
| `make pg-dev` | Connect to dev PostgreSQL |
| `make pg-tunnel` | Open SSH tunnel to prod database |
| `make pg-prod` | Connect to prod PostgreSQL (requires tunnel) |
| `make prod-backup-db` | Backup production database |

### Cache Management

| Command | Description |
|---------|-------------|
| `make layer-cache-health` | Check OCI cache health |
| `make layer-cache-stats` | Show cache statistics |
| `make layer-cache-clean-remote` | Clean remote cache |

## Troubleshooting

### Services not starting

**Symptoms**: Containers exit immediately or fail to start

**Solution**:
```bash
# Check logs for the failing service
make logs-api  # or logs-bot, etc.

# Verify secrets are generated
ls -la infrastructure/secrets/apps/

# Regenerate secrets if missing
make secrets-service-api APP=telegram-bot
```

### Database connection refused

**Symptoms**: API service cannot connect to PostgreSQL

**Solution**:
```bash
# Check if postgres is running
docker ps | grep postgres

# Check postgres logs
make logs-postgres

# Restart postgres
docker restart beef-postgres-dev
```

### SSL certificate issues (production)

**Symptoms**: HTTPS not working, certificate errors

**Solution**:
```bash
# View Traefik logs
make prod-logs-traefik

# Check certificate files
ssh admin@<server> 'ls -la ~/beef-briefing/infrastructure/letsencrypt/'

# Force regenerate certificates
make deploy-regenerate-certs
```

### MinIO/Object Storage connection failed

**Symptoms**: Media uploads fail, card images not rendering

**Solution**:
```bash
# Development: Check MinIO is running
docker ps | grep minio

# Production: Verify credentials in .env.prod
# Run Terraform sync to update credentials
make tf-sync-object-storage
```

## Related Documentation

- [Terraform Infrastructure](terraform/README.md) - Linode provisioning details
- [Secrets Management](secrets/README.md) - Secret generation and storage
- [API Service](../apps/api-service/README.md) - REST API documentation
- [Card Renderer](../apps/card-renderer/README.md) - Card image renderer
- [Telegram Bot](../apps/telegram-bot/README.md) - Bot configuration
- [ML Processor](../apps/ml-processor/README.md) - ML pipeline for analytics
