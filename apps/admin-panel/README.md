# Admin Panel

Secure web admin panel for the Beef Briefing system. Provides real-time statistics, user engagement metrics, and calendar-based activity visualization for Telegram group chat data.

## Features

- 🔐 Session-based authentication with bcrypt password hashing
- 🎨 5 themes: Light, Dark, Business, Cyberpunk, Forest
- 📊 User statistics with message counts, reactions, and media tracking
- 📅 Activity calendar heatmap with year navigation
- 🔄 HTMX-powered dynamic content updates
- 🛡️ Rate limiting (5 login attempts per 15 minutes)

## Tech Stack

- **Backend**: Go 1.23+, Gorilla Mux, Templ templates
- **Frontend**: DaisyUI, Tailwind CSS, HTMX, ECharts
- **Database**: PostgreSQL
- **Auth**: Gorilla Sessions, bcrypt

## Configuration

### Secrets Management (Recommended: File-Based)

For security and ease of use, secrets are now stored in separate files instead of environment variables. This avoids shell escaping issues with special characters (like `$` in bcrypt hashes).

**Quick Start:**

```bash
# Generate secrets to files (recommended)
make admin-panel-set-secrets-files
```

This creates:
- `infrastructure/secrets/admin_password_hash` - Bcrypt hash of admin password
- `infrastructure/secrets/session_secret` - 32-byte random session key (base64-encoded)

Files are automatically mounted in Docker at `/app/secrets/` (read-only).

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | Database user | `postgres` |
| `DB_PASSWORD` | Database password | - |
| `DB_NAME` | Database name | `beef_db` |
| `DB_SSL_MODE` | SSL mode | `disable` |
| `ADMIN_PANEL_PORT` | HTTP server port | `8081` |
| `ADMIN_USERNAME` | Admin username | `admin` |
| `ADMIN_PASSWORD_HASH_FILE` | Path to password hash file | `/app/secrets/admin_password_hash` |
| `SESSION_SECRET_FILE` | Path to session secret file | `/app/secrets/session_secret` |
| `ENVIRONMENT` | `development` or `production` | `development` |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` | `info` |

**Legacy (Deprecated):** You can still use `ADMIN_PASSWORD_HASH` and `SESSION_SECRET` environment variables directly, but file-based storage is preferred.

## Development

### Prerequisites

- Go 1.23+
- Templ CLI: `go install github.com/a-h/templ/cmd/templ@latest`
- PostgreSQL running with Beef Briefing schema

### Generate Secrets

**File-based (Recommended):**

```bash
# From project root
make admin-panel-set-secrets-files

# Or directly
cd apps/admin-panel/tools
go run update_secrets.go -mode=files -secrets-dir ../../../infrastructure/secrets
```

**Environment variables (Legacy):**

```bash
# Update .env file
make admin-panel-set-secrets

# Or manually generate hash
cd apps/admin-panel/tools
go run hash_password_tool.go
```

See `tools/README.md` for more options.

### Run Locally

```bash
# Generate templates
templ generate

# Run the server
go run cmd/main.go
```

### Docker

**Using file-based secrets (recommended):**

```bash
# First, generate secrets
make admin-panel-set-secrets-files

# Build and run with docker-compose
make build-admin-panel
make up
```

The docker-compose configuration automatically mounts `infrastructure/secrets/` to `/app/secrets/`.

**Manual Docker run:**

```bash
# Build
docker build -t beef-briefing-admin-panel .

# Run with secrets mounted
docker run -p 8081:8081 \
  -v $(pwd)/infrastructure/secrets:/app/secrets:ro \
  -e DB_HOST=host.docker.internal \
  -e ADMIN_PASSWORD_HASH_FILE=/app/secrets/admin_password_hash \
  -e SESSION_SECRET_FILE=/app/secrets/session_secret \
  beef-briefing-admin-panel
```

**Legacy (using environment variables):**

```bash
docker run -p 8081:8081 \
  -e DB_HOST=host.docker.internal \
  -e ADMIN_PASSWORD_HASH='$2a$10$...' \
  -e SESSION_SECRET='your-base64-encoded-secret-here' \
  beef-briefing-admin-panel
```

## Project Structure

```
apps/admin-panel/
├── cmd/
│   └── main.go                 # Application entry point
├── internal/
│   ├── auth/
│   │   └── auth.go            # Session & password management
│   ├── config/
│   │   └── config.go          # Environment configuration
│   ├── handler/
│   │   └── handler.go         # HTTP route handlers
│   ├── middleware/
│   │   └── ratelimit.go       # IP-based rate limiting
│   └── repository/
│       └── chat_repo.go       # Database queries
├── templates/
│   ├── layout.templ           # Base layout with navbar
│   ├── login.templ            # Login page
│   ├── dashboard.templ        # Chat list
│   └── chat_detail.templ      # Chat stats & calendar
├── static/
│   ├── css/
│   │   └── admin.css          # Custom styles & themes
│   └── js/
│       └── charts.js          # ECharts calendar
├── tools/
│   ├── update_secrets.go      # Secret generator (file/env modes)
│   ├── hash_password_tool.go  # Legacy hash generator
│   └── README.md              # Tools documentation
├── Dockerfile
├── go.mod
└── README.md
```

## Routes

| Method | Path | Description |
|--------|------|-------------|
| GET | `/auth/login` | Login page |
| POST | `/auth/login` | Login form submission |
| POST | `/auth/logout` | Logout |
| GET | `/` | Dashboard (chat list) |
| GET | `/chats/{id}` | Chat detail page |
| GET | `/chats/{id}/stats-partial` | HTMX: User stats table |
| GET | `/chats/{id}/calendar-data` | HTMX: Calendar heatmap |
| POST | `/theme` | Set theme preference |
| GET | `/static/*` | Static files |

## Security

- **File-based secrets**: Secrets stored in files with 0600 permissions (owner read/write only)
- **Bcrypt password hashing**: Cost factor 10
- **Secure cookies**: HttpOnly, Secure (production), SameSite=Lax
- **Base64-decoded session keys**: Session secrets decoded from base64 to ensure proper 32-byte key length
- **7-day session expiration**
- **IP-based rate limiting**: 5 login attempts per 15 minutes
- **Input validation** and sanitization
- **No sensitive data in logs**
- **Secrets excluded from git**: `.gitignore` prevents accidental commits
