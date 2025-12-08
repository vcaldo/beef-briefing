# Admin Panel Tools

## Secret Management Tools

### 1. update_secrets.go (Recommended)

A comprehensive tool for generating and updating admin panel secrets (password hash and session secret). Supports both file-based storage (recommended) and environment variable storage.

#### Features
- Generates bcrypt password hashes with interactive password input
- Generates cryptographically secure session secrets
- **File-based storage (recommended)**: Writes secrets to separate files, avoiding shell escaping issues
- **Environment variable storage**: Updates `.env` files in-place with proper quoting
- Supports updating both secrets or individually
- Interactive and non-interactive modes for scripting

#### Usage

**File-based mode (recommended):**
```bash
cd apps/admin-panel/tools

# Generate both secrets to files
go run update_secrets.go -mode=files -secrets-dir ../../../infrastructure/secrets

# Update only password hash
go run update_secrets.go -mode=files -secrets-dir ../../../infrastructure/secrets -password-only

# Update only session secret
go run update_secrets.go -mode=files -secrets-dir ../../../infrastructure/secrets -session-only
```

**Environment variable mode:**
```bash
cd apps/admin-panel/tools

# Update .env file
go run update_secrets.go -file ../../../infrastructure/.env.dev

# Update only password hash
go run update_secrets.go -file ../../../infrastructure/.env.dev -password-only

# Update only session secret
go run update_secrets.go -file ../../../infrastructure/.env.dev -session-only
```

**Non-interactive mode (for scripts):**
```bash
echo 'mypassword' | go run update_secrets.go -mode=files -secrets-dir ../../../infrastructure/secrets -no-interactive
```

#### Command-Line Options
- `-mode` (default: env): Output mode: 'env' or 'files'
- `-file`: Path to `.env` file to update (required for env mode)
- `-secrets-dir`: Path to directory for secret files (required for files mode)
- `-password-only`: Only update password hash, skip session secret
- `-session-only`: Only update session secret, skip password
- `-no-interactive`: Read password from stdin instead of prompting
- `-help`: Show help message

#### Makefile Integration

```bash
# File-based (recommended)
make admin-panel-set-secrets-files     # Generate both secrets to files
make admin-panel-set-password-file     # Generate password hash to file
make admin-panel-set-session-file      # Generate session secret to file

# Environment variables (legacy)
make admin-panel-set-secrets           # Update both in .env
make admin-panel-set-password          # Update password in .env
make admin-panel-set-session           # Update session secret in .env
```

#### Why File-Based Storage?

File-based secret storage is recommended because:

1. **No shell escaping issues**: Bcrypt hashes contain `$` which can cause problems with environment variables
2. **Better security**: File permissions (0600) restrict access to the secrets
3. **Container-friendly**: Works seamlessly with Docker secrets and Kubernetes
4. **Cleaner separation**: Secrets are separate from configuration
5. **No quoting needed**: Raw values without worrying about special characters

#### Configuration

The admin panel reads secrets from files when these environment variables are set:

```bash
ADMIN_PASSWORD_HASH_FILE=/app/secrets/admin_password_hash
SESSION_SECRET_FILE=/app/secrets/session_secret
```

In Docker, mount the secrets directory as a volume:

```yaml
volumes:
  - ./infrastructure/secrets:/app/secrets:ro
```

### 2. hash_password_tool.go (Legacy)

The original password hash generator. This tool only displays the hash without updating files.

#### Usage

**Interactive Mode:**
```bash
cd apps/admin-panel/tools
go run hash_password_tool.go
```

**Pipe Mode:**
```bash
echo "your-password" | go run hash_password_tool.go
```

**Note:** Use `update_secrets.go` for new workflows as it provides more functionality.

## Security Notes

- Never commit password hashes or session secrets to version control
- Use a strong password (minimum 8 characters recommended)
- The bcrypt cost factor is set to the default (10 rounds)
- Generate new secrets for each environment (dev, staging, production)
- Session secrets use cryptographically secure random generation (crypto/rand)
- Secret files are created with 0600 permissions (owner read/write only)
