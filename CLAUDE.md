# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go-based Telegram bot system for managing beef briefing subscriptions with REST API and PostgreSQL backend. Deployed on Linode with automatic SSL certificates via Traefik and Let's Encrypt.

**Technology Stack:**
- **Backend**: Go 1.25+
- **ML/Card Rendering**: Python 3.14, Playwright, OpenAI
- **Frontend**: React, TypeScript, Vite (Mini Apps)
- **Database**: PostgreSQL 17 with PostGIS 3.4
- **Storage**: MinIO (dev) / Linode Object Storage (prod)
- **Reverse Proxy**: Traefik v3 with Let's Encrypt SSL
- **Infrastructure**: Terraform (Linode), Docker Compose

## Architecture

### Services

**Go Services:**

1. **api-service** (port 8080): Central REST API with 26+ endpoints across 6 categories
   - **Ingest**: Multipart uploads with JSON metadata + binary files, SHA256 deduplication
   - **Profile Photos**: Upload/retrieve user and chat profile photos
   - **ML Analytics**: Batch message processing for ML pipeline
   - **Cards**: Weekly user stats cards with presigned image URLs
   - **Mini App**: JWT-authenticated endpoints for deck-mini-app, leaderboard-mini-app, and arena-mini-app
   - **Arena**: Match management, shop phase, battle results, leaderboards (18 endpoints)
   - **Auth**: API Key (internal services) and JWT (Mini Apps) authentication

2. **telegram-bot**: Telegram bot that listens to group messages and forwards to API
   - Concurrent media downloads (max 5 simultaneous)
   - Smart photo handling (largest size only)
   - Exponential backoff retry logic
   - 100MB file size limit

3. **import-cli**: CLI tool to import Telegram Desktop exports
   - Streaming parser for large datasets (1M+ messages)
   - Resume support with state tracking
   - Handles group→supergroup migration

**Python Services:**

4. **card-renderer** (port 8051): Renders gamified user stats cards as PNG images
   - HTML/CSS templates with Jinja2
   - Playwright for headless Chromium rendering
   - Theme system with JSON configuration (see [card-renderer README](apps/card-renderer/README.md))
   - MinIO/S3 storage for generated images

5. **ml-processor**: ML pipeline for message analysis
   - Sentiment, humor, toxicity analysis using OpenAI
   - Weekly stats aggregation for user cards
   - Rate-limited API calls with token bucket algorithm

### Database Architecture

**22 tables modeling complete Telegram data structure:**

- **Core**: `chats`, `users`, `updates`
- **Messages**: `messages`, `message_entities`, `message_edits`
- **Reactions**: `message_reactions`, `reaction_counts` (denormalized - stores Telegram message ID, not FK)
- **Media**: `media_files`, `photos`, `stickers`, `games`, `game_photos`
- **Other**: `polls`, `poll_options`, `contacts`, `locations`, `venues`, `dice`

**Key Design Patterns:**
- Content-addressable storage: `file_hash` column enables deduplication across `media_files`, `photos`, `game_photos`
- MinIO path structure: `{mediaType}/{hash[:2]}/{hash}`
- Group migration tracking: `chats.migrated_from_chat_id` links old group ID to new supergroup ID
- Denormalized reactions: Allow storing reactions for messages not yet captured

### Network Architecture (Production)

```
Internet (443/80) → Traefik (SSL termination)
                         ├─→ api.{domain} → API Service (8080)
                         │       ├─→ /api/v1/mini-app/* (public, JWT auth)
                         │       └─→ /api/v1/* (IP restricted)
                         ├─→ cards-api.{domain} → Card Renderer (8051)
                         ├─→ leaderboard.{domain} → Leaderboard Mini App
                         ├─→ deck.{domain} → Deck Mini App
                         ├─→ arena.{domain} → Arena Mini App
                         └─→ {domain}/dashboard → Traefik Dashboard

Internal Docker Network:
  ├─ API Service (8080) ←→ Telegram Bot
  ├─ Card Renderer (8051)
  └─ PostgreSQL (5432)
```

Only Traefik exposes ports externally. All services communicate via `beef-prod-network`.
- `api.{domain}` main API endpoints are protected by IP allowlist
- `api.{domain}/api/v1/mini-app/*` endpoints are public (JWT protected)
- `cards-api.{domain}` is protected by IP allowlist

## Development Commands

### Docker Lifecycle

```bash
make up              # Start all services (dev environment)
make up-build        # Rebuild images and start
make down            # Stop all services
make logs            # Tail logs from all services
make logs-api        # Tail specific service logs
make logs-bot
make clean           # Stop and remove volumes
```

### Building Go Services

```bash
# Build all Go binaries locally
make go-build

# Build specific service
make go-build-api
make go-build-bot
make go-build-import-cli

# Clean build artifacts
make go-clean
```

### Code Quality

```bash
make fmt              # Format all Go code with gofmt
make fmt-check        # Check if code is formatted
```

## Production Deployment

### Initial Setup

```bash
# 1. Configure environment
cp infrastructure/.env.prod.example infrastructure/.env.prod
# Edit .env.prod with your settings

# 2. Setup Terraform
make tf-setup          # Populates terraform.tfvars from .env.prod
make tf-init
make tf-plan
make tf-apply

# 3. Get server IP and configure DNS
make tf-ip            # Point domain A record to this IP

# 4. Generate secrets
make secrets-traefik-password
make secrets-service-api APP=telegram-bot

# 5. Deploy
make deploy           # Full deployment
```

### Subsequent Deployments

```bash
make deploy                    # Standard deployment (rebuilds images)
make deploy-skip-build         # Use existing images (faster)
make deploy-skip-cleanup       # Keep old images for rollback
make rollback                  # Rollback to previous version
```

### SSL Certificates

Traefik automatically handles Let's Encrypt certificates:

- **Storage**: `infrastructure/letsencrypt/acme.json` (gitignored, 600 permissions)
- **Auto-renewal**: 60 days before expiration
- **Staging mode**: For testing without rate limits (see README.md)

```bash
# View Traefik logs
make logs-traefik COMPOSE_FILE=infrastructure/docker-compose.prod.yml

# Regenerate certificates (removes acme.json first)
make deploy-regenerate-certs

# Just remove certificates without deploying
make clean-letsencrypt-certs
```

### Terraform Commands

```bash
make tf-init              # Initialize Terraform
make tf-plan              # Show execution plan
make tf-apply             # Apply changes
make tf-ip                # Get server IP
make tf-ssh               # SSH to server
make tf-connect           # Alternative SSH command
make tf-deploy-check      # Pre-deployment validation
```

## Cache Management

### OCI Layer Cache Overview

The deployment system uses OCI (Open Container Initiative) format layer caching to optimize Docker image transfers between local development and production. This significantly reduces deployment times by transferring only changed image layers instead of entire images.

**Cache Locations:**
- **Local**: `/tmp/beef-briefing-oci-cache/` (development machine)
- **Remote**: `~/beef-briefing/.oci-cache/` (production server)

**Expected Sizes:**
- Normal operations: 2-4GB (2 versions cached)
- After cleanup: Should maintain last 2 deployment versions
- Warning threshold: 5GB (automatic warning during deployment)

### Automatic Cleanup

Cache cleanup happens **automatically during normal deployments**:

```bash
make prod-deploy          # Automatic cleanup (keeps last 2 versions)
make prod-deploy-skip-cleanup  # NO cleanup (cache grows indefinitely)
```

**Important**: Use `prod-deploy-skip-cleanup` sparingly! This flag bypasses all cache cleanup and should only be used when you need to preserve multiple versions for quick rollbacks. Regular use will cause cache accumulation.

**What gets cleaned automatically:**
- Old OCI cache directories (keeps last 2 versions)
- Old Docker images on server (keeps current + previous)
- Dangling Docker images

### Manual Cache Management

**Check cache health:**
```bash
make layer-cache-health        # Quick health check (size + versions)
make layer-cache-stats         # Detailed statistics (blobs per version)
```

**Clean cache completely:**
```bash
make layer-cache-clean-remote  # Nuke entire remote cache (use for emergencies)
make layer-cache-clean         # Clean local cache
```

**Aggressive cleanup (keep only 1 version):**
```bash
make layer-cache-clean-old     # Keep only most recent version (local + remote)
```

### Troubleshooting Cache Issues

**Problem: Cache grew to 38GB**

This indicates cleanup hasn't been running. Possible causes:
1. Too many deployments with `--skip-cleanup` flag
2. Cleanup failures (check deployment logs)
3. Path resolution issues in SSH context

**Solution:**
```bash
# Emergency cleanup (removes entire cache)
make layer-cache-clean-remote

# Or SSH directly
ssh $(make -s tf-ssh-user-host) 'rm -rf ~/beef-briefing/.oci-cache'

# Then verify cleanup worked
make layer-cache-health
```

**Problem: Deployment warnings about cache size**

During deployment, if cache exceeds 5GB, you'll see:
```
WARNING: Remote OCI cache is 8GB (threshold: 5GB)
Consider running: make layer-cache-clean-remote
```

**Action**: If cleanup is running normally but cache is large, you may have:
- Very large images (check image sizes)
- Frequent deployments (many versions being kept)
- Consider using `make layer-cache-clean-old` to keep only 1 version

**Problem: Cleanup verification warnings**

If you see "WARNING: Cleanup incomplete, X versions remain (expected 2)", this indicates cleanup didn't work properly. Check:
- Disk space on server (cleanup fails if disk full)
- Permissions on cache directory
- Review deployment logs for detailed errors

### Monitoring Best Practices

**Regular health checks:**
```bash
# Before major deployment
make layer-cache-health

# After multiple deployments with --skip-cleanup
make layer-cache-health
```

**Deployment workflow:**
1. Normal deployments: Use `make prod-deploy` (automatic cleanup)
2. Risky deployments: Use `make prod-deploy-skip-cleanup` (manual cleanup later)
3. After skipped cleanup: Run `make layer-cache-health` to verify size
4. If cache > 5GB: Run `make layer-cache-clean-old` or `make layer-cache-clean-remote`

**Expected behavior:**
- After each normal deployment: Cache size may increase temporarily during transfer
- After cleanup: Should see "Cache size: XGB -> YGB" with Y ≤ X
- Steady state: 2 versions cached, total size depends on image sizes

## Environment Configuration

### Development vs Production

- **Development**: `infrastructure/.env.dev` + `docker-compose.dev.yml`
  - Uses MinIO for object storage
  - No Traefik/SSL
  - Text logging

- **Production**: `infrastructure/.env.prod` + `docker-compose.prod.yml`
  - Uses Linode Object Storage
  - Traefik with Let's Encrypt SSL
  - JSON logging
  - Secure cookies enabled

### Secrets Management

**Traefik Dashboard**:
```bash
make secrets-traefik-password
```

Updates `TRAEFIK_DASHBOARD_USERS` in `.env.prod` with bcrypt hash ($$2y$$ escaping for docker-compose).

**API Service Keys** (per-application authentication):
```bash
make secrets-service-api APP=telegram-bot
```

Generates a secure random API key for each application. Keys are stored in two locations:
- `infrastructure/secrets/apps/api-service/app_keys/{app}` - for api-service to validate incoming requests
- `infrastructure/secrets/apps/{app}/api_key` - for the app to read when making requests

**Card Renderer Keys** (for gallery access):
```bash
make secrets-card-renderer APP=ml-processor
```

Generates API keys for services that need to access the card-renderer. Keys are stored in:
- `infrastructure/secrets/apps/card-renderer/app_keys/{app}` - for card-renderer to validate incoming requests
- `infrastructure/secrets/apps/{app}/card_renderer_api_key` - for the app to read when making requests

This structure allows each container to mount its own secrets directory without collisions. All `/api/v1/*` endpoints require authentication via `Authorization: Bearer <key>` header. Only `/health` is unauthenticated (for load balancer health checks).

### OpenAI Rate Limiting (ML Processor)

The ML processor service includes built-in rate limiting for OpenAI API calls to comply with tier limits. Rate limiting is enabled by default with Tier 1 limits.

**Configuration** (in `.env.dev` or `.env.prod`):
```bash
# Enable/disable rate limiting
OPENAI_RATE_LIMIT_ENABLED=true
OPENAI_RATE_LIMIT_TIMEOUT=120.0  # Max wait time for capacity (seconds)

# gpt-4o-mini limits (sentiment, humor, questions, NER analyzers)
OPENAI_GPT4O_MINI_TPM=200000     # Tokens per minute
OPENAI_GPT4O_MINI_RPM=500        # Requests per minute

# text-embedding-3-small limits (embeddings, topics analyzers)
OPENAI_EMBEDDING_TPM=1000000
OPENAI_EMBEDDING_RPM=3000

# omni-moderation-latest limits (toxicity analyzer)
OPENAI_MODERATION_TPM=10000
OPENAI_MODERATION_RPM=500
```

**OpenAI Tier Limits Reference**:
| Tier | gpt-4o-mini TPM | gpt-4o-mini RPM | Embedding TPM | Embedding RPM |
|------|-----------------|-----------------|---------------|---------------|
| 1    | 200,000         | 500             | 1,000,000     | 3,000         |
| 2    | 2,000,000       | 5,000           | 1,000,000     | 5,000         |
| 3    | 4,000,000       | 5,000           | 5,000,000     | 5,000         |

**How it works**:
- Uses token bucket algorithm for both RPM and TPM limits per model
- Multiple analyzers sharing the same model (e.g., sentiment, humor, questions, NER all use gpt-4o-mini) coordinate through a shared rate limiter
- When limits are reached, requests wait until capacity is available (up to timeout)
- Token usage is estimated before requests and adjusted after based on actual usage

### Tier Configuration (ML Processor)

The tier system labels users based on their overall score. Tiers are configurable via environment variables.

**Configuration** (in `.env.dev` or `.env.prod`):
```bash
# Format: NAME:MIN_SCORE (ordered from highest to lowest tier)
# Users are assigned to the first tier where their score >= MIN_SCORE
TIER_1=Lendário:81
TIER_2=Bichão:77
TIER_3=CLT:72
TIER_4=Coadjuvante:55
TIER_5=Fióti:32
TIER_6=Random:10
```

| Env Var | Default | Description |
|---------|---------|-------------|
| TIER_1 | Legendary:81 | Highest tier (score >= 81) |
| TIER_2 | Elite:77 | Second tier (score >= 77) |
| TIER_3 | Outstanding:72 | Third tier (score >= 72) |
| TIER_4 | Regular:55 | Fourth tier (score >= 55) |
| TIER_5 | Beginner:32 | Fifth tier (score >= 32) |
| TIER_6 | Rookie:10 | Lowest tier (score >= 10) |

**Format**: `NAME:MIN_SCORE` where MIN_SCORE is the minimum overall score (0-100) for that tier.

### Card Theme Configuration

The default theme for card image generation is configurable via environment variable.

**Configuration** (in `.env.dev` or `.env.prod`):
```bash
# Default theme for card generation
DEFAULT_CARD_THEME=neon_arcade
```

| Env Var | Default | Description |
|---------|---------|-------------|
| DEFAULT_CARD_THEME | neon_arcade | Theme used when generating card images |

**Available themes**: gaming, clean, sticker, meme, vaporwave, blueprint, mythic, noir_luxury, neon_arcade, sticker_retro

Theme files are located in `apps/card-renderer/templates/themes/`. Each theme has:
- `theme.json` (colors/typography configuration)
- `card.html` (HTML/CSS template for 400x600 regular cards)
- `compact_card.html` (optional HTML/CSS template for 300x450 compact cards)

**Compact Cards**:
- Smaller format (300x450 pixels) designed for gallery views
- Include placeholder structures for React apps to overlay dynamic values
- Support the same theme system as regular cards
- Available for: gaming, clean, neon_arcade (and expandable to other themes)

**Compact Card Configuration**:
```bash
# Card dimensions
COMPACT_CARD_WIDTH=300
COMPACT_CARD_HEIGHT=450
```

**API Usage**:
```bash
# Request compact cards
curl -X POST http://localhost:8051/api/v1/render \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "chat_id": -1003280306634,
    "week_start": "2025-01-06",
    "card_type": "compact",
    "theme": "gaming"
  }'

# Compact cards are stored with "_compact" suffix: gaming_compact, clean_compact, etc.
# Filter results by theme:
curl -X GET "http://localhost:8051/api/v1/images?chat_id=-1003280306634&theme=gaming_compact"
```

### Ranked Tournaments Configuration

Control whether daily ranked tournaments run globally or per-group.

**Default Behavior:**
- **Groups are disabled by default** (opt-in model)
- Global setting is enabled by default
- Groups must explicitly enable ranked tournaments

**Environment Variables** (in `.env.dev` or `.env.prod`):
```bash
# Global kill switch for all ranked tournaments
RANKED_TOURNAMENTS_ENABLED=true   # Default: true
```

**Per-Group Control** (via Makefile):

*Development:*
```bash
# Enable for specific group (required to start tournaments)
make ranked-enable CHAT_ID=-1002345678901

# Disable for specific group
make ranked-disable CHAT_ID=-1002345678901

# Check status of all groups
make ranked-status

# Check status of specific group
make ranked-status-chat CHAT_ID=-1002345678901

# Enable all groups (requires confirmation)
make ranked-enable-all

# Disable all groups (requires confirmation)
make ranked-disable-all
```

*Production (requires `make pg-tunnel` in another terminal):*
```bash
# Enable for specific group (required to start tournaments)
make ranked-enable-prod CHAT_ID=-1002345678901

# Disable for specific group
make ranked-disable-prod CHAT_ID=-1002345678901

# Check status of all groups
make ranked-status-prod

# Check status of specific group
make ranked-status-chat-prod CHAT_ID=-1002345678901

# Enable all groups (requires confirmation)
make ranked-enable-all-prod

# Disable all groups (requires confirmation)
make ranked-disable-all-prod
```

**Per-Group Control** (via SQL, alternative):
```sql
-- Enable for specific group
UPDATE chats SET ranked_tournaments_enabled = true WHERE id = <chat_id>;

-- Disable for specific group
UPDATE chats SET ranked_tournaments_enabled = false WHERE id = <chat_id>;

-- Find chat ID by name
SELECT id, title FROM chats WHERE title ILIKE '%group name%';
```

Tournaments run **only if both global AND group settings are enabled**.

### Arena Mini App

Turn-based card battle arena where users build teams from weekly stats cards and compete. See [arena-mini-app README](apps/arena-mini-app/README.md) for full documentation.

**Game Economy**:
| Resource | Cost | Description |
|----------|------|-------------|
| Starting coins | 10 | Coins at match start |
| Card purchase | 3 | Buy a card from shop |
| Reroll | 1 | Refresh shop (before first buy only) |
| Upgrade | 1 | +1 ATK or +3 HP per upgrade |
| Team size | 3 | Cards required for battle |

**Polling Intervals**:
- Lobby (no match): 3s polling `/matches`
- Lobby (in match): 2s polling `/match/{id}`
- Shop: 3s polling `/shop` (continues after team submission)
- Battle/Stats: No polling (single fetch)

**Battle Response Fields**:
| Field | Type | Description |
|-------|------|-------------|
| `damage_dealt` | int | Total damage dealt by the requesting user's team |
| `damage_taken` | int | Total damage taken by the requesting user's team (opponent's damage) |
| `team_a_damage` | int | Total damage dealt by Player A's team (absolute value) |
| `team_b_damage` | int | Total damage dealt by Player B's team (absolute value) |
| `winner_id` | int64? | ID of the winning player, or null for draw |
| `is_draw` | bool | True if both teams dealt equal damage |
| `num_rounds` | int | Number of battle rounds that occurred |
| `events` | array | Detailed battle events (attacks, deaths, etc.) |

**Critical Implementation Notes**:
- **Player-relative damage**: `damage_dealt` and `damage_taken` are calculated from the requesting user's perspective. If the user is Player A, `damage_dealt` = `team_a_damage` and `damage_taken` = `team_b_damage`. Values are swapped for Player B.
- **React error #310**: Prevented by awaiting SDK init before render and initializing timer state to `0`
- **Reroll mechanic**: Permanently disabled after first card purchase (not per-round)
- **Shop polling**: Must continue after team submission to detect battle phase transition
- **Compact cards**: Use `placeholder_positions` metadata for stat overlay positioning

**Development**:
```bash
cd apps/arena-mini-app
pnpm install
pnpm run dev     # Dev server on port 5175
pnpm run build   # Production build
```

## Import CLI Usage

Import Telegram Desktop exports into the system:

```bash
# Build and deploy to production server
make go-build-import-cli-prod

# SSH to server and run
ssh $(make tf-ssh-user-host)
cd ~/beef-briefing/apps/import-cli

# Basic import (requires chat ID)
./bin/import-cli import --chat-id -1003280306634 --export-path /path/to/export

# With media files
./bin/import-cli import --chat-id -1003280306634 --export-path /path/to/export --include-media

# With bot detection (skips bot messages by default)
./bin/import-cli import --chat-id -1003280306634 --export-path /path/to/export --telegram-token YOUR_BOT_TOKEN

# Include bot messages (disable bot filtering)
./bin/import-cli import --chat-id -1003280306634 --export-path /path/to/export --telegram-token YOUR_BOT_TOKEN --skip-bots=false

# Check status
./bin/import-cli status --export-path /path/to/export
```

**Bot Detection**: The import-cli can query the Telegram API to detect bot users and skip their messages during import. This is enabled by default (`--skip-bots=true`) but requires a Telegram bot token. The token can be provided via `--telegram-token` flag or `TELEGRAM_BOT_TOKEN` environment variable. User lookups are cached to minimize API calls. Detected bots and skipped message counts are tracked in the import state and displayed in status output.

**Group Migration**: When importing supergroup exports, the `result.json` contains the old group ID. You must provide the actual supergroup ID using `--chat-id`. The CLI validates the conversion formula: `supergroup_id = -1000000000000 - old_group_id`.

## Common Development Workflows

### Adding a New Telegram Message Type

1. Update database schema in `apps/api-service/internal/migrations/sql/`
2. Add corresponding structs in `apps/api-service/internal/models/telegram.go`
3. Update repository layer in `apps/api-service/internal/repository/`
4. Update service layer in `apps/api-service/internal/services/ingest_service.go`
5. Test with curl using multipart form data

### Adding New Environment Variables

1. Add to both `.env.dev.example` and `.env.prod.example`
2. Update docker-compose files if needed
3. Update Terraform variables in `infrastructure/terraform/variables.tf` if infrastructure-related
4. Update relevant Go service config structs
5. Document in service README.md

## Testing the System

### Local Development Testing

```bash
# Generate API keys first (required before starting services)
make secrets-service-api APP=telegram-bot

# Start all services
make up-build

# Test API health (unauthenticated)
curl http://localhost:8080/health

# Read API key for testing
API_KEY=$(cat infrastructure/secrets/apps/telegram-bot/api_key)

# Test message ingestion (authenticated)
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Authorization: Bearer $API_KEY" \
  -F 'update={"update_id":1,"message":{"message_id":1,"chat":{"id":-100123,"type":"supergroup"},"from":{"id":456,"first_name":"User"},"date":1733611200,"text":"Test"}}'

# Test without auth (should return 401)
curl -X POST http://localhost:8080/api/v1/ingest \
  -F 'update={}'
# Returns: {"error": "missing authorization header"}

# Test Telegram bot
# Send a message to your bot in Telegram
# Check logs: make logs-bot
```

### Testing Patterns

For comprehensive testing documentation, see [TESTING_GUIDELINES.md](TESTING_GUIDELINES.md).

**Quick Reference:**
```bash
cd apps/api-service

go test ./...              # Run all tests (parallel)
go test -p 1 ./...         # Sequential (catches race conditions)
go test -race ./...        # With race detector
go test -v -run TestName ./internal/repository  # Specific test
```

**Three-Layer Testing Strategy:**
- **Repository**: Real PostgreSQL with transaction rollback isolation
- **Service**: Mocked repositories via interfaces
- **Handler**: HTTP testing with `httptest` and mocked services

**Test Utilities**: `apps/api-service/internal/testutil/` provides fixtures, mocks, and database helpers.

## Project Structure

```
beef-briefing/
├── apps/
│   ├── api-service/       # REST API for Telegram data ingestion (includes embedded migrations)
│   ├── telegram-bot/      # Telegram bot client
│   ├── card-renderer/     # Card image renderer (Python/Playwright)
│   ├── ml-processor/      # ML pipeline for message analysis
│   ├── leaderboard-mini-app/  # Telegram Mini App for leaderboard
│   ├── deck-mini-app/     # Telegram Mini App for deck viewing
│   ├── arena-mini-app/    # Telegram Mini App for card battle arena
│   └── import-cli/        # CLI for importing Telegram exports
├── infrastructure/
│   ├── docker-compose.dev.yml     # Development environment
│   ├── docker-compose.prod.yml    # Production with Traefik
│   ├── terraform/                 # Linode infrastructure as code
│   ├── secrets/                   # Secrets directory (gitignored)
│   │   └── apps/
│   │       ├── api-service/app_keys/  # API keys for validation
│   │       └── telegram-bot/          # telegram-bot's API key
│   └── letsencrypt/               # SSL certificates (gitignored)
├── pkg/config/            # Shared configuration package
├── scripts/               # Deployment and utility scripts
├── Makefile              # Build automation
└── CLAUDE.md             # This file
```

## Important Notes

### Media Storage

- **Content-addressable**: Files stored by SHA256 hash
- **Deduplication**: Hash checked across multiple tables before upload
- **Path format**: `{mediaType}/{hash[:2]}/{hash}`
- **Development**: MinIO at localhost:9000
- **Production**: Linode Object Storage (credentials from Terraform)

### Telegram Bot Permissions

The bot must be **admin** in groups to receive:
- All message types
- Message reactions (`message_reaction`)
- Reaction counts (`message_reaction_count`)
- Chat member updates (`my_chat_member` for group migration detection)

### Database Migrations

Migrations are embedded in `apps/api-service/internal/migrations/sql/`. They run automatically when the API service starts, tracked via a `schema_migrations` table.

### Logging Standards

All Go services use `log/slog`:
- **Development**: Text format with DEBUG level
- **Production**: JSON format with INFO level
- Set via `ENVIRONMENT` and `LOG_LEVEL` environment variables

### Traefik Configuration

Production routing rules in `docker-compose.prod.yml`:
- Traefik dashboard: `https://yourdomain.com/dashboard` (basic auth)
- All HTTP traffic redirects to HTTPS
- Let's Encrypt certificates stored in `infrastructure/letsencrypt/acme.json`

### Cross-Compilation

When building binaries for production server (may differ from dev architecture):
```bash
make go-build-import-cli-prod  # Auto-detects remote arch and cross-compiles
make tf-arch                   # Show remote architecture
```