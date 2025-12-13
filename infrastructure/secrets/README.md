# Secrets Management

This directory contains secret files used by the Beef Briefing services. All files in this directory are gitignored and should never be committed to version control.

## Directory Structure

```
secrets/
└── apps/
    ├── dashboard/
    │   └── flask_secret_key      # Flask session secret key
    ├── api-service/
    │   └── analytics_api_key     # API key for analytics endpoints
    └── new-relic/
        └── (New Relic configuration files)
```

## Quick Setup

From the repository root:

```bash
# Generate all secrets at once
make secrets-dashboard
make secrets-analytics-api-key

# For production, also generate Traefik password
make secrets-traefik-password
```

## Secret Files

### Dashboard

#### `flask_secret_key`

Flask session secret key for the analytics dashboard.

**Generate:**
```bash
make secrets-dashboard
```

**Format:** Base64-encoded 32 bytes

**Example:**
```
K7gNU3sdo+OL0wNhqoVWhr3g6s1xYv72ol/pe/Unols=
```

### API Service

#### `analytics_api_key`

API key for authenticating requests to analytics endpoints.

**Generate:**
```bash
make secrets-analytics-api-key
```

**Format:** Base64-encoded 32 bytes

**Example:**
```
xK9mN2pQ4rS7tU0vW3xY6zA8bC1dE4fG5hI8jK2lM3n=
```

## Environment Variables

Services read secrets from files using `*_FILE` environment variables:

| Service | Environment Variable | Default Path |
|---------|---------------------|--------------|
| dashboard | `FLASK_SECRET_KEY_FILE` | `/app/secrets/flask_secret_key` |
| api-service | `ANALYTICS_API_KEY_FILE` | `/app/secrets-api/analytics_api_key` |

## Docker Volume Mounts

Secrets are mounted read-only into containers via docker-compose:

```yaml
# docker-compose.dev.yml / docker-compose.prod.yml
dashboard:
  volumes:
    - ./secrets/apps/dashboard:/app/secrets:ro

api-service:
  volumes:
    - ./secrets/apps/api-service:/app/secrets-api:ro
```

## Manual Generation

If Make targets are unavailable, generate secrets manually:

### Flask Secret Key

```bash
mkdir -p secrets/apps/dashboard
openssl rand -base64 32 > secrets/apps/dashboard/flask_secret_key
chmod 600 secrets/apps/dashboard/flask_secret_key
```

### Analytics API Key

```bash
mkdir -p secrets/apps/api-service
openssl rand -base64 32 > secrets/apps/api-service/analytics_api_key
chmod 600 secrets/apps/api-service/analytics_api_key
```

## File Permissions

All secret files should have `600` permissions (owner read/write only):

```bash
chmod 600 secrets/apps/dashboard/*
chmod 600 secrets/apps/api-service/*
```

The Make targets automatically set correct permissions.

## Security Best Practices

1. **Never commit secrets** - All files in this directory are gitignored
2. **Use file-based secrets** - Avoids shell escaping issues
3. **Rotate secrets regularly** - Regenerate secrets periodically, especially after security incidents
4. **Separate per environment** - Use different secrets for dev, staging, and production
5. **Backup securely** - Store production secrets in a secure password manager or vault
6. **Minimal permissions** - Secret files should be readable only by the owner (600)

## Troubleshooting

### "failed to read analytics API key from file"

The analytics API key file is missing.

**Fix:** Generate with `make secrets-analytics-api-key`

### Dashboard session issues

The Flask secret key may be missing or invalid.

**Fix:** Regenerate with `make secrets-dashboard`

## Related Documentation

- [Dashboard](../../apps/dashboard/README.md) - Dashboard documentation
- [Infrastructure README](../README.md) - Overall infrastructure setup
