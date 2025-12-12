# beef-briefing

A Go-based Telegram bot system for managing beef briefing subscriptions with admin panel, REST API, and PostgreSQL backend. Four Go services (API, bot, admin panel, import CLI) communicate via Docker networks, with Traefik handling SSL termination in production.

## Quick Start

### Development

```bash
# 1. Configure environment
cp infrastructure/.env.dev.example infrastructure/.env.dev

# 2. Generate secrets
make admin-panel-set-secrets-files

# 3. Start services
make up-build

# 4. View logs
make logs
```

**Access:**
- Admin Panel: http://localhost:8081
- API Service: http://localhost:8080
- MinIO Console: http://localhost:9001

### Production

```bash
# 1. Configure environment
cp infrastructure/.env.prod.example infrastructure/.env.prod
# Edit with your values (see MANDATORY_ENV_VARS.md)

# 2. Setup and deploy infrastructure
make tf-setup && make tf-init && make tf-apply

# 3. Configure DNS (point domain A record to server IP)
make tf-ip

# 4. Generate secrets
make admin-panel-set-secrets-files ENV_FILE=infrastructure/.env.prod
make generate-traefik-password
make generate-analytics-api-key

# 5. Deploy
make deploy
```

## Environment Variables

**Required for all environments:**
- `TELEGRAM_BOT_TOKEN` - Bot token from BotFather (`infrastructure/.env.*`)

**Required for production:**
- `DOMAIN_NAME` - Domain for SSL certificates (`infrastructure/.env.prod`)
- `LETSENCRYPT_EMAIL` - Email for Let's Encrypt notifications (`infrastructure/.env.prod`)
- `LINODE_TOKEN` - Linode API token for Terraform (`infrastructure/.env.prod`)
- `DB_PASSWORD` - PostgreSQL password (`infrastructure/.env.prod`)

**Secrets (file-based, recommended):**
- `ADMIN_PASSWORD_HASH_FILE` - Path to bcrypt hash file (generate with `make admin-panel-set-secrets-files`)
- `SESSION_SECRET_FILE` - Path to session secret file (generate with `make admin-panel-set-secrets-files`)
- `ANALYTICS_API_KEY_FILE` - Path to API key file (generate with `make generate-analytics-api-key`)
- `TRAEFIK_DASHBOARD_USERS` - Htpasswd credentials (generate with `make generate-traefik-password`)

See [MANDATORY_ENV_VARS.md](./MANDATORY_ENV_VARS.md) for complete reference.

## Secrets Management

File-based secrets are stored in `infrastructure/secrets/` (gitignored):

| Secret | Make Target | Output Location |
|--------|-------------|-----------------|
| Admin password hash | `make admin-panel-set-secrets-files` | `infrastructure/secrets/apps/admin-panel/admin_password_hash` |
| Session secret | `make admin-panel-set-secrets-files` | `infrastructure/secrets/apps/admin-panel/session_secret` |
| Analytics API key | `make generate-analytics-api-key` | `infrastructure/secrets/apps/api-service/analytics_api_key` |
| Traefik password | `make generate-traefik-password` | Updates `TRAEFIK_DASHBOARD_USERS` in `.env.prod` |

See [SECRETS_GENERATION.md](./SECRETS_GENERATION.md) for detailed instructions.

## Common Commands

| Command | Description |
|---------|-------------|
| `make up` | Start all services |
| `make up-build` | Rebuild images and start |
| `make down` | Stop all services |
| `make logs` | Tail logs from all services |
| `make logs-<service>` | Tail logs from specific service (api, bot, admin-panel, postgres) |
| `make deploy` | Deploy to production |
| `make rollback` | Rollback to previous deployment |
| `make tf-ssh` | SSH into production server |

Run `make help` for full command reference.

## Documentation

### Services
- [apps/api-service/README.md](apps/api-service/README.md) - REST API for Telegram data ingestion with content-addressable storage
- [apps/admin-panel/README.md](apps/admin-panel/README.md) - Web admin interface with session auth, 5 themes, and activity visualization
- [apps/telegram-bot/README.md](apps/telegram-bot/README.md) - Telegram bot with concurrent media downloads and retry logic
- [apps/import-cli/README.md](apps/import-cli/README.md) - CLI for importing Telegram Desktop exports with streaming parser

### Infrastructure
- [infrastructure/terraform/README.md](infrastructure/terraform/README.md) - Terraform configuration for Linode (compute, storage, firewall, DNS)

### Tools
- [apps/admin-panel/tools/README.md](apps/admin-panel/tools/README.md) - Secret generation tools documentation

## Troubleshooting

**SSL certificate not issued:**
1. Verify DNS points to server: `dig yourdomain.com`
2. Check Traefik logs: `make logs-traefik COMPOSE_FILE=infrastructure/docker-compose.prod.yml`
3. Ensure `LETSENCRYPT_EMAIL` is set correctly

**Admin panel login fails:**
1. Regenerate secrets: `make admin-panel-set-secrets-files ENV_FILE=infrastructure/.env.prod`
2. Redeploy: `make deploy`

**Traefik dashboard 401 error:**
1. Regenerate password: `make generate-traefik-password`
2. Verify `$$2y$$` escaping in `.env.prod`
3. Redeploy: `make deploy`
