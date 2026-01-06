# Secrets Management

This directory contains secret files used by the Beef Briefing services. All files in this directory are gitignored and should never be committed to version control.

## Directory Structure

```
secrets/
└── apps/
    ├── api-service/
    │   └── app_keys/
    │       └── telegram-bot    # API key for telegram-bot
    ├── telegram-bot/
    │   └── api_key             # Same key, for telegram-bot to read
    └── new-relic/
        └── (New Relic configuration files)
```

## Quick Setup

From the repository root:

```bash
# Generate API keys for each application
make secrets-service-api APP=telegram-bot

# For production, also generate Traefik password
make secrets-traefik-password
```

## How It Works

Each application that calls the api-service needs an API key:

1. **api-service** reads all keys from `app_keys/` directory to validate incoming requests
2. **telegram-bot** reads its own key from `telegram-bot/api_key` to authenticate requests

The `make secrets-service-api` command generates the same key in both locations:
- `api-service/app_keys/{app}` - for api-service to validate
- `{app}/api_key` - for the app to use

This structure allows each container to mount its own secrets directory without collisions.

## Secret Files

### API Service Keys

API keys for authenticating requests to all `/api/v1/*` endpoints.

**Generate:**
```bash
make secrets-service-api APP=telegram-bot
```

**Format:** Base64-encoded 32 bytes

**Example:**
```
xK9mN2pQ4rS7tU0vW3xY6zA8bC1dE4fG5hI8jK2lM3n=
```

## Environment Variables

Services read secrets from files using environment variables:

| Service | Environment Variable | Container Path |
|---------|---------------------|----------------|
| api-service | `APP_KEYS_DIR` | `/app/secrets/app_keys` |
| telegram-bot | `API_KEY_FILE` | `/app/secrets/api_key` |

## Docker Volume Mounts

Each service mounts its own secrets directory (read-only):

```yaml
# docker-compose.dev.yml / docker-compose.prod.yml
api-service:
  volumes:
    - ./secrets/apps/api-service:/app/secrets:ro

telegram-bot:
  volumes:
    - ./secrets/apps/telegram-bot:/app/secrets:ro
```

## Manual Generation

If Make targets are unavailable, generate secrets manually:

```bash
# Generate a key
KEY=$(openssl rand -base64 32)

# Save for api-service to validate
mkdir -p secrets/apps/api-service/app_keys
echo -n "$KEY" > secrets/apps/api-service/app_keys/telegram-bot
chmod 600 secrets/apps/api-service/app_keys/telegram-bot

# Save for telegram-bot to use
mkdir -p secrets/apps/telegram-bot
echo -n "$KEY" > secrets/apps/telegram-bot/api_key
chmod 600 secrets/apps/telegram-bot/api_key
```

## File Permissions

All secret files should have `600` permissions (owner read/write only):

```bash
chmod 600 secrets/apps/api-service/app_keys/*
chmod 600 secrets/apps/telegram-bot/*
```

The Make targets automatically set correct permissions.

## API Authentication

All `/api/v1/*` endpoints require authentication. Only `/health` is unauthenticated (for load balancer health checks).

**Request format:**
```bash
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -F 'update={...}'
```

**Unauthenticated response:**
```json
{"error": "missing authorization header"}
```

**Invalid key response:**
```json
{"error": "invalid API key"}
```

## Security Best Practices

1. **Never commit secrets** - All files in this directory are gitignored
2. **Use file-based secrets** - Avoids shell escaping issues
3. **Per-app keys** - Each application has its own key for isolation
4. **Rotate secrets regularly** - Regenerate secrets periodically, especially after security incidents
5. **Separate per environment** - Use different secrets for dev, staging, and production
6. **Backup securely** - Store production secrets in a secure password manager or vault
7. **Minimal permissions** - Secret files should be readable only by the owner (600)

## Troubleshooting

### "no app keys configured"

The api-service cannot find any app keys.

**Fix:** Generate with `make secrets-service-api APP=telegram-bot`

### "API_KEY or API_KEY_FILE is required"

The telegram-bot cannot find its API key.

**Fix:** Generate with `make secrets-service-api APP=telegram-bot`

### "missing authorization header" (401)

Request is missing the Authorization header.

**Fix:** Add `-H "Authorization: Bearer YOUR_KEY"` to your request

## Related Documentation

- [Infrastructure README](../README.md) - Overall infrastructure setup
- [CLAUDE.md](../../CLAUDE.md) - Project documentation
