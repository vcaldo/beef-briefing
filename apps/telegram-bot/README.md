# Telegram Bot - Beef Briefing

Live message logging bot for Telegram that captures and stores all incoming messages in real-time from multiple Telegram groups.

## Features

- **Real-time Message Capture**: Logs all incoming messages as they arrive
- **Multiple Message Types**: Supports text, photos, videos, voice, documents, stickers, animations, and video notes
- **Service Message Tracking**: Captures user join/leave events and other service messages
- **Reaction Tracking**: Tracks individual reactions on messages (stored in separate table)
- **Media Storage**: Stores media files in MinIO with SHA256-based deduplication
- **Chat Isolation**: Data is isolated per group/chat for privacy and organization
- **PostgreSQL Storage**: All message metadata stored in PostgreSQL with proper indexing
- **Graceful Shutdown**: Handles SIGINT/SIGTERM signals for clean shutdown

## Architecture

The bot talks directly to PostgreSQL and MinIO without any intermediary API layer:

```
Telegram API → Bot → PostgreSQL (message metadata)
                  → MinIO (media files)
```

### Database Schema

- `chats`: Telegram chat/group information
- `users`: Telegram user information
- `messages`: All message metadata with foreign keys to chats/users
- `service_messages`: Service events (user joined/left)
- `message_reactions`: Individual reactions on messages

### Media Storage

Media files are stored in MinIO using SHA256 hash as the object key for automatic deduplication:
- Same file uploaded multiple times = stored only once
- Hash is stored in `messages.media_sha256` column for retrieval

## Configuration

All configuration is via environment variables. See `.env.example` for full list.

Required:
- `TELEGRAM_BOT_TOKEN`: Your Telegram bot token from @BotFather

Database:
- `DB_HOST`: PostgreSQL host (default: localhost)
- `DB_PORT`: PostgreSQL port (default: 5432)
- `DB_USER`: Database user (default: postgres)
- `DB_PASSWORD`: Database password
- `DB_NAME`: Database name (default: beef_db)

MinIO:
- `MINIO_ENDPOINT`: MinIO endpoint (default: localhost:9000)
- `MINIO_ACCESS_KEY`: MinIO access key (default: minioadmin)
- `MINIO_SECRET_KEY`: MinIO secret key (default: minioadmin)
- `MINIO_BUCKET`: Bucket for media files (default: telegram-media)

## Development

### Local Setup

1. Copy environment variables:
```bash
cp .env.example .env
# Edit .env and set TELEGRAM_BOT_TOKEN
```

2. Start with docker-compose:
```bash
cd infrastructure
docker-compose -f docker-compose.dev.yml up telegram-bot
```

### Manual Build

```bash
go mod download
go build -o telegram-bot ./cmd
./telegram-bot
```

## Message Types

Supported message types:
- `text`: Plain text messages
- `photo`: Photo messages
- `video`: Video files
- `voice`: Voice messages
- `document`: Documents and files
- `sticker`: Stickers (including animated)
- `animation`: GIFs and animations
- `video_note`: Round video messages

Service messages:
- `user_joined`: User joined the group
- `user_left`: User left the group

## Database Schema Compatibility

The schema is designed to be compatible with Telegram's export format (`result.json`) for potential future import features:
- Message entities stored as JSONB for flexibility
- Metadata stored as JSONB for extensibility
- Forwarding information preserved
- Reply chains maintained via `reply_to_message_id`

## Logging

Structured logging using Go's `log/slog`:
- **Development**: Text format, human-readable
- **Production**: JSON format, machine-readable

Log levels: `debug`, `info`, `warn`, `error`

## Graceful Shutdown

The bot handles SIGINT and SIGTERM signals:
1. Stop accepting new updates
2. Allow in-flight message processing to complete
3. Close database connections
4. Close MinIO client
5. Exit cleanly

## Code Structure

```
apps/telegram-bot/
├── cmd/
│   └── main.go              # Entry point, signal handling, logger setup
├── internal/
│   ├── config/
│   │   └── config.go        # Environment variable loading
│   ├── store/
│   │   └── postgres.go      # Database operations (upsert chat/user, insert message)
│   ├── storage/
│   │   └── minio.go         # MinIO client (upload, deduplication, SHA256)
│   └── handler/
│       ├── handler.go       # Message handlers (text, photo, video, etc.)
│       └── service.go       # Service message handlers (join/leave)
├── go.mod
├── go.sum
├── Dockerfile
├── .env.example
└── README.md
```

## Dependencies

- `gopkg.in/telebot.v4`: Telegram bot framework
- `github.com/lib/pq`: PostgreSQL driver
- `github.com/minio/minio-go/v7`: MinIO client
- `github.com/joho/godotenv`: Environment variable loading
- `github.com/kelseyhightower/envconfig`: Configuration parsing

## Future Enhancements

- Message editing detection (already tracked with `edit_date`)
- Forward chain tracking (already supported via `forwarded_from_*` columns)
- Actual media file download and upload to MinIO (placeholder implemented)
- Reaction updates (add/remove reactions)
- More service message types
- Import from Telegram export JSON
