# ML Dashboard

A dev-only web dashboard for exploring ML-processed data from the ml-processor service.

## Overview

The ML Dashboard provides a visual interface to browse and analyze the machine learning results stored in PostgreSQL and Qdrant. It's designed for development and debugging purposes, allowing you to:

- Browse messages with their ML annotations (sentiment, toxicity, humor, questions)
- Perform semantic similarity search using Qdrant embeddings
- Explore topic clusters and their keywords
- View user analytics with aggregated ML statistics

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    ML Dashboard                              │
├─────────────────────────┬───────────────────────────────────┤
│   Frontend (React)      │        Backend (FastAPI)          │
│   Port: 5175            │        Port: 8052                 │
│                         │                                   │
│   - Dashboard           │   /api/stats      - Stats         │
│   - Messages Browser    │   /api/messages   - ML results    │
│   - Semantic Search     │   /api/search     - Qdrant search │
│   - Topic Explorer      │   /api/topics     - Clusters      │
│   - User Analytics      │   /api/users      - User stats    │
└─────────────────────────┴───────────────────────────────────┘
            │                           │
            │                           ├──► PostgreSQL (ML tables)
            │                           └──► Qdrant (embeddings)
            │
            └──► Backend API (localhost:8052)
```

## Quick Start

```bash
# Start the dashboard (builds images if needed)
make ml-dashboard-up-build

# Access the dashboard
open http://localhost:5175

# View logs
make ml-dashboard-logs
```

## Features

### 1. Dashboard
- Processing progress (messages analyzed vs total)
- ML result counts (sentiment, toxicity, humor, questions, entities, topics)
- Qdrant connection status and embedding count

### 2. Messages Browser
- Paginated list of messages with ML annotations
- Filter by:
  - Sentiment (positive, neutral, negative)
  - Toxicity (toxic / not toxic)
  - Humor (humorous / not humorous)
  - Questions (is question / not question)
  - Topic cluster
- Sort by date, toxicity score, or sentiment score
- Click to expand message details with named entities

### 3. Semantic Search
- Text-to-vector search using the same embedding model as ml-processor
- Find semantically similar messages across the chat
- Shows similarity scores and timing metrics
- **Note:** Only available when Qdrant is running (dev environment)

### 4. Topic Explorer
- Browse discovered topic clusters
- View keywords extracted for each topic
- Drill down into topic messages sorted by similarity
- See outlier count (unclustered messages)

### 5. User Analytics
- User list with aggregated ML statistics:
  - Average sentiment (-1 to +1)
  - Toxicity rate (% of toxic messages)
  - Humor rate (% of humorous messages)
  - Question rate (% of questions)
- User profile with sentiment distribution pie chart
- Entity mention frequency
- Card history with weekly stats

## Database Access Modes

### Development (Default)
Connects to the local PostgreSQL container and Qdrant:
- PostgreSQL: `postgres:5432` (docker container)
- Qdrant: `qdrant:6333` (docker container)

```bash
make ml-dashboard-up
```

### Production Database (via SSH Tunnel)
Connect to production PostgreSQL while keeping local Qdrant:

```bash
# Terminal 1: Open SSH tunnel
make pg-tunnel

# Terminal 2: Restart backend with prod connection
docker compose -f infrastructure/docker-compose.dev.yml stop ml-dashboard-backend
DB_HOST=host.docker.internal DB_PORT=5433 \
  docker compose -f infrastructure/docker-compose.dev.yml up -d ml-dashboard-backend
```

**Note:** Semantic search won't work with production data since Qdrant only runs locally.

## API Endpoints

### Stats & Chats
| Endpoint | Description |
|----------|-------------|
| `GET /health` | Health check |
| `GET /api/chats` | List all chats with message counts |
| `GET /api/stats?chat_id=X` | ML processing statistics |

### Messages
| Endpoint | Description |
|----------|-------------|
| `GET /api/messages?chat_id=X` | List messages with ML results |
| `GET /api/messages/{id}` | Single message with entities |

Query parameters for `/api/messages`:
- `limit` (default: 50, max: 200)
- `offset` (default: 0)
- `user_id` - Filter by user
- `sentiment` - `positive`, `neutral`, or `negative`
- `is_toxic` - `true` or `false`
- `is_humorous` - `true` or `false`
- `is_question` - `true` or `false`
- `topic_id` - Filter by topic cluster
- `sort_by` - `date`, `toxicity_score`, or `sentiment_score`
- `sort_order` - `asc` or `desc`

### Search
| Endpoint | Description |
|----------|-------------|
| `GET /api/search/status` | Qdrant availability status |
| `POST /api/search` | Semantic similarity search |

Search request body:
```json
{
  "query": "search text",
  "chat_id": 123,
  "user_id": 456,
  "limit": 20
}
```

### Topics
| Endpoint | Description |
|----------|-------------|
| `GET /api/topics?chat_id=X` | List topic clusters |
| `GET /api/topics/{id}/messages?chat_id=X` | Messages in a topic |

### Users
| Endpoint | Description |
|----------|-------------|
| `GET /api/users?chat_id=X` | List users with ML stats |
| `GET /api/users/{id}/profile?chat_id=X` | User sentiment & entities |
| `GET /api/users/{id}/cards?chat_id=X` | User card history |

## Configuration

### Backend Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_HOST` | `postgres` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | `postgres` | Database user |
| `DB_PASSWORD` | - | Database password |
| `DB_NAME` | `beef_briefing` | Database name |
| `QDRANT_HOST` | `qdrant` | Qdrant host |
| `QDRANT_PORT` | `6333` | Qdrant port |
| `QDRANT_ENABLED` | `true` | Enable Qdrant connection |
| `LOG_LEVEL` | `debug` | Logging level |

### Frontend Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_API_URL` | `http://localhost:8052` | Backend API URL |

## Development

### Backend Structure
```
backend/
├── main.py           # FastAPI app, lifespan, global instances
├── config.py         # Pydantic settings
├── api/
│   ├── messages.py   # /api/messages endpoints
│   ├── search.py     # /api/search endpoints
│   ├── topics.py     # /api/topics endpoints
│   └── users.py      # /api/users endpoints
├── db/
│   └── queries.py    # SQL queries (follows MLQueries pattern)
└── vector/
    ├── qdrant.py     # Qdrant search wrapper
    └── embeddings.py # Sentence transformer for search
```

### Frontend Structure
```
frontend/
├── src/
│   ├── App.tsx           # Router, chat context
│   ├── api/client.ts     # API client
│   ├── types/index.ts    # TypeScript interfaces
│   ├── styles/global.css # Dark theme
│   └── pages/
│       ├── DashboardPage.tsx
│       ├── MessagesPage.tsx
│       ├── SearchPage.tsx
│       ├── TopicsPage.tsx
│       └── UsersPage.tsx
```

### Running Locally (without Docker)

**Backend:**
```bash
cd apps/ml-dashboard/backend
pip install -r requirements.txt
DB_HOST=localhost DB_PORT=5432 python main.py
```

**Frontend:**
```bash
cd apps/ml-dashboard/frontend
npm install
npm run dev
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `make ml-dashboard-up` | Start dashboard services |
| `make ml-dashboard-up-build` | Rebuild and start |
| `make ml-dashboard-down` | Stop dashboard services |
| `make ml-dashboard-logs` | Tail all logs |
| `make ml-dashboard-logs-backend` | Tail backend logs |
| `make ml-dashboard-logs-frontend` | Tail frontend logs |
| `make ml-dashboard-shell` | Shell into backend container |

## Troubleshooting

### "Database not connected" error
Make sure PostgreSQL is running:
```bash
make dev-up  # Start all dev services including postgres
```

### Semantic search unavailable
Qdrant must be running and have embeddings:
```bash
# Check Qdrant status
curl http://localhost:6333/collections/message_embeddings

# If empty, run ml-processor to generate embeddings
make ml-run ML_ARGS="--chat-id YOUR_CHAT_ID"
```

### No chats showing
The database needs message data. Either:
1. Run the telegram-bot to collect messages
2. Import data using import-cli
3. Restore a database backup

### Frontend not loading
Check if the backend is accessible:
```bash
curl http://localhost:8052/health
# Should return: {"status":"healthy"}
```
