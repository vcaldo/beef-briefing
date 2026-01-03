# Beef Briefing

Go-based Telegram bot system for managing beef briefing subscriptions. Includes REST API, PostgreSQL backend, and Linode deployment with Traefik SSL.

## Quick Start (Development)

```bash
# 1. Configure environment
cp infrastructure/.env.dev.example infrastructure/.env.dev
# Edit .env.dev: set TELEGRAM_BOT_TOKEN (required)

# 2. Generate secrets
make secrets-analytics-api-key

# 3. Start services
make up-build

# 4. View logs
make logs
```

**Access:**
- API Service: http://localhost:8080
- MinIO Console: http://localhost:9001

## Mandatory Environment Variables

| Variable | Description | Source |
|----------|-------------|--------|
| `TELEGRAM_BOT_TOKEN` | Telegram Bot API token from @BotFather | `infrastructure/.env.dev` |
| `DB_PASSWORD` | PostgreSQL password | `infrastructure/.env.dev` |
| `ANALYTICS_API_KEY_FILE` | API key for analytics endpoints | `infrastructure/secrets/apps/api-service/analytics_api_key` |

**Production-only:**

| Variable | Description | Source |
|----------|-------------|--------|
| `DOMAIN_NAME` | Domain for SSL certificates | `infrastructure/.env.prod` |
| `LETSENCRYPT_EMAIL` | Email for Let's Encrypt notifications | `infrastructure/.env.prod` |
| `TRAEFIK_DASHBOARD_USERS` | Htpasswd credentials (bcrypt, `$$` escaped) | `infrastructure/.env.prod` |
| `LINODE_TOKEN` | Linode API token for Terraform | `infrastructure/.env.prod` |

## Secrets Generation

**Recommended workflow:** Use file-based secrets (`*_FILE` env vars).

| Secret | Make Target | Output Path |
|--------|-------------|-------------|
| Traefik dashboard password | `make secrets-traefik-password` | Updates `TRAEFIK_DASHBOARD_USERS` in `.env.prod` |
| Analytics API key | `make secrets-analytics-api-key` | `infrastructure/secrets/apps/api-service/analytics_api_key` |

## Service Documentation

| Service | Description |
|---------|-------------|
| [api-service](apps/api-service/README.md) | REST API for ingesting Telegram updates with media uploads |
| [telegram-bot](apps/telegram-bot/README.md) | Telegram bot client that forwards group messages to the API |
| [import-cli](apps/import-cli/README.md) | CLI tool to import Telegram Desktop exports |
| [card-image-generator](apps/card-image-generator/README.md) | Renders gamified user stats cards as PNG images using HTML/CSS templates |
| [ml-processor](apps/ml-processor/README.md) | ML pipeline for sentiment, humor, toxicity analysis |

## Infrastructure Documentation

| Document | Description |
|----------|-------------|
| [infrastructure/](infrastructure/README.md) | Docker Compose environments and deployment overview |
| [infrastructure/secrets/](infrastructure/secrets/README.md) | Secrets management and file-based storage |
| [infrastructure/terraform/](infrastructure/terraform/README.md) | Linode provisioning (instance, storage, firewall, DNS) |

## Commands Reference

Run `make help` for all available targets.

### Docker Lifecycle

```bash
make up              # Start dev services
make up-build        # Rebuild and start
make down            # Stop services
make logs            # Tail all logs
make clean           # Stop and remove volumes
```

### Deployment

```bash
make deploy          # Deploy to production
make deploy-skip-build   # Deploy without rebuilding
make rollback        # Rollback production
```

### Terraform

```bash
make tf-setup        # Setup terraform.tfvars from .env.prod
make tf-init         # Initialize Terraform
make tf-plan         # Preview changes
make tf-apply        # Apply changes
make tf-ip           # Get server IP
```

## Troubleshooting

- **Bot not receiving messages**: Ensure bot is admin in the group and privacy mode is disabled via @BotFather `/setprivacy`
- **SSL certificate not issued**: Check DNS points to server IP (`make tf-ip`) and port 80 is open
- **Missing `htpasswd`**: Install `apache2-utils` (Debian/Ubuntu) or `httpd-tools` (RHEL/CentOS)
- **Analytics API 401**: Generate API key with `make secrets-analytics-api-key`
