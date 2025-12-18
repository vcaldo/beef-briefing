# ML Processor

Local ML analytics processor for Portuguese Telegram chat messages. Runs on GPU (RTX 4070) and communicates with the api-service via REST API.

## Architecture

```
ml-processor (local, GPU)
    │
    ├── GET /api/v1/ml/messages  → api-service → PostgreSQL
    ├── POST /api/v1/ml/results  → api-service → PostgreSQL
    └── Store embeddings         → Qdrant (Docker)
```

## Analysis Types

| Analysis | Description | Provider |
|----------|-------------|----------|
| **Sentiment** | Positive/Neutral/Negative classification | Local or OpenAI/Anthropic |
| **Toxicity** | Hate speech and toxic content detection | Local or Perspective/OpenAI |
| **Embeddings** | 768/1536-dimensional vector representations | Local or OpenAI |
| **Topics** | HDBSCAN clustering with keyword extraction | Local or OpenAI |
| **NER** | Named Entity Recognition (people, places, orgs) | Local (spaCy) or OpenAI |
| **Humor** | Humor detection using Brazilian laugh patterns | Local or OpenAI |
| **Questions** | Question detection and classification | Local (zero-shot) or OpenAI |

## Models Used (Local Providers)

| Model | Purpose | Memory |
|-------|---------|--------|
| `lxyuan/distilbert-base-multilingual-cased-sentiments-student` | Sentiment analysis | ~270MB |
| `ruanchaves/bert-base-portuguese-cased-hatebr` | Toxicity detection (Portuguese) | ~440MB |
| `sentence-transformers/paraphrase-multilingual-mpnet-base-v2` | Embeddings (768-dim) | ~420MB |
| `pt_core_news_lg` | NER (spaCy Portuguese) | ~550MB |
| `facebook/bart-large-mnli` | Question classification (zero-shot) | ~1.6GB |

Total GPU memory: ~3.3GB (with all local models loaded)

## Setup

```bash
# Create virtual environment
python -m venv venv
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt

# Copy environment file
cp .env.example .env
# Edit .env as needed
```

## Usage

### Using Make (Recommended)

```bash
# Start Docker services first (from project root)
make up-build

# Development (local API at localhost:8080)
make ml-run              # Run continuous processing
make ml-run-once         # Run single batch
make ml-run-status       # Check status

# Production (barra-pesada.online)
make ml-run-prod         # Run continuous processing
make ml-run-once-prod    # Run single batch
make ml-run-status-prod  # Check status

# Custom API URL
make ml-run-prod PROD_API_URL=https://custom.domain.com
```

### Direct Python Usage

```bash
# Activate virtual environment
source venv/bin/activate

# Run continuous processing
python main.py

# Run single batch
python main.py --once

# Check status
python main.py --status

# Override batch size
python main.py --limit 100

# Point to different API (e.g., production)
python main.py --api-url https://barra-pesada.online

# Use different API key file
python main.py --api-key-file /path/to/api_key
```

## Configuration

Environment variables (see `.env.example`):

| Variable | Default | Description |
|----------|---------|-------------|
| `API_SERVICE_URL` | `http://localhost:8080` | api-service URL |
| `API_KEY_FILE` | `../../infrastructure/secrets/apps/ml-processor/api_key` | Path to API key |
| `QDRANT_HOST` | `localhost` | Qdrant host |
| `QDRANT_PORT` | `6333` | Qdrant port |
| `BATCH_SIZE` | `500` | Messages per batch |
| `SLEEP_SECONDS` | `60` | Sleep when no messages |
| `DEVICE` | `cuda` | PyTorch device |

### Provider Selection

Each analysis type can use a different provider (`local`, `openai`, `anthropic`, `perspective`):

| Variable | Default | Options |
|----------|---------|---------|
| `SENTIMENT_PROVIDER` | `local` | `local`, `openai`, `anthropic` |
| `TOXICITY_PROVIDER` | `local` | `local`, `perspective`, `openai` |
| `EMBEDDINGS_PROVIDER` | `local` | `local`, `openai` |
| `TOPICS_PROVIDER` | `local` | `local`, `openai` |
| `NER_PROVIDER` | `local` | `local`, `openai` |
| `HUMOR_PROVIDER` | `local` | `local`, `openai` |
| `QUESTIONS_PROVIDER` | `local` | `local`, `openai` |

API keys (required for non-local providers):

| Variable | Required for |
|----------|--------------|
| `OPENAI_API_KEY` | `openai` provider |
| `ANTHROPIC_API_KEY` | `anthropic` provider |
| `PERSPECTIVE_API_KEY` | `perspective` provider |

## Output

Results are stored in PostgreSQL and Qdrant:

| Analysis | Storage | Table/Collection |
|----------|---------|------------------|
| Sentiment | PostgreSQL | `ml_sentiment` |
| Toxicity | PostgreSQL | `ml_toxicity` |
| NER | PostgreSQL | `ml_ner` |
| Topics | PostgreSQL | `ml_topics`, `ml_message_topics` |
| Humor | PostgreSQL | `ml_humor` |
| Questions | PostgreSQL | `ml_questions` |
| Embeddings | Qdrant | `message_embeddings` |
| Processing State | PostgreSQL | `ml_processing_state` |

## Development

### Reset Processing State

To reprocess all messages, use the Makefile targets:

```bash
# Reset dev environment (PostgreSQL + Qdrant)
make ml-clean-dev

# Reset prod environment (requires confirmation)
make ml-clean-prod
```

Or manually clear the ML tables:

```sql
-- Connect to database
docker exec -it beef-briefing-postgres psql -U postgres -d beef_briefing

-- Reset all ML data (reprocess everything)
TRUNCATE ml_user_profiles, ml_user_cards, ml_message_topics, ml_topics,
         ml_ner, ml_humor, ml_questions, ml_toxicity, ml_sentiment,
         ml_processing_state CASCADE;

-- Reset only processing state (keeps old results, will create duplicates)
TRUNCATE ml_processing_state;

-- Reset for a specific chat only
DELETE FROM ml_processing_state WHERE chat_id = -1003280306634;
DELETE FROM ml_sentiment WHERE chat_id = -1003280306634;
DELETE FROM ml_toxicity WHERE chat_id = -1003280306634;
DELETE FROM ml_ner WHERE chat_id = -1003280306634;
DELETE FROM ml_humor WHERE chat_id = -1003280306634;
DELETE FROM ml_questions WHERE chat_id = -1003280306634;
DELETE FROM ml_message_topics WHERE chat_id = -1003280306634;
```

To also clear Qdrant embeddings:

```bash
# Delete entire collection (will be recreated on next run)
curl -X DELETE "http://localhost:6333/collections/message_embeddings"

# Or delete points for specific chat
curl -X POST "http://localhost:6333/collections/message_embeddings/points/delete" \
  -H "Content-Type: application/json" \
  -d '{"filter": {"must": [{"key": "chat_id", "match": {"value": -1003280306634}}]}}'
```
