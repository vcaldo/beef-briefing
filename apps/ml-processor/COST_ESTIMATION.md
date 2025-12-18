# OpenAI Token Cost Estimation - ML Pipeline

**Generated**: 2025-12-18
**Chat ID**: -1002572302334
**Period**: Last 30 days (2025-11-18 to 2025-12-18)

---

## Message Statistics

| Metric | Value |
|--------|-------|
| Total Messages | 23,029 |
| Total Characters | 709,063 |
| Avg Chars/Message | 30.8 |
| Max Chars | 3,914 |

## Token Estimation

Using ~3.5 characters per token (conservative for Portuguese):

- **Total text tokens**: ~202,590 tokens
- **Batch count**: ~231 batches (BATCH_SIZE=100)

---

## Cost Breakdown

### Chat-Based Analyzers (gpt-4o-mini)

**Pricing**: Input $0.15/1M tokens, Output $0.60/1M tokens

| Analyzer | Input Tokens | Output Tokens | Cost |
|----------|--------------|---------------|------|
| Sentiment | 323,400 | 924,000 | $0.60 |
| Humor | 323,400 | 808,500 | $0.53 |
| Questions | 323,400 | 808,500 | $0.53 |
| NER | 323,400 | 1,155,000 | $0.74 |
| **Subtotal** | **1,293,600** | **3,696,000** | **$2.41** |

### Embeddings (text-embedding-3-small)

**Pricing**: $0.02/1M tokens

| Metric | Value |
|--------|-------|
| Input Tokens | 202,590 |
| **Cost** | **$0.01** |

### Toxicity (OpenAI Moderation API)

**Cost**: FREE (included with API key)

---

## Total Estimated Cost

| Component | Cost |
|-----------|------|
| Chat-based (4 analyzers) | $2.41 |
| Embeddings | $0.01 |
| Toxicity (Moderation) | $0.00 |
| **TOTAL** | **~$2.50** |

### Conservative Estimate

Adding 30% buffer for response variations and retries:

**Conservative total: ~$3.25**

---

## Comparison: OpenAI vs Local

| Provider | Cost | Processing Time | Requirements |
|----------|------|-----------------|--------------|
| OpenAI | ~$2.50-3.25 | ~2-3 hours | API key |
| Local | $0 | ~30-60 min | GPU (~3.3GB VRAM) |

---

## Notes

1. **Batch size**: BATCH_SIZE=100 recommended for OpenAI (avoid token limits)
2. **Message truncation**: All analyzers truncate to 500 chars
3. **Model**: gpt-4o-mini used for all chat-based analyzers
4. **Moderation API**: Free and handles toxicity detection
