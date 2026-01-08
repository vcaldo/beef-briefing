# Telegram Bot

Real-time Telegram bot that listens to group messages and forwards updates to the API Service.

## Overview

The Telegram Bot connects to the Telegram API via long polling and captures all messages, edits, and reactions from groups where it's an admin. It downloads media files concurrently and sends updates to the API Service as multipart form data.

## Features

- **Real-time Message Processing**: Captures all updates from Telegram groups
- **Concurrent Media Download**: Up to 5 simultaneous file downloads with connection pooling
- **Smart Photo Handling**: Downloads only the largest photo size to save bandwidth
- **Automatic Retry**: Exponential backoff (1s, 2s, 4s) for API failures
- **File Size Protection**: 100MB limit per file to prevent memory exhaustion
- **Graceful Shutdown**: Clean handling of SIGINT/SIGTERM signals

## Quick Start

```bash
# Start with Docker (recommended)
make up-build
make logs-bot

# Or run locally
export TELEGRAM_BOT_TOKEN="your-token"
export API_SERVICE_URL="http://localhost:8080"
cd apps/telegram-bot
go run ./cmd
```

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `TELEGRAM_BOT_TOKEN` | (required) | Bot token from [@BotFather](https://t.me/botfather) |
| `API_SERVICE_URL` | `http://api-service:8080` | API Service URL |
| `ENVIRONMENT` | `development` | `development` or `production` |
| `LOG_LEVEL` | `info` | Log level |

### Internal Constants

Defined in `internal/constants.go`:

| Constant | Value | Description |
|----------|-------|-------------|
| File Download Timeout | 2 min | Per-file download timeout |
| API Request Timeout | 30 sec | Per-request timeout |
| Max Retry Attempts | 3 | API request retries |
| Max Concurrent Downloads | 5 | Parallel file downloads |
| Max File Size | 100 MB | File size limit |

## Bot Setup

### 1. Create Bot with BotFather

1. Open [@BotFather](https://t.me/botfather) in Telegram
2. Send `/newbot` and follow the prompts
3. Save the bot token
4. Send `/setprivacy` → **DISABLED** (to receive all group messages)
5. Send `/setjoingroups` → **ENABLED**

### 2. Add Bot to Group

1. Add the bot to your Telegram group
2. **Promote bot to admin** (required for reactions)
3. No specific permissions needed beyond admin status

### 3. Configure Environment

Add to `infrastructure/.env.dev`:

```bash
TELEGRAM_BOT_TOKEN=1234567890:ABCdefGHIjklMNOpqrsTUVwxyz
```

## Supported Update Types

| Type | Description | Requirements |
|------|-------------|--------------|
| `message` | Text, photos, videos, documents, stickers, polls, etc. | Bot in group |
| `edited_message` | Message edits | Bot in group |
| `message_reaction` | Individual user reactions | Bot must be admin |
| `message_reaction_count` | Aggregate reaction counts | Bot must be admin |

## Architecture

```
apps/telegram-bot/
├── cmd/main.go                 # Entry point, bot initialization
├── internal/
│   ├── client/api_client.go    # API client with retry logic
│   ├── handlers/update_handler.go  # Update processing
│   └── constants.go            # Timeouts and limits
└── Dockerfile
```

### Update Flow

```
Telegram API → Bot (long polling) → Download media (concurrent)
                                  → Send to API Service (multipart)
                                  → Retry on failure (exponential backoff)
```

## Troubleshooting

### Bot doesn't receive messages

1. Verify bot is admin in the group
2. Check privacy mode is disabled: `/setprivacy` in @BotFather
3. Confirm `TELEGRAM_BOT_TOKEN` is correct
4. Check logs: `make logs-bot`

### Reactions not captured

- Bot **must be admin** to receive reaction updates
- Telegram only sends reactions to admin bots

### Media files not downloading

1. Check internet connectivity to `api.telegram.org`
2. Look for `failed to download file` in logs
3. Verify file isn't larger than 100MB

### API requests failing

1. Check `API_SERVICE_URL` is correct
2. Verify api-service is running: `docker ps | grep api-service`
3. Check api-service logs: `make logs-api`
