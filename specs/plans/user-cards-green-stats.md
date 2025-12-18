# User Cards: Green Stats Implementation Plan

**Tier:** 🟢 Easy (No New Models Needed)
**Estimated Complexity:** Low
**Dependencies:** Existing ML infrastructure only

---

## Overview

These stats leverage existing ML results and message data. No new models required—just SQL aggregations and simple text analysis.

---

## Stats to Implement

### 1. Mood Score
**Source:** `ml_sentiment` table
**Formula:** Weighted average of sentiment scores
**Output:** Scale 1-100 or label ("Cheerful", "Neutral", "Stormy")

```sql
SELECT
    user_id,
    ROUND(AVG(
        score_positive * 100 +
        score_neutral * 50 +
        score_negative * 0
    ), 1) as mood_score
FROM ml_sentiment ms
JOIN messages m ON ms.message_id = m.id
WHERE ms.chat_id = $1
GROUP BY user_id;
```

**Labels:**
- 80-100: "Radiant" ☀️
- 60-79: "Cheerful" 😊
- 40-59: "Neutral" 😐
- 20-39: "Cloudy" 🌧️
- 0-19: "Stormy" ⛈️

---

### 2. Volatility (Mood Swings)
**Source:** `ml_user_profiles.sentiment_variance`
**Formula:** Standard deviation of sentiment scores
**Output:** Scale 1-100 or label

```sql
SELECT
    user_id,
    sentiment_variance,
    CASE
        WHEN sentiment_variance < 0.1 THEN 'Steady'
        WHEN sentiment_variance < 0.2 THEN 'Balanced'
        WHEN sentiment_variance < 0.3 THEN 'Dynamic'
        ELSE 'Chaotic'
    END as volatility_label
FROM ml_user_profiles
WHERE chat_id = $1;
```

---

### 3. Toxicity Rate (Spice Level)
**Source:** `ml_toxicity` table
**Formula:** `toxic_messages / total_messages * 100`
**Output:** Percentage or label

```sql
SELECT
    m.from_user_id as user_id,
    COUNT(*) FILTER (WHERE mt.is_toxic) as toxic_count,
    COUNT(*) as total_count,
    ROUND(100.0 * COUNT(*) FILTER (WHERE mt.is_toxic) / COUNT(*), 2) as toxicity_pct
FROM ml_toxicity mt
JOIN messages m ON mt.message_id = m.id
WHERE mt.chat_id = $1
GROUP BY m.from_user_id;
```

**Labels:**
- 0-2%: "Wholesome" 🌸
- 2-5%: "Mild" 🌶️
- 5-10%: "Spicy" 🔥
- 10%+: "Volcanic" 🌋

---

### 4. Vocabulary Richness
**Source:** `messages.text` (requires text processing)
**Formula:** `unique_words / total_words` (Type-Token Ratio)
**Output:** Scale 0-1 or normalized score

**Implementation Options:**

**Option A: Python in ML Processor**
```python
from collections import Counter
import re

def vocabulary_richness(texts: list[str]) -> float:
    all_words = []
    for text in texts:
        words = re.findall(r'\b\w+\b', text.lower())
        all_words.extend(words)

    if not all_words:
        return 0.0

    unique = len(set(all_words))
    total = len(all_words)

    # Normalized TTR (accounts for text length)
    # Using root TTR: unique / sqrt(total)
    return unique / (total ** 0.5)
```

**Option B: PostgreSQL Function**
```sql
CREATE OR REPLACE FUNCTION calculate_vocabulary_richness(user_id_param BIGINT, chat_id_param BIGINT)
RETURNS NUMERIC AS $$
DECLARE
    all_text TEXT;
    words TEXT[];
    unique_count INT;
    total_count INT;
BEGIN
    SELECT string_agg(COALESCE(text, caption, ''), ' ')
    INTO all_text
    FROM messages
    WHERE from_user_id = user_id_param AND chat_id = chat_id_param;

    words := regexp_split_to_array(lower(all_text), '\s+');
    total_count := array_length(words, 1);

    SELECT COUNT(DISTINCT word) INTO unique_count
    FROM unnest(words) AS word
    WHERE word ~ '^\w+$';

    IF total_count = 0 THEN RETURN 0; END IF;

    RETURN unique_count / sqrt(total_count::numeric);
END;
$$ LANGUAGE plpgsql;
```

**Labels:**
- 15+: "Eloquent" 📚
- 10-15: "Articulate" 📖
- 5-10: "Casual" 💬
- <5: "Minimalist" 🔤

---

### 5. Wordiness (Message Length)
**Source:** `messages.text`
**Formula:** Average character/word count per message
**Output:** Number or label

```sql
SELECT
    from_user_id as user_id,
    ROUND(AVG(LENGTH(COALESCE(text, caption, ''))), 1) as avg_chars,
    ROUND(AVG(array_length(regexp_split_to_array(COALESCE(text, caption, ''), '\s+'), 1)), 1) as avg_words
FROM messages
WHERE chat_id = $1 AND (text IS NOT NULL OR caption IS NOT NULL)
GROUP BY from_user_id;
```

**Labels:**
- 100+ chars: "Novelist" 📝
- 50-100: "Conversationalist" 💭
- 20-50: "Concise" ✍️
- <20: "Telegraphic" ⚡

---

### 6. Emoji Power
**Source:** `messages.text`
**Formula:** Emoji count / message count
**Output:** Rate or label

```sql
-- PostgreSQL regex for emoji detection
SELECT
    from_user_id as user_id,
    COUNT(*) as message_count,
    SUM(
        array_length(
            regexp_matches(
                COALESCE(text, caption, ''),
                '[\U0001F300-\U0001F9FF]', 'g'
            ), 1
        )
    ) as emoji_count,
    ROUND(
        SUM(COALESCE(array_length(regexp_matches(COALESCE(text, caption, ''), '[\U0001F300-\U0001F9FF]', 'g'), 1), 0))::numeric
        / COUNT(*)::numeric,
        2
    ) as emoji_rate
FROM messages
WHERE chat_id = $1
GROUP BY from_user_id;
```

**Labels:**
- 2+/msg: "Emoji Wizard" 🧙
- 1-2/msg: "Expressive" 😄
- 0.5-1/msg: "Balanced" 🙂
- <0.5/msg: "Text Purist" 📄

---

### 7. Activity Score
**Source:** `messages` table
**Formula:** Composite of message count + active days
**Output:** Normalized score

```sql
SELECT
    from_user_id as user_id,
    COUNT(*) as message_count,
    COUNT(DISTINCT DATE(created_at)) as active_days,
    -- Normalize against group max
    ROUND(100.0 * COUNT(*) / MAX(COUNT(*)) OVER(), 1) as activity_percentile
FROM messages
WHERE chat_id = $1
GROUP BY from_user_id;
```

---

### 8. Reaction Magnet
**Source:** `message_reactions` or `reaction_counts`
**Formula:** Total reactions received
**Output:** Count or normalized score

```sql
SELECT
    m.from_user_id as user_id,
    COUNT(mr.id) as reactions_received,
    COUNT(DISTINCT mr.user_id) as unique_reactors
FROM messages m
LEFT JOIN message_reactions mr ON m.id = mr.message_id
WHERE m.chat_id = $1
GROUP BY m.from_user_id;
```

---

## Database Schema

### New Table: `ml_user_card_stats`

```sql
CREATE TABLE ml_user_card_stats (
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,

    -- Green stats
    mood_score NUMERIC(5,2),
    mood_label VARCHAR(20),
    volatility_score NUMERIC(5,4),
    volatility_label VARCHAR(20),
    toxicity_pct NUMERIC(5,2),
    toxicity_label VARCHAR(20),
    vocabulary_score NUMERIC(6,2),
    vocabulary_label VARCHAR(20),
    avg_message_length NUMERIC(6,1),
    wordiness_label VARCHAR(20),
    emoji_rate NUMERIC(5,2),
    emoji_label VARCHAR(20),
    message_count INTEGER,
    active_days INTEGER,
    activity_percentile NUMERIC(5,1),
    reactions_received INTEGER,

    -- Metadata
    messages_analyzed INTEGER,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

    PRIMARY KEY (user_id, chat_id)
);

CREATE INDEX idx_user_card_stats_chat ON ml_user_card_stats(chat_id);

-- Auto-update trigger
CREATE TRIGGER update_user_card_stats_timestamp
    BEFORE UPDATE ON ml_user_card_stats
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

---

## Implementation Steps

### Step 1: Database Migration
1. Create `ml_user_card_stats` table
2. Add indexes for efficient queries

### Step 2: Aggregation Function
Create a PostgreSQL function or Python job to compute all green stats:

```sql
CREATE OR REPLACE FUNCTION refresh_user_card_stats(target_chat_id BIGINT)
RETURNS void AS $$
-- Implementation combining all queries above
$$ LANGUAGE plpgsql;
```

### Step 3: API Endpoint
Add endpoint in `api-service`:

```
GET /api/v1/stats/users/{user_id}/card?chat_id={chat_id}
GET /api/v1/stats/chat/{chat_id}/cards  -- All users in chat
```

### Step 4: Scheduled Refresh
- Option A: Trigger after ML processing batch completes
- Option B: Cron job (e.g., every hour)
- Option C: On-demand with caching

---

## File Locations

| Component | Path |
|-----------|------|
| Migration | `apps/api-service/internal/migrations/sql/005_user_card_stats.sql` |
| Repository | `apps/api-service/internal/repository/card_stats_repo.go` |
| Service | `apps/api-service/internal/services/card_stats_service.go` |
| Handler | `apps/api-service/internal/handlers/card_stats_handler.go` |
| Aggregation Job | `apps/ml-processor/src/stats/green_stats.py` |

---

## Testing

```bash
# After migration
psql -c "SELECT refresh_user_card_stats(-1003280306634);"

# API test
curl -H "Authorization: Bearer $API_KEY" \
  "http://localhost:8080/api/v1/stats/chat/-1003280306634/cards"
```

---

## Output Example

```json
{
  "user_id": 123456789,
  "chat_id": -1003280306634,
  "stats": {
    "mood": { "score": 72.5, "label": "Cheerful" },
    "volatility": { "score": 0.18, "label": "Balanced" },
    "toxicity": { "pct": 3.2, "label": "Mild" },
    "vocabulary": { "score": 12.4, "label": "Articulate" },
    "wordiness": { "avg_chars": 67, "label": "Conversationalist" },
    "emoji": { "rate": 1.3, "label": "Expressive" },
    "activity": { "messages": 1523, "days": 89, "percentile": 94.2 },
    "reactions_received": 342
  },
  "updated_at": "2025-01-15T10:30:00Z"
}
```
