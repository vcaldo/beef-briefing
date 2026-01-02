# How Your Card Score Works

This document explains how your weekly stats card is calculated. For technical implementation details, see [CARD_GENERATION.md](../apps/ml-processor/CARD_GENERATION.md).

## What Are Cards?

Cards are weekly summaries of your chat activity. They capture your personality and engagement style based on your messages and how others interact with you.

- **Generated weekly**: Every Monday through Sunday
- **30-day analysis window**: Your stats reflect the last 30 days for stability
- **Minimum 10 messages**: You need at least 10 messages to receive a card

---

## The 6 Stats

### 1. Aura (0-100)

**What it measures:** Your emotional tone combined with how others receive your messages.

| Component | Weight | Description |
|-----------|--------|-------------|
| Positive sentiment | 55% | Messages with positive emotional tone |
| Neutral sentiment | 5% | Messages with neutral tone |
| Negative sentiment | -10% | Messages with negative tone (subtracts from score) |
| Consistency | 5% | How stable your emotional tone is |
| Positive reactions | 25% | Positive emoji reactions others give you |

**Labels:**
| Score | Label | Meaning |
|-------|-------|---------|
| 80+ | Radiante | Overwhelmingly positive, high engagement |
| 65-79 | Animado | Generally upbeat, well-received |
| 50-64 | Tranquilo | Balanced, neutral-leaning |
| 35-49 | Reservado | Reserved, slightly negative |
| <35 | Introspectivo | Predominantly negative or isolated |

---

### 2. Activity (0-100)

**What it measures:** Your overall engagement volume in the chat.

| Component | Weight | Description |
|-----------|--------|-------------|
| Messages sent | 35% | Total number of messages |
| Message length | 20% | Average length of your messages |
| Reactions sent | 25% | Emoji reactions you give to others |
| Replies sent | 20% | Your replies to other people's messages |

Your score is relative to other chat members. Being in the top 10% of any component gives you a high contribution from that component.

---

### 3. Presence (0-100)

**What it measures:** How consistently you show up in the chat over time.

| Component | Weight | Description |
|-----------|--------|-------------|
| Active days | 25% | What percentage of days you sent at least one message |
| Current streak | 40% | Consecutive days you've been active (ending today) |
| Hours spread | 25% | How many different hours of the day you're active |
| Daily consistency | 10% | How evenly distributed your messages are across days |

Streaks matter the most here. Being present every day builds your score significantly.

---

### 4. Humor (0-100)

**What it measures:** Your comedy impact and how much you make others laugh.

| Component | Weight | Description |
|-----------|--------|-------------|
| Positive reactions | 45% | Reactions with positive sentiment (laughing, hearts, etc.) |
| Unique reactors | 25% | How many different people react positively to you |
| Humorous messages | 15% | Messages our AI detected as humorous |
| Humorous replies | 15% | Funny replies others post in response to you |

This stat is harder to score high on. The global average is around 30, not 50.

---

### 5. Toxicity (0-100%)

**What it measures:** Aggressive or offensive content. This is NOT about being sad or negative in mood.

| Component | Weight | Description |
|-----------|--------|-------------|
| Toxic messages | 70% | Messages flagged by our AI as toxic/aggressive |
| Negative reactions | 25% | Negative emoji reactions you receive (thumbs down, angry, etc.) |
| Negative reactors | 5% | How many different people react negatively to you |

**Important:** Being sad or expressing negative emotions does NOT increase toxicity. That affects your Aura score instead. Toxicity is specifically about aggressive, offensive, or harmful content.

**Labels:**
| Percentage | Label | Meaning |
|------------|-------|---------|
| <2% | Zen | Almost never toxic |
| 2-4% | Leve | Rarely toxic |
| 5-9% | Moderado | Occasionally toxic |
| 10-19% | Picante | Frequently spicy |
| 20%+ | Explosivo | Often toxic |

---

### 6. Popularity (0-100)

**What it measures:** Your social reach and how much engagement your messages attract.

| Component | Weight | Description |
|-----------|--------|-------------|
| Unique reactors | 25% | How many different people react to your messages |
| Unique repliers | 25% | How many different people reply to you |
| Total reactions | 15% | Total reaction count on your messages |
| Total replies | 15% | Total replies to your messages |
| Viral messages | 20% | Messages that got 4 or more reactions |

Like Activity, your Popularity is measured relative to others in the chat.

---

## Your Overall Score (0-100)

Your Overall Score combines all 6 stats into a single number used for ranking.

### How It's Calculated

**Positive Contributions (up to 70 points):**
| Component | Weight |
|-----------|--------|
| Popularity | 20% |
| Presence | 15% |
| Aura | 12% |
| Days streak | 10% |
| Humor | 8% |
| Activity | 5% |

**Negative Penalties (up to 30 points deducted):**
| Component | Weight |
|-----------|--------|
| Toxicity | 12% |
| Negative reactions | 7% |
| Negative messages | 6% |
| Longest gap between posts | 5% |

**Trend Bonuses:**
- Improving trends add up to +5 points per stat
- Declining trends subtract up to -5 points per stat

**Badge Bonuses:**
- Legendary badges: +10 points each
- Epic badges: +7 points each
- Rare badges: +7 points each
- Common badges: +3 points each
- Negative badges: -5 points each

### Tier Labels

| Score | Tier |
|-------|------|
| 85+ | Legendary |
| 70-84 | Elite |
| 55-69 | Outstanding |
| 40-54 | Regular |
| 25-39 | Beginner |
| <25 | Rookie |

---

## Badges

Badges are special achievements displayed on your card. They affect your Overall Score.

### Positive Badges

| Badge | How to Earn | Rarity |
|-------|-------------|--------|
| Radiant | Aura score 80 or higher | Legendary |
| Hyperactive | Top 10% activity in the chat | Epic |
| Regular | Presence score 80 or higher | Epic |
| Comedian | Humor score 70 or higher | Legendary |
| Zen | Toxicity under 2% | Legendary |
| Star | Top 10% popularity in the chat | Legendary |

### Negative Badges

| Badge | What Triggers It | Rarity |
|-------|-----------------|--------|
| Gloomy | Aura score under 30 | Negative |
| Ghost | Bottom 10% activity in the chat | Negative |
| Tourist | Presence under 20 (with at least 10 messages) | Negative |
| Deadpan | Humor score under 10 | Negative |
| Toxic | Toxicity over 20% | Negative |
| Cricket | Bottom 10% popularity (with at least 30 messages) | Negative |

---

## Weekly Trends

Each stat shows how you changed compared to last week:

| Arrow | Meaning |
|-------|---------|
| ↑ | You improved this week |
| ↓ | You declined this week |
| → | No significant change |

Trends that show more than 10% change have a bigger impact on your Overall Score.

---

## Why New Users Start Near Average

If you've only sent a few messages, your scores will be closer to 50 (the average). This is intentional.

**Why?** Without this, someone who sends one amazing message could dominate the rankings over users with hundreds of consistent contributions.

As you send more messages, your scores become more reflective of your actual behavior:
- With 1 message: Your score is mostly pulled toward 50
- With 100 messages: Your score reflects about 67% of your actual performance
- With 500 messages: Your score reflects about 91% of your actual performance

This ensures that rankings are fair and stable, rewarding consistent participation over lucky outliers.
