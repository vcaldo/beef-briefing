# Telegram Bot Service

A Go-based Telegram bot that listens to group messages and forwards all updates (messages, edits, reactions) to the API service for ingestion into the database. The bot automatically downloads media files and sends them as multipart attachments.

## Features

- **Real-time Message Processing**: Listens to all updates in Telegram groups where the bot is an admin
- **Media Download**: Automatically downloads photos, videos, audio, voice messages, documents, animations, and video notes
- **Automatic Retry**: Exponential backoff retry logic (3 attempts: 1s, 2s, 4s delays) for API failures
- **Graceful Shutdown**: Handles SIGINT/SIGTERM signals for clean shutdown
- **Structured Logging**: Environment-based logging (JSON for production, text for development)
- **Timeout Protection**: 2-minute timeout for media file downloads

## Architecture

### Components

- **Bot Handler** (`internal/handlers/update_handler.go`): Processes incoming updates and extracts media files
- **API Client** (`internal/client/api_client.go`): Sends updates to API service with retry logic
- **Main** (`cmd/main.go`): Bot initialization, logger setup, and graceful shutdown

### Update Flow

1. Bot receives update via long polling from Telegram
2. Handler extracts file IDs from message/edited message
3. Bot downloads each file with 2-minute timeout
4. API client creates multipart form with JSON update + file attachments
5. POST request to `{API_SERVICE_URL}/api/v1/ingest`
6. Retry with exponential backoff on 5xx errors or network failures
7. Log success/failure with structured fields

## Configuration

All configuration via environment variables:

### Required

- `TELEGRAM_BOT_TOKEN` - Telegram Bot API token (from [@BotFather](https://t.me/botfather))

### Optional

- `API_SERVICE_URL` (default: `http://api-service:8080`) - URL of the API service ingest endpoint
- `ENVIRONMENT` (default: `development`) - Set to `production` for JSON logging
- `LOG_LEVEL` (default: `info`) - Log level: `debug`, `info`, `warn`, `error`

## Bot Setup

### 1. Create Bot with BotFather

1. Open [@BotFather](https://t.me/botfather) in Telegram
2. Send `/newbot` and follow instructions
3. Save the bot token
4. Send `/setprivacy` and set to **DISABLED** to receive all group messages
5. Send `/setjoingroups` and set to **ENABLED** to allow bot to join groups

### 2. Add Bot to Group

1. Add the bot to your Telegram group
2. **Promote bot to admin** (required to receive all message types including reactions)
3. Grant the following admin permissions:
   - Delete messages (optional, for moderation features)
   - Read messages (implicit with admin status)

### 3. Configure Environment

Set `TELEGRAM_BOT_TOKEN` in `infrastructure/.env.dev`:

```bash
TELEGRAM_BOT_TOKEN=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz
```

## Development

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- Running `api-service` (see `apps/api-service/README.md`)

### Local Development (with Docker)

1. Ensure `infrastructure/.env.dev` has valid `TELEGRAM_BOT_TOKEN`

2. Start all services:

```bash
cd infrastructure
docker-compose -f docker-compose.dev.yml up -d
```

3. View bot logs:

```bash
docker logs -f beef-telegram-bot-dev
```

### Local Development (without Docker)

1. Install dependencies:

```bash
cd apps/telegram-bot
go mod download
```

2. Set environment variables:

```bash
export TELEGRAM_BOT_TOKEN="your-token-here"
export API_SERVICE_URL="http://localhost:8080"
export ENVIRONMENT="development"
export LOG_LEVEL="debug"
```

3. Run the bot:

```bash
cd apps/telegram-bot
go run ./cmd
```

## Supported Update Types

The bot processes the following Telegram update types:

### Messages (`update.message`)

- Text messages
- Photos (all sizes)
- Videos
- Audio files
- Voice messages
- Documents
- Animations (GIFs)
- Video notes
- Locations
- Message entities (mentions, URLs, hashtags, etc.)

### Edited Messages (`update.edited_message`)

- Same as messages above
- Edit history tracked in database

### Reactions (`update.message_reaction`)

- Individual user reactions
- Emoji, custom emoji, and paid reactions
- **⚠️ Requires bot to be admin in the chat** - Telegram only sends reaction updates to admin bots
- Bot must explicitly request `message_reaction` in `allowed_updates` (configured in `cmd/main.go`)

### Reaction Counts (`update.message_reaction_count`)

- Aggregate reaction counts for anonymous reactions
- Used when individual user reactions are not available
- **⚠️ Requires bot to be admin in the chat**
- Bot must explicitly request `message_reaction_count` in `allowed_updates`

> **Note**: By default, Telegram excludes `message_reaction`, `message_reaction_count`, and `chat_member` from the updates sent to bots. The bot explicitly configures `allowed_updates` to include all update types.

## Error Handling

### Retry Logic

The bot retries failed API requests with exponential backoff:
- **Attempt 1**: Immediate
- **Attempt 2**: After 1 second
- **Attempt 3**: After 2 seconds
- **Attempt 4**: After 4 seconds

**No retry** on 4xx errors (client errors like invalid JSON or missing fields).

### File Download Failures

If a media file fails to download:
- Error is logged with `file_id` and error details
- Other files in the same update continue processing
- Update is sent to API service without the failed file
- API service will skip missing files (see `apps/api-service/README.md`)

### Timeouts

- **File download**: 2 minutes per file
- **API request**: 30 seconds total (includes all retry attempts)

## Logging

The bot uses structured logging with `log/slog` following the project guidelines:

### Development Mode (`ENVIRONMENT=development`)

- Human-readable text format
- Log level: `DEBUG` (configurable via `LOG_LEVEL`)
- Example:
  ```
  time=2025-12-07T10:30:00.000Z level=INFO msg="received update" update_id=123456789
  time=2025-12-07T10:30:01.000Z level=INFO msg="downloading media files" count=2
  time=2025-12-07T10:30:05.000Z level=INFO msg="successfully processed update" update_id=123456789 message_id=456 chat_id=-1001234567890
  ```

### Production Mode (`ENVIRONMENT=production`)

- JSON format for machine parsing
- Log level: `INFO` (configurable via `LOG_LEVEL`)
- Example:
  ```json
  {"time":"2025-12-07T10:30:00.000Z","level":"INFO","msg":"received update","update_id":123456789}
  {"time":"2025-12-07T10:30:05.000Z","level":"INFO","msg":"successfully processed update","update_id":123456789,"message_id":456,"chat_id":-1001234567890}
  ```

### Log Fields

- `update_id` - Telegram update identifier
- `message_id` - Message identifier (for message/edit/reaction updates)
- `chat_id` - Chat identifier
- `type` - Update type: `edit`, `reaction`, `reaction_count` (omitted for regular messages)
- `file_id` - Telegram file identifier (for media downloads)
- `size` - File size in bytes
- `error` - Error message (on failures)
- `attempt` - Retry attempt number (on retries)

## Project Structure

```
apps/telegram-bot/
├── cmd/
│   └── main.go                    # Application entry point
├── internal/
│   ├── client/
│   │   └── api_client.go          # API client with retry logic
│   └── handlers/
│       └── update_handler.go      # Update processing and file downloads
├── go.mod
├── Dockerfile
└── README.md
```

## Dependencies

- **github.com/go-telegram/bot** - Telegram Bot API library
- **beef-briefing/pkg/config** - Shared configuration package

## Monitoring

### Health Check

The bot doesn't expose an HTTP endpoint. Monitor health via:

1. **Docker logs**: Check for error messages
   ```bash
   docker logs beef-telegram-bot-dev
   ```

2. **Container status**: Ensure container is running
   ```bash
   docker ps | grep telegram-bot
   ```

3. **Process logs**: Look for successful update processing
   ```bash
   docker logs beef-telegram-bot-dev | grep "successfully processed"
   ```

### Key Metrics to Monitor

- Update processing rate (updates per minute)
- API request failures (retry attempts)
- File download failures (missing media)
- Bot restarts (signal handling)

## Troubleshooting

### Bot doesn't receive messages

1. Check bot is admin in the group
2. Verify `TELEGRAM_BOT_TOKEN` is correct
3. Ensure privacy mode is disabled (via BotFather `/setprivacy`)
4. Check bot logs for errors

### Media files not downloading

1. Verify bot has internet access to `api.telegram.org`
2. Check file download timeout (2 minutes should be sufficient for most files)
3. Look for `failed to download file` errors in logs with `file_id`

### API requests failing

1. Verify `API_SERVICE_URL` is correct and reachable
2. Check `api-service` is running and healthy
3. Look for retry attempts in logs
4. Check `api-service` logs for ingestion errors

### High memory usage

Large media files are downloaded to memory before sending to API. If processing many large videos:
- Monitor container memory usage
- Consider implementing file size limits
- Add streaming upload support (future enhancement)

