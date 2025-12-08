# Import CLI

A CLI tool to import Telegram Desktop export data into the beef-briefing system.

## Features

- **Streaming Parser**: Handles large export files (1M+ messages) without loading everything into memory
- **Resume Support**: Automatically resumes from last processed message if interrupted
- **Media Import**: Optionally import photos, videos, voice messages, and other media
- **Progress Tracking**: Real-time progress display and detailed markdown report generation
- **User Mapping**: Tracks all users discovered during import

## Installation

```bash
# From the import-cli directory
go build -o bin/import-cli ./cmd

# Or using make from repo root
make go-build-import-cli
```

## Usage

### Basic Import

Import messages with a known chat ID:

```bash
./bin/import-cli import --chat-id 2572302334 --export-path ./local_import
```

### Auto-create Chat

Let the tool create the chat from export metadata:

```bash
./bin/import-cli import --create-chat --export-path ./local_import
```

### Import with Media

Include media files (photos, videos, voice messages):

```bash
./bin/import-cli import --chat-id 2572302334 --export-path ./local_import --include-media
```

### Custom Settings

Adjust batch size and delay between messages:

```bash
./bin/import-cli import \
  --chat-id 2572302334 \
  --export-path ./local_import \
  --batch-size 50 \
  --delay-ms 5 \
  --api-url http://localhost:8080
```

### Check Import Status

View progress of an in-progress or completed import:

```bash
./bin/import-cli status --export-path ./local_import
```

### Reset Import State

Clear state to start fresh:

```bash
./bin/import-cli reset --export-path ./local_import
```

## Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--export-path` | `-e` | Path to Telegram export folder (required) | - |
| `--chat-id` | `-c` | Target chat ID | 0 |
| `--create-chat` | - | Auto-create chat from export metadata | false |
| `--include-media` | `-m` | Import media files | false |
| `--batch-size` | `-b` | Messages per batch | 100 |
| `--delay-ms` | `-d` | Delay between batches (ms) | 1 |
| `--api-url` | `-u` | API service URL | http://localhost:8080 |
| `--reset` | - | Reset state before starting | false |
| `--force` | - | Force import even if calculated supergroup ID doesn't match | false |
| `--force-chat-id` | - | Allow importing to different chat ID than previous import | false |
| `--verbose` | `-v` | Enable debug logging | false |

## Group Migration

When a Telegram group is upgraded to a supergroup, the chat ID changes:

- **Regular Group ID**: Positive integer (e.g., `3280306634`)
- **Supergroup ID**: Negative integer with `-100` prefix (e.g., `-1003280306634`)

**Conversion Formula:**
```
supergroup_id = -1000000000000 - old_group_id
```

**Example:**
- Old group ID: `3280306634`
- New supergroup ID: `-1003280306634`

### Importing Supergroup Exports

When importing a supergroup export, the `result.json` file contains the **old group ID** (before migration), but you must provide the **actual supergroup chat ID** using the `--chat-id` flag:

```bash
# This will NOT work - creates chat with wrong ID
./bin/import-cli import --create-chat --export-path ./groups/MyGroup

# Correct - provide the actual supergroup ID
./bin/import-cli import --chat-id -1003280306634 --export-path ./groups/MyGroup
```

The import-cli automatically detects supergroup exports and:
1. Validates the provided chat ID matches the expected conversion formula
2. Creates a migration link between old group ID and new supergroup ID
3. Ensures messages are properly associated with the correct chat

### Getting the Supergroup Chat ID

**Option 1: Admin Panel**
- Navigate to `/chats` page in the admin panel
- Find your supergroup and copy the chat ID

**Option 2: Bot Logs**
- Add the bot to the supergroup
- Check the API service logs for `my_chat_member` updates
- The `chat.id` field contains the supergroup ID

**Option 3: Telegram Link**
- In Telegram Desktop, right-click the supergroup → "Copy Link"
- The link contains the chat ID (may need conversion)

### Force Flags

**`--force`**: Bypass supergroup ID validation

Use when the calculated supergroup ID doesn't match your provided chat ID (rare cases where Telegram's formula differs):

```bash
./bin/import-cli import --chat-id -1001234567890 --export-path ./groups/MyGroup --force
```

**`--force-chat-id`**: Allow re-importing to different chat ID

Use when you want to import the same export to a different chat (e.g., testing):

```bash
./bin/import-cli import --chat-id -1009876543210 --export-path ./groups/MyGroup --force-chat-id
```

## Output Files

After import, the following files are created in the export folder:

- `.import-state.json` - Import state for resume capability
- `import-report.md` - Detailed markdown report with statistics

## Export Format

The tool expects a Telegram Desktop export in JSON format. Export from Telegram Desktop:

1. Open Telegram Desktop
2. Go to Settings → Advanced → Export Telegram data
3. Select JSON format
4. Enable the content types you want to export
5. Export to a folder

The export folder should contain:
- `result.json` - Main export file
- `photos/` - Photo files (if exported)
- `video_files/` - Video files (if exported)
- `voice_messages/` - Voice message files (if exported)
- etc.

## Architecture

```
apps/import-cli/
├── cmd/
│   └── main.go           # Cobra CLI entry point
├── internal/
│   ├── client/           # HTTP client for API communication
│   ├── mapper/           # Export → API format conversion
│   ├── models/           # Data structures
│   ├── parser/           # Streaming JSON parser
│   ├── reporter/         # Markdown report generator
│   └── state/            # Import state management
├── Dockerfile
├── go.mod
└── README.md
```

## Dependencies

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- Standard library (`encoding/json`, `net/http`, etc.)
