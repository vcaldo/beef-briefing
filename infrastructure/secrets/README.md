# Secrets Directory

This directory contains sensitive secrets for the admin panel.

**⚠️ IMPORTANT: Never commit these files to version control!**

## Files

- `admin_password_hash` - Bcrypt hash of the admin password
- `session_secret` - Session secret for cookie encryption

## Usage

These files are automatically read by the admin panel when the following environment variables are set:

```bash
ADMIN_PASSWORD_HASH_FILE=/path/to/infrastructure/secrets/admin_password_hash
SESSION_SECRET_FILE=/path/to/infrastructure/secrets/session_secret
```

## Generating Secrets

Use the provided tool to generate and save secrets to files:

```bash
# Generate both secrets
make admin-panel-set-secrets

# Generate only password hash
make admin-panel-set-password

# Generate only session secret
make admin-panel-set-session
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
