# Card Generation: Implementation Guide

## Overview

User cards are collectible-style stat cards generated **weekly** for each user in a chat. The system uses a **30-day rolling window** for stable personality traits with **7-day trend indicators** for freshness.

### Design Principles

| Principle | Implementation |
|-----------|----------------|
| **Stable personality stats** | 30-day rolling window |
| **Fresh weekly release** | Cards generated every week |
| **Trend visibility** | 7-day vs previous 7-day comparison |
| **Simple architecture** | Single weekly batch job, no daily pre-aggregation |

### Why 30-Day Window?

| Window | Typical Messages | Statistical Quality |
|--------|------------------|---------------------|
| 7 days | 20-100 | High variance, noisy |
| 30 days | 100-500 | Stable, meaningful |

**Problem with 7 days:**
```
User A: Normally cheerful, had one bad day
├── 7-day mood:  45/100 (bad day = 30% weight)  ← Misleading
└── 30-day mood: 72/100 (bad day = 7% weight)   ← Accurate
```

### Stat Window Recommendations

| Stat Category | Window | Rationale |
|---------------|--------|-----------|
| Mood / Sentiment | 30 days | Single bad day doesn't dominate |
| Volatility | 30 days | Variance needs sample size |
| Vocabulary | 30 days | TTR needs ~200+ words |
| Topics | 30 days | Clustering needs critical mass |
| Toxicity | 30 days | Rare events need larger window |
| Chronotype | 30 days | True habits emerge over time |
| Influence | 30 days | Engagement patterns stabilize |
| Activity count | 7 days | Current engagement matters |
| Response time | 30 days | Need enough replies for median |

---

## Architecture

**All processing is batch/job-based. No live/real-time processing.**

```
┌─────────────────────────────────────────────────────────────┐
│  ML Processing Job (scheduled batch)                        │
│  • Runs on configurable interval (e.g., every 6 hours)      │
│  • Processes unprocessed messages in batches                │
│  • Stores: ml_sentiment, ml_toxicity, Qdrant embeddings     │
│                                                             │
│  Parameters:                                                │
│  --chat-id       Target chat to process                     │
│  --batch-size    Messages per batch (default: 500)          │
│  --limit         Max messages per run (default: unlimited)  │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ Processed data accumulates
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Card Generation Job (weekly batch)                         │
│  • Runs once before card release (e.g., Monday 2 AM)        │
│  • Queries processed data for last 30 days                  │
│  • Generates cards for all active users                     │
│                                                             │
│  Parameters:                                                │
│  --chat-id       Target chat to generate cards for          │
│  --week-end      Week end date (default: last Sunday)       │
│  --window-days   Stats window (default: 30)                 │
└─────────────────────────────────────────────────────────────┘
```

### Job Scheduling Options

| Job | Frequency | Trigger |
|-----|-----------|---------|
| ML Processing | Every 6 hours | Cron / APScheduler |
| Card Generation | Weekly (Monday) | Cron / APScheduler |

**Key insight:** No live processing, no daily aggregation. Two batch jobs only.

---

## Database Schema

```sql
-- Weekly user cards (generated once per week)
CREATE TABLE ml_user_cards (
    id SERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    chat_id BIGINT NOT NULL,

    -- Time windows
    week_start DATE NOT NULL,              -- Card release week (Monday)
    week_end DATE NOT NULL,                -- Card release week (Sunday)
    stats_window_start TIMESTAMPTZ NOT NULL,  -- 30 days before week_end
    stats_window_end TIMESTAMPTZ NOT NULL,    -- Same as week_end

    -- Computed stats (JSONB for flexibility)
    stats JSONB NOT NULL,         -- All computed stats in one object

    -- 7-day trends
    trends JSONB,                 -- {mood: +5, activity: -12, ...}

    -- Daily breakdown for sparklines (optional)
    daily_breakdown JSONB,        -- [{date, mood, messages}, ...]

    -- Metadata
    messages_analyzed INTEGER NOT NULL,
    card_version INTEGER DEFAULT 1,
    generated_at TIMESTAMPTZ DEFAULT NOW(),

    UNIQUE(user_id, chat_id, week_start)
);

CREATE INDEX idx_user_cards_chat_week ON ml_user_cards(chat_id, week_start DESC);
CREATE INDEX idx_user_cards_user ON ml_user_cards(user_id, chat_id);
```

---

## Card Generator Implementation

```python
# src/cards/generator.py
from datetime import datetime, timedelta
from dataclasses import dataclass
from typing import Optional
import logging

from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy import text

logger = logging.getLogger(__name__)


@dataclass
class CardConfig:
    """Configuration for card generation."""
    personality_window_days: int = 30  # Window for stable traits
    trend_window_days: int = 7         # Window for trend comparison
    min_messages_for_card: int = 10    # Minimum messages to generate card


class UserCardGenerator:
    """
    Generates weekly user stat cards using 30-day rolling window
    with 7-day trend indicators.
    """

    def __init__(self, session: AsyncSession, config: Optional[CardConfig] = None):
        self.session = session
        self.config = config or CardConfig()

    async def generate_cards_for_chat(
        self,
        chat_id: int,
        week_end: datetime
    ) -> list[dict]:
        """
        Generate cards for all users in a chat.

        Args:
            chat_id: The chat to generate cards for
            week_end: The end of the card week (usually Sunday)

        Returns:
            List of generated card data
        """
        week_start = week_end - timedelta(days=6)
        stats_window_start = week_end - timedelta(days=self.config.personality_window_days)

        # Get all active users in the 30-day window
        users = await self._get_active_users(chat_id, stats_window_start, week_end)

        logger.info(f"Generating cards for {len(users)} users in chat {chat_id}")

        cards = []
        for user_id in users:
            try:
                card = await self.generate_card_for_user(
                    user_id=user_id,
                    chat_id=chat_id,
                    week_start=week_start,
                    week_end=week_end,
                    stats_window_start=stats_window_start
                )
                if card:
                    cards.append(card)
            except Exception as e:
                logger.error(f"Failed to generate card for user {user_id}: {e}")

        return cards

    async def generate_card_for_user(
        self,
        user_id: int,
        chat_id: int,
        week_start: datetime,
        week_end: datetime,
        stats_window_start: datetime
    ) -> Optional[dict]:
        """Generate a single user's card."""

        # Count messages in window
        message_count = await self._count_user_messages(
            user_id, chat_id, stats_window_start, week_end
        )

        if message_count < self.config.min_messages_for_card:
            logger.debug(f"User {user_id} has insufficient messages ({message_count})")
            return None

        # Compute all stats into a single object
        stats = await self._compute_stats(
            user_id, chat_id, stats_window_start, week_end
        )

        # Compute 7-day trends
        trends = await self._compute_trends(
            user_id, chat_id, week_end
        )

        # Optional: daily breakdown for sparklines
        daily_breakdown = await self._compute_daily_breakdown(
            user_id, chat_id, stats_window_start, week_end
        )

        card = {
            "user_id": user_id,
            "chat_id": chat_id,
            "week_start": week_start.isoformat(),
            "week_end": week_end.isoformat(),
            "stats_window_start": stats_window_start.isoformat(),
            "stats_window_end": week_end.isoformat(),
            "stats": stats,
            "trends": trends,
            "daily_breakdown": daily_breakdown,
            "messages_analyzed": message_count,
            "generated_at": datetime.utcnow().isoformat()
        }

        # Save to database
        await self._save_card(card)

        return card

    async def _compute_stats(
        self,
        user_id: int,
        chat_id: int,
        window_start: datetime,
        window_end: datetime
    ) -> dict:
        """Compute all card stats."""

        # Mood - from ml_sentiment
        mood_query = text("""
            SELECT
                AVG(ms.score_positive * 100 + ms.score_neutral * 50) as mood_score,
                STDDEV(ms.score_positive - ms.score_negative) as sentiment_variance
            FROM ml_sentiment ms
            JOIN messages m ON ms.message_id = m.id
            WHERE m.from_user_id = :user_id
              AND m.chat_id = :chat_id
              AND m.created_at BETWEEN :window_start AND :window_end
        """)

        result = await self.session.execute(mood_query, {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end
        })
        mood_row = result.fetchone()

        mood_score = float(mood_row.mood_score or 50)
        volatility = float(mood_row.sentiment_variance or 0)

        # Toxicity - from ml_toxicity
        toxicity_query = text("""
            SELECT
                COUNT(*) FILTER (WHERE mt.is_toxic) as toxic_count,
                COUNT(*) as total_count
            FROM ml_toxicity mt
            JOIN messages m ON mt.message_id = m.id
            WHERE m.from_user_id = :user_id
              AND m.chat_id = :chat_id
              AND m.created_at BETWEEN :window_start AND :window_end
        """)

        result = await self.session.execute(toxicity_query, {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end
        })
        tox_row = result.fetchone()

        toxicity_pct = (
            (tox_row.toxic_count / tox_row.total_count * 100)
            if tox_row.total_count > 0 else 0
        )

        # Activity & engagement
        activity_query = text("""
            SELECT
                COUNT(*) as message_count,
                COUNT(DISTINCT DATE(created_at)) as active_days,
                AVG(LENGTH(COALESCE(text, caption, ''))) as avg_length
            FROM messages
            WHERE from_user_id = :user_id
              AND chat_id = :chat_id
              AND created_at BETWEEN :window_start AND :window_end
        """)

        result = await self.session.execute(activity_query, {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end
        })
        activity_row = result.fetchone()

        # Reactions received
        reactions_query = text("""
            SELECT COUNT(*) as reactions_received
            FROM message_reactions mr
            JOIN messages m ON mr.message_id = m.id
            WHERE m.from_user_id = :user_id
              AND m.chat_id = :chat_id
              AND m.created_at BETWEEN :window_start AND :window_end
        """)

        result = await self.session.execute(reactions_query, {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end
        })
        reactions_row = result.fetchone()

        return {
            "mood": {
                "score": round(mood_score, 1),
                "label": self._mood_label(mood_score)
            },
            "volatility": {
                "score": round(volatility, 3),
                "label": self._volatility_label(volatility)
            },
            "toxicity": {
                "pct": round(toxicity_pct, 2),
                "label": self._toxicity_label(toxicity_pct)
            },
            "activity": {
                "messages": activity_row.message_count,
                "active_days": activity_row.active_days,
                "avg_length": round(float(activity_row.avg_length or 0), 1)
            },
            "reactions_received": reactions_row.reactions_received
        }

    async def _compute_trends(
        self,
        user_id: int,
        chat_id: int,
        week_end: datetime
    ) -> dict:
        """
        Compute 7-day trends by comparing this week vs previous week.
        """
        this_week_start = week_end - timedelta(days=6)
        prev_week_start = this_week_start - timedelta(days=7)
        prev_week_end = this_week_start - timedelta(days=1)

        # This week's mood
        this_week_mood = await self._get_period_mood(
            user_id, chat_id, this_week_start, week_end
        )

        # Previous week's mood
        prev_week_mood = await self._get_period_mood(
            user_id, chat_id, prev_week_start, prev_week_end
        )

        # This week's activity
        this_week_activity = await self._get_period_activity(
            user_id, chat_id, this_week_start, week_end
        )

        # Previous week's activity
        prev_week_activity = await self._get_period_activity(
            user_id, chat_id, prev_week_start, prev_week_end
        )

        return {
            "mood": {
                "delta": round(this_week_mood - prev_week_mood, 1) if prev_week_mood else None,
                "direction": self._trend_direction(this_week_mood, prev_week_mood)
            },
            "activity": {
                "delta": this_week_activity - prev_week_activity if prev_week_activity else None,
                "direction": self._trend_direction(this_week_activity, prev_week_activity)
            }
        }

    async def _get_period_mood(
        self,
        user_id: int,
        chat_id: int,
        start: datetime,
        end: datetime
    ) -> float:
        """Get average mood for a period."""
        query = text("""
            SELECT AVG(ms.score_positive * 100 + ms.score_neutral * 50) as mood
            FROM ml_sentiment ms
            JOIN messages m ON ms.message_id = m.id
            WHERE m.from_user_id = :user_id
              AND m.chat_id = :chat_id
              AND m.created_at BETWEEN :start AND :end
        """)

        result = await self.session.execute(query, {
            "user_id": user_id,
            "chat_id": chat_id,
            "start": start,
            "end": end
        })
        row = result.fetchone()
        return float(row.mood or 50)

    async def _get_period_activity(
        self,
        user_id: int,
        chat_id: int,
        start: datetime,
        end: datetime
    ) -> int:
        """Get message count for a period."""
        query = text("""
            SELECT COUNT(*) as count
            FROM messages
            WHERE from_user_id = :user_id
              AND chat_id = :chat_id
              AND created_at BETWEEN :start AND :end
        """)

        result = await self.session.execute(query, {
            "user_id": user_id,
            "chat_id": chat_id,
            "start": start,
            "end": end
        })
        row = result.fetchone()
        return int(row.count or 0)

    async def _compute_daily_breakdown(
        self,
        user_id: int,
        chat_id: int,
        window_start: datetime,
        window_end: datetime
    ) -> list[dict]:
        """
        Compute daily stats for sparklines/mini-charts on cards.
        """
        query = text("""
            SELECT
                DATE(m.created_at) as day,
                COUNT(*) as messages,
                AVG(ms.score_positive * 100 + ms.score_neutral * 50) as mood
            FROM messages m
            LEFT JOIN ml_sentiment ms ON ms.message_id = m.id
            WHERE m.from_user_id = :user_id
              AND m.chat_id = :chat_id
              AND m.created_at BETWEEN :window_start AND :window_end
            GROUP BY DATE(m.created_at)
            ORDER BY day
        """)

        result = await self.session.execute(query, {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end
        })

        return [
            {
                "date": str(row.day),
                "messages": row.messages,
                "mood": round(float(row.mood or 50), 1)
            }
            for row in result.fetchall()
        ]

    # Label helper methods
    @staticmethod
    def _mood_label(score: float) -> str:
        if score >= 80: return "Radiant"
        if score >= 60: return "Cheerful"
        if score >= 40: return "Neutral"
        if score >= 20: return "Cloudy"
        return "Stormy"

    @staticmethod
    def _volatility_label(variance: float) -> str:
        if variance < 0.1: return "Steady"
        if variance < 0.2: return "Balanced"
        if variance < 0.3: return "Dynamic"
        return "Chaotic"

    @staticmethod
    def _toxicity_label(pct: float) -> str:
        if pct < 2: return "Wholesome"
        if pct < 5: return "Mild"
        if pct < 10: return "Spicy"
        return "Volcanic"

    @staticmethod
    def _trend_direction(current: float, previous: float) -> str:
        if previous is None or previous == 0:
            return "neutral"
        diff = current - previous
        if diff > 5:
            return "up"
        if diff < -5:
            return "down"
        return "stable"

    async def _get_active_users(self, chat_id, start, end) -> list[int]:
        """Get all users who sent messages in the window."""
        query = text("""
            SELECT DISTINCT from_user_id
            FROM messages
            WHERE chat_id = :chat_id
              AND created_at BETWEEN :start AND :end
              AND from_user_id IS NOT NULL
        """)
        result = await self.session.execute(query, {
            "chat_id": chat_id,
            "start": start,
            "end": end
        })
        return [row.from_user_id for row in result.fetchall()]

    async def _count_user_messages(self, user_id, chat_id, start, end) -> int:
        """Count user's messages in window."""
        query = text("""
            SELECT COUNT(*) as count
            FROM messages
            WHERE from_user_id = :user_id
              AND chat_id = :chat_id
              AND created_at BETWEEN :start AND :end
        """)
        result = await self.session.execute(query, {
            "user_id": user_id,
            "chat_id": chat_id,
            "start": start,
            "end": end
        })
        return result.fetchone().count

    async def _save_card(self, card: dict) -> None:
        """Save card to database."""
        query = text("""
            INSERT INTO ml_user_cards (
                user_id, chat_id, week_start, week_end,
                stats_window_start, stats_window_end,
                stats, trends, daily_breakdown, messages_analyzed
            ) VALUES (
                :user_id, :chat_id, :week_start, :week_end,
                :stats_window_start, :stats_window_end,
                :stats, :trends, :daily_breakdown, :messages_analyzed
            )
            ON CONFLICT (user_id, chat_id, week_start)
            DO UPDATE SET
                stats = EXCLUDED.stats,
                trends = EXCLUDED.trends,
                daily_breakdown = EXCLUDED.daily_breakdown,
                messages_analyzed = EXCLUDED.messages_analyzed,
                generated_at = NOW()
        """)

        import json
        await self.session.execute(query, {
            "user_id": card["user_id"],
            "chat_id": card["chat_id"],
            "week_start": card["week_start"],
            "week_end": card["week_end"],
            "stats_window_start": card["stats_window_start"],
            "stats_window_end": card["stats_window_end"],
            "stats": json.dumps(card["stats"]),
            "trends": json.dumps(card["trends"]),
            "daily_breakdown": json.dumps(card["daily_breakdown"]),
            "messages_analyzed": card["messages_analyzed"]
        })
        await self.session.commit()
```

---

## Configuration

### Environment Variables

```bash
# Database (can target local or production)
CARDS_DATABASE_URL=postgresql://user:pass@localhost:5432/beef

# Environment file selection
CARDS_ENV_FILE=.env        # Default, or .env.prod for production
```

### Targeting Production from Local

The card generator can run locally while targeting production database. This is useful for:
- Generating cards using local compute resources
- Testing card generation against real data
- Manual/ad-hoc card regeneration

**Example: Local-to-Production Config**

Create `ml-processor/.env.prod`:

```bash
# .env.prod - Target production from local machine

# Database: Connect directly to production PostgreSQL
CARDS_DATABASE_URL=postgresql://ml_processor:password@prod-db.example.com:5432/beef
```

**Usage:**

```bash
# Run with production config
CARDS_ENV_FILE=.env.prod python -m src.cards.main --chat-id -1003280306634

# Or export inline
CARDS_DATABASE_URL="postgresql://..." python -m src.cards.main --chat-id -1003280306634
```

**SSH Tunnel for Database Access:**

```bash
# Terminal 1: Create SSH tunnel
ssh -L 5433:localhost:5432 user@prod-server

# Terminal 2: Run card generator
CARDS_DATABASE_URL=postgresql://user:pass@localhost:5433/beef python -m src.cards.main --chat-id -1003280306634
```

| Target | Database URL |
|--------|--------------|
| Local dev | `postgresql://...@localhost:5432/beef` |
| Production (direct) | `postgresql://...@prod-db:5432/beef` |
| Production (tunnel) | `postgresql://...@localhost:5433/beef` |

### Database Module

```python
# src/database.py
import os
from contextlib import asynccontextmanager
from sqlalchemy.ext.asyncio import create_async_engine, AsyncSession
from sqlalchemy.orm import sessionmaker
from dotenv import load_dotenv

# Load environment file (supports CARDS_ENV_FILE override)
env_file = os.getenv("CARDS_ENV_FILE", ".env")
load_dotenv(env_file)

DATABASE_URL = os.getenv("CARDS_DATABASE_URL")
if not DATABASE_URL:
    raise ValueError("CARDS_DATABASE_URL environment variable is required")

# Convert to async URL if needed
if DATABASE_URL.startswith("postgresql://"):
    DATABASE_URL = DATABASE_URL.replace("postgresql://", "postgresql+asyncpg://")

engine = create_async_engine(DATABASE_URL, echo=False)
async_session = sessionmaker(engine, class_=AsyncSession, expire_on_commit=False)


@asynccontextmanager
async def get_session():
    async with async_session() as session:
        yield session
```

---

## Entry Point (CLI Job)

**The card generator runs as a batch job, not a daemon.**

```python
# src/cards/main.py
import asyncio
import argparse
from datetime import datetime, timedelta
import logging
import sys
import os

from src.database import get_session
from src.cards.generator import UserCardGenerator, CardConfig

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


async def generate_weekly_cards(
    chat_id: int,
    week_end: datetime = None,
    window_days: int = 30,
    min_messages: int = 10
) -> int:
    """
    Generate cards for a chat as a batch job.

    Args:
        chat_id: The chat to generate cards for
        week_end: End of the week (defaults to last Sunday)
        window_days: Stats calculation window in days (default: 30)
        min_messages: Minimum messages required for card generation (default: 10)

    Returns:
        Number of cards generated
    """
    if week_end is None:
        # Default to last Sunday
        today = datetime.utcnow().date()
        days_since_sunday = (today.weekday() + 1) % 7
        week_end = datetime.combine(
            today - timedelta(days=days_since_sunday),
            datetime.max.time()
        )

    config = CardConfig(
        personality_window_days=window_days,
        trend_window_days=7,
        min_messages_for_card=min_messages
    )

    logger.info(f"Generating cards for chat {chat_id}")
    logger.info(f"Week end: {week_end.date()}, Window: {window_days} days, Min messages: {min_messages}")

    async with get_session() as session:
        generator = UserCardGenerator(session, config)
        cards = await generator.generate_cards_for_chat(chat_id, week_end)

        logger.info(f"Generated {len(cards)} cards for chat {chat_id}")
        return len(cards)


def main():
    parser = argparse.ArgumentParser(
        description="Card Generator - Batch job for generating weekly user cards"
    )
    parser.add_argument(
        "--chat-id", type=int, required=True,
        help="Target chat ID to generate cards for"
    )
    parser.add_argument(
        "--week-end", type=str, default=None,
        help="Week end date in YYYY-MM-DD format (default: last Sunday)"
    )
    parser.add_argument(
        "--window-days", type=int, default=30,
        help="Stats calculation window in days (default: 30)"
    )
    parser.add_argument(
        "--min-messages", type=int, default=10,
        help="Minimum messages required for card generation (default: 10)"
    )

    args = parser.parse_args()

    week_end = None
    if args.week_end:
        week_end = datetime.fromisoformat(args.week_end)

    result = asyncio.run(generate_weekly_cards(
        chat_id=args.chat_id,
        week_end=week_end,
        window_days=args.window_days,
        min_messages=args.min_messages
    ))

    sys.exit(0 if result >= 0 else 1)


if __name__ == "__main__":
    main()
```

### Usage Examples

```bash
# Generate cards with defaults (30-day window, last Sunday)
python -m src.cards.main --chat-id -1003280306634

# Generate cards for a specific week
python -m src.cards.main --chat-id -1003280306634 --week-end 2025-01-19

# Generate with custom window (60 days for more stable stats)
python -m src.cards.main --chat-id -1003280306634 --window-days 60

# Lower threshold for small/new groups
python -m src.cards.main --chat-id -1003280306634 --min-messages 5

# Full custom run
python -m src.cards.main --chat-id -1003280306634 --week-end 2025-01-19 --window-days 45 --min-messages 15
```

---

## Card Output Example

```json
{
  "user_id": 123456789,
  "chat_id": -1003280306634,
  "week_start": "2025-01-13",
  "week_end": "2025-01-19",
  "stats_window_start": "2024-12-20",
  "stats_window_end": "2025-01-19",
  "stats": {
    "mood": {"score": 72.5, "label": "Cheerful"},
    "volatility": {"score": 0.18, "label": "Balanced"},
    "toxicity": {"pct": 3.2, "label": "Mild"},
    "activity": {"messages": 234, "active_days": 25, "avg_length": 67.3},
    "reactions_received": 89,
    "chronotype": {"type": "Night Owl", "peak_hour": 23},
    "conversation_style": {"type": "Initiator", "starter_pct": 68.5},
    "influence": {"score": 78.5, "label": "Respected"},
    "topics": [
      {"name": "Tech", "confidence": 0.45},
      {"name": "Gaming", "confidence": 0.30}
    ],
    "humor": {"score": 45.2, "label": "Witty"},
    "interests": {"top_orgs": ["Google", "OpenAI"]}
  },
  "trends": {
    "mood": {"delta": 5.2, "direction": "up"},
    "activity": {"delta": -12, "direction": "down"}
  },
  "daily_breakdown": [
    {"date": "2024-12-20", "messages": 8, "mood": 71.2},
    {"date": "2024-12-21", "messages": 12, "mood": 75.0}
  ],
  "messages_analyzed": 234,
  "generated_at": "2025-01-20T00:15:00Z"
}
```

---

## Visual Card Layout (Reference)

```
┌─────────────────────────────────────┐
│  @username              [Week 3]    │
│  ─────────────────────────────────  │
│                                     │
│  MOOD: Cheerful (72) ↑+5           │  ← 30-day score + 7-day trend
│  ████████████░░░░░░░░              │  ← Sparkline from daily_breakdown
│                                     │
│  ⚔️ ATK: 45    🛡️ DEF: 78          │  ← Derived from toxicity/support
│  🧠 INT: 82    ✨ CHA: 89          │  ← Vocabulary/reactions
│                                     │
│  Style: Night Owl 🦉               │  ← Chronotype
│  Topics: Tech, Gaming              │  ← Top 2 topics
│                                     │
│  📊 234 msgs | 25 active days      │
└─────────────────────────────────────┘
```

---

## Scheduling

**All processing runs as batch jobs. No daemon/continuous process.**

### Option 1: Cron (Recommended for Production)

```bash
# ML Processing: Every 6 hours
0 */6 * * * cd /app && python -m src.main process --chat-id -1003280306634 --batch-size 500

# Card Generation: Every Monday at 2 AM
0 2 * * 1 cd /app && python -m src.cards.main --chat-id -1003280306634
```

### Option 2: APScheduler (In-Process)

```python
# src/scheduler.py
from apscheduler.schedulers.blocking import BlockingScheduler
from apscheduler.triggers.cron import CronTrigger
from apscheduler.triggers.interval import IntervalTrigger

scheduler = BlockingScheduler()

# ML Processing every 6 hours
scheduler.add_job(
    run_ml_processing,
    trigger=IntervalTrigger(hours=6),
    args=[chat_id, 500],  # chat_id, batch_size
    id="ml_processing",
    name="Process unanalyzed messages"
)

# Card generation every Monday at 2 AM
scheduler.add_job(
    generate_weekly_cards,
    trigger=CronTrigger(day_of_week='mon', hour=2),
    args=[chat_id],
    id="weekly_cards",
    name="Generate weekly user cards"
)

scheduler.start()
```

### Option 3: Manual/One-Off Runs

```bash
# Process all unprocessed messages for a chat
python -m src.main process --chat-id -1003280306634

# Process with limits (for testing)
python -m src.main process --chat-id -1003280306634 --limit 100 --batch-size 50

# Generate cards for a specific week
python -m src.cards.main --chat-id -1003280306634 --week-end 2025-01-19

# Generate cards with custom window
python -m src.cards.main --chat-id -1003280306634 --window-days 60
```

### Job Parameters Reference

| Parameter | ML Processing | Card Generation | Description |
|-----------|---------------|-----------------|-------------|
| `--chat-id` | Required | Required | Target Telegram chat ID |
| `--batch-size` | Optional (500) | N/A | Messages per batch |
| `--limit` | Optional (∞) | N/A | Max messages to process |
| `--week-end` | N/A | Optional (last Sunday) | Card week end date |
| `--window-days` | N/A | Optional (30) | Stats calculation window |

---

## Related Documentation

For detailed stat implementations, see:
- `docs/plans/user-cards-green-stats.md` - Green tier (no new models)
- `docs/plans/user-cards-yellow-stats.md` - Yellow tier (aggregations + embeddings)
- `docs/plans/user-cards-red-stats.md` - Red tier (new ML models)

For API endpoints that serve cards:
- `local_docs/USER_CARDS_API_IMPLEMENTATION.md` - REST API spec

---

## Summary

This card generation system provides:

1. **Stable stats**: 30-day rolling window for personality traits
2. **Fresh releases**: Weekly card generation
3. **Trend indicators**: 7-day vs previous 7-day comparison
4. **Simple architecture**: Single weekly batch job, no daily pre-aggregation
5. **Flexible schema**: JSONB stats for easy extension

---

**Last Updated**: 2025-12-17
**Status**: Implementation Guide
