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
│       ├── admin-panel/      # Admin password hash & session secret
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
make admin-panel-set-secrets-files
make generate-analytics-api-key

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
make admin-panel-set-secrets-files
make generate-traefik-password
make generate-analytics-api-key

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
- `admin-panel/admin_password_hash`
- `admin-panel/session_secret`
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

| Service | Port | Description |
|---------|------|-------------|
| postgres | 5432 | PostgreSQL 17 with PostGIS 3.4 |
| minio | 9000/9001 | S3-compatible object storage |
| api-service | 8080 | REST API for Telegram data ingestion |
| telegram-bot | - | Telegram bot client |
| admin-panel | 8081 | Web admin interface |

### Production (`docker-compose.prod.yml`)

| Service | Port | Description |
|---------|------|-------------|
| traefik | 80/443 | Reverse proxy with Let's Encrypt SSL |
| postgres | 5432 (internal) | PostgreSQL with PostGIS |
| api-service | 8080 (internal) | REST API |
| telegram-bot | - | Telegram bot client |
| admin-panel | 8081 (internal) | Admin interface (via Traefik at `/admin`) |
| newrelic-infra | - | Infrastructure monitoring (optional) |

## Secrets Management

See [secrets/README.md](secrets/README.md) for detailed secrets documentation.

**Quick reference:**

```bash
# Generate all admin panel secrets
make admin-panel-set-secrets-files

# Generate Traefik dashboard password (production)
make generate-traefik-password

# Generate analytics API key
make generate-analytics-api-key
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
| `https://{domain}/admin` | Admin Panel |
| `https://{domain}/traefik-dashboard` | Traefik Dashboard (basic auth) |

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
                           +--> Admin Panel (/admin)
                           +--> Traefik Dashboard (/traefik-dashboard)

Internal Docker Network (beef-prod-network):
  +-- API Service (8080) <--> Telegram Bot
  +-- Admin Panel (8081)
  +-- PostgreSQL (5432) - NOT exposed externally
```

## Related Documentation

- [Terraform Infrastructure](terraform/README.md) - Linode provisioning details
- [Secrets Management](secrets/README.md) - Secret generation and storage
- [API Service](../apps/api-service/README.md) - REST API documentation
- [Admin Panel](../apps/admin-panel/README.md) - Web interface documentation
- [Telegram Bot](../apps/telegram-bot/README.md) - Bot configuration
