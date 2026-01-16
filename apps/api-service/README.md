# API Service

Central REST API for ingesting Telegram data and serving analytics endpoints.

## Overview

The API Service is the hub of the Beef Briefing system. It receives messages from the Telegram Bot, stores data in PostgreSQL, manages media files in MinIO, and serves endpoints for ML processing, user cards, and Mini Apps.

## Features

- **Multipart Ingestion**: Accepts JSON metadata + binary file uploads in a single request
- **Content-Addressable Storage**: Deduplicates media files using SHA256 hashing
- **Complete Telegram Support**: All message types including text, media, stickers, polls, reactions
- **JWT Authentication**: Secure Mini App endpoints with Telegram init_data validation
- **API Key Authentication**: Service-to-service authentication for internal endpoints

## Quick Start

```bash
# Start with Docker (recommended)
make up-build

# Or run locally
cd apps/api-service
go mod download
go run ./cmd
```

**Access Points:**
- API: http://localhost:8080
- Health: http://localhost:8080/health

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `API_PORT` | `8080` | Service port |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | Database user |
| `DB_PASSWORD` | `` | Database password |
| `DB_NAME` | `beef_briefing` | Database name |
| `MINIO_ENDPOINT` | `localhost:9000` | MinIO endpoint |
| `MINIO_ACCESS_KEY` | `minioadmin` | MinIO access key |
| `MINIO_SECRET_KEY` | `minioadmin` | MinIO secret key |
| `MINIO_BUCKET` | `telegram-media` | Storage bucket |
| `JWT_SECRET_KEY` | `` | JWT signing key (enables Mini App auth) |
| `CORS_ORIGINS` | `` | Allowed CORS origins |
| `APP_KEYS_DIR` | `` | Directory containing API keys |
| `ENVIRONMENT` | `development` | `development` or `production` |
| `LOG_LEVEL` | `info` | Log level |
| `MAX_UPLOAD_SIZE_MB` | `100` | Max upload size in MB |

## Authentication

### API Key (Internal Services)

All `/api/v1/*` endpoints (except Mini App) require API key authentication:

```bash
curl -H "Authorization: Bearer $(cat infrastructure/secrets/apps/telegram-bot/api_key)" \
  http://localhost:8080/api/v1/ml/status
```

Generate keys with: `make secrets-service-api APP=<app-name>`

### JWT (Mini Apps)

Mini App endpoints (`/api/v1/mini-app/*`) use JWT tokens:

1. POST `/api/v1/mini-app/auth` with Telegram `init_data`
2. Receive JWT token (valid 24 hours)
3. Include in subsequent requests: `Authorization: Bearer <token>`

## API Reference

### Health Check

#### GET `/health`

```bash
curl http://localhost:8080/health
# Returns: OK
```

---

### Ingest Endpoints

#### POST `/api/v1/ingest`

Ingest Telegram updates with optional media files.

**Content-Type**: `multipart/form-data`

**Fields:**
- `update` (required): JSON Telegram Update object
- `{file_id}` (optional): Binary file content keyed by Telegram file_id

```bash
# Text message
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Authorization: Bearer $API_KEY" \
  -F 'update={"update_id":1,"message":{"message_id":1,"chat":{"id":-100123,"type":"supergroup"},"from":{"id":456,"first_name":"User"},"date":1733611200,"text":"Hello!"}}'

# With media
curl -X POST http://localhost:8080/api/v1/ingest \
  -H "Authorization: Bearer $API_KEY" \
  -F 'update={"update_id":2,"message":{"message_id":2,"chat":{"id":-100123},"from":{"id":456},"date":1733611300,"document":{"file_id":"ABC123","file_name":"doc.pdf"}}}' \
  -F 'ABC123=@doc.pdf'
```

---

### ML Endpoints

#### GET `/api/v1/ml/messages`

Fetch unprocessed messages for ML analysis.

**Parameters:**
- `limit` (optional, default: 500, max: 1000)

```bash
curl -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v1/ml/messages?limit=100"
```

#### POST `/api/v1/ml/results`

Submit ML analysis results.

```bash
curl -X POST http://localhost:8080/api/v1/ml/results \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "results": [{
      "message_id": 12345,
      "chat_id": -1003280306634,
      "sentiment": {"label": "positive", "scores": {"positive": 0.85}},
      "toxicity": {"is_toxic": false, "score": 0.95}
    }],
    "processor_version": "v1.0.0"
  }'
```

#### GET `/api/v1/ml/status`

Get ML processing statistics.

```bash
curl -H "Authorization: Bearer $API_KEY" http://localhost:8080/api/v1/ml/status
```

---

### Profile Photo Endpoints

#### POST `/api/v1/profile-photos/user`

Upload user profile photos (multipart form with `metadata` + photo files).

#### POST `/api/v1/profile-photos/chat`

Upload chat profile photos.

#### GET `/api/v1/users/{id}/photo`

Get presigned URL for user profile photo.

**Parameters:** `size` (optional): `small`, `medium`, `large`

#### GET `/api/v1/chats/{id}/photo`

Get presigned URL for chat profile photo.

---

### Cards Endpoints

#### GET `/api/v1/cards`

Get all cards for a chat (leaderboard view).

**Parameters:**
- `chat_id` (required)
- `week` (optional): Week start date (YYYY-MM-DD)
- `sort_by` (optional): `mood`, `influence`, `activity`, `reactions`
- `order` (optional): `asc`, `desc`
- `limit` (optional, default: 50)
- `offset` (optional, default: 0)

```bash
curl -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v1/cards?chat_id=-1003280306634&sort_by=mood"
```

#### GET `/api/v1/cards/{user_id}`

Get a single user's card for a specific week.

#### GET `/api/v1/cards/{user_id}/history`

Get a user's card history across multiple weeks.

#### GET `/api/v1/cards/weeks`

Get available weeks with generated cards.

#### GET `/api/v1/cards/{user_id}/image`

Get presigned URL for a user's card image.

**Parameters:**
- `chat_id` (required)
- `week` (optional)
- `theme` (optional)
- `expires` (optional, 60-86400 seconds)

---

### Mini App Endpoints

JWT-protected endpoints for Telegram Mini Apps.

#### POST `/api/v1/mini-app/auth`

Exchange Telegram init_data for JWT token.

```bash
curl -X POST http://localhost:8080/api/v1/mini-app/auth \
  -H "Content-Type: application/json" \
  -d '{"init_data": "<telegram_init_data>"}'
```

**Response:**
```json
{
  "user_id": 123456789,
  "token": "eyJhbGciOiJIUzI1NiI...",
  "chat_id": -1001234567890,
  "expires_in": 86400
}
```

#### GET `/api/v1/mini-app/stats`

Get chat overview statistics.

**Parameters:** `chat_id`, `period` (optional: `7d`, `30d`, `90d`, `all`)

#### GET `/api/v1/mini-app/activity`

Get daily activity timeline.

#### GET `/api/v1/mini-app/leaderboard`

Get user rankings.

**Parameters:**
- `chat_id` (required)
- `period` (optional)
- `metric` (optional): `message_count`, `reactions_sent`, `reactions_received`, `active_days`
- `page`, `limit` (pagination)

#### GET `/api/v1/mini-app/media-overview`

Get media statistics including type distribution, timeline, and top senders.

**Parameters:**
- `chat_id` (required)
- `period` (optional): `7d`, `30d`, `90d`, `all` (default: `30d`)
- `limit` (optional): Top senders count, 1-50 (default: `10`)
- `tz` (optional): Timezone for date grouping

**Response:** Stats (with trends), distribution by type, daily activity timeline, top media senders.

#### GET `/api/v1/mini-app/gallery/weeks`

List weeks with available card images.

#### GET `/api/v1/mini-app/gallery/images`

Get card images for a specific week.

#### GET `/api/v1/mini-app/gallery/image/{id}`

Get presigned URL for a specific card image.

---

### Utility Endpoints

#### GET `/api/v1/users`

List all user IDs.

#### GET `/api/v1/chats`

List all chat IDs.

## Database Schema

22 tables modeling the complete Telegram data structure:

| Category | Tables |
|----------|--------|
| Core | `chats`, `users`, `updates` |
| Messages | `messages`, `message_entities`, `message_edits` |
| Reactions | `message_reactions`, `reaction_counts` |
| Media | `media_files`, `photos`, `stickers`, `games`, `game_photos` |
| Other | `polls`, `poll_options`, `contacts`, `locations`, `venues`, `dice` |

Migrations are embedded and run automatically on startup.

## Architecture

```
apps/api-service/
├── cmd/main.go                 # Entry point
├── internal/
│   ├── handlers/               # HTTP handlers
│   ├── middleware/             # Auth, CORS middleware
│   ├── models/                 # Domain models
│   ├── repository/             # Database access
│   ├── services/               # Business logic
│   └── migrations/sql/         # Embedded migrations
└── Dockerfile
```

## Concurrency Patterns

The API service uses several concurrency patterns for thread-safe operation. All patterns have been verified with Go's race detector (`go test -race`).

### 1. Per-Match Mutex (Battle Coordination)

**Location:** `internal/services/battle_service.go:25`

When multiple participants submit their teams simultaneously, the `CheckAndStartBattle` method uses per-match mutexes to prevent race conditions:

```go
type BattleService struct {
    matchMutexes sync.Map // map[string]*sync.Mutex for per-match locking
}

func (s *BattleService) CheckAndStartBattle(ctx context.Context, matchID string) {
    mutexInterface, _ := s.matchMutexes.LoadOrStore(matchID, &sync.Mutex{})
    mutex := mutexInterface.(*sync.Mutex)
    mutex.Lock()
    defer mutex.Unlock()
    // ... check and start battle
}
```

**Why sync.Map:** Each match needs its own lock, but we don't know which matches will be active. `sync.Map` provides efficient concurrent access with `LoadOrStore` for atomic "get or create" semantics.

**Cleanup:** Mutex entries are intentionally NOT deleted after use. Deleting while other goroutines may be waiting on `LoadOrStore` creates a race condition. Since match IDs are unique UUIDs and battles only happen once, the memory overhead is negligible.

### 2. Background Goroutine with Timeout

**Location:** `internal/services/arena_shop_delegation.go:46-50`

When a player submits their team, we check if all players are ready and potentially start the battle. This happens asynchronously to avoid blocking the HTTP response:

```go
func (s *ArenaService) SubmitTeam(ctx context.Context, matchID string, userID int64) (*EnhancedShopResponse, error) {
    // ... submit team

    // Create detached context with timeout for background work
    asyncCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    go func() {
        defer cancel()
        s.checkAndStartBattle(asyncCtx, matchID)
    }()

    return s.shopService.GetShop(ctx, matchID, userID)
}
```

**Why detached context:** The HTTP request context may be cancelled after the response is sent. Using `context.Background()` with a timeout ensures the background work completes independently.

**Timeout:** 10-second timeout prevents goroutines from hanging indefinitely if something goes wrong.

### 3. Transaction Helper (Database Operations)

**Location:** `internal/dbutil/transaction.go`

A helper function encapsulates the commit/rollback pattern with panic recovery:

```go
func WithTransaction(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer func() {
        if p := recover(); p != nil {
            tx.Rollback()
            panic(p)
        }
    }()
    if err := fn(tx); err != nil {
        tx.Rollback()
        return err
    }
    return tx.Commit()
}
```

**Panic recovery:** If the callback panics, the transaction is rolled back before re-panicking. This prevents leaving transactions in an inconsistent state.

### 4. Service-Level Transaction Management

Services that need atomic operations manage their own transactions:

**Pattern 1: Defer Rollback** (`internal/services/ingest_service.go:104-142`)
```go
tx, err := s.db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback() // Safe: no-op after successful commit

// ... operations using tx ...

return tx.Commit()
```

**Pattern 2: Optional External Transaction** (`internal/repository/ml_repo.go`)
```go
func (r *MLRepo) StoreSentiment(ctx context.Context, results []SentimentResult, dbtx DBTX) error {
    // If no external transaction provided, create internal one
    if dbtx == nil {
        tx, err := r.db.BeginTx(ctx, nil)
        if err != nil {
            return err
        }
        defer tx.Rollback()
        dbtx = tx
        // ... operations ...
        return tx.Commit()
    }
    // Use provided transaction (caller manages commit/rollback)
    // ... operations ...
    return nil
}
```

This allows both standalone use and composition into larger transactions.

### Thread Safety Summary

| Component | Pattern | Purpose |
|-----------|---------|---------|
| `BattleService.matchMutexes` | `sync.Map` of `*sync.Mutex` | Serialize battle start per match |
| `SubmitTeam` | Background goroutine + timeout | Non-blocking async battle check |
| `WithTransaction` | Defer + panic recovery | Safe transaction lifecycle |
| Repository methods | DBTX interface | Flexible transaction composition |

### No Deadlocks

The codebase has been audited for deadlock potential:

1. **Single lock family:** Only `matchMutexes` exists; no AB/BA lock ordering possible
2. **Short transactions:** All DB transactions complete quickly (no long-running locks)
3. **No nested transactions:** Transactions don't call methods that start new transactions
4. **Context timeouts:** All background work has timeout protection

## Troubleshooting

### 401 Unauthorized

- Check API key is valid: `cat infrastructure/secrets/apps/<app>/api_key`
- Ensure `Authorization: Bearer <key>` header is set
- For Mini Apps, verify JWT token hasn't expired

### Media Upload Failures

- Check file size is under `MAX_UPLOAD_SIZE_MB`
- Verify MinIO is running: `docker ps | grep minio`
- Check MinIO credentials in environment

### Database Connection Issues

- Verify PostgreSQL is running: `docker ps | grep postgres`
- Check connection string in logs
- Ensure migrations have run (check for `schema_migrations` table)
