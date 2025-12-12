# Secrets Generation Guide

This guide covers all secrets that must be generated for the beef-briefing application.

## Prerequisites

- Go 1.25+ installed (for admin panel secrets)
- `htpasswd` command available (for Traefik password)
  - Debian/Ubuntu: `sudo apt install apache2-utils`
  - macOS: Pre-installed
  - RHEL/CentOS: `sudo yum install httpd-tools`
- `openssl` command available (for analytics API key)

## 1. Admin Panel Secrets

**Purpose:** Generate bcrypt password hash and session secret for admin panel authentication.

**Make Target (Recommended):**
```bash
# Generate both password hash and session secret to files
make admin-panel-set-secrets-files

# Or specify environment file explicitly
make admin-panel-set-secrets-files ENV_FILE=infrastructure/.env.prod
```

**Individual Secrets:**
```bash
# Password hash only
make admin-panel-set-password-file

# Session secret only
make admin-panel-set-session-file
```

**Output Location:**
```
infrastructure/secrets/apps/admin-panel/
├── admin_password_hash    # Bcrypt hash (permissions: 600)
└── session_secret         # Base64-encoded 32-byte key (permissions: 600)
```

**Interactive Process:**
1. Prompts for admin password
2. Prompts for password confirmation
3. Generates bcrypt hash (cost factor 10)
4. Generates 32 random bytes for session secret
5. Base64 encodes session secret
6. Writes files with 600 permissions

**Legacy Mode (environment file):**
```bash
# Update .env file directly (not recommended for production)
make admin-panel-set-secrets ENV_FILE=infrastructure/.env.dev
```

**Scripted/Non-Interactive Mode:**
```bash
echo "mypassword" | make admin-panel-set-password-file
```

---

## 2. Traefik Dashboard Password

**Purpose:** Generate htpasswd credentials for Traefik dashboard basic auth.

**Make Target:**
```bash
make generate-traefik-password
```

**Output:** Updates `TRAEFIK_DASHBOARD_USERS` in `infrastructure/.env.prod`

**Interactive Process:**
1. Prompts for username (default: admin)
2. Prompts for password (hidden input)
3. Prompts for password confirmation
4. Validates password strength (warns if < 8 characters)
5. Generates bcrypt hash using `htpasswd -nbB`
6. Escapes `$` to `$$` for docker-compose compatibility
7. Updates or creates `TRAEFIK_DASHBOARD_USERS` in `.env.prod`

**Output Format:**
```
TRAEFIK_DASHBOARD_USERS=admin:$$2y$$05$$gHL5l9SFHfFGrJm4gVQ65OrLFsMfSrZK7GwDPFJRE5gJQTDRvdlT2
```

**Notes:**
- `$$` escaping is required for docker-compose variable substitution
- Requires `.env.prod` file to exist before running

---

## 3. Analytics API Key

**Purpose:** Generate secure random API key for analytics endpoints.

**Make Target:**
```bash
make generate-analytics-api-key
```

**Output Location:**
```
infrastructure/secrets/apps/api-service/analytics_api_key
```

**Process:**
1. Creates directory `infrastructure/secrets/apps/api-service/` if needed
2. Generates 32 random bytes using OpenSSL
3. Base64 encodes the bytes
4. Saves to file with 600 permissions
5. Prints key to console for reference

**Example Output:**
```
Generating analytics API key...
API key generated and saved to infrastructure/secrets/apps/api-service/analytics_api_key
Key: K7xY2mN9pQrS3tUvW8zA1bC4dE5fG6hI7jK8lM9nO0p=
This key will be deployed when you run 'make deploy'
```

**Notes:**
- Key is automatically deployed to production via volume mount
- File permissions set to 600 (owner read/write only)

---

## Quick Reference

| Secret | Make Target | Output | Prerequisites |
|--------|-------------|--------|---------------|
| Admin password + session | `make admin-panel-set-secrets-files` | `infrastructure/secrets/apps/admin-panel/` | Go 1.25+ |
| Admin password only | `make admin-panel-set-password-file` | `admin_password_hash` file | Go 1.25+ |
| Session secret only | `make admin-panel-set-session-file` | `session_secret` file | Go 1.25+ |
| Traefik password | `make generate-traefik-password` | `.env.prod` line | htpasswd |
| Analytics API key | `make generate-analytics-api-key` | `analytics_api_key` file | openssl |

---

## First-Time Setup Checklist

### Development
```bash
# 1. Copy environment template
cp infrastructure/.env.dev.example infrastructure/.env.dev

# 2. Generate admin secrets
make admin-panel-set-secrets-files

# 3. (Optional) Generate analytics key for testing
make generate-analytics-api-key

# 4. Start services
make up-build
```

### Production
```bash
# 1. Copy environment template
cp infrastructure/.env.prod.example infrastructure/.env.prod

# 2. Edit .env.prod with required values:
#    - TELEGRAM_BOT_TOKEN
#    - DOMAIN_NAME
#    - LETSENCRYPT_EMAIL
#    - DB_PASSWORD
#    - LINODE_TOKEN

# 3. Generate all secrets
make admin-panel-set-secrets-files ENV_FILE=infrastructure/.env.prod
make generate-traefik-password
make generate-analytics-api-key

# 4. Deploy
make tf-setup && make tf-init && make tf-apply
make deploy
```

---

## Security Considerations

1. **File Permissions:** All secret files are created with 600 permissions (owner read/write only)
2. **Git Exclusion:** `infrastructure/secrets/` is in `.gitignore`
3. **Docker Mounts:** Secrets are mounted read-only in containers
4. **Password Strength:** Traefik password generator warns for passwords < 8 characters
5. **Bcrypt Cost:** Admin password uses bcrypt cost factor 10 (secure but fast enough for login)

---

## Troubleshooting

**"htpasswd: command not found"**
```bash
# Debian/Ubuntu
sudo apt install apache2-utils

# macOS (pre-installed, check PATH)
which htpasswd

# RHEL/CentOS
sudo yum install httpd-tools
```

**"Permission denied" when generating secrets**
```bash
# Ensure secrets directory exists with correct permissions
mkdir -p infrastructure/secrets/apps/admin-panel
mkdir -p infrastructure/secrets/apps/api-service
chmod 700 infrastructure/secrets
```

**Session secret validation fails**
- Session secret must be at least 32 bytes when base64-decoded
- Regenerate with `make admin-panel-set-session-file`

**Traefik 401 Unauthorized after password change**
1. Verify `$$` escaping in `.env.prod`: `grep TRAEFIK_DASHBOARD_USERS infrastructure/.env.prod`
2. The line should contain `$$2y$$` not `$2y$`
3. Redeploy: `make deploy`
