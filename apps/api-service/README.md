# Telegram Data Ingestion API

A Go-based REST API service for ingesting Telegram group chat data, including messages, reactions, media files, and locations. The API accepts multipart uploads containing message metadata and media files for storage in PostgreSQL and MinIO.

## Features

- **Multipart Ingestion**: Accepts JSON metadata + binary file uploads in a single request
- **Content-Addressable Storage**: Deduplicates media files using SHA256 hashing
- **Complete Message Support**: All Telegram message types including text, media, stickers, games, polls, contacts, venues, dice, and locations
- **Reaction Tracking**: Tracks both individual reactions and aggregate reaction counts
- **Edit History**: Maintains full audit trail of message edits
- **Reply Threading**: Tracks message reply chains
- **Cross-Table Deduplication**: Files are deduplicated across all media tables before MinIO upload

## Architecture

### Components

- **API Service** (Go 1.25): REST API for ingesting Telegram updates with media uploads
- **PostgreSQL**: Main database for structured data storage
- **MinIO**: Content-addressable object storage for media files (photos, videos, audio, documents, etc.)
- **External Bot**: Client responsible for downloading files from Telegram and uploading to this API

### Database Schema

22 tables modeling the complete Telegram data structure:

**Core Entities**:
- `chats`, `users` - Chat and user profiles (chats include `migrated_from_chat_id` for group→supergroup linking)
- `updates` - Raw webhook payloads with deduplication

**Messages**:
- `messages` - All message types
- `message_entities` - Text formatting (bold, links, mentions, etc.)
- `message_edits` - Edit history audit trail

**Reactions** (denormalized - see design note below):
- `message_reactions` - Individual user reactions
- `reaction_counts` - Aggregate anonymous counts

**Media Files**:
- `media_files` - Video, audio, voice, document, animation, video_note, sticker
- `photos` - Photo sizes (multiple per message)
- `stickers` - Sticker-specific metadata (linked to media_files)
- `games`, `game_photos` - Game content and photos

**Other Message Types**:
- `polls`, `poll_options` - Poll questions and answers
- `contacts` - Shared contact information
- `locations` - Geographic coordinates
- `venues` - Location with place info (linked to locations)
- `dice` - Animated emoji results

**Key Features**:
- `file_hash` column in media tables enables content-addressable storage
- Cross-table deduplication: hash checked across `media_files`, `photos`, and `game_photos`
- MinIO stores files once per unique hash at path: `{mediaType}/{hash[:2]}/{hash}`

**Design Note - Denormalized Reactions**:
The `message_id` in reaction tables stores the Telegram message ID (not a FK to `messages.id`). This allows storing reactions for messages that may not have been captured (e.g., reactions to old messages before the bot joined).

**Design Note - Group to Supergroup Migration**:
When a Telegram group is upgraded to a supergroup, the chat ID changes completely (e.g., `123456789` → `-100123456789`). Telegram sends a `my_chat_member` update with `chat.migrate_from_chat_id` set to the old group ID. The API detects this and stores the relationship in `chats.migrated_from_chat_id`, allowing queries to link messages from both the old group and new supergroup as belonging to the same logical chat.

See [`apps/postgres/migrations/`](../postgres/migrations/) for full schema.

## API Endpoints

### POST `/api/v1/ingest`

Ingests Telegram updates with optional media file uploads via multipart form data.

**Content-Type**: `multipart/form-data`

**Form Fields**:
- `update` (required): JSON string containing the Telegram `Update` object
- `{fileID}` (optional, multiple): Binary file content for each media file referenced in the update. The field name must match the `file_id` from Telegram.

**Example: Text Message (no media)**

```bash
curl -X POST http://localhost:8080/api/v1/ingest \
  -F 'update={
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
  }'
```

**Example: Message with Photo**

```bash
curl -X POST http://localhost:8080/api/v1/ingest \
  -F 'update={
    "update_id": 123456790,
    "message": {
      "message_id": 124,
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
      "date": 1733611300,
      "photo": [
        {
          "file_id": "AgACAgIAAxkBAAIBCGZj...",
          "file_unique_id": "AQADAgATxzK3CV4AA",
          "file_size": 1024,
          "width": 320,
          "height": 240
        },
        {
          "file_id": "AgACAgIAAxkBAAIBCWZj...",
          "file_unique_id": "AQADAgATxzK3CV4AB",
          "file_size": 4096,
          "width": 1280,
          "height": 720
        }
      ],
      "caption": "Check out this photo!"
    }
  }' \
  -F 'AgACAgIAAxkBAAIBCGZj...=@photo1.jpg' \
  -F 'AgACAgIAAxkBAAIBCWZj...=@photo2.jpg'
```

**Example: Message with Document**

```bash
curl -X POST http://localhost:8080/api/v1/ingest \
  -F 'update={
    "update_id": 123456791,
    "message": {
      "message_id": 125,
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
      "date": 1733611400,
      "document": {
        "file_id": "BQACAgIAAxkBAAIBCmZj...",
        "file_unique_id": "AgADAgATxzK3CV4AC",
        "file_name": "report.pdf",
        "mime_type": "application/pdf",
        "file_size": 102400
      }
    }
  }' \
  -F 'BQACAgIAAxkBAAIBCmZj...=@report.pdf'
```

**Example: Reaction Update**

```bash
curl -X POST http://localhost:8080/api/v1/ingest \
  -F 'update={
    "update_id": 123456792,
    "message_reaction": {
      "chat": {
        "id": -1001234567890,
        "type": "supergroup",
        "title": "My Group"
      },
      "message_id": 123,
      "user": {
        "id": 987654321,
        "is_bot": false,
        "first_name": "John"
      },
      "date": 1733611500,
      "old_reaction": [],
      "new_reaction": [
        {
          "type": "emoji",
          "emoji": "👍"
        }
      ]
    }
  }'
```

**Example: Sticker Message**

```bash
curl -X POST http://localhost:8080/api/v1/ingest \
  -F 'update={
    "update_id": 123456793,
    "message": {
      "message_id": 126,
      "chat": {"id": -1001234567890, "type": "supergroup"},
      "from": {"id": 987654321, "first_name": "John"},
      "date": 1733611600,
      "sticker": {
        "file_id": "CAACAgIAAxkBAAIBDGZj...",
        "file_unique_id": "AgADAgATstK3CV4",
        "type": "regular",
        "width": 512,
        "height": 512,
        "is_animated": false,
        "is_video": false,
        "emoji": "😀",
        "set_name": "HotCherry",
        "file_size": 15432
      }
    }
  }' \
  -F 'CAACAgIAAxkBAAIBDGZj...=@sticker.webp'
```

**Example: Poll Message**

```bash
curl -X POST http://localhost:8080/api/v1/ingest \
  -F 'update={
    "update_id": 123456794,
    "message": {
      "message_id": 127,
      "chat": {"id": -1001234567890, "type": "supergroup"},
      "from": {"id": 987654321, "first_name": "John"},
      "date": 1733611700,
      "poll": {
        "id": "5678901234567890123",
        "question": "What is your favorite color?",
        "options": [
          {"text": "Red", "voter_count": 0},
          {"text": "Blue", "voter_count": 0},
          {"text": "Green", "voter_count": 0}
        ],
        "total_voter_count": 0,
        "is_closed": false,
        "is_anonymous": true,
        "type": "regular",
        "allows_multiple_answers": false
      }
    }
  }'
```

**Example: Contact Message**

```bash
curl -X POST http://localhost:8080/api/v1/ingest \
  -F 'update={
    "update_id": 123456795,
    "message": {
      "message_id": 128,
      "chat": {"id": -1001234567890, "type": "supergroup"},
      "from": {"id": 987654321, "first_name": "John"},
      "date": 1733611800,
      "contact": {
        "phone_number": "+1234567890",
        "first_name": "Jane",
        "last_name": "Doe",
        "user_id": 123456789
      }
    }
  }'
```

**Example: Venue Message**

```bash
curl -X POST http://localhost:8080/api/v1/ingest \
  -F 'update={
    "update_id": 123456796,
    "message": {
      "message_id": 129,
      "chat": {"id": -1001234567890, "type": "supergroup"},
      "from": {"id": 987654321, "first_name": "John"},
      "date": 1733611900,
      "venue": {
        "location": {
          "latitude": 40.7128,
          "longitude": -74.0060
        },
        "title": "Empire State Building",
        "address": "350 Fifth Avenue, New York, NY 10118",
        "foursquare_id": "4b5b5c7ef964a520c8a228e3"
      }
    }
  }'
```

**Example: Dice Message**

```bash
curl -X POST http://localhost:8080/api/v1/ingest \
  -F 'update={
    "update_id": 123456797,
    "message": {
      "message_id": 130,
      "chat": {"id": -1001234567890, "type": "supergroup"},
      "from": {"id": 987654321, "first_name": "John"},
      "date": 1733612000,
      "dice": {
        "emoji": "🎲",
        "value": 4
      }
    }
  }'
```

**Response**: `200 OK`

```json
{
  "status": "ok"
}
```

**Error Responses**:

- `400 Bad Request` - Invalid multipart form, missing `update` field, or malformed JSON
- `500 Internal Server Error` - Database or storage failure

### GET `/health`

Health check endpoint.

**Response**: `200 OK` with body `OK`

## Media Upload Details

### File Naming Convention

Multipart form field names **must match** the Telegram `file_id` exactly. For example:
- If photo has `file_id: "AgACAgIAAxkBAAIBCGZj..."`, the form field must be named `AgACAgIAAxkBAAIBCGZj...`
- The API extracts files by looking up `file_id` from the JSON metadata

### Supported Media Types

The API handles all Telegram message types:

**Media Files** (stored in MinIO):
- **Photos**: Multiple sizes per message (array of `PhotoSize` objects)
- **Videos**: Single video file with metadata
- **Audio**: Music files with performer/title metadata
- **Voice**: Voice messages
- **Documents**: General files with filename and MIME type
- **Animations**: GIF files
- **Video Notes**: Circular video messages
- **Stickers**: Static (WebP), animated (TGS), and video (WebM) stickers
- **Games**: Game content with photos and optional animation

**Metadata Only** (no file storage):
- **Polls**: Questions with options (regular and quiz types)
- **Contacts**: Shared phone contacts
- **Locations**: Geographic coordinates
- **Venues**: Location with place information
- **Dice**: Animated emoji results (🎲, 🎯, 🏀, ⚽, 🎳, 🎰)

### Missing Files

If a media file is referenced in the JSON metadata but not included in the multipart upload:
- The API logs a warning with the `file_id`
- The media is **skipped** (not inserted into database)
- The request **succeeds** (returns 200 OK)
- Other media in the same message are processed normally

This allows partial uploads if some files fail to download on the client side.

### Content-Addressable Storage

Media files are stored in MinIO using SHA256 hashing with database-level deduplication:

1. **Hash Calculation**: SHA256 hash computed from file binary content
2. **Database Check**: Query `media_files`, `photos`, and `game_photos` tables for existing hash
3. **Deduplication**: If hash found in DB, reuse existing MinIO object key (skip upload)
4. **Upload**: If hash not found, upload to MinIO with key `{mediaType}/{hash[:2]}/{hash}`
5. **Database**: Insert record with object key (either existing or new)

**Benefits**:
- Same file uploaded multiple times only stored once
- Cross-table deduplication (photo shared between message and game)
- No MinIO round-trip for duplicate detection (DB query only)
- Storage cost reduced for commonly shared media

**Example**:
```
Message 1: Photo A (hash: abc123...) → DB miss → Upload to MinIO → Insert record
Message 2: Photo A (hash: abc123...) → DB hit → Skip upload → Insert record with same key
Message 3: Game Photo A (hash: abc123...) → DB hit → Skip upload → Insert game_photos record
Message 4: Photo B (hash: def456...) → DB miss → Upload to MinIO → Insert record

Result: 2 files in MinIO, 4 records in database (across tables)
```

## Configuration

All configuration via environment variables:

### Required

None (all have defaults except where noted)

### Upload Limits

- `MAX_UPLOAD_SIZE_MB` (default: `100`) - Maximum total size of multipart request in megabytes

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

2. Edit `.env.dev` and configure settings (all have defaults, customize as needed)

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

### Testing the API

**Test text message ingestion**:

```bash
curl -X POST http://localhost:8080/api/v1/ingest \
  -F 'update={"update_id":1,"message":{"message_id":1,"chat":{"id":-100123,"type":"supergroup","title":"Test"},"from":{"id":456,"first_name":"User"},"date":1733611200,"text":"Test message"}}'
```

**Test with file upload**:

```bash
# Create a test file
echo "test content" > test.txt

# Upload with message
curl -X POST http://localhost:8080/api/v1/ingest \
  -F 'update={"update_id":2,"message":{"message_id":2,"chat":{"id":-100123,"type":"supergroup"},"from":{"id":456,"first_name":"User"},"date":1733611300,"document":{"file_id":"test123","file_unique_id":"uniq123","file_name":"test.txt","mime_type":"text/plain","file_size":13}}}' \
  -F 'test123=@test.txt'
```

**Check health**:

```bash
curl http://localhost:8080/health
```

## Storage Architecture

### MinIO Object Structure

Files are stored using content-addressable naming:

```
telegram-media/
├── photo/
│   ├── ab/
│   │   └── abc123def456...  (SHA256 hash)
│   └── cd/
│       └── cde234efg567...
├── video/
│   └── 12/
│       └── 123456abc789...
├── audio/
│   └── de/
│       └── def789ghi012...
├── document/
│   └── 34/
│       └── 345678bcd901...
├── sticker/
│   └── ef/
│       └── efg456hij789...
├── game_photo/
│   └── 56/
│       └── 567890abc123...
├── voice/
├── video_note/
└── animation/
```

**Benefits**:
- Predictable object keys from file content
- Automatic deduplication
- Easy verification (re-hash file and check if exists)
- Efficient storage distribution via hash prefix

### Database Records

Every message gets database records even if media already exists:

```sql
-- Multiple messages can reference same file
SELECT
    m.message_id,
    m.text,
    mf.telegram_file_id,
    mf.file_hash,
    mf.minio_object_key
FROM messages m
JOIN media_files mf ON mf.message_id = m.id
WHERE mf.file_hash = 'abc123...';

-- Find all messages sharing the same media file
SELECT
    COUNT(*) as share_count,
    mf.file_hash,
    mf.file_name
FROM media_files mf
GROUP BY mf.file_hash, mf.file_name
HAVING COUNT(*) > 1;
```

## Ingestion Flow

The complete ingestion process:

1. **Receive Request**: Multipart form parsed with size limit
2. **Extract Update**: JSON deserialized from `update` field
3. **Extract Files**: Binary content read from remaining form fields into `map[file_id][]byte`
4. **Begin Transaction**: Database transaction started
5. **Insert Update Record**: Raw update stored in `updates` table
6. **Process by Type**:
   - **Message**: Upsert chat, user → Insert message → Process entities → Process media → Process location
   - **Edited Message**: Same as message + insert edit history
   - **Reaction**: Upsert chat/user → Insert reaction
   - **Reaction Count**: Upsert chat → Update counts
7. **For Each Media File**:
   - Lookup file content from files map by `file_id`
   - If not found: Log warning and skip
   - If found: Compute SHA256 → Check DB for existing hash → Reuse object key or upload new → Insert database record
8. **Commit Transaction**: All database changes persisted
9. **Return Response**: 200 OK

**Transactional Guarantees**:
- Database and MinIO operations in same request
- MinIO uploads happen before transaction commit
- Transaction rolls back on any database error
- Duplicate file uploads are safe (idempotent)

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
│   │   └── ingest_handler.go      # HTTP request parsing and response
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
│   ├── services/
│   │   └── ingest_service.go      # Business logic and transaction management
│   └── storage/
│       └── minio_client.go        # MinIO storage client
├── go.mod
└── Dockerfile
```

## Shared Packages

```
pkg/config/                         # Shared configuration package
├── config.go                       # Config loading with envconfig
└── go.mod
```
