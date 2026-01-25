# Beef Briefing - Technical Documentation

Technical documentation for the Beef Briefing system architecture and services.

## Architecture Overview

```
                                    Internet
                                       │
                                       ▼
                              ┌─────────────────┐
                              │     Traefik     │
                              │  (SSL/Routing)  │
                              └────────┬────────┘
                                       │
    ┌──────────────┬───────────────────┼───────────────────┬──────────────┐
    │              │                   │                   │              │
    ▼              ▼                   ▼                   ▼              ▼
┌────────┐  ┌────────────┐  ┌─────────────────┐  ┌────────────┐  ┌────────┐
│ Arena  │  │ Leaderboard│  │  API Service    │  │    Deck    │  │  ML    │
│Mini App│  │  Mini App  │  │   (Go:8080)     │  │  Mini App  │  │Dashboard
└────────┘  └────────────┘  └────────┬────────┘  └────────────┘  └────────┘
                                     │
        ┌────────────────────────────┼────────────────────────────┐
        │                            │                            │
        ▼                            ▼                            ▼
┌───────────────┐         ┌─────────────────────┐         ┌───────────────┐
│ Telegram Bot  │         │    PostgreSQL       │         │ Card Renderer │
│     (Go)      │────────▶│     (Database)      │◀────────│  (Py:8051)    │
└───────────────┘         └─────────────────────┘         └───────────────┘
        │                            ▲                            │
        │                            │                            │
        ▼                            │                            ▼
┌───────────────┐         ┌─────────────────────┐         ┌───────────────┐
│   Telegram    │         │    ML Processor     │         │ MinIO/S3      │
│     API       │         │   (Python/OpenAI)   │         │  (Storage)    │
└───────────────┘         └─────────────────────┘         └───────────────┘
                                     │
                                     ▼
                          ┌─────────────────────┐
                          │       Qdrant        │
                          │    (Embeddings)     │
                          └─────────────────────┘
```

## Services

### Backend Services

| Service | Language | Port | Description |
|---------|----------|------|-------------|
| [api-service](api-service/README.md) | Go | 8080 | Central REST API for data ingestion and retrieval |
| [telegram-bot](telegram-bot/README.md) | Go | - | Real-time Telegram message listener |
| [card-renderer](card-renderer/README.md) | Python | 8051 | Gamified stats card image generator |
| [ml-processor](ml-processor/README.md) | Python | - | ML analytics pipeline (sentiment, humor, toxicity) |
| [ml-dashboard](ml-dashboard/README.md) | Python/React | 5173 | ML analytics dashboard with FastAPI backend |
| [import-cli](import-cli/README.md) | Go | - | Telegram Desktop export importer |

### Telegram Mini Apps

| Service | Language | Port | Description |
|---------|----------|------|-------------|
| [arena-mini-app](arena-mini-app/README.md) | React | 5175 | Turn-based card battle arena |
| [deck-mini-app](deck-mini-app/README.md) | React | 5174 | Card gallery for browsing weekly cards |
| [leaderboard-mini-app](leaderboard-mini-app/README.md) | React | 5173 | Stats leaderboard and user profiles |

## Data Flow

### Real-time Ingestion

```
Telegram Group → Telegram API → telegram-bot → api-service → PostgreSQL
                                     │                           │
                                     └──── Media Files ──────────┼──→ MinIO
```

1. **telegram-bot** receives messages via Telegram Bot API (long polling)
2. Downloads media files concurrently (max 5 simultaneous)
3. Sends updates to **api-service** as multipart form data
4. **api-service** stores message metadata in PostgreSQL
5. Media files are deduplicated by SHA256 hash and stored in MinIO

### ML Processing

```
PostgreSQL → ml-processor → PostgreSQL (ML results)
                  │
                  └──→ Qdrant (embeddings)
```

1. **ml-processor** fetches unprocessed messages from api-service
2. Runs sentiment, humor, toxicity, and entity analysis
3. Generates weekly user stats cards with aggregated metrics
4. Stores embeddings in Qdrant for similarity search

### Card Generation

```
PostgreSQL (ml_user_cards) → card-renderer → MinIO (PNG images)
```

1. **card-renderer** queries weekly user stats from database
2. Renders HTML/CSS templates with Playwright (headless Chromium)
3. Stores generated PNG images in MinIO with presigned URLs

### Mini App Flow

```
Telegram Mini App → api-service (JWT auth) → PostgreSQL
                                    │
                                    └──→ MinIO (presigned URLs)
```

1. Mini Apps authenticate using Telegram init_data
2. API validates signature and issues JWT tokens
3. Authenticated requests fetch stats and card images

### Arena Game Flow

```
arena-mini-app → api-service (JWT auth) → PostgreSQL (matches, participants)
                      │
                      └──→ Shop: Deal cards from weekly pool
                      └──→ Battle: Calculate damage, determine winner
                      └──→ Leaderboard: Track tournament rankings
```

1. Players join matches via `/matches` endpoint
2. Shop phase: Buy cards, upgrade stats (ATK/HP)
3. Battle phase: Automated turn-based combat
4. Results: ELO-based rankings and tournament points

## Database Schema

30+ tables modeling the complete Telegram data structure:

- **Core**: `chats`, `users`, `updates`
- **Messages**: `messages`, `message_entities`, `message_edits`
- **Reactions**: `message_reactions`, `reaction_counts`
- **Media**: `media_files`, `photos`, `stickers`, `games`, `game_photos`
- **Other**: `polls`, `poll_options`, `contacts`, `locations`, `venues`, `dice`
- **ML Results**: `ml_sentiment`, `ml_toxicity`, `ml_humor`, `ml_ner`, `ml_user_cards`, `ml_user_card_images`
- **Arena Game**: `matches`, `match_participants`, `match_rounds`, `ranked_tournaments`, `tournament_participants`

## Development

### Prerequisites

- Go 1.25+
- Python 3.14+
- Node.js 20+
- Docker & Docker Compose
- PostgreSQL 17+

### Quick Start

```bash
# Clone and configure
git clone <repo>
cd beef-briefing
cp infrastructure/.env.dev.example infrastructure/.env.dev
# Edit .env.dev with your TELEGRAM_BOT_TOKEN

# Generate secrets
make secrets-service-api APP=telegram-bot

# Start all services
make up-build

# View logs
make logs
```

### Common Commands

```bash
make up              # Start dev services
make up-build        # Rebuild and start
make down            # Stop services
make logs            # Tail all logs
make logs-api        # Tail api-service logs
make logs-bot        # Tail telegram-bot logs
```

See [infrastructure/README.md](../infrastructure/README.md) for deployment and production setup.

## Authentication

### API Key Authentication

Internal services use API keys stored in `infrastructure/secrets/apps/`:

```bash
# Generate API key for a service
make secrets-service-api APP=telegram-bot
```

### JWT Authentication (Mini Apps)

Mini Apps use JWT tokens obtained through Telegram init_data validation:

1. Mini App sends Telegram init_data to `/api/v1/mini-app/auth`
2. API validates HMAC-SHA256 signature using bot token
3. Returns JWT valid for 24 hours

## Storage

### PostgreSQL

Main database for structured data. Migrations auto-run on api-service startup.

### MinIO (Development) / S3 (Production)

Content-addressable storage for media files:
- Path pattern: `{mediaType}/{hash[:2]}/{hash}`
- Automatic deduplication via SHA256
- Presigned URLs for secure access

### Qdrant

Vector database for message embeddings (768/1536 dimensions).
