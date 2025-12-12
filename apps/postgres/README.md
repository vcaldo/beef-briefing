# PostgreSQL Database

PostgreSQL 17 with PostGIS 3.4 database for the Beef Briefing system. Stores all Telegram chat data including messages, reactions, media metadata, and user information.

## Features

- **22 tables** modeling complete Telegram data structure
- **Content-addressable storage** for media deduplication via SHA256 hashes
- **Group migration tracking** for supergroup upgrades
- **Denormalized reactions** for handling reactions to uncaptured messages
- **Automatic timestamps** via `updated_at` triggers

## Schema Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        CORE ENTITIES                            │
├─────────────────────────────────────────────────────────────────┤
│  chats          │  users         │  updates                     │
│  - id (PK)      │  - id (PK)     │  - update_id (unique)        │
│  - type         │  - is_bot      │  - update_type               │
│  - title        │  - first_name  │  - raw_data (JSONB)          │
│  - migrated_from│  - username    │                              │
└─────────────────┴────────────────┴──────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────────┐
│                         MESSAGES                                │
├─────────────────────────────────────────────────────────────────┤
│  messages            │  message_entities    │  message_edits    │
│  - chat_id (FK)      │  - message_id (FK)   │  - message_id (FK)│
│  - user_id (FK)      │  - entity_type       │  - edit_date      │
│  - message_id        │  - offset/length     │  - prev/new text  │
│  - text/caption      │  - url/user_id       │                   │
│  - reply_to_msg_id   │                      │                   │
└──────────────────────┴──────────────────────┴───────────────────┘
                           │
           ┌───────────────┼───────────────┐
           ▼               ▼               ▼
┌──────────────────┬───────────────┬──────────────────────────────┐
│      MEDIA       │   REACTIONS   │     EXTENDED TYPES           │
├──────────────────┼───────────────┼──────────────────────────────┤
│  media_files     │  msg_reactions│  polls / poll_options        │
│  photos          │  (denorm.)    │  contacts                    │
│  stickers        │               │  locations / venues          │
│  games           │  reaction_    │  dice                        │
│  game_photos     │  counts       │                              │
└──────────────────┴───────────────┴──────────────────────────────┘
```

## Tables

### Core Tables

| Table | Description |
|-------|-------------|
| `chats` | Telegram chats (groups, supergroups, channels, private) |
| `users` | Telegram users with profile information |
| `updates` | Raw Telegram webhook payloads for audit/debugging |

### Message Tables

| Table | Description |
|-------|-------------|
| `messages` | All message types with text, caption, metadata |
| `message_entities` | Text formatting (bold, links, mentions, etc.) |
| `message_edits` | Edit history audit trail |

### Reaction Tables (Denormalized)

| Table | Description |
|-------|-------------|
| `message_reactions` | Individual user reactions |
| `reaction_counts` | Aggregate anonymous counts |

**Design Note:** Reaction tables use Telegram message ID (not FK) to allow storing reactions for messages not yet captured.

### Media Tables

| Table | Description |
|-------|-------------|
| `media_files` | Video, audio, voice, document, animation, video_note, sticker |
| `photos` | Photo sizes (multiple per message) |
| `stickers` | Sticker-specific metadata (linked to media_files) |
| `games` | Game content with title/description |
| `game_photos` | Game photos (multiple per game) |

### Extended Message Types

| Table | Description |
|-------|-------------|
| `polls` | Poll questions with type and options |
| `poll_options` | Poll answer choices with voter counts |
| `contacts` | Shared phone contacts |
| `locations` | Geographic coordinates |
| `venues` | Location with place information |
| `dice` | Animated emoji results |

## Enums

| Enum | Values |
|------|--------|
| `chat_type` | `private`, `group`, `supergroup`, `channel` |
| `media_type` | `photo`, `video`, `audio`, `voice`, `document`, `animation`, `video_note`, `sticker` |
| `reaction_type` | `emoji`, `custom_emoji`, `paid` |
| `poll_type` | `regular`, `quiz` |

## Key Design Patterns

### Content-Addressable Storage

Media files use SHA256 hashing for deduplication:

```sql
-- file_hash column enables cross-table deduplication
SELECT file_hash, minio_object_key FROM media_files WHERE file_hash = 'abc123...';
SELECT file_hash, minio_object_key FROM photos WHERE file_hash = 'abc123...';
SELECT file_hash, minio_object_key FROM game_photos WHERE file_hash = 'abc123...';
```

MinIO storage path: `{mediaType}/{hash[:2]}/{hash}`

### Group Migration

When a Telegram group upgrades to a supergroup:

```sql
-- Old group ID stored in migrated_from_chat_id
SELECT id, title, migrated_from_chat_id FROM chats WHERE id = -1003280306634;
-- Returns: -1003280306634 | My Group | 3280306634

-- Find messages from both old and new chat
SELECT m.* FROM messages m
JOIN chats c ON m.chat_id = c.id
WHERE c.id = -1003280306634 OR c.migrated_from_chat_id = 3280306634;
```

### Denormalized Reactions

Reactions store Telegram message ID, not database FK:

```sql
-- Find reactions by joining on (chat_id, message_id)
SELECT mr.*, m.text FROM message_reactions mr
LEFT JOIN messages m ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
WHERE mr.chat_id = -1003280306634;
```

This allows storing reactions for messages sent before the bot joined.

## Useful Queries

### Message Statistics by Chat

```sql
SELECT
    c.title,
    COUNT(m.id) as total_messages,
    COUNT(DISTINCT m.user_id) as unique_users,
    MIN(m.date) as first_message,
    MAX(m.date) as last_message
FROM chats c
JOIN messages m ON c.id = m.chat_id
GROUP BY c.id, c.title;
```

### Top Users by Message Count

```sql
SELECT
    u.first_name,
    u.username,
    COUNT(m.id) as message_count
FROM users u
JOIN messages m ON u.id = m.user_id
WHERE m.chat_id = -1003280306634
GROUP BY u.id
ORDER BY message_count DESC
LIMIT 10;
```

### Media Files by Type

```sql
SELECT
    media_type,
    COUNT(*) as count,
    SUM(file_size) as total_bytes
FROM media_files
GROUP BY media_type
ORDER BY count DESC;
```

### Reaction Summary

```sql
SELECT
    emoji_value,
    COUNT(*) as reaction_count
FROM message_reactions
WHERE chat_id = -1003280306634 AND NOT is_removed
GROUP BY emoji_value
ORDER BY reaction_count DESC;
```

### Thread Traversal (Recursive)

```sql
WITH RECURSIVE thread AS (
    SELECT * FROM messages WHERE id = $1
    UNION ALL
    SELECT m.* FROM messages m
    INNER JOIN thread t ON m.id = t.reply_to_message_id
)
SELECT * FROM thread;
```

### Daily Activity Heatmap

```sql
SELECT
    DATE(date) as day,
    COUNT(*) as message_count
FROM messages
WHERE chat_id = -1003280306634
  AND date >= NOW() - INTERVAL '1 year'
GROUP BY DATE(date)
ORDER BY day;
```

## Migrations

Migrations are in `migrations/` and run automatically when the PostgreSQL container starts.

| File | Description |
|------|-------------|
| `001_initial.sql` | Complete schema with all tables, indexes, triggers |

### Running Migrations Manually

```bash
# Connect to database
docker exec -it beef-postgres-dev psql -U postgres -d beef_briefing

# Run migration file
\i /docker-entrypoint-initdb.d/001_initial.sql
```

## Seeds

Seed data for development/testing is in `seeds/`.

| File | Description |
|------|-------------|
| `001_initial_data.sql` | Sample data for development |

## Docker Configuration

### Development

```yaml
# docker-compose.dev.yml
postgres:
  image: postgis/postgis:17-3.4
  environment:
    POSTGRES_USER: postgres
    POSTGRES_PASSWORD: changeme
    POSTGRES_DB: beef_briefing
  volumes:
    - postgres_data:/var/lib/postgresql/data
    - ./apps/postgres/migrations:/docker-entrypoint-initdb.d
```

### Production

```yaml
# docker-compose.prod.yml
postgres:
  image: postgis/postgis:17-3.4
  environment:
    POSTGRES_USER: ${DB_USER}
    POSTGRES_PASSWORD: ${DB_PASSWORD}
    POSTGRES_DB: ${DB_NAME}
  volumes:
    - ${POSTGRES_DATA_PATH}/pgdata:/var/lib/postgresql/data
    - ./apps/postgres/migrations:/docker-entrypoint-initdb.d
```

Production uses Linode block storage mounted at `${POSTGRES_DATA_PATH}`.

## Backup & Restore

### Backup

```bash
# From repo root (production)
make backup-prod-db

# Manual backup
docker exec beef-postgres-dev pg_dump -U postgres beef_briefing > backup.sql

# Backup with compression
docker exec beef-postgres-dev pg_dump -U postgres -Fc beef_briefing > backup.dump
```

### Restore

```bash
# From SQL file
docker exec -i beef-postgres-dev psql -U postgres beef_briefing < backup.sql

# From compressed dump
docker exec -i beef-postgres-dev pg_restore -U postgres -d beef_briefing < backup.dump
```

## Related Documentation

- [API Service](../api-service/README.md) - Data ingestion logic
- [Infrastructure](../../infrastructure/README.md) - Docker Compose setup
- [Terraform](../../infrastructure/terraform/README.md) - Block storage provisioning
