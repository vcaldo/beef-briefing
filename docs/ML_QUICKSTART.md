# ML Quickstart Guide

This guide walks you through processing messages and generating cards from scratch, assuming you have a database already populated with messages but no ML data processed yet.

## Prerequisites

- Docker and Docker Compose installed
- `.env.dev` configured (copy from `.env.dev.example`)
- OpenAI API key set in `.env.dev` (required for ML analysis)
- Database populated with messages (via telegram-bot or import-cli)

## Step 1: Start Services

```bash
# Start all services (PostgreSQL, MinIO, ml-processor, card-image-generator, etc.)
make up-build
```

Wait for all services to be healthy. Check with:

```bash
docker ps
```

## Step 2: Generate API Keys (First Time Only)

```bash
# Generate API keys for services that need to communicate
make secrets-service-api APP=telegram-bot
make secrets-service-api APP=leaderboard
make secrets-card-image-generator APP=ml-processor
```

## Step 3: Check Current Status

Before processing, check what's in the database:

```bash
make ml-run-status
```

This shows:
- Total messages in the chat
- Messages already processed
- Messages pending processing

## Step 4: Process Messages with ML

Process all unprocessed messages through the ML pipeline (sentiment, humor, toxicity, NER, topics, embeddings):

```bash
# Process all messages (may take a while depending on volume)
make ml-run

# Or process with a limit first to test
make ml-run ML_ARGS="--limit 100"

# Or process a specific date range
make ml-run ML_ARGS="--from-date 2024-12-01 --to-date 2024-12-31"
```

For large datasets, run in batches:

```bash
# Process in smaller batches with status checks
make ml-run ML_ARGS="--limit 1000"
make ml-run-status
# Repeat until done
```

## Step 5: Generate Card Data

Once messages are processed, generate weekly user cards:

```bash
# Generate cards for a specific week (week_start should be a Monday)
make ml-run-cards ML_ARGS="--week 2024-12-16 --timezone America/Sao_Paulo"

# Or let it auto-detect the current week
make ml-run-cards ML_ARGS="--timezone America/Sao_Paulo"

# With custom parameters
make ml-run-cards ML_ARGS="--week 2024-12-16 --timezone America/Sao_Paulo --window-days 30 --min-messages 10"
```

Parameters:
- `--week`: Week start date (Monday, YYYY-MM-DD). Default: current week
- `--timezone`: IANA timezone for week boundaries (required)
- `--window-days`: Rolling window for stats calculation (default: 30)
- `--min-messages`: Minimum messages required for card generation (default: 10)

## Step 6: Render Card Images

Render the cards as PNG images:

```bash
# Render with default theme (gaming)
make ml-run-render ML_ARGS="--week 2024-12-16"

# Render with a specific theme
make ml-run-render ML_ARGS="--week 2024-12-16 --theme clean"

# Force re-render (overwrites existing images)
make ml-run-render ML_ARGS="--week 2024-12-16 --force"
```

## Step 7: Render All Themes

To render cards with all available themes:

```bash
# Available themes: gaming, clean, sticker, meme, vaporwave, blueprint, mythic, noir_luxury
for theme in gaming clean sticker meme vaporwave blueprint mythic noir_luxury; do
  echo "Rendering theme: $theme"
  make ml-run-render ML_ARGS="--week 2024-12-16 --theme $theme --force"
done
```

## Full Pipeline Script

Here's a complete script to run the entire pipeline:

```bash
#!/bin/bash
# Full ML pipeline from scratch

# Configuration
WEEK="2024-12-16"  # Adjust to your target week (should be Monday)
TIMEZONE="America/Sao_Paulo"
THEMES="gaming clean sticker meme vaporwave blueprint mythic noir_luxury"

# 1. Check status
echo "=== Current Status ==="
make ml-run-status

# 2. Process all messages
echo "=== Processing Messages ==="
make ml-run

# 3. Generate cards
echo "=== Generating Cards ==="
make ml-run-cards ML_ARGS="--week $WEEK --timezone $TIMEZONE"

# 4. Render all themes
echo "=== Rendering All Themes ==="
for theme in $THEMES; do
  echo "Rendering: $theme"
  make ml-run-render ML_ARGS="--week $WEEK --theme $theme --force"
done

echo "=== Done ==="
make ml-run-status
```

## Cleaning Up

If you need to start fresh:

```bash
# Clean ALL ML data (PostgreSQL + Qdrant) - DESTRUCTIVE
make ml-clean-dev

# Clean only cards for a specific chat/week
make ml-clean-cards-dev ML_ARGS="--week 2024-12-16"

# Clean all cards for a chat (requires confirmation)
make ml-clean-cards-dev ML_ARGS="--force"
```

## Useful Commands Reference

| Command | Description |
|---------|-------------|
| `make ml-run-status` | Show processing status |
| `make ml-run` | Process messages through ML pipeline |
| `make ml-run-once` | Process single batch (for testing) |
| `make ml-run-cards` | Generate weekly user cards |
| `make ml-run-render` | Render card images |
| `make ml-clean-dev` | Clean all ML data |
| `make ml-clean-cards-dev` | Clean specific cards |
| `make ml-shell` | Open shell in ml-processor container |
| `make docker-logs-ml-processor` | View ml-processor logs |

## Specifying a Different Chat

By default, commands use the chat ID configured in `scripts/ml-processor.sh`. To use a different chat:

```bash
make ml-run ML_ARGS="--chat-id -1003280306634 --limit 100"
make ml-run-cards ML_ARGS="--chat-id -1003280306634 --week 2024-12-16 --timezone America/Sao_Paulo"
make ml-run-render ML_ARGS="--chat-id -1003280306634 --week 2024-12-16"
```

## Troubleshooting

### "Container not running"

```bash
make up-build
```

### "API key not configured"

```bash
make secrets-card-image-generator APP=ml-processor
make up-build  # Restart to pick up new secrets
```

### "Card image generator service is not healthy"

Check if the card-image-generator is running:

```bash
docker logs beef-card-image-generator-dev
curl http://localhost:8051/health
```

### "No cards found for this chat"

Run card generation first:

```bash
make ml-run-cards ML_ARGS="--week 2024-12-16 --timezone America/Sao_Paulo"
```

### OpenAI Rate Limits

If you hit OpenAI rate limits, the processor will automatically wait. You can also process smaller batches:

```bash
make ml-run ML_ARGS="--batch-size 50 --limit 500"
```
