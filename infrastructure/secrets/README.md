# Secrets Directory

This directory contains sensitive secrets for various services including the admin panel and monitoring tools.

**⚠️ IMPORTANT: Never commit these files to version control!**

## Directory Structure

```
secrets/
├── admin_password_hash          # (root level) Bcrypt hash of the admin password
├── session_secret               # (root level) Session secret for cookie encryption
├── admin-panel/                 # Admin panel specific secrets
│   ├── admin_password_hash
│   └── session_secret
└── new-relic/                   # New Relic monitoring configuration
    └── newrelic-infra.yml
```

## Files

### Root Level
- `admin_password_hash` - Bcrypt hash of the admin password
- `session_secret` - Session secret for cookie encryption

### admin-panel/
- `admin_password_hash` - Admin panel Bcrypt hash of the admin password
- `session_secret` - Admin panel session secret for cookie encryption

### new-relic/
- `newrelic-infra.yml` - New Relic infrastructure agent configuration file

### Traefik Dashboard Credentials

**Note:** Traefik dashboard credentials are **not** stored as separate files in this directory. Instead, they are managed via the `TRAEFIK_DASHBOARD_USERS` environment variable in `.env.prod`.

To generate Traefik dashboard credentials:

```bash
make generate-traefik-password
```

This will generate bcrypt-hashed credentials in htpasswd format and update the `infrastructure/.env.prod` file automatically.

## Usage

These files are automatically read by the admin panel when the following environment variables are set:

```bash
ADMIN_PASSWORD_HASH_FILE=/path/to/infrastructure/secrets/admin_password_hash
SESSION_SECRET_FILE=/path/to/infrastructure/secrets/session_secret
```

## Generating Secrets

Use the provided tool to generate and save secrets to files:

```bash
# Generate both secrets and save to files
make admin-panel-set-secrets-files

# Generate only password hash and save to file
make admin-panel-set-password-file

# Generate only session secret and save to file
make admin-panel-set-session-file
```

## Docker Configuration

In docker-compose, mount this directory as a volume:

```yaml
volumes:
  - ./infrastructure/secrets:/app/secrets:ro
```

Then set the environment variables:

```yaml
environment:
  - ADMIN_PASSWORD_HASH_FILE=/app/secrets/admin_password_hash
  - SESSION_SECRET_FILE=/app/secrets/session_secret
```

## New Relic Configuration

The New Relic infrastructure agent uses the configuration file at `new-relic/newrelic-infra.yml`.

