"""
Pluggable stat calculators for card generation.

Each calculator is an independent function that queries data and returns
a stat result. To add/remove stats, just modify the CALCULATORS registry.

Design Principles:
1. Each calculator is self-contained with its own query
2. Calculators return None if insufficient data
3. Labels are computed from scores using simple thresholds
4. Easy to add new calculators without changing generator logic
"""

import logging
from dataclasses import dataclass
from datetime import datetime
from typing import Callable

from sqlalchemy import text
from sqlalchemy.engine import Engine

logger = logging.getLogger(__name__)


@dataclass
class StatResult:
    """Result from a stat calculator."""

    value: dict  # The stat data (flexible schema per stat type)


# Type alias for calculator functions
StatCalculator = Callable[[Engine, int, int, datetime, datetime], StatResult | None]


def _execute_single(engine: Engine, query: str, params: dict) -> dict | None:
    """Execute query expecting 0 or 1 row."""
    with engine.connect() as conn:
        result = conn.execute(text(query), params)
        row = result.mappings().fetchone()
        return dict(row) if row else None


def _execute_many(engine: Engine, query: str, params: dict) -> list[dict]:
    """Execute query expecting multiple rows."""
    with engine.connect() as conn:
        result = conn.execute(text(query), params)
        return [dict(row) for row in result.mappings()]


# =========================================
# LABEL FUNCTIONS
# =========================================


def _mood_label(score: float) -> str:
    """Convert mood score (0-100) to label."""
    if score >= 80:
        return "Radiante"
    elif score >= 65:
        return "Animado"
    elif score >= 50:
        return "Tranquilo"
    elif score >= 35:
        return "Reservado"
    else:
        return "Introspectivo"


def _volatility_label(score: float) -> str:
    """Convert volatility (0-1) to label."""
    if score < 0.15:
        return "Estavel"
    elif score < 0.30:
        return "Equilibrado"
    elif score < 0.50:
        return "Dinamico"
    else:
        return "Intenso"


def _toxicity_label(pct: float) -> str:
    """Convert toxicity percentage to label."""
    if pct < 2:
        return "Zen"
    elif pct < 5:
        return "Leve"
    elif pct < 10:
        return "Moderado"
    elif pct < 20:
        return "Picante"
    else:
        return "Explosivo"


def _chronotype_label(peak_hour: int) -> str:
    """Convert peak activity hour to chronotype label."""
    if 5 <= peak_hour < 9:
        return "Madrugador"
    elif 9 <= peak_hour < 12:
        return "Matutino"
    elif 12 <= peak_hour < 14:
        return "Almoceiro"
    elif 14 <= peak_hour < 18:
        return "Vespertino"
    elif 18 <= peak_hour < 22:
        return "Noturno"
    else:
        return "Coruja"


def _humor_label(score: float) -> str:
    """Convert humor score to label."""
    if score >= 0.7:
        return "Comediante"
    elif score >= 0.4:
        return "Engracado"
    elif score >= 0.2:
        return "Espirituoso"
    else:
        return "Serio"


# =========================================
# STAT CALCULATORS
# =========================================


def calculate_mood(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
) -> StatResult | None:
    """
    Calculate mood score from sentiment analysis.

    Mood is computed as: (positive * 100) + (neutral * 50) + (negative * 0)
    Returns score 0-100 with label.
    """
    query = """
        SELECT
            AVG(
                ms.score_positive * 100 +
                ms.score_neutral * 50 +
                ms.score_negative * 0
            ) as mood_score
        FROM ml_sentiment ms
        JOIN messages m ON ms.message_id = m.id
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
    """
    result = _execute_single(
        engine,
        query,
        {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end,
        },
    )

    if not result or result.get("mood_score") is None:
        return None

    score = round(float(result["mood_score"]), 1)
    return StatResult(value={"score": score, "label": _mood_label(score)})


def calculate_volatility(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
) -> StatResult | None:
    """
    Calculate mood volatility from sentiment variance.

    Volatility is the standard deviation of (positive - negative) scores.
    Returns 0-1 normalized score with label.
    """
    query = """
        SELECT
            STDDEV(ms.score_positive - ms.score_negative) as volatility
        FROM ml_sentiment ms
        JOIN messages m ON ms.message_id = m.id
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
    """
    result = _execute_single(
        engine,
        query,
        {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end,
        },
    )

    if not result or result.get("volatility") is None:
        return None

    # STDDEV of (-1 to 1) range gives ~0-0.5 typical values, normalize to 0-1
    raw = float(result["volatility"])
    score = round(min(raw, 1.0), 2)
    return StatResult(value={"score": score, "label": _volatility_label(score)})


def calculate_toxicity(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
) -> StatResult | None:
    """
    Calculate toxicity percentage from toxicity detection.

    Returns percentage of toxic messages with label.
    """
    query = """
        SELECT
            COUNT(*) as total,
            COUNT(*) FILTER (WHERE mt.is_toxic = true) as toxic_count
        FROM ml_toxicity mt
        JOIN messages m ON mt.message_id = m.id
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
    """
    result = _execute_single(
        engine,
        query,
        {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end,
        },
    )

    if not result or result.get("total", 0) == 0:
        return None

    total = int(result["total"])
    toxic = int(result.get("toxic_count") or 0)
    pct = round((toxic / total) * 100, 1) if total > 0 else 0

    return StatResult(value={"pct": pct, "label": _toxicity_label(pct)})


def calculate_activity(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
) -> StatResult | None:
    """
    Calculate activity metrics from messages.

    Returns message count, active days, and average message length.
    """
    query = """
        SELECT
            COUNT(*) as messages,
            COUNT(DISTINCT DATE(m.date)) as active_days,
            AVG(LENGTH(COALESCE(m.text, m.caption, ''))) as avg_length
        FROM messages m
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
    """
    result = _execute_single(
        engine,
        query,
        {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end,
        },
    )

    if not result or result.get("messages", 0) == 0:
        return None

    return StatResult(
        value={
            "messages": int(result["messages"]),
            "active_days": int(result["active_days"]),
            "avg_length": round(float(result["avg_length"] or 0), 1),
        }
    )


def calculate_reactions_received(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
) -> StatResult | None:
    """
    Calculate reactions received on user's messages.

    Note: message_reactions is denormalized (uses Telegram message_id),
    so we join on (chat_id, message_id) to match with messages table.
    """
    query = """
        SELECT COUNT(*) as reactions
        FROM message_reactions mr
        JOIN messages m ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
          AND (mr.is_removed = false OR mr.is_removed IS NULL)
    """
    result = _execute_single(
        engine,
        query,
        {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end,
        },
    )

    if not result:
        return None

    return StatResult(value=int(result.get("reactions") or 0))


def calculate_chronotype(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
) -> StatResult | None:
    """
    Calculate peak activity hour (chronotype).

    Returns the hour with most messages and a label.
    """
    query = """
        SELECT
            EXTRACT(HOUR FROM m.date) as hour,
            COUNT(*) as count
        FROM messages m
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
        GROUP BY EXTRACT(HOUR FROM m.date)
        ORDER BY count DESC
        LIMIT 1
    """
    result = _execute_single(
        engine,
        query,
        {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end,
        },
    )

    if not result or result.get("hour") is None:
        return None

    peak_hour = int(result["hour"])
    return StatResult(
        value={"peak_hour": peak_hour, "type": _chronotype_label(peak_hour)}
    )


def calculate_humor(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
) -> StatResult | None:
    """
    Calculate humor score from humor detection.

    Returns average humor score and humorous message percentage.
    """
    query = """
        SELECT
            AVG(mh.score) as avg_score,
            COUNT(*) FILTER (WHERE mh.is_humorous = true) as humorous_count,
            COUNT(*) as total
        FROM ml_humor mh
        JOIN messages m ON mh.message_id = m.id
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
    """
    result = _execute_single(
        engine,
        query,
        {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end,
        },
    )

    if not result or result.get("total", 0) == 0:
        return None

    score = round(float(result["avg_score"] or 0), 2)
    return StatResult(value={"score": score, "label": _humor_label(score)})


# =========================================
# CALCULATOR REGISTRY
# =========================================

# Registry of all stat calculators - add/remove entries to change what gets computed
CALCULATORS: dict[str, StatCalculator] = {
    "mood": calculate_mood,
    "volatility": calculate_volatility,
    "toxicity": calculate_toxicity,
    "activity": calculate_activity,
    "reactions_received": calculate_reactions_received,
    "chronotype": calculate_chronotype,
    "humor": calculate_humor,
}
