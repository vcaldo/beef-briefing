# Mandatory Environment Variables Checklist

## Required for All Environments

### Telegram Bot Token
- [ ] `TELEGRAM_BOT_TOKEN`
  - **Description:** Bot token from BotFather (required for bot operation)
  - **Files:**
    - `pkg/config/config.go:27` - `envconfig:"TELEGRAM_BOT_TOKEN" required:"true"`
    - `infrastructure/.env.dev.example:26`
    - `infrastructure/.env.prod.example:26`
  - **Format:** `123456789:ABCdefGHIjklMNOpqrsTUVwxyz`

## Required for Production

### Domain & SSL

- [ ] `DOMAIN_NAME`
  - **Description:** Domain name for SSL certificates and routing
  - **Files:**
    - `infrastructure/.env.prod.example:74`
    - `infrastructure/docker-compose.prod.yml` (Traefik labels)
  - **Format:** `example.com`

- [ ] `LETSENCRYPT_EMAIL`
  - **Description:** Email for Let's Encrypt certificate notifications
  - **Files:**
    - `infrastructure/.env.prod.example:78`
    - `infrastructure/docker-compose.prod.yml` (Traefik command)
  - **Format:** `admin@example.com`

### Infrastructure (Terraform)

- [ ] `LINODE_TOKEN`
  - **Description:** Linode API token for infrastructure provisioning
  - **Files:**
    - `infrastructure/.env.prod.example:106`
    - `Makefile` (tf-setup target)
  - **Format:** 64-character alphanumeric string
  - **Source:** Linode Cloud Manager > API Tokens

### Database

- [ ] `DB_PASSWORD`
  - **Description:** PostgreSQL password
  - **Files:**
    - `pkg/config/config.go:17` - `envconfig:"DB_PASSWORD"`
    - `infrastructure/.env.prod.example:11`
    - `infrastructure/docker-compose.prod.yml`
  - **Default:** Empty (must be set for production)

### Secrets (Must Generate)

- [ ] `ANALYTICS_API_KEY_FILE` or `ANALYTICS_API_KEY`
  - **Description:** API key for analytics endpoints
  - **Files:**
    - `pkg/config/config.go:41-42`
    - `infrastructure/.env.prod.example:54`
  - **Generate:** `make secrets-analytics-api-key`

- [ ] `TRAEFIK_DASHBOARD_USERS`
  - **Description:** Htpasswd credentials for Traefik dashboard (bcrypt)
  - **Files:**
    - `infrastructure/.env.prod.example:85`
    - `infrastructure/docker-compose.prod.yml`
  - **Generate:** `make secrets-traefik-password`
  - **Format:** `username:$$2y$$05$$hashedpassword` ($$-escaped for docker-compose)

## Optional Variables

### Object Storage (Auto-synced from Terraform)

- [ ] `MINIO_ENDPOINT` - Default: `localhost:9000` (dev), synced from Terraform (prod)
- [ ] `MINIO_ACCESS_KEY` - Default: `minioadmin` (dev), synced from Terraform (prod)
- [ ] `MINIO_SECRET_KEY` - Default: `minioadmin` (dev), synced from Terraform (prod)
- [ ] `MINIO_BUCKET` - Default: `telegram-media`
- [ ] `MINIO_USE_SSL` - Default: `false` (dev), `true` (prod)

**Note:** Run `make tf-sync-object-storage-env` after `make tf-apply` to auto-populate these from Terraform outputs.

### Application Settings

- [ ] `ENVIRONMENT` - Default: `development` | `production`
- [ ] `LOG_LEVEL` - Default: `info` | Options: `debug`, `info`, `warn`, `error`
- [ ] `API_PORT` - Default: `8080`
- [ ] `DASHBOARD_PORT` - Default: `8050`
- [ ] `MAX_UPLOAD_SIZE_MB` - Default: `100`

### New Relic (Optional)

- [ ] `NEW_RELIC_APP_NAME` - Base name for APM (e.g., `beef-briefing`)
- [ ] `NEW_RELIC_LICENSE_KEY` - License key for APM instrumentation
- [ ] `NEW_RELIC_API_KEY` - API key for infrastructure monitoring (Terraform)
- [ ] `NEW_RELIC_ACCOUNT_ID` - Account ID for Terraform
- [ ] `NEW_RELIC_REGION` - Default: `EU` | Options: `EU`, `US`

**Note:** Both `NEW_RELIC_APP_NAME` and `NEW_RELIC_LICENSE_KEY` must be set to enable APM.
