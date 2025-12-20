# Card Generation Pipeline

This document describes how user stats cards are generated, including the exact formulas for each stat calculation.

## Overview

Cards are weekly summaries of user behavior in a chat, computed from ML analysis results. The pipeline:

```
messages → ML analysis → stat calculators → ml_user_cards → card-image-generator → PNG images
```

### Generation Cycle

- **Frequency**: Weekly (Monday to Sunday)
- **Stats Window**: 30-day rolling window for stable personality traits
- **Trends**: Week-over-week comparison
- **Minimum Messages**: 10 (configurable)

### Command

```bash
# Generate cards for current week
make ml-run-cards ML_ARGS="--timezone America/Sao_Paulo"

# Generate cards for specific week
make ml-run-cards ML_ARGS="--timezone America/Sao_Paulo --week 2024-12-16"
```

---

## Stat Calculations

All stats are computed from the 30-day rolling window ending on the week's Sunday.

### 1. Mood (0-100)

Measures overall emotional tone from sentiment analysis.

**Formula:**
```sql
SELECT AVG(
    score_positive * 100 +
    score_neutral * 50 +
    score_negative * 0
) as mood_score
FROM ml_sentiment ms
JOIN messages m ON ms.message_id = m.id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end
```

**Interpretation:**
- Positive messages contribute 100 points
- Neutral messages contribute 50 points
- Negative messages contribute 0 points
- Average across all messages in window

**Labels:**

| Score | Label | Description |
|-------|-------|-------------|
| 80+ | Radiante | Overwhelmingly positive |
| 65-79 | Animado | Generally upbeat |
| 50-64 | Tranquilo | Balanced, neutral-leaning |
| 35-49 | Reservado | Reserved, slightly negative |
| <35 | Introspectivo | Predominantly negative |

**Source:** `ml_sentiment` table → [calculators.py:144-187](src/cards/calculators.py#L144-L187)

---

### 2. Volatility (0-1)

Measures emotional consistency - how much sentiment varies between messages.

**Formula:**
```sql
SELECT STDDEV(score_positive - score_negative) as volatility
FROM ml_sentiment ms
JOIN messages m ON ms.message_id = m.id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end
```

**Interpretation:**
- Low values = consistent emotional tone
- High values = frequent mood swings
- Capped at 1.0

**Labels:**

| Score | Label | Description |
|-------|-------|-------------|
| <0.15 | Estavel | Very consistent tone |
| 0.15-0.29 | Equilibrado | Mostly stable |
| 0.30-0.49 | Dinamico | Variable moods |
| ≥0.50 | Intenso | Highly variable |

**Source:** `ml_sentiment` table → [calculators.py:190-231](src/cards/calculators.py#L190-L231)

---

### 3. Toxicity (%)

Percentage of messages flagged as toxic by the toxicity detector.

**Formula:**
```sql
SELECT
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE is_toxic = true) as toxic_count
FROM ml_toxicity mt
JOIN messages m ON mt.message_id = m.id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end

-- Result: (toxic_count / total) * 100
```

**Labels:**

| Percentage | Label | Description |
|------------|-------|-------------|
| <2% | Zen | Almost never toxic |
| 2-4% | Leve | Rarely toxic |
| 5-9% | Moderado | Occasionally toxic |
| 10-19% | Picante | Frequently spicy |
| ≥20% | Explosivo | Often toxic |

**Source:** `ml_toxicity` table → [calculators.py:234-276](src/cards/calculators.py#L234-L276)

---

### 4. Activity

Measures engagement level through message metrics.

**Formulas:**
```sql
SELECT
    COUNT(*) as messages,
    COUNT(DISTINCT DATE(date)) as active_days,
    AVG(LENGTH(COALESCE(text, caption, ''))) as avg_length
FROM messages
WHERE user_id = :user_id
  AND chat_id = :chat_id
  AND date BETWEEN :window_start AND :window_end
```

**Components:**
- `messages`: Total message count in window
- `active_days`: Number of distinct days with messages
- `avg_length`: Average message length (text or caption)

**Source:** `messages` table → [calculators.py:279-323](src/cards/calculators.py#L279-L323)

---

### 5. Reactions Received

Count of reactions received on the user's messages.

**Formula:**
```sql
SELECT COUNT(*) as reactions
FROM message_reactions mr
JOIN messages m ON mr.chat_id = m.chat_id
                AND mr.message_id = m.message_id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end
  AND (mr.is_removed = false OR mr.is_removed IS NULL)
```

**Note:** The `message_reactions` table uses Telegram's `message_id` (not our internal FK), so the join is on `(chat_id, message_id)`.

**Source:** `message_reactions` + `messages` tables → [calculators.py:326-364](src/cards/calculators.py#L326-L364)

---

### 6. Comedy (0-1)

Hybrid score combining ML humor detection with laugh reactions received.

**Formula:**
```python
# ML Component (30% weight)
ml_score = AVG(score) WHERE is_humorous = true

# Reactions Component (70% weight)
laugh_reactions = COUNT(*) of laugh emoji reactions on user's messages
reactions_score = min(log2(1 + laugh_reactions) / 10, 1.0)

# Combined Score
comedy_score = (ml_score * 0.3) + (reactions_score * 0.7)
```

**SQL for laugh reactions:**
```sql
SELECT COUNT(*) as laugh_count
FROM message_reactions mr
JOIN messages m ON mr.chat_id = m.chat_id
                AND mr.message_id = m.message_id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end
  AND mr.emoji_value = ANY(:laugh_emojis)
  AND (mr.is_removed = false OR mr.is_removed IS NULL)
```

**Laugh Emojis:**
```python
LAUGH_EMOJIS = [
    # Classic laughs
    "😂", "🤣", "😆", "😄", "😅", "😸", "😹",
    # "I'm dead" / melting
    "🫠", "💀", "☠️", "⚰️",
    # Crying (from laughing)
    "😭",
    # Loud reactions
    "📢", "🗣️",
    # Physical comedy reactions
    "🤸", "🏃", "💨", "🐒", "🤡",
]
```

**Labels:**

| Score | Label | Description |
|-------|-------|-------------|
| ≥0.7 | Comediante | Professional-level funny |
| 0.4-0.69 | Engracado | Reliably funny |
| 0.2-0.39 | Espirituoso | Occasionally witty |
| <0.2 | Serio | Not focused on humor |

**Output includes:**
- `score`: Combined comedy score (0-1)
- `label`: Category label
- `humor_pct`: % of messages flagged as humorous by ML
- `laugh_reactions`: Raw count of laugh reactions

**Source:** `ml_humor` + `message_reactions` tables → [calculators.py:418-508](src/cards/calculators.py#L418-L508)

---

### 7. Chronotype

Peak activity hour, identifying when the user is most active.

**Formula:**
```sql
SELECT
    EXTRACT(HOUR FROM date AT TIME ZONE :timezone) as hour,
    COUNT(*) as count
FROM messages
WHERE user_id = :user_id
  AND chat_id = :chat_id
  AND date BETWEEN :window_start AND :window_end
GROUP BY EXTRACT(HOUR FROM date AT TIME ZONE :timezone)
ORDER BY count DESC
LIMIT 1
```

**Labels:**

| Hour Range | Label | Description |
|------------|-------|-------------|
| 5-8 | Madrugador | Early bird |
| 9-11 | Matutino | Morning person |
| 12-13 | Almoceiro | Lunch-time active |
| 14-17 | Vespertino | Afternoon person |
| 18-21 | Noturno | Evening person |
| 22-4 | Coruja | Night owl |

**Note:** The timezone parameter affects both week boundaries and chronotype calculation.

**Source:** `messages` table → [calculators.py:367-415](src/cards/calculators.py#L367-L415)

---

## Trend Calculations

Each stat includes a trend comparing the current week to the previous week.

**Structure:**
```python
{
    "delta": float,           # Absolute change
    "direction": str,         # "up", "down", or "stable"
    "pct_change": float       # Percentage change
}
```

**Direction Thresholds:**
- `up`: Change > +5%
- `down`: Change < -5%
- `stable`: Change within ±5%

**Stats with trends:**
- `mood`: Delta in score
- `activity`: Delta in message count
- `comedy`: Delta in score
- `volatility`: Delta in score
- `toxicity`: Delta in percentage
- `reactions`: Delta in count

---

## Badge Derivation

Badges are derived from stats by the card-image-generator service. Up to 4 badges are displayed per card.

### Available Badges

| Badge | Condition | Rarity | Color |
|-------|-----------|--------|-------|
| Ray of Sunshine | mood ≥ 90 | Legendary | Gold gradient |
| Optimist | mood ≥ 75 | Epic | Purple gradient |
| Stand-Up King | comedy ≥ 0.7 | Legendary | Gold gradient |
| Class Clown | comedy ≥ 0.5 | Epic | Purple gradient |
| Night Owl | chronotype = "Coruja" | Rare | Blue gradient |
| Early Bird | chronotype = "Madrugador" | Rare | Blue gradient |
| Chatterbox | messages ≥ 500 | Legendary | Gold gradient |
| Active Voice | messages ≥ 200 | Epic | Purple gradient |
| Regular | messages ≥ 100 | Rare | Blue gradient |
| Zen Master | toxicity < 1% | Legendary | Gold gradient |
| Peacekeeper | toxicity < 5% | Epic | Purple gradient |
| Beloved | reactions ≥ 100 | Legendary | Gold gradient |
| Popular | reactions ≥ 50 | Epic | Purple gradient |

### Rarity Levels

1. **Common** - Basic achievements (gray)
2. **Rare** - Notable achievements (blue)
3. **Epic** - Impressive achievements (purple)
4. **Legendary** - Exceptional achievements (gold)

**Source:** [card-image-generator/src/renderer/template_loader.py:112-171](../card-image-generator/src/renderer/template_loader.py#L112-L171)

---

## Card Storage Schema

### `ml_user_cards` Table

```sql
CREATE TABLE ml_user_cards (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,
    week_start DATE NOT NULL,
    week_end DATE NOT NULL,
    stats_window_start DATE NOT NULL,
    stats_window_end DATE NOT NULL,
    stats JSONB NOT NULL,
    trends JSONB,
    messages_analyzed INTEGER NOT NULL,
    timezone VARCHAR(64) NOT NULL,
    card_version INTEGER NOT NULL DEFAULT 1,
    generated_at TIMESTAMPTZ NOT NULL,
    UNIQUE(user_id, chat_id, week_start, card_version)
);
```

### Stats JSON Structure

```json
{
  "mood": {
    "score": 72.5,
    "label": "Animado"
  },
  "volatility": {
    "score": 0.23,
    "label": "Equilibrado"
  },
  "toxicity": {
    "pct": 3.2,
    "label": "Leve"
  },
  "activity": {
    "messages": 145,
    "active_days": 12,
    "avg_length": 42.3
  },
  "reactions_received": 67,
  "chronotype": {
    "peak_hour": 21,
    "type": "Noturno"
  },
  "comedy": {
    "score": 0.45,
    "label": "Engracado",
    "humor_pct": 15.2,
    "laugh_reactions": 23
  }
}
```

### Trends JSON Structure

```json
{
  "mood": {
    "delta": 5.2,
    "direction": "up",
    "pct_change": 7.7
  },
  "activity": {
    "delta": -12,
    "direction": "down",
    "pct_change": -8.3
  },
  "comedy": {
    "delta": 0.05,
    "direction": "stable",
    "pct_change": 3.1
  }
}
```

---

## Card Image Generation

After stats are computed, the card-image-generator service renders PNG images.

### Process

1. **Query cards** from `ml_user_cards` for specified chat/week
2. **Transform stats** to template context (normalize percentages, derive badges)
3. **Render HTML** using Jinja2 templates with theme
4. **Screenshot** HTML using Playwright (Chromium headless)
5. **Upload PNG** to MinIO with SHA256 hash
6. **Store reference** in `ml_user_card_images` table

### Image Specifications

- **Dimensions**: 400x600 pixels (logical)
- **Scale Factor**: 2x (renders at 800x1200 for retina displays)
- **Format**: PNG
- **Storage Path**: `cards/{chat_id}/{week_start}/{user_id}.png`

### Render Command

```bash
# Render images for a week
make ml-run-render ML_ARGS="--week 2024-12-16"

# Force re-render
make ml-run-render ML_ARGS="--week 2024-12-16 --force"
```

### API Endpoints

```bash
# Render cards
POST /api/v1/render
{
  "chat_id": -1003280306634,
  "week_start": "2024-12-16",
  "theme": "gaming"
}

# Get image URL
GET /api/v1/image/{image_id}?expires=3600
```

See [card-image-generator README](../card-image-generator/README.md) for full API documentation.

---

## Adding New Stats

To add a new stat calculator:

1. **Create calculator function** in [calculators.py](src/cards/calculators.py):
   ```python
   def calculate_new_stat(
       engine: Engine,
       user_id: int,
       chat_id: int,
       window_start: datetime,
       window_end: datetime,
       timezone: str | None = None,
   ) -> StatResult | None:
       # Query and compute
       return StatResult(value={"score": 42, "label": "Example"})
   ```

2. **Add to registry**:
   ```python
   CALCULATORS: dict[str, StatCalculator] = {
       # ... existing calculators
       "new_stat": calculate_new_stat,
   }
   ```

3. **Update template** in card-image-generator to display the new stat

4. **(Optional) Add badge derivation** in template_loader.py

---

## Troubleshooting

### No cards generated

```bash
# Check minimum message threshold
make ml-run-cards ML_ARGS="--min-messages 5 --timezone America/Sao_Paulo"

# Verify ML processing is complete
make ml-run-status
```

### Missing stats

If a stat returns `None`, check:
- ML analysis tables have data for the user/window
- The query filters are correct (user_id, chat_id, date range)
- Sufficient messages exist (some stats need minimum data)

### Timezone issues

- Always use IANA timezone names: `America/Sao_Paulo`, not `BRT`
- Timezone affects week boundaries (Monday 00:00 local time)
- Chronotype peak hours are calculated in the specified timezone

### Re-generating cards

```bash
# Delete cards for a week
make ml-clean-cards-dev CHAT_ID=-1003280306634 WEEK=2024-12-16

# Regenerate
make ml-run-cards ML_ARGS="--timezone America/Sao_Paulo --week 2024-12-16"
```
