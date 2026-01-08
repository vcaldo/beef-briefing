# ML Processor

ML analytics pipeline for analyzing Telegram messages and generating weekly user stats cards.

## Overview

The ML Processor fetches unprocessed messages from the API Service, runs various ML analyses (sentiment, humor, toxicity, etc.), and stores results back in the database. It also generates weekly aggregated stats cards for each user with scores for Aura, Activity, Presence, Humor, Toxicity, and Popularity.

## Features

- **Multiple Analysis Types**: Sentiment, toxicity, humor, questions, NER, embeddings, topics
- **Flexible Providers**: Local models, OpenAI, Anthropic, or Perspective API
- **Rate Limiting**: Built-in OpenAI rate limiting with token bucket algorithm
- **GPU Support**: Optimized for CUDA/GPU processing
- **Weekly Cards**: Aggregated user stats with Bayesian smoothing

## Quick Start

```bash
# Start Docker services
make up-build

# Run batch processing (development)
make ml-run

# Check status
make ml-run-status

# Generate weekly cards
make ml-run-cards ML_ARGS="--timezone America/Sao_Paulo"

# Run with specific week
make ml-run-cards ML_ARGS="--timezone America/Sao_Paulo --week 2025-01-06"
```

## Configuration

### Core Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `API_SERVICE_URL` | `http://localhost:8080` | API Service URL |
| `API_KEY_FILE` | `../../infrastructure/secrets/apps/ml-processor/api_key` | API key path |
| `QDRANT_HOST` | `localhost` | Qdrant vector DB host |
| `QDRANT_PORT` | `6333` | Qdrant port |
| `BATCH_SIZE` | `500` | Messages per batch (use ≤100 for OpenAI) |
| `SLEEP_SECONDS` | `60` | Sleep when no messages |
| `DEVICE` | `cuda` | PyTorch device |

### Provider Selection

Each analysis type can use a different provider:

| Variable | Default | Options |
|----------|---------|---------|
| `SENTIMENT_PROVIDER` | `local` | `local`, `openai`, `anthropic` |
| `TOXICITY_PROVIDER` | `local` | `local`, `perspective`, `openai` |
| `EMBEDDINGS_PROVIDER` | `local` | `local`, `openai` |
| `TOPICS_PROVIDER` | `local` | `local`, `openai` |
| `NER_PROVIDER` | `local` | `local`, `openai` |
| `HUMOR_PROVIDER` | `local` | `local`, `openai` |
| `QUESTIONS_PROVIDER` | `local` | `local`, `openai` |

### API Keys (Required for Non-Local Providers)

| Variable | Provider |
|----------|----------|
| `OPENAI_API_KEY` | OpenAI |
| `ANTHROPIC_API_KEY` | Anthropic |
| `PERSPECTIVE_API_KEY` | Perspective |

### OpenAI Rate Limiting

| Variable | Default | Description |
|----------|---------|-------------|
| `OPENAI_RATE_LIMIT_ENABLED` | `true` | Enable rate limiting |
| `OPENAI_RATE_LIMIT_TIMEOUT` | `120.0` | Max wait time (seconds) |
| `OPENAI_GPT4O_MINI_TPM` | `200000` | Tokens per minute |
| `OPENAI_GPT4O_MINI_RPM` | `500` | Requests per minute |
| `OPENAI_EMBEDDING_TPM` | `1000000` | Embedding tokens/min |
| `OPENAI_EMBEDDING_RPM` | `3000` | Embedding requests/min |

## Commands

### Make Targets

| Command | Description |
|---------|-------------|
| `make ml-run` | Run batch processing |
| `make ml-run-once` | Process single batch |
| `make ml-run-status` | Check processing status |
| `make ml-run-continuous` | Run daemon mode |
| `make ml-run-cards` | Generate weekly cards |
| `make ml-shell` | Open shell in container |

### Command Options

```bash
# Process options
--chat-id ID        # Target chat ID
--limit N           # Max messages to process
--batch-size N      # Messages per batch
--from-date D       # Process from date (YYYY-MM-DD)
--to-date D         # Process until date (YYYY-MM-DD)

# Cards options
--timezone TZ       # IANA timezone (REQUIRED)
--week D            # Week start date (YYYY-MM-DD, Monday)
--window-days N     # Rolling window (default: 30)
--min-messages N    # Minimum messages (default: 10)
```

### Examples

```bash
# Process 100 messages
make ml-run ML_ARGS="--limit 100"

# Process date range
make ml-run ML_ARGS="--from-date 2025-01-01 --to-date 2025-01-07"

# Generate cards for specific week
make ml-run-cards ML_ARGS="--timezone America/Sao_Paulo --week 2025-01-06"
```

## Analysis Types

| Analysis | Description | Local Model |
|----------|-------------|-------------|
| Sentiment | Positive/Neutral/Negative | `distilbert-base-multilingual-cased-sentiments-student` |
| Toxicity | Hate speech detection | `bert-base-portuguese-cased-hatebr` |
| Embeddings | Vector representations | `paraphrase-multilingual-mpnet-base-v2` |
| NER | Named entities | `pt_core_news_lg` (spaCy) |
| Humor | Joke/sarcasm detection | Custom patterns |
| Questions | Question classification | `bart-large-mnli` (zero-shot) |
| Topics | Topic clustering | HDBSCAN |

**GPU Memory**: ~3.3GB with all local models loaded.

## User Cards

Weekly aggregated stats per user:

| Stat | Description | Source |
|------|-------------|--------|
| Aura | Emotional tone + reception (0-100) | Sentiment + emoji reactions |
| Activity | Engagement volume (0-100) | Messages + reactions + replies |
| Presence | Consistency over time (0-100) | Active days, streak, hours |
| Humor | Comedy impact (0-100) | Humor detection + reactions |
| Toxicity | Negative impact (0-100%) | Toxicity classifier |
| Popularity | Social gravity (0-100) | Unique reactors/repliers |

All metrics use **progressive Bayesian smoothing** to stabilize scores for low-volume users.

See [CARD_GENERATION.md](CARD_GENERATION.md) for detailed formulas.

## Storage

| Data | Storage | Table/Collection |
|------|---------|------------------|
| Sentiment | PostgreSQL | `ml_sentiment` |
| Toxicity | PostgreSQL | `ml_toxicity` |
| Humor | PostgreSQL | `ml_humor` |
| NER | PostgreSQL | `ml_ner` |
| Questions | PostgreSQL | `ml_questions` |
| Topics | PostgreSQL | `ml_topics`, `ml_message_topics` |
| Embeddings | Qdrant | `message_embeddings` |
| User Cards | PostgreSQL | `ml_user_cards` |

## Development

### Reset Processing State

```bash
# Reset dev environment (PostgreSQL + Qdrant)
make ml-clean-dev

# Reset specific chat
make ml-clean-cards-dev CHAT_ID=-1003280306634

# Reset specific week
make ml-clean-cards-dev CHAT_ID=-1003280306634 WEEK=2025-01-06
```

### Manual Reset

```sql
-- Reset all ML data
TRUNCATE ml_user_cards, ml_message_topics, ml_topics,
         ml_ner, ml_humor, ml_questions, ml_toxicity,
         ml_sentiment, ml_processing_state CASCADE;
```

### Reset Qdrant

```bash
# Delete collection
curl -X DELETE "http://localhost:6333/collections/message_embeddings"
```

## Troubleshooting

### No messages being processed

1. Check API Service is running
2. Verify API key: `cat infrastructure/secrets/apps/ml-processor/api_key`
3. Check there are unprocessed messages: `make ml-run-status`

### OpenAI rate limit errors

- Reduce `BATCH_SIZE` to ≤100 for OpenAI providers
- Increase `OPENAI_RATE_LIMIT_TIMEOUT`
- Check your OpenAI tier limits

### GPU out of memory

- Reduce `BATCH_SIZE`
- Switch some analyzers to OpenAI provider
- Use CPU: `DEVICE=cpu`
