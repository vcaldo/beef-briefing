# Sentiment Refactor: Metrics Design

## Overview

Refactor the metrics system to incorporate sentiment analysis and create 6 distinct user metrics. Each metric uses weighted signals from available data sources, with strategic overlap where it strengthens the metric.

---

## The 6 Metrics

### 1. VIBE SCORE (Mood + Reception)

**Goal**: User's emotional expression combined with how positively others respond to them.

**Components**:
| Signal | Weight | Description |
|--------|--------|-------------|
| Positive message ratio | 35% | % of messages with positive sentiment |
| Neutral message ratio | 5% | % of messages with neutral sentiment |
| Negative message ratio | -30% | % of messages with negative sentiment (reduces score) |
| Sentiment consistency | 5% | Low volatility = stable mood |
| Positive reactions received | 25% | People respond well to you |

**Key Point**: Combines "how you express yourself" with "how others receive you" for a holistic vibe assessment.

---

### 2. ACTIVITY (Volume Produced)

**Goal**: How much the user contributes to the group.

**Components**:
| Signal | Weight | Description |
|--------|--------|-------------|
| Messages sent | 35% | Raw volume |
| Average message length | 20% | Effort per message |
| Reactions sent to others | 25% | Engaging with others' content |
| Replies sent to others | 20% | Participating in conversations |

**Key Point**: Measures output volume, including social engagement (reactions/replies sent).

---

### 3. PRESENCE (Consistency vs Bursts)

**Goal**: Track users who show up consistently vs those who appear in bursts.

**Components**:
| Signal | Weight | Description |
|--------|--------|-------------|
| Active days ratio | 25% | Days active / total days in window |
| Current streak | 40% | Consecutive days active |
| Hours spread | 25% | Unique hours with messages (distribution) |
| Activity variance | 10% | Low variance = consistent; High = bursty (inverted) |

**Key Point**: Distinguishes between:
- **Consistent user**: Posts spread across many days/hours
- **Bursty user**: Intense activity in short periods, then disappears

The "activity variance" signal measures the standard deviation of daily message counts - lower variance means more consistent presence.

---

### 4. HUMOR (Comedy Impact)

**Goal**: How funny others find the user.

**Components**:
| Signal | Weight | Description |
|--------|--------|-------------|
| Positive reactions received | 45% | People react positively (emosent-py) |
| Unique users who reacted positively | 25% | Broad appeal |
| Messages classified as humorous | 15% | Intentional comedy (ml_humor) |
| Humorous replies received | 15% | Banter quality |

**Important**: For "messages classified as humorous", exclude emoji-only messages from the count. The ml_humor classifier may flag emoji-only messages as "humorous", but these shouldn't count toward intentional comedy.

**Emoji Sentiment**: Use emosent-py library to classify reaction sentiment:
- Positive: sentiment_score > 0.2
- Negative: sentiment_score < -0.2
- Neutral: -0.2 to 0.2

---

### 5. TOXICITY (Negative Impact)

**Goal**: How toxic/aggressive the user makes the group feel.

**Components**:
| Signal | Weight | Description |
|--------|--------|-------------|
| Toxic messages (ml_toxicity) | 60% | ML classifier for offensive content |
| Negative reactions received | 25% | Others' rejection (emosent-py < -0.2) |
| Unique users who reacted negatively | 15% | Broad disapproval |

**Key Point**: Being sad is NOT toxic. Negative sentiment affects Vibe Score, not Toxicity. Toxicity is reserved for aggressive/offensive content.

---

### 6. POPULARITY (Social Gravity)

**Goal**: How much attention the user attracts.

**Components**:
| Signal | Weight | Description |
|--------|--------|-------------|
| Unique users who reacted | 25% | Audience breadth |
| Unique users who replied | 25% | Conversation starters |
| Total reactions received | 15% | Raw engagement |
| Total replies received | 15% | Discussion generation |
| Viral messages (4+ reactions) | 20% | Peak moments |

---

## Signal Overlap Strategy

Some signals intentionally appear in multiple metrics with different weights:

| Signal | Metrics Using It | Reasoning |
|--------|------------------|-----------|
| Positive reactions | Vibe (10%), Humor (45%), Popularity (15%) | Core positive feedback signal |
| Unique positive reactors | Humor (25%), Popularity (30%) | Audience breadth |
| Negative reactions | Toxicity only | Keep toxicity distinct |
| Negative sentiment | Vibe only | Sadness ≠ Toxicity |
| Messages sent | Activity only | Volume separate from reception |
| Active days | Presence only | Temporal patterns separate |

---

## Badges

### Positive Badges
| Badge | Metric | Threshold |
|-------|--------|-----------|
| Radiant | Vibe Score | >= 80 |
| Hyperactive | Activity | Top 10% |
| Regular | Presence | >= 80% active days |
| Comedian | Humor | >= 70 |
| Star | Popularity | Top 10% |
| Zen | Toxicity | < 2% |

### Negative Badges
| Badge | Metric | Threshold |
|-------|--------|-----------|
| Gloomy | Vibe Score | < 30 |
| Ghost | Activity | Bottom 10% |
| Tourist | Presence | < 20% active days, min 10 msgs |
| Deadpan | Humor | < 10 |
| Toxic | Toxicity | > 20% |
| Cricket | Popularity | Bottom 10%, min 30 msgs |

---

## Technical Notes

### Emoji Sentiment Classification
Use **emosent-py** library (MIT License) based on research by Kralj Novak et al. (2015). Provides sentiment scores for 751 emojis.

### Humor Classifier Filtering
The ml_humor analyzer should be filtered to exclude:
- Emoji-only messages
- Very short messages (< 3 words)

This ensures "intentional humor" counts actual jokes/funny content, not just emoji reactions.

### Presence: Consistency vs Bursts
To calculate activity variance:
1. Get daily message counts for the user in the window
2. Calculate standard deviation
3. Normalize to 0-1 scale
4. Invert: low variance = high presence score

### Weight Normalization
- All signals normalized to 0-1 range before applying weights
- Counts: `min(count / 90th_percentile, 1)`
- Percentages: already 0-1
- Final scores: 0-100 scale for display

### Bayesian Smoothing
Apply sample-size adjustment for fairness:
```
smoothed = (n × raw + k × global) / (n + k)
```
Where k=50 messages minimum for confidence.

---

## Files to Modify

- `apps/ml-processor/requirements.txt` - add emosent-py
- `apps/ml-processor/src/utils/emoji_sentiment.py` - new utility
- `apps/ml-processor/src/cards/calculators.py` - implement 6 calculators
- `apps/ml-processor/src/cards/generator.py` - update card generation
