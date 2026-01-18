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
| Upgrade | 1 | +3 ATK or +3 HP per upgrade |
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

The codebase uses comprehensive test patterns across three layers: Repository, Service, and Handler. Below are the established patterns for writing new tests.

#### Running Tests

```bash
cd apps/api-service

# Run all tests (parallel by default)
go test ./...

# Run tests sequentially (to catch flakiness/race conditions)
go test -p 1 ./...

# Run with race detector enabled
go test -race ./...

# Run specific test package
go test -v ./internal/repository

# Run specific test
go test -v -run TestFunctionName ./internal/repository

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
go tool cover -func=coverage.out | grep "total:"
```

#### Repository Layer Testing

Repository tests use **real PostgreSQL database** with transaction-based isolation. This ensures tests are realistic and catch database-specific issues.

**Key Patterns:**

1. **Database Setup & Teardown**:
   ```go
   func TestRepositoryMethod(t *testing.T) {
       db := testutil.SetupTestDB()
       defer testutil.TeardownTestDB(db)

       // Test code...
   }
   ```

2. **Transaction Isolation**:
   ```go
   tx := db.BeginTx(context.Background(), nil)
   defer tx.Rollback()

   repo := repository.NewUserRepository(tx)
   // Test using repo
   // Transaction automatically rolls back, no cleanup needed
   ```

3. **Direct Database Verification**:
   ```go
   // After operation, verify directly in DB
   var user User
   err := db.QueryRow("SELECT id, first_name FROM users WHERE id = $1", userID).
       Scan(&user.ID, &user.FirstName)
   if err != nil {
       t.Fatalf("User not found in database: %v", err)
   }
   ```

4. **NULL Value Handling**:
   - Empty strings (`""`) are stored as SQL `NULL` for optional fields
   - Use `sql.NullString` for fields that might be NULL
   - Test both populated and NULL scenarios

5. **Table-Driven Tests**:
   ```go
   tests := []struct {
       name    string
       input   interface{}
       want    interface{}
       wantErr bool
   }{
       {"Case 1", inputA, expectedA, false},
       {"Case 2", inputB, expectedB, true},
   }

   for _, tt := range tests {
       t.Run(tt.name, func(t *testing.T) {
           // Test logic
       })
   }
   ```

**Example Test Files**:
- `internal/repository/helpers_test.go` - NULL conversion functions (100% coverage)
- `internal/repository/user_repo_test.go` - User CRUD operations (83.3% coverage)
- `internal/repository/message_repo_test.go` - Message operations (93.5% coverage)
- `internal/repository/media_repo_test.go` - Media handling (81.4% coverage)

#### Service Layer Testing

Service tests use **mocked repositories** via interfaces. This isolates business logic from database concerns.

**Key Patterns:**

1. **Mock Setup**:
   ```go
   mockRepo := new(MockRepository)
   service := NewService(mockRepo)
   ```

2. **Mock Expectations**:
   ```go
   mockRepo.On("GetUser", mock.MatchedBy(func(ctx context.Context) bool {
       return true
   }), userID).Return(&user, nil)

   // Test code

   mockRepo.AssertExpectations(t)
   ```

3. **Error Scenarios**:
   ```go
   mockRepo.On("GetUser", mock.Anything, -1).
       Return(nil, errors.New("user not found"))
   ```

4. **Dependency Injection**:
   ```go
   type Service struct {
       repo RepositoryInterface  // Interface, not concrete type
   }

   func NewService(repo RepositoryInterface) *Service {
       return &Service{repo: repo}
   }
   ```

**Example Test Files**:
- `internal/services/ml_service_test.go` - ML processing (87.8% coverage)
- `internal/services/match_service_test.go` - Game match logic
- `internal/services/tournament_service_test.go` - Tournament management

#### Handler Layer Testing

Handler tests use **HTTP testing** with mocked services via `httptest`.

**Key Patterns:**

1. **HTTP Request/Response**:
   ```go
   req, _ := http.NewRequest("GET", "/api/v1/cards/123?chat_id=456", nil)
   rec := httptest.NewRecorder()

   handler.GetCard(rec, req)

   if rec.Code != http.StatusOK {
       t.Errorf("Expected 200, got %d", rec.Code)
   }
   ```

2. **Mock Service Setup**:
   ```go
   mockService := new(MockCardService)
   handler := NewCardHandler(mockService)
   ```

3. **JSON Response Validation**:
   ```go
   var resp CardResponse
   json.Unmarshal(rec.Body.Bytes(), &resp)
   ```

4. **Authentication Testing**:
   ```go
   // Test missing auth header
   req, _ := http.NewRequest("POST", "/api/v1/ingest", nil)
   rec := httptest.NewRecorder()
   handler.IngestUpdate(rec, req)
   // Should return 401
   ```

**Example Test Files**:
- `internal/handlers/card_handler_test.go` - Card endpoints (76.5% coverage)
- `internal/handlers/arena_handler_test.go` - Arena game endpoints (59% coverage)
- `internal/handlers/mini_app_handler_test.go` - Mini app endpoints

#### Test Utilities (testutil package)

The `testutil` package (`apps/api-service/internal/testutil/`) provides comprehensive testing utilities organized into four files:

**1. Database Utilities** (`db.go`):
- `SetupTestDB(t *testing.T) *TestDB` - Create test database connection
  - Reads configuration from environment variables (TEST_DB_HOST, TEST_DB_PORT, etc.)
  - Sets up connection pool with reasonable defaults (5 max connections, 2 idle)
  - Verifies connection with Ping()
- `TeardownTestDB(t *testing.T, tdb *TestDB)` - Close test database connection gracefully
- `WithTestTransaction(t *testing.T, db *sql.DB, fn func(tx *sql.Tx))` - Execute test code in automatic rollback transaction
  - Perfect for test isolation without modifying database
  - Panics are captured and cleanup is guaranteed
- `WithTestTransactionContext(ctx context.Context, t *testing.T, db *sql.DB, fn func(ctx context.Context, tx *sql.Tx))` - Same as WithTestTransaction but accepts context for timeouts/cancellation
- `CleanupTables(t *testing.T, tx *sql.Tx, tables ...string)` - Truncate tables within a transaction
  - Uses CASCADE to handle foreign key constraints
- `DBTX` interface - Satisfied by both *sql.DB and *sql.Tx for flexible code

**Usage Pattern**:
```go
func TestSomething(t *testing.T) {
    tdb := testutil.SetupTestDB(t)
    defer testutil.TeardownTestDB(t, tdb)

    testutil.WithTestTransaction(t, tdb.DB, func(tx *sql.Tx) {
        repo := repository.NewUserRepository(tx)
        // Test code using repo
        // Changes will be rolled back after this function returns
    })
}
```

**2. Sample Data Fixtures** (`fixtures.go`):
Helper functions that return valid test data with sensible defaults. All functions are customizable via variants:

*User Data*:
- `SampleUser() User` - Returns basic test user
- `SampleUserWithID(id int64) User` - Custom user ID
- `SampleBotUser() User` - Bot user for testing bot-specific logic

*Chat Data*:
- `SampleChat() Chat` - Returns supergroup (most common type)
- `SampleChatWithID(id int64) Chat` - Custom chat ID
- `SamplePrivateChat() Chat` - Private chat for 1-on-1 testing

*Message Data*:
- `SampleMessage() Message` - Text message from sample user in sample chat
- `SampleMessageWithID(id int64) Message` - Custom message ID
- `SampleMessageWithText(text string) Message` - Custom message text
- `SamplePhotoMessage() Message` - Message with photo attachment (multiple sizes)

*Telegram Update Data*:
- `SampleTelegramUpdate() Update` - Standard text message update
- `SampleTelegramUpdateWithID(id int64) Update` - Custom update ID
- `SampleEditedMessageUpdate() Update` - Update with edited message
- `SampleReactionUpdate() Update` - Message reaction update (e.g., 👍)
- `SampleReactionCountUpdate() Update` - Reaction count aggregation update

*Arena Game Data*:
- `SampleMatch() Match` - Regular match in "open" status
- `SampleMatchWithID(id string) Match` - Custom match ID
- `SampleMatchWithStatus(status MatchStatus) Match` - Custom match status
- `SampleRankedMatch() Match` - Ranked tournament match
- `SampleShopPhaseMatch() Match` - Match in active shop phase
- `SampleBattlePhaseMatch() Match` - Match in active battle phase
- `SampleCompletedMatch() Match` - Completed match with winner
- `SampleParticipant(matchID, userID) Participant` - Match participant
- `SampleParticipantWithTeam(matchID, userID) Participant` - Participant who submitted team

**3. Mock Services & Storage** (`mocks.go`):

*MinIO Storage Mock* - `MockMinIOClient`:
- Implements `storage.MinIOClientInterface` for testing file uploads
- Stores uploaded data in memory (thread-safe)
- Methods:
  - `UploadMedia(ctx, fileID, data, mimeType, mediaType) (objectKey, fileHash, error)` - Mock file upload
  - `GetObject(ctx, objectKey) (reader, size, contentType, error)` - Retrieve stored object
  - `GetObjectURL(objectKey) string` - Generate mock object URL
  - `GetPresignedURL(ctx, objectKey, expiry) (url, error)` - Generate presigned URL with expiry
  - `GetPresignedURLSeconds(ctx, objectKey, expirySeconds) (url, error)` - Presigned URL with seconds
- Configuration:
  - `PresignedURLBase` - Customize base URL (default: "https://mock-storage.example.com")
  - `Storage map[string]*MockObject` - Access uploaded objects directly
  - `UploadError`, `GetObjectError`, `GetPresignedURLError` - Error injection
- Tracking:
  - `UploadCalls`, `GetObjectCalls`, `GetPresignedURLCalls` - Call counters
  - `NextUploadObjectKey`, `NextUploadFileHash`, `NextPresignedURL` - Configurable responses
- Methods:
  - `Reset()` - Clear all storage and reset counters
  - `SetUploadResult(objectKey, fileHash, err)` - Configure next upload response
  - `SetPresignedURL(url, err)` - Configure next presigned URL
  - `AddObject(objectKey, data, contentType)` - Directly add object for setup

*New Relic Mocks*:
- `MockNewRelicApp` - Mock New Relic application
  - `RecordCustomEvent(eventType, params) error` - Track custom events
  - `Shutdown(timeout)` - Graceful shutdown
  - Access recorded events: `GetCustomEvents()`, `GetCustomEventsByType()`, `CustomEventCount()`
- `MockNewRelicTransaction` - Mock transaction for segment/error tracking
  - `StartSegment(name) *MockSegmentHandle` - Start a named segment
  - `NoticeError(err)` - Record an error
  - `End()` - Mark transaction as ended

*Shop Mock* - `MockDealer`:
- Implements `shop.DealerInterface` for card dealing
- Methods:
  - `GetCardCount(ctx, chatID) (count, error)` - Mock card pool size
  - `DealCards(ctx, chatID, count) ([]*ShopCard, error)` - Deal cards from pool
- Configuration:
  - `CardCount` - Set available card count
  - `GetCardCountError`, `DealCardsError` - Error injection
- Tracking:
  - `GetCardCountCalls`, `DealCardsCalls` - Call counters
  - `SetCardCount(count)` - Configure card count
  - `Reset()` - Clear state

*Ingest Service Mock* - `MockIngestService`:
- Mock for handler layer testing
- Methods:
  - `ProcessUpdate(ctx, update, files) error` - Mock update processing
  - `SetProcessUpdateError(err)` - Configure error response
- Tracking:
  - `ProcessUpdateCalled` - Was method called?
  - `LastUpdate` - Last update received
  - `LastFiles` - Last files map received

**4. Mock Repositories** (`mock_repositories.go`):

*MockGameRepository* - Complete in-memory game state for testing:
- **Match Storage**: `Matches map[string]*Match`
- **Participant Storage**: `Participants map[string]map[int64]*Participant`
- **Round Storage**: `Rounds map[string][]*MatchRound`
- **Tournament Storage**: `Tournaments map[int64]*RankedTournament`
- **Tournament Participants**: `TournamentParticipants map[int64]map[int64]*TournamentParticipant`

*Methods*:
```go
// Match methods
CreateMatch(ctx, match) error
GetMatch(ctx, matchID) (*Match, error)
GetActiveMatches(ctx, chatID, format, status) ([]*Match, error)
StartShopPhase(ctx, matchID) error
StartBattlePhase(ctx, matchID) error
CompleteMatch(ctx, matchID, winnerID) error

// Participant methods
AddParticipant(ctx, matchID, userID) error
GetParticipant(ctx, matchID, userID) (*Participant, error)
RemoveParticipant(ctx, matchID, userID) error
SubmitTeam(ctx, matchID, userID, teamSize) error
UpdateParticipantShop(ctx, matchID, userID, coins, cards) error

// Tournament methods
GetOrCreateTournament(ctx, chatID, date) (*RankedTournament, error)
GetTournamentByID(ctx, id) (*RankedTournament, error)
AddTournamentParticipant(ctx, tournamentID, userID) error
RemoveTournamentParticipant(ctx, tournamentID, userID) error
CloseTournamentRegistration(ctx, tournamentID) error
CompleteTournament(ctx, tournamentID) error
```

*Features*:
- Thread-safe with mutex protection
- Full state persistence in memory
- Comprehensive error injection for all methods
- Call tracking for all operations (useful for verifying behavior)
- `Reset()` - Clear all state and reset counters
- Suitable for testing entire game workflows

**Example Usage**:
```go
func TestShopService(t *testing.T) {
    mockRepo := testutil.NewMockGameRepository()
    mockRepo.CreateMatchError = nil // No errors

    service := NewShopService(mockRepo)

    // Create a match using the service
    match, _ := service.CreateMatch(ctx, &Match{...})

    // Verify the mock was called
    if mockRepo.CreateMatchCalls != 1 {
        t.Errorf("Expected CreateMatch to be called once")
    }

    // Verify state was stored
    if stored, ok := mockRepo.Matches[match.ID]; !ok {
        t.Error("Match not found in repository")
    }
}
```

#### Coverage Targets by Layer

| Layer | Target | Current | Method |
|-------|--------|---------|--------|
| Repository | 65%+ | 44.3% | Real DB + transactions |
| Service | 70%+ | 54.0% | Mocked repositories |
| Handler | 75%+ | 63.9% | HTTP testing |
| **Overall** | **70%** | **39.3%** | Multi-layer integration |

#### Test Quality Standards

1. **Independence**: Tests must not depend on execution order. Can run in any order, including with `-p 1` (sequential).

2. **Cleanup**: All tests properly clean up after themselves:
   - Repository tests: Transaction rollback
   - Service tests: Mock expectations assertion
   - Handler tests: Response cleanup

3. **Isolation**: No shared state between tests. Each test is self-contained.

4. **Determinism**: Tests produce consistent results across multiple runs (checked with `-race` flag).

5. **Speed**: Tests run efficiently:
   - Repository layer: ~0.5-1.0s per package
   - Service layer: ~0.1-0.5s per package
   - Handler layer: ~0.2-1.0s per package
   - Total: ~30-60s parallel execution

#### Adding New Tests

When adding tests for a new feature or bug fix:

1. **Identify the layer**: Repository (data access), Service (business logic), or Handler (HTTP)
2. **Choose test type**:
   - **Repository**: Real DB with transaction rollback
   - **Service**: Mocked repository interfaces
   - **Handler**: HTTP test with mocked service
3. **Use table-driven format** for multiple similar test cases
4. **Test happy path + error scenarios**
5. **Verify with assertions** (direct queries for repository, mock assertions for service/handler)
6. **Run with `-race` flag** to catch concurrency issues
7. **Document edge cases** in test names (`TestFunctionName_EdgeCaseDescription`)

**Template for Repository Test**:
```go
func TestUpsertUser_NewUser(t *testing.T) {
    db := testutil.SetupTestDB()
    defer testutil.TeardownTestDB(db)

    tx := db.BeginTx(context.Background(), nil)
    defer tx.Rollback()

    repo := repository.NewUserRepository(tx)
    user := testutil.SampleUser()

    err := repo.UpsertUser(context.Background(), user)
    if err != nil {
        t.Fatalf("UpsertUser failed: %v", err)
    }

    // Verify in database
    var retrieved User
    row := tx.QueryRow("SELECT id, first_name FROM users WHERE id = $1", user.ID)
    if err := row.Scan(&retrieved.ID, &retrieved.FirstName); err != nil {
        t.Fatalf("User not in database: %v", err)
    }
}
```

**Template for Service Test**:
```go
func TestGetUserCard_Happy(t *testing.T) {
    mockRepo := new(MockRepository)
    mockRepo.On("GetUserCard", mock.Anything, userID).
        Return(&card, nil)

    service := NewCardService(mockRepo)
    result, err := service.GetUserCard(context.Background(), userID)

    if err != nil {
        t.Fatalf("GetUserCard failed: %v", err)
    }
    mockRepo.AssertExpectations(t)
}
```

**Template for Handler Test**:
```go
func TestGetCard_Success(t *testing.T) {
    mockService := new(MockCardService)
    mockService.On("GetUserCard", mock.Anything, userID).
        Return(&card, nil)

    handler := NewCardHandler(mockService)
    req, _ := http.NewRequest("GET", "/cards/123", nil)
    rec := httptest.NewRecorder()

    handler.GetUserCard(rec, req)

    if rec.Code != http.StatusOK {
        t.Errorf("Expected 200, got %d", rec.Code)
    }
}
```

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
DISTILLED_AESTHETICS_PROMPT = """
<frontend_aesthetics>
You tend to converge toward generic, "on distribution" outputs. In frontend design, this creates what users call the "AI slop" aesthetic. Avoid this: make creative, distinctive frontends that surprise and delight. Focus on:

Typography: Choose fonts that are beautiful, unique, and interesting. Avoid generic fonts like Arial and Inter; opt instead for distinctive choices that elevate the frontend's aesthetics.

Color & Theme: Commit to a cohesive aesthetic. Use CSS variables for consistency. Dominant colors with sharp accents outperform timid, evenly-distributed palettes. Draw from IDE themes and cultural aesthetics for inspiration.

Motion: Use animations for effects and micro-interactions. Prioritize CSS-only solutions for HTML. Use Motion library for React when available. Focus on high-impact moments: one well-orchestrated page load with staggered reveals (animation-delay) creates more delight than scattered micro-interactions.

Backgrounds: Create atmosphere and depth rather than defaulting to solid colors. Layer CSS gradients, use geometric patterns, or add contextual effects that match the overall aesthetic.

Avoid generic AI-generated aesthetics:
- Overused font families (Inter, Roboto, Arial, system fonts)
- Clichéd color schemes (particularly purple gradients on white backgrounds)
- Predictable layouts and component patterns
- Cookie-cutter design that lacks context-specific character

Interpret creatively and make unexpected choices that feel genuinely designed for the context. Vary between light and dark themes, different fonts, different aesthetics. You still tend to converge on common choices (Space Grotesk, for example) across generations. Avoid this: it is critical that you think outside the box!
</frontend_aesthetics>
"""