# Legacy Export Generator

CLI tool to export messages from a legacy PostgreSQL database to Telegram-compatible `result.json` format.

## Overview

This tool queries messages from a legacy database with a specific schema and transforms them into the Telegram Desktop export JSON format, enabling import via the `import-cli` tool.

## Usage

```bash
legacy-export-generator \
  --db-host localhost \
  --db-port 5432 \
  --db-name legacy_db \
  --db-user postgres \
  --db-password secret \
  --start-date 2025-01-01 \
  --end-date 2025-12-31 \
  --chat-name "Bate-Papo UOL 2025" \
  --chat-type "private_supergroup" \
  --chat-id 2572302334 \
  --output result.json
```

## Flags

### Database Connection
| Flag | Default | Description |
|------|---------|-------------|
| `--db-host` | `localhost` | Database host |
| `--db-port` | `5432` | Database port |
| `--db-name` | (required) | Database name |
| `--db-user` | (required) | Database username |
| `--db-password` | (required) | Database password |

### Date Range
| Flag | Description |
|------|-------------|
| `--start-date` | Start date for export (format: YYYY-MM-DD) |
| `--end-date` | End date for export (format: YYYY-MM-DD) |

### Filtering
| Flag | Description |
|------|-------------|
| `--source-chat-id` | Filter by source chat ID from legacy database (optional) |

### Output Metadata
| Flag | Default | Description |
|------|---------|-------------|
| `--chat-name` | (required) | Chat name for export metadata |
| `--chat-type` | `private_supergroup` | Chat type for export metadata |
| `--chat-id` | (required) | Chat ID for export metadata |
| `--output` | `result.json` | Output file path |

### Other
| Flag | Description |
|------|-------------|
| `-v, --verbose` | Enable verbose logging |
| `--version` | Show version information |

## Legacy Database Schema

The tool expects a `messages` table with the following structure:

```sql
CREATE TABLE messages (
    id SERIAL PRIMARY KEY,
    message_id BIGINT NOT NULL,
    message_type TEXT NOT NULL,
    timestamp TIMESTAMP NOT NULL,
    chat_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    reply_to_message_id BIGINT,
    first_name TEXT,
    last_name TEXT,
    username TEXT,
    display_name TEXT,
    content JSONB NOT NULL,
    moderated BOOLEAN DEFAULT FALSE
);
```

### Supported Message Types
- `text` - Text messages (content is a JSON string)
- `photo` - Photo messages (media fields left empty in export)
- `sticker` - Sticker messages (media fields left empty in export)
- `video` - Video messages (media fields left empty in export)
- `generic` - Service messages (skipped in export)

## Output Format

The generated `result.json` follows the Telegram Desktop export format:

```json
{
  "name": "Chat Name",
  "type": "private_supergroup",
  "id": 2572302334,
  "messages": [
    {
      "id": 1,
      "type": "message",
      "date": "2025-04-22T15:53:58",
      "date_unixtime": "1745336038",
      "from": "User Name",
      "from_id": "user123456",
      "reply_to_message_id": 5,
      "text": "Hello world",
      "text_entities": [
        {"type": "plain", "text": "Hello world"}
      ]
    }
  ]
}
```

## ID Handling

- Sequential IDs (1, 2, 3...) are generated for export compatibility
- Reply references are resolved using a two-pass approach:
  1. First pass: Build mapping from original `message_id` to sequential ID
  2. Second pass: Transform messages with resolved reply IDs
- If a reply references a message outside the export range, the reply reference is omitted

## Building

```bash
# From project root
make go-build-legacy-export

# Or directly
cd apps/legacy-export-generator
go build -o bin/legacy-export-generator ./cmd
```

## Docker

```bash
docker build -t legacy-export-generator .
docker run legacy-export-generator --help
```
