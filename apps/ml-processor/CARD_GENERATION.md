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

## Core Concepts

### Bayesian Smoothing

All metrics use Bayesian smoothing to prevent outliers from dominating when sample sizes are small. This ensures that users with few messages have scores closer to the global mean, while high-volume users' scores reflect their actual behavior.

**Formula:**
```
smoothed_score = (n * raw_score + k * global_mean) / (n + k)
```

Where:
- `n` = number of samples (messages analyzed)
- `raw_score` = the calculated raw score
- `k` = smoothing constant (50 by default)
- `global_mean` = default mean for the metric

**Practical effect:**
- User with 1 message and raw score 100: `(1 * 100 + 50 * 50) / (1 + 50) = 51.0`
- User with 100 messages and raw score 100: `(100 * 100 + 50 * 50) / (100 + 50) = 83.3`
- User with 500 messages and raw score 100: `(500 * 100 + 50 * 50) / (500 + 50) = 95.5`

### 90th Percentile Normalization

Count-based metrics (messages, reactions, etc.) are normalized using the 90th percentile value of the chat, making scores relative to the chat's activity level.

**Formula:**
```
normalized = min(count / p90_value, 1.0)
```

Where `p90_value` is computed per-chat:
```sql
SELECT PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY count) as p90
FROM (
    SELECT user_id, COUNT(*) as count
    FROM messages
    WHERE chat_id = :chat_id
      AND date BETWEEN :window_start AND :window_end
    GROUP BY user_id
) user_counts
```

### Emoji Sentiment Classification

Reaction emojis are classified using the [emosent-py](https://pypi.org/project/emosent-py/) library, which provides sentiment scores based on crowdsourced annotations from 751 emojis.

**Thresholds:**
- **Positive**: sentiment_score > 0.2
- **Neutral**: sentiment_score between -0.2 and 0.2
- **Negative**: sentiment_score < -0.2

**Source:** `src/utils/emoji_sentiment.py`

---

## Stat Calculations

All stats are computed from a 30-day rolling window ending on the week's Sunday. Each metric produces a score from 0-100 (except toxicity which is a percentage).

### 1. Vibe Score (0-100)

Measures overall emotional tone combining message sentiment with reaction reception.

**Key Point:** Combines "how you express yourself" with "how others receive you" for a holistic vibe assessment.

**Components:**

| Component | Weight | Description |
|-----------|--------|-------------|
| Positive Ratio | +55% | Messages with positive sentiment (score_positive > 0.5) |
| Neutral Ratio | +5% | Messages with neutral sentiment (score_neutral > 0.5) |
| Negative Ratio | -10% | Messages with negative sentiment (score_negative > 0.5) **[SUBTRACTS]** |
| Consistency | +5% | 1 - STDDEV(positive - negative), measures emotional stability |
| Positive Reactions | +25% | Ratio of positive emoji reactions received |

**Formula:**
```python
# Sentiment ratios from ml_sentiment
positive_ratio = COUNT(score_positive > 0.5) / total_messages
neutral_ratio = COUNT(score_neutral > 0.5) / total_messages
negative_ratio = COUNT(score_negative > 0.5) / total_messages

# Consistency (lower variance = higher consistency)
sentiment_stddev = STDDEV(score_positive - score_negative)
consistency = max(0, 1 - sentiment_stddev)

# Positive reactions ratio (using emosent classification)
positive_reactions = COUNT(*) WHERE emoji sentiment > 0.2
total_reactions = COUNT(*) all reactions received
positive_reaction_ratio = positive_reactions / total_reactions if total > 0 else 0.5

# Combined raw score
raw_score = (
    55 * positive_ratio +
    5 * neutral_ratio -
    10 * negative_ratio +
    5 * consistency +
    25 * positive_reaction_ratio
)

# Scale to 0-100 (max=90, min=-10)
scaled_score = ((raw_score + 10) / 100) * 100
clamped_score = max(0, min(100, scaled_score))
final_score = bayesian_smooth(clamped_score, total_messages, global_mean=50)
```

**SQL for sentiment analysis:**
```sql
SELECT
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE score_positive > 0.5) as positive_count,
    COUNT(*) FILTER (WHERE score_neutral > 0.5) as neutral_count,
    COUNT(*) FILTER (WHERE score_negative > 0.5) as negative_count,
    STDDEV(score_positive - score_negative) as sentiment_stddev
FROM ml_sentiment ms
JOIN messages m ON ms.message_id = m.id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end
```

**SQL for positive reactions received:**
```sql
SELECT
    COUNT(*) as total_reactions,
    array_agg(DISTINCT mr.emoji_value) as unique_emojis
FROM message_reactions mr
JOIN messages m ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end
  AND (mr.is_removed = false OR mr.is_removed IS NULL)
```

The emoji values are then classified in Python using the emosent library.

**Labels:**

| Score | Label | Description |
|-------|-------|-------------|
| 80+ | Radiante | Overwhelmingly positive, high engagement |
| 65-79 | Animado | Generally upbeat, well-received |
| 50-64 | Tranquilo | Balanced, neutral-leaning |
| 35-49 | Reservado | Reserved, slightly negative |
| <35 | Introspectivo | Predominantly negative or isolated |

**Badges:**

| Badge | Condition | Rarity |
|-------|-----------|--------|
| Radiant | score >= 80 | legendary |
| Gloomy | score < 30 | negative |

**Source:** [calculators.py:139-252](src/cards/calculators.py#L139-L252)

---

### 2. Activity (0-100)

Measures engagement level through volume of messages, reactions, and replies.

**Components:**

| Component | Weight | Description |
|-----------|--------|-------------|
| Messages Sent | 35% | Total message count, normalized to P90 |
| Avg Message Length | 20% | Average text length, normalized to P90 |
| Reactions Sent | 25% | Reactions given to others, normalized to P90 |
| Replies Sent | 20% | Replies to other users' messages, normalized to P90 |

**Formula:**
```python
# Normalize each component to 0-1 using chat's 90th percentile
msg_norm = min(messages_sent / p90_messages, 1.0)
len_norm = min(avg_length / p90_length, 1.0)
react_norm = min(reactions_sent / p90_reactions, 1.0)
reply_norm = min(replies_sent / p90_replies, 1.0)

# Weighted combination
raw_score = (
    0.35 * msg_norm +
    0.20 * len_norm +
    0.25 * react_norm +
    0.20 * reply_norm
) * 100

# Apply Bayesian smoothing
final_score = bayesian_smooth(raw_score, messages_sent, global_mean=50)
```

**SQL for user metrics:**
```sql
SELECT
    COUNT(*) as messages_sent,
    AVG(LENGTH(COALESCE(text, caption, ''))) as avg_length
FROM messages
WHERE user_id = :user_id
  AND chat_id = :chat_id
  AND date BETWEEN :window_start AND :window_end
```

**SQL for reactions sent:**
```sql
SELECT COUNT(*) as reactions_sent
FROM message_reactions mr
WHERE mr.user_id = :user_id
  AND mr.chat_id = :chat_id
  AND mr.date >= :window_start
  AND mr.date <= :window_end
  AND (mr.is_removed = false OR mr.is_removed IS NULL)
```

**SQL for replies sent to others:**
```sql
SELECT COUNT(*) as replies_sent
FROM messages m
JOIN messages parent ON m.reply_to_message_id = parent.message_id
                     AND m.chat_id = parent.chat_id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end
  AND parent.user_id != m.user_id  -- Only replies to OTHER users
```

**SQL for P90 normalization (example for messages):**
```sql
SELECT PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY count) as p90
FROM (
    SELECT user_id, COUNT(*) as count
    FROM messages
    WHERE chat_id = :chat_id
      AND date BETWEEN :window_start AND :window_end
    GROUP BY user_id
) user_counts
```

**Badges:**

| Badge | Condition | Rarity |
|-------|-----------|--------|
| Hyperactive | Top 10% in chat | epic |
| Ghost | Bottom 10% in chat | negative |

**Note:** Percentile badges are calculated per-chat, not globally.

**Source:** [calculators.py:255-407](src/cards/calculators.py#L255-L407)

---

### 3. Presence (0-100)

Measures consistency of participation over time.

**Components:**

| Component | Weight | Description |
|-----------|--------|-------------|
| Active Days Ratio | 25% | active_days / total_window_days |
| Streak | 40% | Consecutive days active ending on window_end |
| Hours Spread | 25% | unique_hours / 24, measures hourly diversity |
| Daily Consistency | 10% | 1 - normalized_stddev(daily_message_counts) |

**Formula:**
```python
# Calculate total days in window
total_days = (window_end - window_start).days + 1

# Active days ratio
active_days_ratio = active_days / total_days

# Streak calculation (consecutive days ending on window_end)
# Walk backwards from window_end counting consecutive active days
streak_days = consecutive_active_days_from_end

# Normalize streak (cap at window size for 0-1)
streak_norm = min(streak_days / total_days, 1.0)

# Hours spread (how many different hours user is active)
hours_spread = unique_hours / 24

# Daily consistency (lower variance = higher consistency)
# Calculate stddev of daily message counts, normalize by max observed
daily_consistency = 1 - (stddev_daily / max_daily_count) if max > 0 else 1.0

# Weighted combination
raw_score = (
    0.25 * active_days_ratio +
    0.40 * streak_norm +
    0.25 * hours_spread +
    0.10 * daily_consistency
) * 100

# Apply Bayesian smoothing
final_score = bayesian_smooth(raw_score, total_messages, global_mean=50)
```

**SQL for presence metrics:**
```sql
SELECT
    COUNT(DISTINCT date_trunc('day', date AT TIME ZONE :timezone)) as active_days,
    COUNT(DISTINCT EXTRACT(HOUR FROM date AT TIME ZONE :timezone)) as unique_hours
FROM messages
WHERE user_id = :user_id
  AND chat_id = :chat_id
  AND date BETWEEN :window_start AND :window_end
```

**SQL for daily consistency:**
```sql
SELECT
    date_trunc('day', date AT TIME ZONE :timezone) as day,
    COUNT(*) as daily_count
FROM messages
WHERE user_id = :user_id
  AND chat_id = :chat_id
  AND date BETWEEN :window_start AND :window_end
GROUP BY date_trunc('day', date AT TIME ZONE :timezone)
```

**Streak Algorithm (Python):**
```python
def calculate_streak(daily_dates: list[date], window_end: date) -> int:
    """Count consecutive active days ending on window_end."""
    if not daily_dates:
        return 0

    active_set = set(daily_dates)
    streak = 0
    current = window_end.date()

    while current in active_set:
        streak += 1
        current -= timedelta(days=1)

    return streak
```

**Badges:**

| Badge | Condition | Rarity |
|-------|-----------|--------|
| Regular | score >= 80 | epic |
| Tourist | score < 20 AND messages >= 10 | negative |

**Source:** [calculators.py:410-545](src/cards/calculators.py#L410-L545)

---

### 4. Humor (0-100)

Measures comedy impact through ML humor detection and positive reactions.

**Components:**

| Component | Weight | Description |
|-----------|--------|-------------|
| Positive Reactions | 45% | Reactions with emoji sentiment > 0.2 |
| Unique Positive Reactors | 25% | Distinct users giving positive reactions |
| Humorous Messages | 15% | % of messages flagged by ML humor detector |
| Humorous Replies | 15% | % of replies to user that are humorous |

**Formula:**
```python
# Positive reactions score
positive_reactions_score = positive_reaction_count / total_reactions if total > 0 else 0

# Unique reactors breadth
unique_reactors_score = unique_positive_reactors / total_chat_users

# ML humor percentage (excluding emoji-only messages)
# Messages that are ONLY emojis are filtered out to avoid false positives
humorous_pct = humorous_count / total_analyzed if total > 0 else 0

# Humorous replies received
humorous_replies_score = humorous_replies / total_replies if total > 0 else 0

# Weighted combination
raw_score = (
    0.45 * positive_reactions_score +
    0.25 * unique_reactors_score +
    0.15 * humorous_pct +
    0.15 * humorous_replies_score
) * 100

# Apply Bayesian smoothing
final_score = bayesian_smooth(raw_score, total_messages, global_mean=30)
```

**SQL for reactions with emoji sentiment:**
```sql
SELECT
    COUNT(*) as total_reactions,
    array_agg(DISTINCT mr.emoji_value) as unique_emojis,
    array_agg(DISTINCT mr.user_id) as reactor_user_ids
FROM message_reactions mr
JOIN messages m ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end
  AND (mr.is_removed = false OR mr.is_removed IS NULL)
```

**SQL for ML humor (excluding emoji-only messages):**
```sql
SELECT
    COUNT(*) as total_analyzed,
    COUNT(*) FILTER (WHERE mh.is_humorous = true) as humorous_count
FROM ml_humor mh
JOIN messages m ON mh.message_id = m.id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end
  -- Exclude emoji-only messages (only emojis/symbols/whitespace)
  AND LENGTH(REGEXP_REPLACE(COALESCE(m.text, m.caption, ''), '[^\w\s]', '', 'g')) > 0
```

**SQL for humorous replies received:**
```sql
SELECT
    COUNT(*) as total_replies,
    COUNT(*) FILTER (WHERE mh.is_humorous = true) as humorous_replies
FROM messages reply
JOIN messages parent ON reply.reply_to_message_id = parent.message_id
                     AND reply.chat_id = parent.chat_id
LEFT JOIN ml_humor mh ON mh.message_id = reply.id
WHERE parent.user_id = :user_id
  AND parent.chat_id = :chat_id
  AND parent.date BETWEEN :window_start AND :window_end
  AND reply.user_id != parent.user_id
```

**Badges:**

| Badge | Condition | Rarity |
|-------|-----------|--------|
| Comedian | score >= 70 | legendary |
| Deadpan | score < 10 | negative |

**Source:** [calculators.py:548-709](src/cards/calculators.py#L548-L709)

---

### 5. Toxicity (0-100%)

Measures negative impact through toxic message detection and negative reactions.

**Key Point:** Being sad is NOT toxic. Negative sentiment affects Vibe Score, not Toxicity. Toxicity is reserved for aggressive/offensive content.

**Components:**

| Component | Weight | Description |
|-----------|--------|-------------|
| Toxic Messages | 70% | % of messages flagged by ML toxicity detector |
| Negative Reactions | 25% | % of reactions received with emoji sentiment < -0.2 |
| Unique Negative Reactors | 5% | % of reactors giving negative reactions (broad disapproval) |

**Formula:**
```python
# Toxic message percentage from ML
toxic_pct = toxic_count / total_analyzed if total > 0 else 0

# Negative reactions percentage
negative_reactions_pct = negative_reaction_count / total_reactions if total > 0 else 0

# Unique negative reactors breadth
negative_reactors_pct = unique_negative_reactors / total_unique_reactors if total > 0 else 0

# Weighted combination (as percentage)
raw_pct = (
    0.70 * toxic_pct +
    0.25 * negative_reactions_pct +
    0.05 * negative_reactors_pct
) * 100

# Apply Bayesian smoothing
final_pct = bayesian_smooth(raw_pct, total_messages, global_mean=5)
```

**SQL for toxic messages:**
```sql
SELECT
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE is_toxic = true) as toxic_count
FROM ml_toxicity mt
JOIN messages m ON mt.message_id = m.id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end
```

**SQL for reactions:**
```sql
SELECT
    COUNT(*) as total_reactions,
    array_agg(mr.emoji_value) as emojis,
    array_agg(DISTINCT mr.user_id) as reactor_ids
FROM message_reactions mr
JOIN messages m ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end
  AND (mr.is_removed = false OR mr.is_removed IS NULL)
```

**Labels:**

| Percentage | Label | Description |
|------------|-------|-------------|
| <2% | Zen | Almost never toxic |
| 2-4% | Leve | Rarely toxic |
| 5-9% | Moderado | Occasionally toxic |
| 10-19% | Picante | Frequently spicy |
| ≥20% | Explosivo | Often toxic |

**Badges:**

| Badge | Condition | Rarity |
|-------|-----------|--------|
| Zen | pct < 2 | legendary |
| Toxic | pct > 20 | negative |

**Source:** [calculators.py:712-821](src/cards/calculators.py#L712-L821)

---

### 6. Popularity (0-100)

Measures social gravity through engagement breadth and viral content.

**Components:**

| Component | Weight | Description |
|-----------|--------|-------------|
| Unique Reactors | 25% | Distinct users who reacted, normalized to P90 |
| Unique Repliers | 25% | Distinct users who replied, normalized to P90 |
| Total Reactions | 15% | Total reaction count, normalized to P90 |
| Total Replies | 15% | Total reply count, normalized to P90 |
| Viral Messages | 20% | Messages with 4+ reactions, normalized to P90 |

**Formula:**
```python
# Normalize each component to 0-1 using chat's 90th percentile
unique_reactors_norm = min(unique_reactors / p90_unique_reactors, 1.0)
unique_repliers_norm = min(unique_repliers / p90_unique_repliers, 1.0)
total_reactions_norm = min(total_reactions / p90_total_reactions, 1.0)
total_replies_norm = min(total_replies / p90_total_replies, 1.0)
viral_norm = min(viral_count / p90_viral, 1.0)

# Weighted combination
raw_score = (
    0.25 * unique_reactors_norm +
    0.25 * unique_repliers_norm +
    0.15 * total_reactions_norm +
    0.15 * total_replies_norm +
    0.20 * viral_norm
) * 100

# Apply Bayesian smoothing
final_score = bayesian_smooth(raw_score, total_messages, global_mean=30)
```

**SQL for unique reactors:**
```sql
SELECT COUNT(DISTINCT mr.user_id) as unique_reactors
FROM message_reactions mr
JOIN messages m ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end
  AND (mr.is_removed = false OR mr.is_removed IS NULL)
```

**SQL for unique repliers:**
```sql
SELECT COUNT(DISTINCT reply.user_id) as unique_repliers
FROM messages reply
JOIN messages m ON reply.reply_to_message_id = m.message_id
               AND reply.chat_id = m.chat_id
WHERE m.user_id = :user_id
  AND m.chat_id = :chat_id
  AND m.date BETWEEN :window_start AND :window_end
  AND reply.user_id != m.user_id
```

**SQL for viral messages (4+ reactions):**
```sql
SELECT COUNT(*) as viral_count
FROM (
    SELECT m.message_id
    FROM messages m
    JOIN message_reactions mr ON mr.chat_id = m.chat_id
                              AND mr.message_id = m.message_id
    WHERE m.user_id = :user_id
      AND m.chat_id = :chat_id
      AND m.date BETWEEN :window_start AND :window_end
      AND (mr.is_removed = false OR mr.is_removed IS NULL)
    GROUP BY m.message_id
    HAVING COUNT(*) >= 4
) viral
```

**Badges:**

| Badge | Condition | Rarity |
|-------|-----------|--------|
| Star | Top 10% in chat | legendary |
| Cricket | Bottom 10% in chat AND messages >= 30 | negative |

**Note:** Percentile badges are calculated per-chat, not globally.

**Source:** [calculators.py:824-979](src/cards/calculators.py#L824-L979)

---

### 7. Overall Score (0-100)

Holistic score combining all 6 metrics with weighted importance, trend modifiers, and badge modifiers. **Used for card ranking.**

**Formula (3 phases):**

**Phase 1: Base Score (70% positive / 30% negative penalty)**

| Component | Type | Weight | Source |
|-----------|------|--------|--------|
| Popularity | Positive | 20% | `popularity.score` |
| Presence | Positive | 15% | `presence.score` |
| Vibe | Positive | 12% | `vibe.score` |
| Days Streak | Positive | 10% | `presence.streak` (normalized to 0-100) |
| Humor | Positive | 8% | `humor.score` |
| Activity | Positive | 5% | `activity.score` |
| Toxicity | Negative | 12% | `toxicity.pct` |
| Negative Reactions | Negative | 7% | `toxicity.negative_reactions` / total |
| Negative Messages | Negative | 6% | `vibe.negative_ratio` |
| Longest Gap | Negative | 5% | Max days between posts (normalized) |

```python
# Calculate positive contribution (70 points max)
positive = (
    0.20 * popularity_score +
    0.15 * presence_score +
    0.12 * vibe_score +
    0.10 * streak_normalized +
    0.08 * humor_score +
    0.05 * activity_score
)

# Calculate negative penalty (30 points max)
negative = (
    0.12 * toxicity_pct +
    0.07 * negative_reaction_ratio +
    0.06 * negative_msg_ratio +
    0.05 * gap_normalized
)

base_score = positive - negative  # Range: -30 to +70
```

**Phase 2: Trend Modifiers**

For each of the 6 base metrics, add points based on trend direction:
- Upward trend >10% change: **+5 points**
- Upward trend ≤10% change: **+3 points**
- Downward trend ≤10% change: **-3 points**
- Downward trend >10% change: **-5 points**

For toxicity, direction is inverted (decreasing toxicity = good).

Max impact: ±30 points (if all 6 metrics trend same direction).

**Phase 3: Badge Modifiers**

Sum of modifiers for all earned badges:
- Legendary badge: **+5 points**
- Epic badge: **+3 points**
- Negative badge: **-5 points**

**Final Score:**
```python
final_score = clamp(base_score + trend_modifier + badge_modifier, 0, 100)
```

**Labels:**

| Score | Label |
|-------|-------|
| 85+ | Lendario |
| 70-84 | Elite |
| 55-69 | Destacado |
| 40-54 | Regular |
| 25-39 | Iniciante |
| <25 | Novato |

**Source:** [calculators.py](src/cards/calculators.py), [generator.py](src/cards/generator.py)

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

**Direction Logic:**
- `up`: delta > 0
- `down`: delta < 0
- `stable`: delta == 0

**Percentage Change Formula:**
```python
def calc_pct_change(current: float, previous: float) -> float:
    if previous == 0:
        return 100.0 if current > 0 else 0.0
    return round((current - previous) / abs(previous) * 100, 1)
```

**Stats with trends:**
- `vibe`: Delta in score (0-100)
- `activity`: Delta in score (0-100)
- `presence`: Delta in score (0-100)
- `humor`: Delta in score (0-100)
- `popularity`: Delta in score (0-100)
- `toxicity`: Delta in percentage (0-100%)

---

## Badge Summary

All badges are evaluated per-chat, with percentile-based badges using chat-specific rankings.

**Note:** Badges directly affect the Overall Score via badge modifiers:
- Legendary badges: +5 points
- Epic badges: +3 points
- Negative badges: -5 points

### Positive Badges

| Badge | Metric | Condition | Rarity |
|-------|--------|-----------|--------|
| Radiant | Vibe | score >= 80 | legendary |
| Hyperactive | Activity | Top 10% in chat | epic |
| Regular | Presence | score >= 80 | epic |
| Comedian | Humor | score >= 70 | legendary |
| Zen | Toxicity | pct < 2% | legendary |
| Star | Popularity | Top 10% in chat | legendary |

### Negative Badges

| Badge | Metric | Condition | Rarity |
|-------|--------|-----------|--------|
| Gloomy | Vibe | score < 30 | negative |
| Ghost | Activity | Bottom 10% in chat | negative |
| Tourist | Presence | score < 20 AND msgs >= 10 | negative |
| Deadpan | Humor | score < 10 | negative |
| Toxic | Toxicity | pct > 20% | negative |
| Cricket | Popularity | Bottom 10% in chat AND msgs >= 30 | negative |

### Rarity Levels

1. **Common** - Basic achievements (gray)
2. **Rare** - Notable achievements (blue)
3. **Epic** - Impressive achievements (purple)
4. **Legendary** - Exceptional achievements (gold)
5. **Negative** - Anti-achievements (red)

**Source:** [card-image-generator/src/renderer/template_loader.py](../card-image-generator/src/renderer/template_loader.py)

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
    UNIQUE(user_id, chat_id, week_start)
);
```

### Stats JSON Structure

```json
{
  "vibe": {
    "score": 72.5,
    "label": "Animado",
    "positive_ratio": 0.45,
    "negative_ratio": 0.12,
    "positive_reactions_ratio": 0.68
  },
  "activity": {
    "score": 65.3,
    "messages": 145,
    "avg_length": 42.3,
    "reactions_sent": 87,
    "replies_sent": 34
  },
  "presence": {
    "score": 78.2,
    "active_days": 22,
    "streak": 8,
    "hours_spread": 14
  },
  "humor": {
    "score": 55.0,
    "label": "Engracado",
    "humorous_pct": 15.2,
    "positive_reactions": 42
  },
  "toxicity": {
    "pct": 3.2,
    "label": "Leve",
    "toxic_count": 5,
    "total_analyzed": 156
  },
  "popularity": {
    "score": 61.8,
    "unique_reactors": 23,
    "unique_repliers": 18,
    "viral_messages": 3
  },
  "overall": {
    "score": 58.5,
    "label": "Destacado",
    "positive_contribution": 45.2,
    "negative_penalty": 8.7,
    "longest_gap_days": 3,
    "trend_modifier": 12.0,
    "badge_modifier": 10.0
  }
}
```

### Trends JSON Structure

```json
{
  "vibe": {
    "delta": 5.2,
    "direction": "up",
    "pct_change": 7.7
  },
  "activity": {
    "delta": -8.1,
    "direction": "down",
    "pct_change": -11.0
  },
  "presence": {
    "delta": 0.0,
    "direction": "stable",
    "pct_change": 0.0
  },
  "humor": {
    "delta": 3.5,
    "direction": "up",
    "pct_change": 6.8
  },
  "toxicity": {
    "delta": -1.2,
    "direction": "down",
    "pct_change": -27.3
  },
  "popularity": {
    "delta": 12.4,
    "direction": "up",
    "pct_change": 25.1
  }
}
```

---

## Card Image Generation

After stats are computed, the card-image-generator service renders PNG images.

### Process

1. **Query cards** from `ml_user_cards` for specified chat/week
2. **Transform stats** to template context (normalize, derive badges)
3. **Compute rankings** per category for top 3 display
4. **Render HTML** using Jinja2 templates with theme
5. **Screenshot** HTML using Playwright (Chromium headless)
6. **Upload PNG** to MinIO with SHA256 hash
7. **Store reference** in `ml_user_card_images` table

### Category Rankings

Top 3 users per category are shown on cards. Rankings are computed with this query:

```sql
WITH category_ranks AS (
    SELECT
        user_id,
        ROW_NUMBER() OVER (ORDER BY (stats->'vibe'->>'score')::float DESC NULLS LAST) AS vibe_rank,
        ROW_NUMBER() OVER (ORDER BY (stats->'activity'->>'score')::float DESC NULLS LAST) AS activity_rank,
        ROW_NUMBER() OVER (ORDER BY (stats->'presence'->>'score')::float DESC NULLS LAST) AS presence_rank,
        ROW_NUMBER() OVER (ORDER BY (stats->'humor'->>'score')::float DESC NULLS LAST) AS humor_rank,
        ROW_NUMBER() OVER (ORDER BY (stats->'toxicity'->>'pct')::float ASC NULLS LAST) AS toxicity_rank,
        ROW_NUMBER() OVER (ORDER BY (stats->'popularity'->>'score')::float DESC NULLS LAST) AS popularity_rank,
        ROW_NUMBER() OVER (ORDER BY (stats->'overall'->>'score')::float DESC NULLS LAST) AS overall_rank
    FROM ml_user_cards
    WHERE chat_id = :chat_id
      AND week_start = :week_start
)
SELECT * FROM category_ranks
WHERE vibe_rank <= 3 OR activity_rank <= 3 OR presence_rank <= 3
   OR humor_rank <= 3 OR toxicity_rank <= 3 OR popularity_rank <= 3
   OR overall_rank <= 3
```

**Note:** Toxicity is sorted ASC (lowest = best), all others including Overall DESC (highest = best).

**Card ranking:** Cards are ordered by `overall.score` DESC for the primary ranking display.

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
       raw_score = ...
       smoothed = _bayesian_smooth(raw_score, sample_count, global_mean=50)
       return StatResult(value={"score": smoothed, "label": "Example"})
   ```

2. **Add to registry**:
   ```python
   CALCULATORS: dict[str, StatCalculator] = {
       # ... existing calculators
       "new_stat": calculate_new_stat,
   }
   ```

3. **Update generator.py** to include new stat in trends calculation

4. **Update template_loader.py** in card-image-generator:
   - Add to `STAT_CONFIG`
   - Add badge rules if applicable
   - Update `TemplateContext` dataclass

5. **Update queries.py** to include in category rankings if needed

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
- Bayesian smoothing may pull low-sample scores toward global mean

### Timezone issues

- Always use IANA timezone names: `America/Sao_Paulo`, not `BRT`
- Timezone affects week boundaries (Monday 00:00 local time)
- Presence streak and hours spread are timezone-aware
- Storage: timezone is stored with each card for reference

### Low scores with few messages

This is expected behavior due to Bayesian smoothing:
- Users with few messages have scores closer to the global mean
- As message count increases, score reflects actual behavior
- This prevents outliers from dominating rankings

### Re-generating cards

```bash
# Delete cards for a week
make ml-clean-cards-dev CHAT_ID=-1003280306634 WEEK=2024-12-16

# Regenerate
make ml-run-cards ML_ARGS="--timezone America/Sao_Paulo --week 2024-12-16"
```
