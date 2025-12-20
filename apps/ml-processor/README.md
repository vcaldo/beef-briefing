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

# Run continuous processing (daemon mode)
python main.py continuous --chat-id -1003280306634

# Run batch processing (all unprocessed messages)
python main.py process --chat-id -1003280306634

# Run batch with limit
python main.py process --chat-id -1003280306634 --limit 1000

# Process messages within a date range
python main.py process --chat-id -1003280306634 --from-date 2024-11-01 --to-date 2024-12-01

# Process messages from a specific date onwards
python main.py process --chat-id -1003280306634 --from-date 2024-11-18

# Process messages until a specific date
python main.py process --chat-id -1003280306634 --to-date 2024-12-01

# Check processing status
python main.py status --chat-id -1003280306634

# Generate weekly user cards (aggregates ML results)
python main.py generate-cards --chat-id -1003280306634

# Generate cards for a specific week
python main.py generate-cards --chat-id -1003280306634 --week 2024-12-16

# Generate cards with custom window and minimum messages
python main.py generate-cards --chat-id -1003280306634 --window-days 30 --min-messages 10
```

## Configuration

Environment variables (see `.env.example`):

| Variable | Default | Description |
|----------|---------|-------------|
| `API_SERVICE_URL` | `http://localhost:8080` | api-service URL |
| `API_KEY_FILE` | `../../infrastructure/secrets/apps/ml-processor/api_key` | Path to API key |
| `QDRANT_HOST` | `localhost` | Qdrant host |
| `QDRANT_PORT` | `6333` | Qdrant port |
| `BATCH_SIZE` | `500` | Messages per batch (set via `ML_BATCH_SIZE` in docker-compose). **Note:** When using OpenAI providers, keep this ≤100 to avoid token limits. |
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

**Important:** OpenAI chat-based analyzers (sentiment, humor, questions, NER) batch all messages into a single prompt. Use `BATCH_SIZE ≤ 100` when using OpenAI providers to avoid exceeding token limits. The embeddings and moderation APIs have proper internal batching and can handle larger batch sizes.

### OpenAI Rate Limiting

When using OpenAI providers, built-in rate limiting ensures you stay within your tier limits. Rate limiting is **enabled by default** with Tier 1 limits.

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENAI_RATE_LIMIT_ENABLED` | `true` | Enable/disable rate limiting |
| `OPENAI_RATE_LIMIT_TIMEOUT` | `120.0` | Max seconds to wait for capacity |
| `OPENAI_GPT4O_MINI_TPM` | `200000` | gpt-4o-mini tokens per minute |
| `OPENAI_GPT4O_MINI_RPM` | `500` | gpt-4o-mini requests per minute |
| `OPENAI_EMBEDDING_TPM` | `1000000` | text-embedding-3-small tokens per minute |
| `OPENAI_EMBEDDING_RPM` | `3000` | text-embedding-3-small requests per minute |
| `OPENAI_MODERATION_TPM` | `10000` | omni-moderation-latest tokens per minute |
| `OPENAI_MODERATION_RPM` | `500` | omni-moderation-latest requests per minute |

**Model to Analyzer Mapping:**
- `gpt-4o-mini`: sentiment, humor, questions, NER (shared limits)
- `text-embedding-3-small`: embeddings, topics (shared limits)
- `omni-moderation-latest`: toxicity

**How it works:**
- Uses token bucket algorithm for both RPM and TPM limits
- Multiple analyzers sharing the same model coordinate through a shared rate limiter
- When limits are reached, requests wait until capacity is available (up to timeout)
- Token usage is estimated before requests and adjusted after based on actual usage

**Tier Reference (adjust variables for your tier):**

| Tier | gpt-4o-mini TPM | gpt-4o-mini RPM | Embedding TPM | Embedding RPM |
|------|-----------------|-----------------|---------------|---------------|
| 1    | 200,000         | 500             | 1,000,000     | 3,000         |
| 2    | 2,000,000       | 5,000           | 1,000,000     | 5,000         |
| 3    | 4,000,000       | 5,000           | 5,000,000     | 5,000         |

### New Relic APM (optional)

Enable APM monitoring by setting both variables:

| Variable | Description |
|----------|-------------|
| `NEW_RELIC_APP_NAME` | Base app name (e.g., `beef-briefing`) |
| `NEW_RELIC_LICENSE_KEY` | Your New Relic license key |

When enabled, the processor reports:
- **Background tasks**: `process_batch`, `status`, `continuous`
- **Function traces**: Analyzer execution, database operations, Qdrant storage
- **Custom metrics**: `Custom/MLProcessor/BatchSize`, `Custom/MLProcessor/TotalProcessed`, `Custom/Qdrant/EmbeddingsStored`
- **Custom attributes**: `chat_id`, `batch_size`, `limit`, `from_date`, `to_date`

The app name in New Relic will be: `{NEW_RELIC_APP_NAME}-ml-processor-{ENVIRONMENT}`

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
| **User Cards** | PostgreSQL | `ml_user_cards` |

### User Cards

Weekly aggregated stats per user, generated via `generate-cards` command:

| Stat | Description | Source |
|------|-------------|--------|
| `mood` | Mood score (0-100) with label | `ml_sentiment` |
| `volatility` | Mood consistency (0-1) | `ml_sentiment` stddev |
| `toxicity` | % of toxic messages | `ml_toxicity` |
| `activity` | Messages, active days, avg length | `messages` |
| `reactions_received` | Total reactions on user's messages | `message_reactions` |
| `chronotype` | Peak activity hour and label | `messages` hour distribution |
| `comedy` | Combined humor score (ML 30% + laugh reactions 70%) | `ml_humor` + `message_reactions` |

Cards use a 30-day rolling window for stable personality traits, with week-over-week trend comparisons.

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
DELETE FROM ml_user_cards WHERE chat_id = -1003280306634;
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
