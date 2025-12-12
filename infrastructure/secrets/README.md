# Secrets Management

This directory contains secret files used by the Beef Briefing services. All files in this directory are gitignored and should never be committed to version control.

## Directory Structure

```
secrets/
└── apps/
    ├── admin-panel/
    │   ├── admin_password_hash    # Bcrypt hash of admin password
    │   └── session_secret         # Base64-encoded 32-byte session key
    ├── api-service/
    │   └── analytics_api_key      # API key for analytics endpoints
    └── new-relic/
        └── (New Relic configuration files)
```

## Quick Setup

From the repository root:

```bash
# Generate all secrets at once
make admin-panel-set-secrets-files
make generate-analytics-api-key

# For production, also generate Traefik password
make generate-traefik-password
```

## Secret Files

### Admin Panel

#### `admin_password_hash`

Bcrypt hash of the admin panel login password.

**Generate:**
```bash
make admin-panel-set-secrets-files
# or
make admin-panel-set-password-file
```

**Format:** Raw bcrypt hash string (starts with `$2a$` or `$2b$`)

**Example:**
```
$2a$10$N9qo8uLOickgx2ZMRZoMy.MQDGGf0xMVYqXkV4WwZ6V8VV5Oe.Sei
```

#### `session_secret`

Cryptographically secure random key for session encryption.

**Generate:**
```bash
make admin-panel-set-secrets-files
# or
make admin-panel-set-session-file
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
make generate-analytics-api-key
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
| admin-panel | `ADMIN_PASSWORD_HASH_FILE` | `/app/secrets/admin_password_hash` |
| admin-panel | `SESSION_SECRET_FILE` | `/app/secrets/session_secret` |
| api-service | `ANALYTICS_API_KEY_FILE` | `/app/secrets-api/analytics_api_key` |

## Docker Volume Mounts

Secrets are mounted read-only into containers via docker-compose:

```yaml
# docker-compose.dev.yml / docker-compose.prod.yml
admin-panel:
  volumes:
    - ./secrets/apps/admin-panel:/app/secrets:ro

api-service:
  volumes:
    - ./secrets/apps/api-service:/app/secrets-api:ro
```

## Manual Generation

If Make targets are unavailable, generate secrets manually:

### Admin Password Hash

```bash
# Interactive
cd apps/admin-panel/tools
go run update_secrets.go -mode=files -secrets-dir ../../../infrastructure/secrets/apps/admin-panel

# Non-interactive
echo 'your-password' | go run update_secrets.go \
  -mode=files \
  -secrets-dir ../../../infrastructure/secrets/apps/admin-panel \
  -no-interactive
```

### Session Secret

```bash
# Using openssl
openssl rand -base64 32 > secrets/apps/admin-panel/session_secret
chmod 600 secrets/apps/admin-panel/session_secret
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
chmod 600 secrets/apps/admin-panel/*
chmod 600 secrets/apps/api-service/*
```

The Make targets automatically set correct permissions.

## Security Best Practices

1. **Never commit secrets** - All files in this directory are gitignored
2. **Use file-based secrets** - Avoids shell escaping issues with `$` in bcrypt hashes
3. **Rotate secrets regularly** - Regenerate secrets periodically, especially after security incidents
4. **Separate per environment** - Use different secrets for dev, staging, and production
5. **Backup securely** - Store production secrets in a secure password manager or vault
6. **Minimal permissions** - Secret files should be readable only by the owner (600)

## Troubleshooting

### "ADMIN_PASSWORD_HASH or ADMIN_PASSWORD_HASH_FILE must be set"

The admin panel cannot find the password hash. Check:

1. Secret file exists: `ls -la secrets/apps/admin-panel/admin_password_hash`
2. File has content: `cat secrets/apps/admin-panel/admin_password_hash`
3. Docker volume is mounted correctly in docker-compose

**Fix:** Regenerate with `make admin-panel-set-secrets-files`

### "session secret must be at least 32 bytes"

The session secret is too short or improperly encoded.

**Fix:** Regenerate with `make admin-panel-set-session-file`

### Login fails after regenerating password

The password hash doesn't match what you're typing.

**Check:** Ensure you're using the same password you entered during generation.

**Fix:** Regenerate with `make admin-panel-set-password-file` using your desired password.

### "failed to read analytics API key from file"

The analytics API key file is missing.

**Fix:** Generate with `make generate-analytics-api-key`

## Related Documentation

- [Admin Panel Tools](../../apps/admin-panel/tools/README.md) - Detailed tool documentation
- [Admin Panel Config](../../apps/admin-panel/README.md) - Configuration reference
- [Infrastructure README](../README.md) - Overall infrastructure setup
