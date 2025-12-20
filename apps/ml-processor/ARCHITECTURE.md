# ML Processor Architecture

Detailed technical documentation for the ML analytics processor service.

## Overview

The ML processor is a local Python service that analyzes Portuguese Telegram chat messages using GPU-accelerated ML models. It operates as a batch processor, fetching unprocessed messages from the api-service, running inference, and posting results back.

The processor includes **7 analyzers** with pluggable providers (local GPU models, OpenAI, Anthropic, Perspective API).

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│                                ML Processor                                       │
│                                                                                   │
│  ┌────────────┐ ┌────────────┐ ┌────────────┐ ┌────────────┐                     │
│  │ Sentiment  │ │  Toxicity  │ │ Embeddings │ │   Topics   │                     │
│  │ (DistilBERT│ │ (BERT-PT/  │ │  (MPNet/   │ │ (HDBSCAN)  │                     │
│  │ /OpenAI)   │ │ OpenAI)    │ │ OpenAI)    │ │            │                     │
│  └─────┬──────┘ └─────┬──────┘ └─────┬──────┘ └─────┬──────┘                     │
│        │              │              │              │                             │
│  ┌─────┴──────┐ ┌─────┴──────┐ ┌─────┴──────┐                                    │
│  │    NER     │ │   Humor    │ │ Questions  │                                    │
│  │ (spaCy/    │ │ (Heuristic/│ │ (Zero-shot/│                                    │
│  │ OpenAI)    │ │ OpenAI)    │ │ OpenAI)    │                                    │
│  └─────┬──────┘ └─────┬──────┘ └─────┬──────┘                                    │
│        │              │              │                                            │
│        └──────────────┼──────────────┘                                            │
│                       │                                                           │
│                ┌──────▼──────┐      ┌──────────────┐                             │
│                │  Pipeline   │──────│    Card      │                             │
│                │  Processor  │      │  Generator   │                             │
│                └──────┬──────┘      └──────┬───────┘                             │
└───────────────────────┼────────────────────┼─────────────────────────────────────┘
                        │                    │
          ┌─────────────┼─────────────┐      │
          │             │             │      │
          ▼             ▼             ▼      ▼
   ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────────┐
   │   API    │  │PostgreSQL│  │  Qdrant  │  │ Card Image     │
   │ Service  │  │(via API) │  │ (direct) │  │ Generator      │
   └──────────┘  └──────────┘  └──────────┘  └────────────────┘
```

---

## Analyzers

### 1. Sentiment Analysis

Classifies messages into positive, neutral, or negative sentiment.

**Providers:**

| Provider | Model | Memory |
|----------|-------|--------|
| `local` | `lxyuan/distilbert-base-multilingual-cased-sentiments-student` | ~270MB GPU |
| `openai` | `gpt-4o-mini` | N/A |
| `anthropic` | `claude-3-haiku-20240307` | N/A |

**Local Model Details:**
- **Architecture**: DistilBERT (distilled from multilingual BERT)
- **Languages**: 100+ languages including Portuguese
- **Output**: 3-class classification with probability distribution

**How it works (local)**:
1. Text tokenized using WordPiece tokenizer
2. Tokens pass through 6 transformer layers
3. [CLS] token representation classified into 3 sentiment classes
4. Softmax produces probability distribution

**Output format**:
```python
{
    "label": "positive",       # dominant sentiment
    "score_positive": 0.85,
    "score_neutral": 0.10,
    "score_negative": 0.05,
    "confidence": 0.85
}
```

**Storage**: `ml_sentiment` table

---

### 2. Toxicity Detection

Detects hate speech and toxic content in Portuguese messages.

**Providers:**

| Provider | Model/API | Memory |
|----------|-----------|--------|
| `local` | `ruanchaves/bert-base-portuguese-cased-hatebr` | ~440MB GPU |
| `perspective` | Google Perspective API | N/A |
| `openai` | `omni-moderation-latest` | N/A |

**Local Model Details:**
- **Architecture**: BERT base (cased, Portuguese)
- **Training data**: HateBR corpus (Brazilian Portuguese hate speech)
- **Output**: Binary classification (hateful/non-hateful)

**How it works (local)**:
1. Portuguese text tokenized with PT-specific vocabulary
2. 12 transformer layers process the sequence
3. Binary classification head outputs hate speech probability

**Output format**:
```python
{
    "is_toxic": True,
    "label": "hateful",  # or "non-hateful"
    "score": 0.92        # confidence
}
```

**Note**: The HateBR model was trained specifically on Brazilian Portuguese informal text, making it well-suited for Telegram chat analysis.

**Storage**: `ml_toxicity` table

---

### 3. Embedding Generation

Generates dense vector representations for semantic similarity and clustering.

**Providers:**

| Provider | Model | Dimensions | Memory |
|----------|-------|------------|--------|
| `local` | `sentence-transformers/paraphrase-multilingual-mpnet-base-v2` | 768 | ~420MB GPU |
| `openai` | `text-embedding-3-small` | 1536 | N/A |
| `openai` | `text-embedding-3-large` | 3072 | N/A |

**Local Model Details:**
- **Architecture**: MPNet (Microsoft's improved BERT variant)
- **Languages**: 50+ languages with strong Portuguese support

**How it works (local)**:
1. Text encoded through 12 MPNet layers
2. Mean pooling over token embeddings produces sentence embedding
3. L2 normalization for cosine similarity compatibility

**Use cases**:
- Semantic search (find similar messages)
- Topic clustering (group related discussions)
- Duplicate detection
- User behavior analysis

**Storage**: Qdrant vector database (`message_embeddings` collection)

---

### 4. Topic Clustering

Clusters messages by semantic similarity using HDBSCAN on embeddings.

**Providers:**

| Provider | Method |
|----------|--------|
| `local` | HDBSCAN on local embeddings + TF-IDF keywords |
| `openai` | HDBSCAN on OpenAI embeddings + TF-IDF keywords |

**Clustering Parameters:**
```python
min_cluster_size = 5
min_samples = 3
cluster_selection_epsilon = 0.0
metric = "euclidean"
```

**How it works**:
1. Fetch embeddings for messages in batch
2. Run HDBSCAN clustering algorithm
3. Extract keywords from each cluster using TF-IDF
4. Assign outliers to topic_id = -1

**Output format**:
```python
{
    "topic_id": 3,           # cluster ID (-1 for outliers)
    "similarity": 0.85,      # distance to cluster centroid
    "is_outlier": False
}
```

**Storage**: `ml_topics` (cluster metadata) + `ml_message_topics` (message assignments)

---

### 5. Named Entity Recognition (NER)

Extracts people, places, organizations, and other entities from messages.

**Providers:**

| Provider | Model | Languages |
|----------|-------|-----------|
| `local` | spaCy `pt_core_news_lg` | Portuguese |
| `openai` | `gpt-4o-mini` (structured output) | Multi |

**Local Model Details:**
- **Memory**: ~550MB
- **Entity types**: PER, ORG, LOC, MISC, DATE, etc.

**How it works (local)**:
1. Text processed through spaCy pipeline
2. NER component identifies entity spans
3. Each entity returned with type, text, and position

**Output format**:
```python
[
    {
        "entity_type": "PER",
        "entity_text": "João Silva",
        "start_pos": 15,
        "end_pos": 25,
        "confidence": 0.95
    }
]
```

**Storage**: `ml_ner` table

---

### 6. Humor Detection

Detects humorous content using Brazilian Portuguese patterns and ML.

**Providers:**

| Provider | Method |
|----------|--------|
| `local` | Heuristic-based (laugh patterns + emojis) |
| `openai` | `gpt-4o-mini` |

**Heuristic Signals (local):**
```python
# Laugh patterns (weight: 0.4)
patterns = [r"k{3,}", r"(ha){2,}", r"(rs){2,}", r"(hua){2,}", r"(he){3,}", r"(hi){3,}"]

# Laugh emojis (weight: 0.3)
emojis = ["😂", "🤣", "😆", "😹", "🤭", "😁"]

# Multiple emojis bonus (weight: 0.2)
# Threshold: 0.4 for is_humorous = True
```

**Output format**:
```python
{
    "is_humorous": True,
    "humor_type": "laugh_pattern",  # or "emoji", etc.
    "score": 0.75
}
```

**Storage**: `ml_humor` table

---

### 7. Question Detection

Classifies messages as questions and identifies question types.

**Providers:**

| Provider | Method |
|----------|--------|
| `local` | Heuristics + zero-shot (`facebook/bart-large-mnli`) |
| `openai` | `gpt-4o-mini` |

**Detection Logic (local):**
1. **Heuristic 1**: Ends with `?` → score 0.95 (no model call)
2. **Heuristic 2**: Starts with question words → score 0.85
3. **Heuristic 3**: Zero-shot classification if no patterns detected

**Question Words:**
- `quem`, `qual`, `quando`, `onde`, `quanto`, `como`, `por que`, `o que`

**Question Types:**
- `yes_no`: "é", "está", "foi", "tem", "pode", "vai"
- `factual`: "quem", "qual", "quando", "onde", "quanto"
- `opinion`: "acha", "pensa", "prefere"
- `rhetorical`: "né", "hein"
- `clarification`: "como assim", "não entendi"

**Output format**:
```python
{
    "is_question": True,
    "question_type": "factual",
    "score": 0.92
}
```

**Storage**: `ml_questions` table

---

## Data Flow

### Processing Pipeline

```
1. FETCH
   GET /api/v1/ml/messages?limit=500
   └── Returns messages NOT in ml_processing_state

2. ANALYZE (parallel)
   ├── Sentiment  → ml_sentiment
   ├── Toxicity   → ml_toxicity
   ├── Embeddings → Qdrant
   ├── NER        → ml_ner
   ├── Humor      → ml_humor
   └── Questions  → ml_questions

3. CLUSTER (after embeddings)
   Topics → ml_topics + ml_message_topics

4. MARK PROCESSED
   └── Update ml_processing_state

5. REPEAT or SLEEP
   └── If has_more=true, continue; else sleep 60s
```

### Card Generation Pipeline

```
1. GET WEEK BOUNDS
   └── Monday 00:00 → Sunday 23:59:59 (in timezone)

2. GET STATS WINDOW
   └── 30-day rolling window ending on week Sunday

3. FOR EACH ACTIVE USER
   ├── Check minimum message threshold
   ├── Compute 7 stats (see CARD_GENERATION.md)
   ├── Compare to previous week for trends
   └── Upsert to ml_user_cards

4. RENDER IMAGES (optional)
   └── POST to card-image-generator service
```

See [CARD_GENERATION.md](CARD_GENERATION.md) for detailed stat calculation formulas.

---

## Database Schema

```
┌───────────────────┐  ┌───────────────────┐  ┌───────────────────┐
│ml_processing_state│  │   ml_sentiment    │  │    ml_toxicity    │
├───────────────────┤  ├───────────────────┤  ├───────────────────┤
│ message_id (PK)   │  │ message_id (PK)   │  │ message_id (PK)   │
│ chat_id           │  │ chat_id           │  │ chat_id           │
│ processor_ver     │  │ label             │  │ is_toxic          │
│ processed_at      │  │ score_positive    │  │ label             │
└───────────────────┘  │ score_neutral     │  │ score             │
                       │ score_negative    │  │ created_at        │
                       │ confidence        │  └───────────────────┘
                       │ created_at        │
                       └───────────────────┘

┌───────────────────┐  ┌───────────────────┐  ┌───────────────────┐
│      ml_ner       │  │     ml_humor      │  │   ml_questions    │
├───────────────────┤  ├───────────────────┤  ├───────────────────┤
│ id (PK)           │  │ message_id (PK)   │  │ message_id (PK)   │
│ message_id        │  │ chat_id           │  │ chat_id           │
│ chat_id           │  │ is_humorous       │  │ is_question       │
│ entity_type       │  │ humor_type        │  │ question_type     │
│ entity_text       │  │ score             │  │ score             │
│ start_pos         │  │ created_at        │  │ created_at        │
│ end_pos           │  └───────────────────┘  └───────────────────┘
│ confidence        │
│ created_at        │
└───────────────────┘

┌───────────────────┐  ┌───────────────────┐
│    ml_topics      │  │ ml_message_topics │
├───────────────────┤  ├───────────────────┤
│ id (PK)           │  │ id (PK)           │
│ chat_id           │  │ message_id        │
│ topic_id          │  │ chat_id           │
│ keywords          │  │ topic_id          │
│ message_count     │  │ similarity        │
│ created_at        │  │ is_outlier        │
└───────────────────┘  │ created_at        │
                       └───────────────────┘

┌───────────────────────────┐  ┌───────────────────────────┐
│      ml_user_cards        │  │    ml_user_card_images    │
├───────────────────────────┤  ├───────────────────────────┤
│ id (PK)                   │  │ id (PK)                   │
│ user_id                   │  │ card_id (FK)              │
│ chat_id                   │  │ user_id                   │
│ week_start                │  │ chat_id                   │
│ week_end                  │  │ week_start                │
│ stats_window_start        │  │ storage_path              │
│ stats_window_end          │  │ file_hash                 │
│ stats (JSONB)             │  │ file_size                 │
│ trends (JSONB)            │  │ width, height             │
│ messages_analyzed         │  │ theme                     │
│ timezone                  │  │ template_version          │
│ card_version              │  │ card_data_version         │
│ generated_at              │  │ generated_at              │
└───────────────────────────┘  └───────────────────────────┘
```

### Qdrant Collection

```python
Collection: "message_embeddings"
{
    "vectors": {
        "size": 768,        # or 1536 for OpenAI
        "distance": "Cosine"
    },
    "payload": {
        "message_id": int,      # DB primary key
        "chat_id": int,         # For filtering by chat
        "user_id": int,         # For filtering by user
        "text_preview": str     # First 100 chars
    }
}
```

---

## Performance Characteristics

### Batch Processing (Local GPU)

| Batch Size | GPU Memory | Throughput |
|------------|------------|------------|
| 32         | ~2.5GB     | ~200 msg/s |
| 64         | ~3.5GB     | ~350 msg/s |
| 128        | ~5.5GB     | ~500 msg/s |
| 256        | ~8GB       | ~650 msg/s |

*Measured on RTX 4070 (12GB VRAM)*

### Memory Usage (All Local Models)

```
Models loaded:          ~3.3GB GPU
├── Sentiment (DistilBERT)    ~270MB
├── Toxicity (BERT-PT)        ~440MB
├── Embeddings (MPNet)        ~420MB
├── NER (spaCy)               ~550MB
├── Questions (BART-MNLI)     ~1.6GB
└── Humor (heuristic)         ~0MB

Per-batch overhead:     ~50MB per 100 messages
Qdrant client:          ~50MB RAM
API client:             ~20MB RAM
Python overhead:        ~200MB RAM
─────────────────────────────────────────────
Total (idle):           ~3.5GB GPU + 300MB RAM
Total (processing):     ~4-5GB GPU + 500MB RAM
```

---

## Error Handling

### Retry Logic

```python
# API calls use exponential backoff
max_retries = 3
base_delay = 1.0  # seconds
max_delay = 30.0

for attempt in range(max_retries):
    try:
        response = api_call()
        break
    except (ConnectionError, Timeout):
        delay = min(base_delay * (2 ** attempt), max_delay)
        sleep(delay)
```

### Partial Failures

- If one analyzer fails, others still run
- If Qdrant upsert fails, PostgreSQL results still posted
- Failed messages are logged but don't stop the batch
- Processing state only marked after successful storage

---

## Configuration

### Provider Selection

Each analyzer can use a different provider:

| Variable | Default | Options |
|----------|---------|---------|
| `SENTIMENT_PROVIDER` | `local` | `local`, `openai`, `anthropic` |
| `TOXICITY_PROVIDER` | `local` | `local`, `perspective`, `openai` |
| `EMBEDDINGS_PROVIDER` | `local` | `local`, `openai` |
| `TOPICS_PROVIDER` | `local` | `local`, `openai` |
| `NER_PROVIDER` | `local` | `local`, `openai` |
| `HUMOR_PROVIDER` | `local` | `local`, `openai` |
| `QUESTIONS_PROVIDER` | `local` | `local`, `openai` |

### Rate Limiting (OpenAI)

When using OpenAI providers, built-in rate limiting prevents exceeding tier limits:

```python
# Token bucket algorithm per model
gpt-4o-mini: sentiment, humor, questions, NER (shared limits)
text-embedding-3-small: embeddings, topics (shared limits)
omni-moderation-latest: toxicity
```

See [README.md](README.md) for detailed rate limiting configuration.

### Environment Variables

| Variable | Purpose | Tuning Notes |
|----------|---------|--------------|
| `BATCH_SIZE` | Messages per API fetch | Higher = more GPU memory, better throughput |
| `SLEEP_SECONDS` | Wait when queue empty | Lower = more responsive, higher API load |
| `DEVICE` | cuda/cpu | CPU is 10-20x slower |

---

## Future Improvements

### Short-term

- [ ] **Batch GPU optimization**: Use HuggingFace `Dataset` for proper DataLoader batching
- [ ] **Progress bar**: Add tqdm for long-running continuous mode
- [ ] **Graceful shutdown**: Handle SIGTERM properly, finish current batch
- [ ] **Health endpoint**: Add `/health` for monitoring when running as service
- [ ] **Metrics export**: Prometheus metrics for processing rate, errors, queue depth

### Medium-term

- [ ] **Incremental processing**: Process new messages as they arrive
  - WebSocket connection to api-service
  - Near real-time analysis (<1s latency)

- [ ] **Emotion detection**: Beyond sentiment
  - Detect: joy, anger, fear, sadness, surprise, disgust
  - Model: Fine-tuned multilingual emotion classifier
  - New table: `ml_emotions`

### Long-term

- [ ] **Conversation threading**: Link related messages
  - Use embedding similarity + temporal proximity
  - Build conversation graphs
  - New table: `ml_threads`

- [ ] **Anomaly detection**: Unusual behavior patterns
  - Sudden sentiment shifts
  - Topic deviation from norm
  - Activity pattern changes

- [ ] **Multi-GPU support**: For larger deployments
  - Model parallelism across GPUs
  - Pipeline parallelism for throughput

### Model Improvements

- [ ] **Fine-tune on domain data**: Train on actual chat data for better accuracy
- [ ] **Ensemble models**: Combine multiple toxicity detectors
- [ ] **Confidence calibration**: Better uncertainty estimates
- [ ] **Portuguese-specific sentiment**: Train on PT social media data

---

## Troubleshooting

### Common Issues

**Out of GPU memory**
```bash
# Reduce batch size
BATCH_SIZE=100 python main.py
# Or use CPU (slower)
DEVICE=cpu python main.py
```

**Slow processing**
```bash
# Check GPU utilization
nvidia-smi -l 1
# Should see ~80-95% GPU utilization during inference
```

**API connection errors**
```bash
# Verify API is running
curl http://localhost:8080/health
# Check API key
cat ../../infrastructure/secrets/apps/ml-processor/api_key
```

**Qdrant connection errors**
```bash
# Verify Qdrant is running
curl http://localhost:6333/collections
# Check Docker
docker ps | grep qdrant
```

---

## References

- [HuggingFace Transformers](https://huggingface.co/docs/transformers)
- [Sentence Transformers](https://www.sbert.net/)
- [Qdrant Documentation](https://qdrant.tech/documentation/)
- [HDBSCAN Documentation](https://hdbscan.readthedocs.io/)
- [spaCy Portuguese Models](https://spacy.io/models/pt)
- [HateBR Dataset Paper](https://aclanthology.org/2022.lrec-1.777/)
- [DistilBERT Paper](https://arxiv.org/abs/1910.01108)
- [MPNet Paper](https://arxiv.org/abs/2004.09297)
