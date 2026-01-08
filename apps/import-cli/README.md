# Import CLI

CLI tool for importing Telegram Desktop export data into the Beef Briefing system.

## Overview

The Import CLI parses Telegram Desktop JSON exports and sends messages to the API Service. It handles large exports (1M+ messages) efficiently with streaming parsing and supports resume if interrupted.

## Features

- **Streaming Parser**: Handles large exports without memory exhaustion
- **Resume Support**: Automatically resumes from last processed message
- **Media Import**: Optional import of photos, videos, voice messages
- **Progress Tracking**: Real-time progress and markdown report generation
- **Bot Filtering**: Optional detection and filtering of bot messages

## Quick Start

```bash
# Build the CLI
make go-build-import-cli

# Basic import
./apps/import-cli/bin/import-cli import \
  --chat-id -1003280306634 \
  --export-path ./telegram-export

# Check import status
./apps/import-cli/bin/import-cli status --export-path ./telegram-export
```

## Installation

```bash
# Build locally
cd apps/import-cli
go build -o bin/import-cli ./cmd

# Or via make
make go-build-import-cli

# For production server (cross-compile)
make go-build-import-cli-prod
```

## Commands

### import

Import messages from a Telegram export.

```bash
./bin/import-cli import \
  --chat-id -1003280306634 \
  --export-path ./telegram-export \
  --include-media
```

### status

Check import progress.

```bash
./bin/import-cli status --export-path ./telegram-export
```

### reset

Clear import state to start fresh.

```bash
./bin/import-cli reset --export-path ./telegram-export
```

## Options

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--export-path` | `-e` | (required) | Path to Telegram export folder |
| `--chat-id` | `-c` | `0` | Target chat ID |
| `--create-chat` | | `false` | Auto-create chat from export metadata |
| `--include-media` | `-m` | `false` | Import media files |
| `--batch-size` | `-b` | `100` | Messages per batch |
| `--delay-ms` | `-d` | `1` | Delay between batches (ms) |
| `--api-url` | `-u` | `http://localhost:8080` | API Service URL |
| `--telegram-token` | | | Bot token for bot detection |
| `--skip-bots` | | `true` | Skip bot messages |
| `--force` | | `false` | Bypass supergroup ID validation |
| `--force-chat-id` | | `false` | Allow different chat ID than previous |
| `--verbose` | `-v` | `false` | Enable debug logging |

## Group Migration

When importing a supergroup, provide the **actual supergroup ID**, not the old group ID from the export:

```bash
# The export contains old group ID: 3280306634
# But the actual supergroup ID is: -1003280306634

# Correct:
./bin/import-cli import --chat-id -1003280306634 --export-path ./export

# Wrong (will create chat with incorrect ID):
./bin/import-cli import --create-chat --export-path ./export
```

**Conversion Formula:**
```
supergroup_id = -1000000000000 - old_group_id
```

### Finding the Supergroup ID

1. **Bot logs**: Add bot to group, check logs for `chat.id`
2. **Telegram link**: Right-click group → Copy Link (contains ID)
3. **Admin panel**: Check `/chats` page

## Export Format

Export from Telegram Desktop:

1. Open Telegram Desktop
2. Settings → Advanced → Export Telegram data
3. Select **JSON format**
4. Enable desired content types
5. Export to a folder

The export folder should contain:
```
telegram-export/
├── result.json          # Main export file
├── photos/              # Photo files (if exported)
├── video_files/         # Videos (if exported)
├── voice_messages/      # Voice messages (if exported)
└── ...
```

## Output Files

After import, these files are created in the export folder:
- `.import-state.json` - Resume state
- `import-report.md` - Statistics report

## Architecture

```
apps/import-cli/
├── cmd/main.go           # Cobra CLI entry
├── internal/
│   ├── client/           # HTTP client
│   ├── mapper/           # Export → API format conversion
│   ├── parser/           # Streaming JSON parser
│   ├── reporter/         # Markdown report generator
│   └── state/            # Import state management
└── Dockerfile
```

## Troubleshooting

### Import stuck or slow

- Reduce `--batch-size` if API is overwhelmed
- Check API Service logs: `make logs-api`
- Resume will continue from last checkpoint

### Supergroup ID mismatch

- Use `--force` to bypass validation
- Double-check the supergroup ID calculation

### Media not importing

- Ensure `--include-media` flag is set
- Check media files exist in export folder
- Verify file sizes are under 100MB
