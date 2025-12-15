# ML Processor Architecture

Detailed technical documentation for the ML analytics processor service.

## Overview

The ML processor is a local Python service that analyzes Portuguese Telegram chat messages using GPU-accelerated ML models. It operates as a batch processor, fetching unprocessed messages from the api-service, running inference, and posting results back.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              ML Processor                                    │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                       │
│  │  Sentiment   │  │   Toxicity   │  │  Embedding   │                       │
│  │  Analyzer    │  │   Detector   │  │   Encoder    │                       │
│  │  (DistilBERT)│  │   (BERT-PT)  │  │   (MPNet)    │                       │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘                       │
│         │                 │                 │                                │
│         └─────────────────┼─────────────────┘                                │
│                           │                                                  │
│                    ┌──────▼──────┐                                          │
│                    │  Pipeline   │                                          │
│                    │  Processor  │                                          │
│                    └──────┬──────┘                                          │
└───────────────────────────┼─────────────────────────────────────────────────┘
                            │
              ┌─────────────┼─────────────┐
              │             │             │
              ▼             ▼             ▼
       ┌──────────┐  ┌──────────┐  ┌──────────┐
       │   API    │  │ PostgreSQL│  │  Qdrant  │
       │ Service  │  │(via API)  │  │ (direct) │
       └──────────┘  └──────────┘  └──────────┘
```

## Models

### 1. Sentiment Analysis

**Model**: `lxyuan/distilbert-base-multilingual-cased-sentiments-student`

- **Architecture**: DistilBERT (distilled from multilingual BERT)
- **Languages**: 100+ languages including Portuguese
- **Output**: 3-class classification (positive, neutral, negative)
- **Memory**: ~270MB GPU

**How it works**:
1. Text is tokenized using WordPiece tokenizer
2. Tokens pass through 6 transformer layers
3. [CLS] token representation is classified into 3 sentiment classes
4. Softmax produces probability distribution

**Output format**:
```python
{
    "label": "positive",  # dominant sentiment
    "score_positive": 0.85,
    "score_neutral": 0.10,
    "score_negative": 0.05,
}
```

### 2. Toxicity Detection

**Model**: `ruanchaves/bert-base-portuguese-cased-hatebr`

- **Architecture**: BERT base (cased, Portuguese)
- **Training data**: HateBR corpus (Brazilian Portuguese hate speech)
- **Output**: Binary classification (hateful/non-hateful)
- **Memory**: ~440MB GPU

**How it works**:
1. Portuguese text tokenized with PT-specific vocabulary
2. 12 transformer layers process the sequence
3. Binary classification head outputs hate speech probability
4. Model returns boolean `is_toxic` with confidence score

**Output format**:
```python
{
    "is_toxic": True,
    "label": "hateful",  # or "non-hateful"
    "score": 0.92,       # confidence
}
```

**Note**: This model was trained specifically on Brazilian Portuguese informal text, making it well-suited for Telegram chat analysis.

### 3. Embedding Generation

**Model**: `sentence-transformers/paraphrase-multilingual-mpnet-base-v2`

- **Architecture**: MPNet (Microsoft's improved BERT variant)
- **Dimensions**: 768-dimensional dense vectors
- **Languages**: 50+ languages with strong Portuguese support
- **Memory**: ~420MB GPU

**How it works**:
1. Text encoded through 12 MPNet layers
2. Mean pooling over token embeddings produces sentence embedding
3. L2 normalization for cosine similarity compatibility
4. Output: 768-dim float32 vector

**Use cases**:
- Semantic search (find similar messages)
- Topic clustering (group related discussions)
- Duplicate detection
- User behavior analysis

## Data Flow

### Processing Pipeline

```
1. FETCH
   GET /api/v1/ml/messages?limit=500
   └── Returns messages NOT in ml_processing_state

2. ANALYZE (parallel on GPU)
   ├── Sentiment: batch inference → scores
   ├── Toxicity: batch inference → is_toxic
   └── Embeddings: batch encode → 768-dim vectors

3. STORE EMBEDDINGS
   Qdrant.upsert(message_ids, embeddings, metadata)
   └── Stored with chat_id, user_id for filtering

4. POST RESULTS
   POST /api/v1/ml/results
   └── Inserts into ml_sentiment, ml_toxicity, ml_processing_state

5. REPEAT or SLEEP
   └── If has_more=true, continue; else sleep 60s
```

### Database Schema

```
ml_processing_state          ml_sentiment                 ml_toxicity
┌─────────────────┐          ┌─────────────────┐          ┌─────────────────┐
│ message_id (PK) │          │ message_id (PK) │          │ message_id (PK) │
│ chat_id         │          │ chat_id         │          │ chat_id         │
│ processor_ver   │          │ label           │          │ is_toxic        │
│ processed_at    │          │ score_positive  │          │ label           │
└─────────────────┘          │ score_neutral   │          │ score           │
                             │ score_negative  │          │ created_at      │
                             │ confidence      │          └─────────────────┘
                             │ created_at      │
                             └─────────────────┘
```

### Qdrant Collection

```python
Collection: "message_embeddings"
{
    "vectors": {
        "size": 768,
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

## Performance Characteristics

### Batch Processing

| Batch Size | GPU Memory | Throughput |
|------------|------------|------------|
| 32         | ~2.5GB     | ~200 msg/s |
| 64         | ~3.5GB     | ~350 msg/s |
| 128        | ~5.5GB     | ~500 msg/s |
| 256        | ~8GB       | ~650 msg/s |

*Measured on RTX 4070 (12GB VRAM)*

### Memory Usage

```
Base models loaded:     ~1.7GB GPU
Per-batch overhead:     ~50MB per 100 messages
Qdrant client:          ~50MB RAM
API client:             ~20MB RAM
Python overhead:        ~200MB RAM
─────────────────────────────────────
Total (idle):           ~2GB GPU + 300MB RAM
Total (processing):     ~3-4GB GPU + 500MB RAM
```

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

- If sentiment fails for a message, toxicity still runs
- If Qdrant upsert fails, API results still posted
- Failed messages are logged but don't stop the batch
- Processing state only marked after successful API post

## Configuration

### Environment Variables

| Variable | Purpose | Tuning Notes |
|----------|---------|--------------|
| `BATCH_SIZE` | Messages per API fetch | Higher = more GPU memory, better throughput |
| `SLEEP_SECONDS` | Wait when queue empty | Lower = more responsive, higher API load |
| `DEVICE` | cuda/cpu | CPU is 10-20x slower |

### Model Selection

Models can be swapped via environment variables:

```bash
# Use different sentiment model
SENTIMENT_MODEL=nlptown/bert-base-multilingual-uncased-sentiment

# Use different toxicity model (if you find a better PT model)
TOXICITY_MODEL=your-org/your-model

# Use larger embedding model for better quality
EMBEDDING_MODEL=sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2
```

## Future Improvements

### Short-term

- [ ] **Batch GPU optimization**: Use HuggingFace `Dataset` for proper DataLoader batching
- [ ] **Progress bar**: Add tqdm for long-running continuous mode
- [ ] **Graceful shutdown**: Handle SIGTERM properly, finish current batch
- [ ] **Health endpoint**: Add `/health` for monitoring when running as service
- [ ] **Metrics export**: Prometheus metrics for processing rate, errors, queue depth

### Medium-term

- [ ] **Topic modeling**: Implement HDBSCAN clustering on embeddings
  - Cluster messages by semantic similarity
  - Extract topic keywords using c-TF-IDF
  - Store in `ml_topics` and `ml_message_topics` tables

- [ ] **User profiles**: Aggregate ML results per user
  - Average sentiment over time
  - Toxicity rate calculation
  - Topic preferences
  - Store in `ml_user_profiles` table

- [ ] **Incremental processing**: Process new messages as they arrive
  - WebSocket connection to api-service
  - Near real-time analysis (<1s latency)

### Long-term

- [ ] **Named Entity Recognition (NER)**
  - Extract people, places, organizations
  - Model: `neuralmind/bert-base-portuguese-cased`
  - New table: `ml_entities`

- [ ] **Emotion detection**: Beyond sentiment
  - Detect: joy, anger, fear, sadness, surprise, disgust
  - Model: Fine-tuned multilingual emotion classifier
  - New table: `ml_emotions`

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

## References

- [HuggingFace Transformers](https://huggingface.co/docs/transformers)
- [Sentence Transformers](https://www.sbert.net/)
- [Qdrant Documentation](https://qdrant.tech/documentation/)
- [HateBR Dataset Paper](https://aclanthology.org/2022.lrec-1.777/)
- [DistilBERT Paper](https://arxiv.org/abs/1910.01108)
- [MPNet Paper](https://arxiv.org/abs/2004.09297)
