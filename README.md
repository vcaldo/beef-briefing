# beef-briefing

A Go-based Telegram bot for managing beef briefing subscriptions with admin panel, REST API, and PostgreSQL backend. Deployed on Linode with automatic SSL certificates via Traefik and Let's Encrypt.

## Table of Contents

- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Development Setup](#development-setup)
- [Production Deployment](#production-deployment)
- [SSL Certificates & Reverse Proxy](#ssl-certificates--reverse-proxy)
- [Makefile Commands](#makefile-commands)
- [Project Structure](#project-structure)

## Architecture

### Services

**Production Stack:**
- **traefik**: Reverse proxy with automatic SSL (Let's Encrypt)
- **admin-panel**: Web-based admin interface (Go + templ)
- **api-service**: REST API for content management (Go)
- **telegram-bot**: Telegram bot client (Go)
- **postgres**: PostgreSQL 17 with PostGIS 3.4
- **newrelic-infra**: New Relic monitoring (optional)

**Development Only:**
- **minio**: S3-compatible object storage (development only)

### Network Architecture

```
Internet (443/80) → Traefik (SSL termination)
                         ├─→ Admin Panel (/admin)
                         └─→ Traefik Dashboard (/traefik-dashboard)

Internal Docker Network:
  ├─ API Service (8080) ←→ Telegram Bot
  ├─ Admin Panel (8081)
  └─ PostgreSQL (5432)
```

All services communicate via a single bridge network (`beef-prod-network`). Only Traefik exposes ports externally (80/443).

## Prerequisites

### Development
- Docker & Docker Compose
- Go 1.25+ (for local builds)
- Make (for automation)
- templ CLI (for admin panel templates)

### Production Deployment
- Linode account with API token
- Domain name with DNS control
- Terraform 1.0+
- SSH key pair
- `htpasswd` utility (apache2-utils on Debian/Ubuntu)

## Development Setup

### 1. Clone Repository

```bash
git clone <repository-url>
cd beef-briefing
```

### 2. Configure Environment

```bash
cp infrastructure/.env.dev.example infrastructure/.env.dev
# Edit .env.dev with your configuration
```

### 3. Generate Admin Secrets

```bash
make admin-panel-set-secrets-files ENV_FILE=infrastructure/.env.dev
```

### 4. Start Services

```bash
# Start all services
make up

# Or build and start
make up-build

# View logs
make logs
```

### 5. Access Services

- **Admin Panel**: http://localhost:8081
- **API Service**: http://localhost:8080
- **MinIO Console**: http://localhost:9001 (admin/minioadmin)

## Production Deployment

### Initial Setup

#### 1. Configure Terraform Variables

```bash
# Copy environment template
cp infrastructure/.env.prod.example infrastructure/.env.prod

# Edit .env.prod and set:
# - LINODE_TOKEN
# - DOMAIN_NAME
# - LETSENCRYPT_EMAIL
# - Database credentials
# - Telegram bot token
# - etc.

# Setup Terraform variables from .env
make tf-setup
```

#### 2. Deploy Infrastructure

```bash
# Initialize Terraform
make tf-init

# Review deployment plan
make tf-plan

# Deploy infrastructure (Linode instance, firewall, block storage, object storage)
make tf-apply
```

#### 3. Configure DNS

Point your domain's A record to the Linode IP:

```bash
# Get the IP address
make tf-ip

# Configure DNS A record:
# yourdomain.com → <linode-ip>
```

Verify DNS propagation:

```bash
dig +short yourdomain.com
```

#### 4. Generate Secrets

```bash
# Admin panel password and session secret
make admin-panel-set-secrets-files ENV_FILE=infrastructure/.env.prod

# Traefik dashboard password
make generate-traefik-password
```

#### 5. Deploy Application

```bash
# Full deployment (build, transfer, start services)
make deploy

# Monitor deployment
make logs-traefik COMPOSE_FILE=infrastructure/docker-compose.prod.yml
```

Initial deployment takes 2-5 minutes for Let's Encrypt certificate generation.

### Subsequent Deployments

```bash
# Standard deployment (rebuilds images)
make deploy

# Skip build (use existing images - faster)
make deploy-skip-build

# Skip cleanup (keep old images - useful for rollback)
make deploy-skip-cleanup
```

### Rollback

```bash
# Rollback to previous deployment (with confirmation)
make rollback

# Force rollback (no confirmation)
make rollback-force
```

## SSL Certificates & Reverse Proxy

The production environment uses **Traefik v3** as a reverse proxy with automatic Let's Encrypt SSL certificates.

### Exposed Endpoints

| URL | Service | Authentication | Description |
|-----|---------|----------------|-------------|
| `https://yourdomain.com/admin` | Admin Panel | Session-based | Main admin interface |
| `https://yourdomain.com/traefik-dashboard` | Traefik Dashboard | Basic Auth | Traefik monitoring dashboard |

**Note:** All HTTP (port 80) traffic is automatically redirected to HTTPS (port 443).

### Certificate Management

#### Automatic Renewal

Traefik automatically renews Let's Encrypt certificates 60 days before expiration. No manual intervention required.

#### Monitor Certificate Status

```bash
# View Traefik logs
make logs-traefik COMPOSE_FILE=infrastructure/docker-compose.prod.yml

# Check certificate expiration
openssl s_client -connect yourdomain.com:443 -servername yourdomain.com </dev/null 2>/dev/null | openssl x509 -noout -dates
```

#### Certificate Storage

- **Location**: `infrastructure/letsencrypt/acme.json` (on server at `~/beef-briefing/letsencrypt/acme.json`)
- **Permissions**: 600 (owner read/write only) - required by Traefik
- **Git**: Excluded via `.gitignore` (never committed)

#### Backup Certificates

```bash
# Backup certificates before server migration
ssh $(make tf-ssh-user-host) 'cat ~/beef-briefing/letsencrypt/acme.json' > acme.json.backup
```

#### Restore Certificates

```bash
# Restore certificates to server
scp acme.json.backup $(make tf-ssh-user-host):~/beef-briefing/letsencrypt/acme.json
ssh $(make tf-ssh-user-host) 'chmod 600 ~/beef-briefing/letsencrypt/acme.json'

# Restart Traefik
ssh $(make tf-ssh-user-host) 'cd ~/beef-briefing && docker compose restart traefik'
```

### Testing with Let's Encrypt Staging

For testing SSL setup without hitting rate limits (50 certs/week):

1. Edit `infrastructure/docker-compose.prod.yml`
2. Uncomment the staging resolver lines in the Traefik service
3. Change `certresolver` in all labels from `letsencrypt` to `letsencrypt-staging`
4. Deploy: `make deploy`

**Note:** Staging certificates will show browser warnings (not trusted). Switch back to production after testing.

### Rotating Traefik Dashboard Password

```bash
# Generate new password
make generate-traefik-password

# Redeploy to apply changes
make deploy
```

### Troubleshooting

#### Certificate Not Issued

**Symptoms:** Traefik logs show ACME errors, site not accessible via HTTPS

**Common Causes:**
1. DNS not pointing to server
2. Firewall blocking port 80 (needed for HTTP challenge)
3. Invalid email address

**Solution:**

```bash
# 1. Check DNS resolution
dig yourdomain.com

# 2. Verify firewall allows port 80 (Linode opens 80/443 by default via Terraform)
ssh $(make tf-ssh-user-host) 'sudo iptables -L -n | grep -E "(80|443)"'

# 3. Check Traefik logs for specific errors
make logs-traefik COMPOSE_FILE=infrastructure/docker-compose.prod.yml

# 4. Verify LETSENCRYPT_EMAIL is set correctly
grep LETSENCRYPT_EMAIL infrastructure/.env.prod

# 5. Delete acme.json to retry (use staging first!)
ssh $(make tf-ssh-user-host) 'rm ~/beef-briefing/letsencrypt/acme.json'
ssh $(make tf-ssh-user-host) 'cd ~/beef-briefing && docker compose restart traefik'
```

#### Admin Panel Shows "Connection Refused"

**Cause:** Admin panel not configured for HTTPS or Traefik labels missing

**Solution:**

```bash
# 1. Verify admin-panel service has Traefik labels
grep -A 10 "admin-panel:" infrastructure/docker-compose.prod.yml | grep traefik

# 2. Verify SECURE_COOKIES is set to true
grep SECURE_COOKIES infrastructure/docker-compose.prod.yml

# 3. Restart services
ssh $(make tf-ssh-user-host) 'cd ~/beef-briefing && docker compose restart'

# 4. Check admin-panel logs
make logs-admin-panel COMPOSE_FILE=infrastructure/docker-compose.prod.yml
```

#### Traefik Dashboard 401 Unauthorized

**Cause:** Invalid htpasswd credentials or incorrect escaping

**Solution:**

```bash
# 1. Regenerate password with proper escaping
make generate-traefik-password

# 2. Verify format in .env.prod (should have $$2y$$ not $2y$)
grep TRAEFIK_DASHBOARD_USERS infrastructure/.env.prod

# 3. Redeploy
make deploy
```

#### Let's Encrypt Rate Limits

**Cause:** Too many certificate requests (50 per domain per week)

**Symptoms:** ACME errors in Traefik logs mentioning "rate limit exceeded"

**Solution:**

1. Use staging environment for testing (unlimited requests)
2. Wait for rate limit to reset (1 week)
3. Restore from backup if available
4. Consider using alternative ACME provider (e.g., ZeroSSL)

#### Sessions Invalid After Deployment

**Expected Behavior:** Setting `SECURE_COOKIES=true` invalidates all existing sessions when first enabling HTTPS.

**Solution:** Users must log in again. This is a one-time security upgrade and expected behavior.

### Security Considerations

1. **HTTPS Only**: All traffic encrypted via TLS 1.2+
2. **Basic Auth**: Traefik dashboard protected by bcrypt-hashed passwords
3. **Secure Cookies**: Admin panel sessions only transmitted over HTTPS (`SECURE_COOKIES=true`)
4. **File Permissions**: `acme.json` restricted to owner only (600)
5. **Docker Socket**: Mounted read-only to Traefik
6. **Network Isolation**: Services communicate on internal Docker network
7. **No Direct Port Exposure**: Only Traefik exposes ports (80/443)

### Adding More Services Behind Traefik

To add API service or other services behind Traefik:

1. Remove port exposure from service in `docker-compose.prod.yml`
2. Add Traefik labels:

```yaml
api-service:
  # Remove: ports: - "8080:8080"
  labels:
    - "traefik.enable=true"
    - "traefik.http.routers.api.rule=Host(`${DOMAIN_NAME}`) && PathPrefix(`/api`)"
    - "traefik.http.routers.api.entrypoints=websecure"
    - "traefik.http.routers.api.tls.certresolver=letsencrypt"
    - "traefik.http.services.api.loadbalancer.server.port=8080"
    - "traefik.http.routers.api.middlewares=api-strip"
    - "traefik.http.middlewares.api-strip.stripprefix.prefixes=/api"
```

3. Update internal service references to use Docker network hostname (`http://api-service:8080`)

## Makefile Commands

### Docker Lifecycle

```bash
make up              # Start all services
make up-build        # Rebuild images and start services
make down            # Stop all services
make restart         # Restart all services
make ps              # Show running containers
make clean           # Stop services and remove volumes
make prune           # Remove all containers, images, volumes, networks
```

### Logs

```bash
make logs                  # Tail logs from all services
make logs-api              # Tail logs from api-service
make logs-bot              # Tail logs from telegram-bot
make logs-postgres         # Tail logs from postgres
make logs-admin-panel      # Tail logs from admin-panel
make logs-traefik          # Tail logs from traefik (production only)
```

### Building

```bash
make build                 # Rebuild all images
make build-api             # Rebuild api-service image
make build-bot             # Rebuild telegram-bot image
make build-admin-panel     # Rebuild admin-panel image
```

### Traefik & SSL

```bash
make generate-traefik-password
                          # Generate Traefik dashboard password

make logs-traefik COMPOSE_FILE=infrastructure/docker-compose.prod.yml
                          # View Traefik logs (production)
```

### Admin Panel Secrets

```bash
make admin-panel-set-secrets-files ENV_FILE=infrastructure/.env.prod
                          # Generate both password and session secret (recommended)

make admin-panel-set-password-file ENV_FILE=infrastructure/.env.prod
                          # Update admin password only

make admin-panel-set-session-file ENV_FILE=infrastructure/.env.prod
                          # Update session secret only
```

### Terraform

```bash
make tf-init              # Initialize Terraform
make tf-plan              # Show Terraform execution plan
make tf-apply             # Apply Terraform configuration
make tf-destroy           # Destroy infrastructure (DANGEROUS!)
make tf-ip                # Get server IP address
make tf-ssh               # SSH into server
make tf-deploy-check      # Pre-deployment validation
```

### Deployment

```bash
make deploy               # Deploy to production
make deploy-skip-build    # Deploy without rebuilding images
make deploy-skip-cleanup  # Deploy without cleaning old images
make rollback             # Rollback to previous version
make rollback-force       # Rollback without confirmation
```

### Full Command Reference

Run `make help` to see all available commands with descriptions.

## Project Structure

```
beef-briefing/
├── apps/
│   ├── admin-panel/        # Go web admin interface (templ)
│   ├── api-service/        # Go REST API
│   ├── telegram-bot/       # Go Telegram bot client
│   └── postgres/
│       ├── migrations/     # SQL database migrations
│       └── seeds/          # SQL seed data
├── infrastructure/
│   ├── docker-compose.dev.yml    # Development environment
│   ├── docker-compose.prod.yml   # Production environment (with Traefik)
│   ├── .env.dev.example          # Development env template
│   ├── .env.prod.example         # Production env template
│   ├── letsencrypt/              # SSL certificates (gitignored)
│   ├── secrets/                  # Secrets directory (gitignored)
│   │   └── apps/
│   │       ├── admin-panel/      # Admin password & session secret
│   │       └── new-relic/        # New Relic config
│   └── terraform/                # Infrastructure as Code (Linode)
├── scripts/
│   ├── deploy.sh                 # Main deployment script
│   ├── rollback.sh               # Rollback script
│   └── common.sh                 # Shared functions
└── Makefile                      # Build automation
```

## Environment Variables

### Required for Production

| Variable | Description | Example |
|----------|-------------|---------|
| `DOMAIN_NAME` | Domain for SSL certificates | `example.com` |
| `LETSENCRYPT_EMAIL` | Email for Let's Encrypt notifications | `admin@example.com` |
| `TRAEFIK_DASHBOARD_USERS` | Htpasswd credentials for dashboard | `admin:$$2y$$05$$...` |
| `DB_PASSWORD` | PostgreSQL password | `secure_password` |
| `TELEGRAM_BOT_TOKEN` | Telegram bot API token | `123456:ABC-DEF...` |
| `MINIO_ACCESS_KEY` | Linode Object Storage access key | From Terraform |
| `MINIO_SECRET_KEY` | Linode Object Storage secret key | From Terraform |
| `MINIO_BUCKET` | Object storage bucket name | `beef-briefing-media` |

See `infrastructure/.env.prod.example` for full list with descriptions.
