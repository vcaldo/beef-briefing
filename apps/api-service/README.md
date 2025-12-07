# Telegram Data Ingestion API

A Go-based REST API service for ingesting and storing Telegram group chat data, including messages, reactions, media files, and locations.

## Features

- **Message Ingestion**: Captures text messages, media messages, voice messages, and locations
- **Reaction Tracking**: Tracks both individual reactions and aggregate reaction counts
- **Media Storage**: Downloads media from Telegram and stores in MinIO object storage
- **Edit History**: Maintains full audit trail of message edits
- **Reply Threading**: Tracks message reply chains
- **Idempotent Updates**: Handles duplicate webhook deliveries gracefully

## Architecture

### Components

- **API Service** (Go 1.25): REST API for receiving Telegram webhook updates
- **PostgreSQL**: Main database for structured data storage
- **MinIO**: Object storage for media files (photos, videos, audio, documents, etc.)
- **Telegram Bot API Client**: Downloads media files with exponential backoff retry

### Database Schema

12 tables modeling the Telegram data structure:

- `chats`, `users` - Core entities
- `updates` - Raw webhook payloads with deduplication
- `messages`, `message_entities`, `message_edits` - Message data and history
- `message_reactions`, `reaction_counts` - Individual and aggregate reactions
- `media_files`, `photos`, `locations` - Media and location data

See [`apps/postgres/migrations/001_initial.sql`](../postgres/migrations/001_initial.sql) for full schema.

## API Endpoints

### POST `/api/v1/updates`

Receives Telegram webhook updates.

**Request Body**: Telegram `Update` object (JSON)

```json
{
  "update_id": 123456789,
  "message": {
    "message_id": 123,
    "chat": {
      "id": -1001234567890,
      "type": "supergroup",
      "title": "My Group"
    },
    "from": {
      "id": 987654321,
      "is_bot": false,
      "first_name": "John"
    },
    "date": 1733611200,
    "text": "Hello, world!"
  }
}
```

**Response**: `200 OK`

```json
{
  "status": "ok"
}
```

### GET `/health`

Health check endpoint.

**Response**: `200 OK` with body `OK`

## Configuration

All configuration via environment variables:

### Required

- `TELEGRAM_BOT_TOKEN` - Telegram bot token for file downloads

### Database

- `DB_HOST` (default: `localhost`)
- `DB_PORT` (default: `5432`)
- `DB_USER` (default: `postgres`)
- `DB_PASSWORD` (default: ``)
- `DB_NAME` (default: `beef_briefing`)
- `DB_SSL_MODE` (default: `disable`)

### MinIO

- `MINIO_ENDPOINT` (default: `localhost:9000`)
- `MINIO_ACCESS_KEY` (default: `minioadmin`)
- `MINIO_SECRET_KEY` (default: `minioadmin`)
- `MINIO_BUCKET` (default: `telegram-media`)
- `MINIO_USE_SSL` (default: `false`)

### Application

- `API_PORT` (default: `8080`)
- `ENVIRONMENT` (default: `development`) - Set to `production` for JSON logging
- `LOG_LEVEL` (default: `info`) - `debug`, `info`, `warn`, `error`

## Development

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- PostgreSQL 16+
- MinIO

### Setup

1. Copy environment template:

```bash
cp infrastructure/.env.template infrastructure/.env.dev
```

2. Edit `.env.dev` and set your `TELEGRAM_BOT_TOKEN`

3. Start services:

```bash
cd infrastructure
docker-compose -f docker-compose.dev.yml up -d
```

4. The API will be available at `http://localhost:8080`

### Local Development (without Docker)

1. Install dependencies:

```bash
cd apps/api-service
go mod download
```

2. Set environment variables in `.env` file at project root

3. Run migrations manually on your local PostgreSQL instance

4. Start the service:

```bash
cd apps/api-service
go run ./cmd
```

## Media Download Strategy

Media files are downloaded **synchronously** from Telegram during webhook processing:

1. Webhook received with message containing media
2. Transaction started
3. File downloaded from Telegram (with exponential backoff retry)
4. File uploaded to MinIO
5. Metadata saved to database
6. Transaction committed

This ensures consistency between database records and object storage. Failed downloads result in transaction rollback.

### Retry Logic

File downloads use exponential backoff with 3 retry attempts:
- Attempt 1: Immediate
- Attempt 2: 1 second delay
- Attempt 3: 2 seconds delay
- Attempt 4: 4 seconds delay

## Reaction Handling

The API stores **both** individual reactions and aggregate counts:

### Individual Reactions (`message_reactions`)

From `MessageReactionUpdated` events (requires admin permissions + explicit opt-in):
- Tracks which user added/removed which reaction
- Maintains full history with `is_removed` flag
- Supports emoji, custom emoji, and paid reactions

### Aggregate Counts (`reaction_counts`)

From `MessageReactionCountUpdated` events:
- Anonymous reaction totals
- Updated in batches (delays up to a few minutes)
- Useful for public display

## Threading Model

Messages store direct parent only via `reply_to_message_id`. To traverse full thread:

```sql
WITH RECURSIVE thread AS (
  SELECT * FROM messages WHERE id = $1
  UNION ALL
  SELECT m.* FROM messages m
  INNER JOIN thread t ON m.id = t.reply_to_message_id
)
SELECT * FROM thread;
```

## Project Structure

```
apps/api-service/
├── cmd/
│   └── main.go                    # Application entry point
├── internal/
│   ├── handlers/
│   │   └── webhook_handler.go     # HTTP webhook handler
│   ├── models/
│   │   └── telegram.go            # Domain models
│   ├── repository/
│   │   ├── chat_repo.go           # Chat repository
│   │   ├── helpers.go             # SQL helper functions
│   │   ├── media_repo.go          # Media repository
│   │   ├── message_repo.go        # Message repository
│   │   ├── reaction_repo.go       # Reaction repository
│   │   ├── update_repo.go         # Update repository
│   │   └── user_repo.go           # User repository
│   ├── storage/
│   │   └── minio_client.go        # MinIO storage client
│   └── telegram/
│       └── file_client.go         # Telegram file downloader
├── go.mod
└── Dockerfile
```

## Shared Packages

```
pkg/config/                         # Shared configuration package
├── config.go                       # Config loading with envconfig
└── go.mod
```

## Next Steps

To complete the implementation, you'll need to:

1. **Create a Telegram Bot**:
   - Talk to [@BotFather](https://t.me/botfather)
   - Run `/newbot` and follow instructions
   - Copy the bot token to your `.env.dev` file

2. **Set Webhook** (bot implementation):
   - Create a separate `telegram-bot` service
   - Set webhook URL to `https://yourdomain.com/api/v1/updates`
   - Enable reaction tracking in bot settings

3. **Enable Reactions** in your group:
   - Make bot admin in the group
   - Enable "Can read all messages" permission
   - Reaction updates require explicit opt-in via Bot API

## License

[Your License Here]
