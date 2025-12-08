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
| `--verbose` | `-v` | Enable debug logging | false |

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
