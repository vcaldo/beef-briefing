"""
Pluggable stat calculators for card generation.

Each calculator is an independent function that queries data and returns
a stat result. To add/remove stats, just modify the CALCULATORS registry.

Design Principles:
1. Each calculator is self-contained with its own query
2. Calculators return None if insufficient data
3. Labels are computed from scores using simple thresholds
4. Progressive Bayesian smoothing: k decays from 30 to 5 as samples grow
5. All rankings are per-chat
"""

import logging
from dataclasses import dataclass
from datetime import date, datetime, timedelta
from typing import Callable

from sqlalchemy import text
from sqlalchemy.engine import Engine

from src.utils.emoji_sentiment import is_positive_reaction, is_negative_reaction

logger = logging.getLogger(__name__)

# Bayesian smoothing parameters for progressive k
BAYESIAN_K_MAX = 30  # Max k for very few samples (1-5 messages)
BAYESIAN_K_MIN = 5   # Min k for many samples (100+ messages)
BAYESIAN_K_HALF_LIFE = 20  # Messages needed to halve the difference from k_min

# Default global means for Bayesian smoothing (when chat has insufficient data)
DEFAULT_GLOBAL_MEANS = {
    "aura": 55.0,  # Slightly positive baseline
    "activity": 50.0,  # Average activity
    "presence": 50.0,  # Average presence
    "humor": 30.0,  # Most people aren't comedians
    "toxicity": 5.0,  # Low baseline toxicity
    "popularity": 30.0,  # Average popularity
}


@dataclass
class StatResult:
    """Result from a stat calculator."""

    value: dict  # The stat data (flexible schema per stat type)


# Type alias for calculator functions
StatCalculator = Callable[..., StatResult | None]


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
# UTILITY FUNCTIONS
# =========================================


def _get_progressive_k(n_samples: int) -> float:
    """
    Calculate progressive k that decays exponentially as sample size grows.

    - k_max (30): Maximum k for very few samples (1-5 messages)
    - k_min (5): Minimum k for many samples (100+ messages)
    - half_life (20): Messages needed to halve the difference from k_min

    This allows users with more data to reach their true scores faster,
    while still protecting against outliers for low-message users.
    """
    return BAYESIAN_K_MIN + (BAYESIAN_K_MAX - BAYESIAN_K_MIN) * (
        0.5 ** (n_samples / BAYESIAN_K_HALF_LIFE)
    )


def _bayesian_smooth(raw_score: float, n_samples: int, global_mean: float) -> float:
    """
    Apply Bayesian smoothing to adjust for sample size.

    Formula: smoothed = (n * raw + k * global) / (n + k)

    Uses progressive k: with few samples, k is high and the score regresses
    toward the global mean. With many samples, k decreases and the raw score
    dominates.
    """
    k = _get_progressive_k(n_samples)
    return (n_samples * raw_score + k * global_mean) / (n_samples + k)


def _get_chat_p90(engine: Engine, chat_id: int, column_query: str, params: dict) -> float:
    """
    Get 90th percentile for a metric within a chat.

    Used to normalize counts to 0-1 range.
    """
    query = f"""
        SELECT PERCENTILE_CONT(0.9) WITHIN GROUP (ORDER BY val) as p90
        FROM ({column_query}) sub
    """
    result = _execute_single(engine, query, params)
    return float(result.get("p90") or 1) if result else 1.0


def _normalize_count(count: int, p90: float) -> float:
    """Normalize count to 0-1 using 90th percentile cap."""
    if p90 <= 0:
        return 0.0
    return min(count / p90, 1.0)


def _clamp(value: float, min_val: float = 0.0, max_val: float = 100.0) -> float:
    """Clamp value to range."""
    return max(min_val, min(max_val, value))


# =========================================
# LABEL FUNCTIONS
# =========================================


def _aura_label(score: float) -> str:
    """Convert aura score (0-100) to label."""
    if score >= 80:
        return "Radiant"
    elif score >= 65:
        return "Lively"
    elif score >= 50:
        return "Chill"
    elif score >= 35:
        return "Reserved"
    else:
        return "Introspective"


def _presence_label(score: float) -> str:
    """Convert presence score (0-100) to label."""
    if score >= 80:
        return "Regular"
    elif score >= 60:
        return "Frequent"
    elif score >= 40:
        return "Occasional"
    elif score >= 20:
        return "Sporadic"
    else:
        return "Ghost"


def _toxicity_label(pct: float) -> str:
    """Convert toxicity percentage to label."""
    if pct < 2:
        return "Zen"
    elif pct < 5:
        return "Mild"
    elif pct < 10:
        return "Moderate"
    elif pct < 20:
        return "Spicy"
    else:
        return "Explosive"


def _humor_label(score: float) -> str:
    """Convert humor score (0-100) to label."""
    if score >= 70:
        return "Comedian"
    elif score >= 50:
        return "Funny"
    elif score >= 30:
        return "Witty"
    elif score >= 10:
        return "Low-key"
    else:
        return "Serious"


def _activity_label(score: float) -> str:
    """Convert activity score (0-100) to label."""
    if score >= 80:
        return "Hyperactive"
    elif score >= 60:
        return "Active"
    elif score >= 40:
        return "Moderate"
    elif score >= 20:
        return "Casual"
    else:
        return "Quiet"


def _popularity_label(score: float) -> str:
    """Convert popularity score (0-100) to label."""
    if score >= 80:
        return "Star"
    elif score >= 60:
        return "Popular"
    elif score >= 40:
        return "Known"
    elif score >= 20:
        return "Low-key"
    else:
        return "Reserved"


# Default tiers (used when tiers not provided via config)
DEFAULT_TIERS: list[tuple[str, int]] = [
    ("Legendary", 81),
    ("Elite", 77),
    ("Outstanding", 72),
    ("Regular", 55),
    ("Beginner", 32),
    ("Rookie", 10),
]


def _overall_label(score: float, tiers: list[tuple[str, int]] | None = None) -> str:
    """Convert overall score (0-100) to label using configured tiers."""
    if tiers is None:
        tiers = DEFAULT_TIERS
    for name, min_score in tiers:
        if score >= min_score:
            return name
    return tiers[-1][0] if tiers else "Unknown"


# =========================================
# STAT CALCULATORS
# =========================================


def calculate_aura(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
    timezone: str | None = None,
) -> StatResult | None:
    """
    Calculate Aura Score (0-100) combining sentiment and social reception.

    Components:
    - 35% positive message ratio
    - 5% neutral message ratio
    - -30% negative message ratio (subtracts)
    - 5% sentiment consistency (1 - stddev)
    - 25% positive reactions received (emoji sentiment > 0.2)
    """
    # Query sentiment stats
    sentiment_query = """
        SELECT
            COUNT(*) as total_msgs,
            COUNT(*) FILTER (WHERE ms.label = 'positive') as positive_count,
            COUNT(*) FILTER (WHERE ms.label = 'neutral') as neutral_count,
            COUNT(*) FILTER (WHERE ms.label = 'negative') as negative_count,
            COALESCE(STDDEV(ms.score_positive - ms.score_negative), 0) as sentiment_stddev
        FROM ml_sentiment ms
        JOIN messages m ON ms.message_id = m.id
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
    """
    sentiment_result = _execute_single(
        engine,
        sentiment_query,
        {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end,
        },
    )

    if not sentiment_result or sentiment_result.get("total_msgs", 0) == 0:
        return None

    total_msgs = int(sentiment_result["total_msgs"])
    positive_count = int(sentiment_result.get("positive_count") or 0)
    neutral_count = int(sentiment_result.get("neutral_count") or 0)
    negative_count = int(sentiment_result.get("negative_count") or 0)
    sentiment_stddev = float(sentiment_result.get("sentiment_stddev") or 0)

    # Query reactions received
    reactions_query = """
        SELECT mr.emoji_value
        FROM message_reactions mr
        JOIN messages m ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
          AND mr.emoji_value IS NOT NULL
          AND (mr.is_removed = false OR mr.is_removed IS NULL)
    """
    reactions_result = _execute_many(
        engine,
        reactions_query,
        {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end,
        },
    )

    # Classify reactions using emoji sentiment
    total_reactions = len(reactions_result)
    positive_reactions = sum(
        1 for r in reactions_result if is_positive_reaction(r["emoji_value"])
    )

    # Calculate ratios
    positive_ratio = positive_count / total_msgs if total_msgs > 0 else 0
    neutral_ratio = neutral_count / total_msgs if total_msgs > 0 else 0
    negative_ratio = negative_count / total_msgs if total_msgs > 0 else 0

    # Consistency: lower stddev = more consistent = higher score
    # STDDEV of (-1 to 1) range gives ~0-0.5 typical, normalize to 0-1
    consistency = max(0, 1 - min(sentiment_stddev * 2, 1))

    # Positive reactions ratio
    positive_reactions_ratio = (
        positive_reactions / total_reactions if total_reactions > 0 else 0.5
    )

    # Calculate weighted aura score
    # Formula: 55*pos + 5*neutral - 5*neg + 5*consistency + 30*pos_reactions
    raw_score = (
        55 * positive_ratio
        + 5 * neutral_ratio
        - 5 * negative_ratio
        + 5 * consistency
        + 30 * positive_reactions_ratio
    )

    # Scale to 0-100 (max possible is 55+5+5+30=95, min is -5)
    # Normalize: (-5 to 95) -> (0 to 100)
    scaled_score = ((raw_score + 5) / 100) * 100
    scaled_score = _clamp(scaled_score, 0, 100)

    # Apply Bayesian smoothing
    global_mean = DEFAULT_GLOBAL_MEANS["aura"]
    smoothed_score = _bayesian_smooth(scaled_score, total_msgs, global_mean)
    final_score = round(_clamp(smoothed_score, 0, 100), 1)

    return StatResult(
        value={
            "score": final_score,
            "label": _aura_label(final_score),
            "positive_ratio": round(positive_ratio * 100, 1),
            "negative_ratio": round(negative_ratio * 100, 1),
            "positive_reactions": positive_reactions,
            "total_reactions": total_reactions,
        }
    )


def calculate_activity(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
    timezone: str | None = None,
) -> StatResult | None:
    """
    Calculate Activity Score (0-100) measuring contribution volume.

    Components:
    - 35% messages sent (normalized to chat p90)
    - 20% average message length (normalized to chat p90)
    - 25% reactions sent to others (normalized to chat p90)
    - 20% replies sent to others (normalized to chat p90)
    """
    base_params = {
        "user_id": user_id,
        "chat_id": chat_id,
        "window_start": window_start,
        "window_end": window_end,
    }

    # Get user's raw stats
    user_query = """
        SELECT
            COUNT(*) as messages_sent,
            AVG(LENGTH(COALESCE(m.text, m.caption, ''))) as avg_length
        FROM messages m
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
    """
    user_result = _execute_single(engine, user_query, base_params)

    if not user_result or user_result.get("messages_sent", 0) == 0:
        return None

    messages_sent = int(user_result["messages_sent"])
    avg_length = float(user_result.get("avg_length") or 0)

    # Get reactions sent (user reacting to others' messages)
    reactions_sent_query = """
        SELECT COUNT(*) as reactions_sent
        FROM message_reactions mr
        WHERE mr.user_id = :user_id
          AND mr.chat_id = :chat_id
          AND mr.date >= :window_start
          AND mr.date <= :window_end
          AND (mr.is_removed = false OR mr.is_removed IS NULL)
    """
    reactions_result = _execute_single(engine, reactions_sent_query, base_params)
    reactions_sent = int(reactions_result.get("reactions_sent") or 0) if reactions_result else 0

    # Get replies sent to others (excluding self-replies)
    replies_sent_query = """
        SELECT COUNT(*) as replies_sent
        FROM messages m
        JOIN messages parent ON m.reply_to_message_id = parent.message_id
                             AND m.chat_id = parent.chat_id
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
          AND parent.user_id != m.user_id
    """
    replies_result = _execute_single(engine, replies_sent_query, base_params)
    replies_sent = int(replies_result.get("replies_sent") or 0) if replies_result else 0

    # Get chat-wide p90 for normalization
    chat_params = {
        "chat_id": chat_id,
        "window_start": window_start,
        "window_end": window_end,
    }

    # P90 for messages
    messages_p90_query = """
        SELECT COUNT(*) as val
        FROM messages m
        WHERE m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
        GROUP BY m.user_id
    """
    messages_p90 = _get_chat_p90(engine, chat_id, messages_p90_query, chat_params)

    # P90 for avg_length
    length_p90_query = """
        SELECT AVG(LENGTH(COALESCE(m.text, m.caption, ''))) as val
        FROM messages m
        WHERE m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
        GROUP BY m.user_id
    """
    length_p90 = _get_chat_p90(engine, chat_id, length_p90_query, chat_params)

    # P90 for reactions sent
    reactions_p90_query = """
        SELECT COUNT(*) as val
        FROM message_reactions mr
        WHERE mr.chat_id = :chat_id
          AND mr.date >= :window_start
          AND mr.date <= :window_end
          AND (mr.is_removed = false OR mr.is_removed IS NULL)
        GROUP BY mr.user_id
    """
    reactions_p90 = _get_chat_p90(engine, chat_id, reactions_p90_query, chat_params)

    # P90 for replies sent
    replies_p90_query = """
        SELECT COUNT(*) as val
        FROM messages m
        JOIN messages parent ON m.reply_to_message_id = parent.message_id
                             AND m.chat_id = parent.chat_id
        WHERE m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
          AND parent.user_id != m.user_id
        GROUP BY m.user_id
    """
    replies_p90 = _get_chat_p90(engine, chat_id, replies_p90_query, chat_params)

    # Normalize each component to 0-1
    norm_messages = _normalize_count(messages_sent, messages_p90)
    norm_length = _normalize_count(int(avg_length), length_p90) if length_p90 > 0 else 0
    norm_reactions = _normalize_count(reactions_sent, reactions_p90)
    norm_replies = _normalize_count(replies_sent, replies_p90)

    # Calculate weighted score (0-1)
    raw_score = (
        0.35 * norm_messages
        + 0.20 * norm_length
        + 0.25 * norm_reactions
        + 0.20 * norm_replies
    )

    # Scale to 0-100
    scaled_score = raw_score * 100

    # Apply Bayesian smoothing
    global_mean = DEFAULT_GLOBAL_MEANS["activity"]
    smoothed_score = _bayesian_smooth(scaled_score, messages_sent, global_mean)
    final_score = round(_clamp(smoothed_score, 0, 100), 1)

    return StatResult(
        value={
            "score": final_score,
            "label": _activity_label(final_score),
            "messages": messages_sent,
            "avg_length": round(avg_length, 1),
            "reactions_sent": reactions_sent,
            "replies_sent": replies_sent,
        }
    )


def calculate_presence(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
    timezone: str | None = None,
) -> StatResult | None:
    """
    Calculate Presence Score (0-100) measuring consistency vs bursts.

    Components:
    - 25% active days ratio (days_active / total_days)
    - 40% current streak (consecutive days ending on window_end)
    - 25% hours spread (unique hours / 24)
    - 10% activity variance (1 - normalized_stddev, lower variance = higher score)
    """
    tz = timezone or "UTC"
    base_params = {
        "user_id": user_id,
        "chat_id": chat_id,
        "window_start": window_start,
        "window_end": window_end,
        "timezone": tz,
    }

    # Get daily activity counts and unique hours
    daily_query = """
        SELECT
            DATE(m.date AT TIME ZONE :timezone) as activity_date,
            COUNT(*) as daily_count
        FROM messages m
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
        GROUP BY DATE(m.date AT TIME ZONE :timezone)
        ORDER BY activity_date
    """
    daily_results = _execute_many(engine, daily_query, base_params)

    if not daily_results:
        return None

    # Get unique hours spread
    hours_query = """
        SELECT COUNT(DISTINCT EXTRACT(HOUR FROM m.date AT TIME ZONE :timezone)) as unique_hours
        FROM messages m
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
    """
    hours_result = _execute_single(engine, hours_query, base_params)
    unique_hours = int(hours_result.get("unique_hours") or 0) if hours_result else 0

    # Calculate metrics
    active_dates = [r["activity_date"] for r in daily_results]
    daily_counts = [r["daily_count"] for r in daily_results]
    active_days = len(active_dates)

    # Total days in window
    total_days = (window_end.date() - window_start.date()).days + 1

    # Active days ratio
    active_days_ratio = active_days / total_days if total_days > 0 else 0

    # Hours spread ratio
    hours_spread = unique_hours / 24

    # Calculate streak (consecutive days ending on window_end)
    window_end_date = window_end.date()
    streak = 0
    if active_dates:
        # Convert to date objects if needed
        active_date_set = set()
        for d in active_dates:
            if isinstance(d, datetime):
                active_date_set.add(d.date())
            elif isinstance(d, date):
                active_date_set.add(d)
            else:
                active_date_set.add(d)

        # Count consecutive days from window_end backwards
        check_date = window_end_date
        while check_date in active_date_set and check_date >= window_start.date():
            streak += 1
            check_date -= timedelta(days=1)

    # Normalize streak (max meaningful streak is window_days)
    max_streak = total_days
    streak_ratio = streak / max_streak if max_streak > 0 else 0

    # Calculate activity variance (stddev of daily counts)
    if len(daily_counts) > 1:
        mean_count = sum(daily_counts) / len(daily_counts)
        variance = sum((c - mean_count) ** 2 for c in daily_counts) / len(daily_counts)
        stddev = variance ** 0.5
        # Normalize stddev (typical range 0-50), invert so low variance = high score
        normalized_stddev = min(stddev / 50, 1)
        variance_score = 1 - normalized_stddev
    else:
        variance_score = 1.0  # Single day = consistent

    # Calculate weighted score
    raw_score = (
        0.25 * active_days_ratio
        + 0.40 * streak_ratio
        + 0.25 * hours_spread
        + 0.10 * variance_score
    )

    # Scale to 0-100
    scaled_score = raw_score * 100

    # Apply Bayesian smoothing
    total_messages = sum(daily_counts)
    global_mean = DEFAULT_GLOBAL_MEANS["presence"]
    smoothed_score = _bayesian_smooth(scaled_score, total_messages, global_mean)
    final_score = round(_clamp(smoothed_score, 0, 100), 1)

    return StatResult(
        value={
            "score": final_score,
            "label": _presence_label(final_score),
            "active_days": active_days,
            "total_days": total_days,
            "streak": streak,
            "unique_hours": unique_hours,
        }
    )


def calculate_humor(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
    timezone: str | None = None,
) -> StatResult | None:
    """
    Calculate Humor Score (0-100) measuring comedy impact.

    Components:
    - 45% positive reactions received (emoji sentiment > 0.2)
    - 25% unique users who reacted positively
    - 15% messages classified as humorous (ml_humor, excluding emoji-only)
    - 15% humorous replies received
    """
    base_params = {
        "user_id": user_id,
        "chat_id": chat_id,
        "window_start": window_start,
        "window_end": window_end,
    }

    # Get all reactions on user's messages
    reactions_query = """
        SELECT mr.emoji_value, mr.user_id as reactor_id
        FROM message_reactions mr
        JOIN messages m ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
          AND mr.emoji_value IS NOT NULL
          AND (mr.is_removed = false OR mr.is_removed IS NULL)
    """
    reactions = _execute_many(engine, reactions_query, base_params)

    # Classify reactions
    total_reactions = len(reactions)
    positive_reactions = sum(
        1 for r in reactions if is_positive_reaction(r["emoji_value"])
    )
    unique_positive_reactors = len(set(
        r["reactor_id"] for r in reactions if is_positive_reaction(r["emoji_value"])
    ))

    # Get total unique users in chat for normalization
    chat_users_query = """
        SELECT COUNT(DISTINCT user_id) as total_users
        FROM messages m
        WHERE m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
          AND m.user_id IS NOT NULL
    """
    chat_users_result = _execute_single(
        engine, chat_users_query,
        {"chat_id": chat_id, "window_start": window_start, "window_end": window_end}
    )
    total_chat_users = int(chat_users_result.get("total_users") or 1) if chat_users_result else 1

    # Get ML humor stats (excluding emoji-only messages)
    humor_query = """
        SELECT
            COUNT(*) as total_analyzed,
            COUNT(*) FILTER (WHERE mh.is_humorous = true) as humorous_count
        FROM ml_humor mh
        JOIN messages m ON mh.message_id = m.id
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
          AND LENGTH(REGEXP_REPLACE(COALESCE(m.text, m.caption, ''), '[^\\w\\s]', '', 'g')) > 0
    """
    humor_result = _execute_single(engine, humor_query, base_params)

    total_analyzed = int(humor_result.get("total_analyzed") or 0) if humor_result else 0
    humorous_count = int(humor_result.get("humorous_count") or 0) if humor_result else 0

    # Get humorous replies received (replies to user's messages that are marked humorous)
    humor_replies_query = """
        SELECT COUNT(*) as humor_replies
        FROM messages reply
        JOIN messages parent ON reply.reply_to_message_id = parent.message_id
                             AND reply.chat_id = parent.chat_id
        JOIN ml_humor mh ON mh.message_id = reply.id AND mh.is_humorous = true
        WHERE parent.user_id = :user_id
          AND parent.chat_id = :chat_id
          AND parent.date >= :window_start
          AND parent.date <= :window_end
          AND reply.user_id != :user_id
    """
    humor_replies_result = _execute_single(engine, humor_replies_query, base_params)
    humor_replies = int(humor_replies_result.get("humor_replies") or 0) if humor_replies_result else 0

    # Get total replies received for normalization
    total_replies_query = """
        SELECT COUNT(*) as total_replies
        FROM messages reply
        JOIN messages parent ON reply.reply_to_message_id = parent.message_id
                             AND reply.chat_id = parent.chat_id
        WHERE parent.user_id = :user_id
          AND parent.chat_id = :chat_id
          AND parent.date >= :window_start
          AND parent.date <= :window_end
          AND reply.user_id != :user_id
    """
    total_replies_result = _execute_single(engine, total_replies_query, base_params)
    total_replies = int(total_replies_result.get("total_replies") or 0) if total_replies_result else 0

    # Need some data to compute humor
    if total_reactions == 0 and total_analyzed == 0:
        return None

    # Calculate ratios
    positive_reactions_ratio = (
        positive_reactions / total_reactions if total_reactions > 0 else 0
    )
    unique_reactors_ratio = (
        unique_positive_reactors / total_chat_users if total_chat_users > 1 else 0
    )
    humorous_ratio = (
        humorous_count / total_analyzed if total_analyzed > 0 else 0
    )
    humor_replies_ratio = (
        humor_replies / total_replies if total_replies > 0 else 0
    )

    # Calculate weighted score
    raw_score = (
        0.45 * positive_reactions_ratio
        + 0.25 * unique_reactors_ratio
        + 0.15 * humorous_ratio
        + 0.15 * humor_replies_ratio
    )

    # Scale to 0-100
    scaled_score = raw_score * 100

    # Apply Bayesian smoothing
    n_samples = max(total_reactions, total_analyzed)
    global_mean = DEFAULT_GLOBAL_MEANS["humor"]
    smoothed_score = _bayesian_smooth(scaled_score, n_samples, global_mean)
    final_score = round(_clamp(smoothed_score, 0, 100), 1)

    return StatResult(
        value={
            "score": final_score,
            "label": _humor_label(final_score),
            "positive_reactions": positive_reactions,
            "unique_positive_reactors": unique_positive_reactors,
            "humorous_messages": humorous_count,
            "humor_replies": humor_replies,
        }
    )


def calculate_toxicity(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
    timezone: str | None = None,
) -> StatResult | None:
    """
    Calculate Toxicity Score (0-100 percentage) measuring negative impact.

    Components:
    - 60% toxic messages (ml_toxicity.is_toxic = true)
    - 25% negative reactions received (emoji sentiment < -0.2)
    - 15% unique users who reacted negatively
    """
    base_params = {
        "user_id": user_id,
        "chat_id": chat_id,
        "window_start": window_start,
        "window_end": window_end,
    }

    # Query ML toxicity stats
    toxicity_query = """
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
    toxicity_result = _execute_single(engine, toxicity_query, base_params)

    if not toxicity_result or toxicity_result.get("total", 0) == 0:
        return None

    total_analyzed = int(toxicity_result["total"])
    toxic_count = int(toxicity_result.get("toxic_count") or 0)

    # Query reactions received
    reactions_query = """
        SELECT mr.emoji_value, mr.user_id as reactor_id
        FROM message_reactions mr
        JOIN messages m ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
          AND mr.emoji_value IS NOT NULL
          AND (mr.is_removed = false OR mr.is_removed IS NULL)
    """
    reactions = _execute_many(engine, reactions_query, base_params)

    # Classify reactions
    total_reactions = len(reactions)
    negative_reactions = sum(
        1 for r in reactions if is_negative_reaction(r["emoji_value"])
    )
    unique_negative_reactors = len(set(
        r["reactor_id"] for r in reactions if is_negative_reaction(r["emoji_value"])
    ))

    # Get total unique reactors for normalization
    total_reactors = len(set(r["reactor_id"] for r in reactions))

    # Calculate ratios
    toxic_ratio = toxic_count / total_analyzed if total_analyzed > 0 else 0
    negative_reactions_ratio = (
        negative_reactions / total_reactions if total_reactions > 0 else 0
    )
    unique_negative_ratio = (
        unique_negative_reactors / total_reactors if total_reactors > 0 else 0
    )

    # Calculate weighted score (this is a toxicity percentage)
    # Being sad is NOT toxic - negative sentiment affects Aura, not Toxicity
    # Toxicity is reserved for aggressive/offensive content
    raw_score = (
        0.70 * toxic_ratio
        + 0.25 * negative_reactions_ratio
        + 0.05 * unique_negative_ratio
    )

    # Scale to 0-100 (as percentage)
    scaled_score = raw_score * 100

    # Apply Bayesian smoothing
    global_mean = DEFAULT_GLOBAL_MEANS["toxicity"]
    smoothed_score = _bayesian_smooth(scaled_score, total_analyzed, global_mean)
    final_score = round(_clamp(smoothed_score, 0, 100), 1)

    return StatResult(
        value={
            "pct": final_score,
            "label": _toxicity_label(final_score),
            "toxic_messages": toxic_count,
            "total_analyzed": total_analyzed,
            "negative_reactions": negative_reactions,
            "unique_negative_reactors": unique_negative_reactors,
        }
    )


def calculate_popularity(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
    timezone: str | None = None,
) -> StatResult | None:
    """
    Calculate Popularity Score (0-100) measuring social gravity.

    Components:
    - 25% unique users who reacted
    - 25% unique users who replied
    - 15% total reactions received
    - 15% total replies received
    - 20% viral messages (4+ reactions)
    """
    base_params = {
        "user_id": user_id,
        "chat_id": chat_id,
        "window_start": window_start,
        "window_end": window_end,
    }
    chat_params = {
        "chat_id": chat_id,
        "window_start": window_start,
        "window_end": window_end,
    }

    # Get reaction stats
    reaction_stats_query = """
        SELECT
            COUNT(*) as total_reactions,
            COUNT(DISTINCT mr.user_id) as unique_reactors
        FROM message_reactions mr
        JOIN messages m ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
          AND (mr.is_removed = false OR mr.is_removed IS NULL)
    """
    reaction_stats = _execute_single(engine, reaction_stats_query, base_params)
    total_reactions = int(reaction_stats.get("total_reactions") or 0) if reaction_stats else 0
    unique_reactors = int(reaction_stats.get("unique_reactors") or 0) if reaction_stats else 0

    # Get reply stats
    reply_stats_query = """
        SELECT
            COUNT(*) as total_replies,
            COUNT(DISTINCT reply.user_id) as unique_repliers
        FROM messages reply
        JOIN messages parent ON reply.reply_to_message_id = parent.message_id
                             AND reply.chat_id = parent.chat_id
        WHERE parent.user_id = :user_id
          AND parent.chat_id = :chat_id
          AND parent.date >= :window_start
          AND parent.date <= :window_end
          AND reply.user_id != :user_id
    """
    reply_stats = _execute_single(engine, reply_stats_query, base_params)
    total_replies = int(reply_stats.get("total_replies") or 0) if reply_stats else 0
    unique_repliers = int(reply_stats.get("unique_repliers") or 0) if reply_stats else 0

    # Get viral messages (4+ reactions)
    viral_query = """
        SELECT COUNT(*) as viral_count
        FROM (
            SELECT m.message_id
            FROM messages m
            JOIN message_reactions mr ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
            WHERE m.user_id = :user_id
              AND m.chat_id = :chat_id
              AND m.date >= :window_start
              AND m.date <= :window_end
              AND (mr.is_removed = false OR mr.is_removed IS NULL)
            GROUP BY m.message_id
            HAVING COUNT(*) >= 4
        ) viral
    """
    viral_result = _execute_single(engine, viral_query, base_params)
    viral_count = int(viral_result.get("viral_count") or 0) if viral_result else 0

    # Get user's message count for message-based normalization
    message_count_query = """
        SELECT COUNT(*) as msg_count
        FROM messages m
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
    """
    msg_result = _execute_single(engine, message_count_query, base_params)
    message_count = int(msg_result.get("msg_count") or 0) if msg_result else 0

    if message_count == 0:
        return None

    # Get chat-wide p90 for normalization
    # P90 for unique reactors
    reactors_p90_query = """
        SELECT COUNT(DISTINCT mr.user_id) as val
        FROM message_reactions mr
        JOIN messages m ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
        WHERE m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
          AND (mr.is_removed = false OR mr.is_removed IS NULL)
        GROUP BY m.user_id
    """
    reactors_p90 = _get_chat_p90(engine, chat_id, reactors_p90_query, chat_params)

    # P90 for unique repliers
    repliers_p90_query = """
        SELECT COUNT(DISTINCT reply.user_id) as val
        FROM messages reply
        JOIN messages parent ON reply.reply_to_message_id = parent.message_id
                             AND reply.chat_id = parent.chat_id
        WHERE parent.chat_id = :chat_id
          AND parent.date >= :window_start
          AND parent.date <= :window_end
          AND reply.user_id != parent.user_id
        GROUP BY parent.user_id
    """
    repliers_p90 = _get_chat_p90(engine, chat_id, repliers_p90_query, chat_params)

    # P90 for total reactions
    total_reactions_p90_query = """
        SELECT COUNT(*) as val
        FROM message_reactions mr
        JOIN messages m ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
        WHERE m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
          AND (mr.is_removed = false OR mr.is_removed IS NULL)
        GROUP BY m.user_id
    """
    total_reactions_p90 = _get_chat_p90(engine, chat_id, total_reactions_p90_query, chat_params)

    # P90 for total replies
    total_replies_p90_query = """
        SELECT COUNT(*) as val
        FROM messages reply
        JOIN messages parent ON reply.reply_to_message_id = parent.message_id
                             AND reply.chat_id = parent.chat_id
        WHERE parent.chat_id = :chat_id
          AND parent.date >= :window_start
          AND parent.date <= :window_end
          AND reply.user_id != parent.user_id
        GROUP BY parent.user_id
    """
    total_replies_p90 = _get_chat_p90(engine, chat_id, total_replies_p90_query, chat_params)

    # P90 for viral count
    viral_p90_query = """
        SELECT COUNT(*) as val
        FROM (
            SELECT m.user_id, m.message_id
            FROM messages m
            JOIN message_reactions mr ON mr.chat_id = m.chat_id AND mr.message_id = m.message_id
            WHERE m.chat_id = :chat_id
              AND m.date >= :window_start
              AND m.date <= :window_end
              AND (mr.is_removed = false OR mr.is_removed IS NULL)
            GROUP BY m.user_id, m.message_id
            HAVING COUNT(*) >= 4
        ) viral
        GROUP BY viral.user_id
    """
    viral_p90 = _get_chat_p90(engine, chat_id, viral_p90_query, chat_params)

    # Normalize each component
    norm_unique_reactors = _normalize_count(unique_reactors, reactors_p90)
    norm_unique_repliers = _normalize_count(unique_repliers, repliers_p90)
    norm_total_reactions = _normalize_count(total_reactions, total_reactions_p90)
    norm_total_replies = _normalize_count(total_replies, total_replies_p90)
    norm_viral = _normalize_count(viral_count, viral_p90) if viral_p90 > 0 else 0

    # Calculate weighted score
    raw_score = (
        0.25 * norm_unique_reactors
        + 0.25 * norm_unique_repliers
        + 0.15 * norm_total_reactions
        + 0.15 * norm_total_replies
        + 0.20 * norm_viral
    )

    # Scale to 0-100
    scaled_score = raw_score * 100

    # Apply Bayesian smoothing
    global_mean = DEFAULT_GLOBAL_MEANS["popularity"]
    smoothed_score = _bayesian_smooth(scaled_score, message_count, global_mean)
    final_score = round(_clamp(smoothed_score, 0, 100), 1)

    return StatResult(
        value={
            "score": final_score,
            "label": _popularity_label(final_score),
            "unique_reactors": unique_reactors,
            "unique_repliers": unique_repliers,
            "total_reactions": total_reactions,
            "total_replies": total_replies,
            "viral_messages": viral_count,
        }
    )


def _calculate_longest_gap(
    active_dates: list[date], window_start: date, window_end: date
) -> int:
    """
    Calculate the longest gap (in days) between posts within the window.

    Considers gaps:
    1. From window_start to first post
    2. Between consecutive posts
    3. From last post to window_end
    """
    if not active_dates:
        return (window_end - window_start).days + 1  # Entire window is a gap

    sorted_dates = sorted(active_dates)

    gaps = []
    # Gap from window start to first post
    gaps.append((sorted_dates[0] - window_start).days)

    # Gaps between consecutive posts
    for i in range(1, len(sorted_dates)):
        gaps.append((sorted_dates[i] - sorted_dates[i - 1]).days - 1)

    # Gap from last post to window end
    gaps.append((window_end - sorted_dates[-1]).days)

    return max(gaps) if gaps else 0


def calculate_overall_score(
    engine: Engine,
    user_id: int,
    chat_id: int,
    window_start: datetime,
    window_end: datetime,
    timezone: str | None = None,
    existing_stats: dict | None = None,
    tiers: list[tuple[str, int]] | None = None,
) -> StatResult | None:
    """
    Calculate Overall Score (0-100) combining all metrics with weighted importance.

    Formula:
    - 70% from positive metrics (Popularity 20%, Presence 15%, Aura 12%,
      Streak 10%, Humor 8%, Activity 5%)
    - 30% penalty from negative metrics (Toxicity 12%, Negative Reactions 7%,
      Negative Messages 6%, Longest Gap 5%)

    Uses existing_stats to avoid re-querying if available.
    """
    if existing_stats is None:
        return None  # Requires pre-computed stats from generator

    # Extract scores from existing stats (default to 0 if missing)
    popularity_score = existing_stats.get("popularity", {}).get("score", 0)
    presence_score = existing_stats.get("presence", {}).get("score", 0)
    aura_score = existing_stats.get("aura", {}).get("score", 0)
    humor_score = existing_stats.get("humor", {}).get("score", 0)
    activity_score = existing_stats.get("activity", {}).get("score", 0)
    toxicity_pct = existing_stats.get("toxicity", {}).get("pct", 0)

    # Extract streak from presence (normalize to 0-100 based on window days)
    streak_days = existing_stats.get("presence", {}).get("streak", 0)
    total_days = existing_stats.get("presence", {}).get("total_days", 30)
    streak_normalized = min((streak_days / total_days) * 100, 100) if total_days > 0 else 0

    # Extract negative message ratio from aura (already 0-100)
    negative_msg_ratio = existing_stats.get("aura", {}).get("negative_ratio", 0)

    # Extract negative reactions ratio from toxicity
    negative_reactions = existing_stats.get("toxicity", {}).get("negative_reactions", 0)
    total_reactions = existing_stats.get("aura", {}).get("total_reactions", 0)
    if total_reactions == 0:
        total_reactions = existing_stats.get("popularity", {}).get("total_reactions", 0)
    negative_reaction_ratio = (
        (negative_reactions / total_reactions * 100) if total_reactions > 0 else 0
    )

    # Query active dates for longest gap calculation
    tz = timezone or "UTC"
    daily_query = """
        SELECT DISTINCT DATE(m.date AT TIME ZONE :timezone) as activity_date
        FROM messages m
        WHERE m.user_id = :user_id
          AND m.chat_id = :chat_id
          AND m.date >= :window_start
          AND m.date <= :window_end
    """
    active_dates_result = _execute_many(
        engine,
        daily_query,
        {
            "user_id": user_id,
            "chat_id": chat_id,
            "window_start": window_start,
            "window_end": window_end,
            "timezone": tz,
        },
    )

    # Convert to date objects
    active_dates = []
    for r in active_dates_result:
        d = r["activity_date"]
        if isinstance(d, datetime):
            active_dates.append(d.date())
        elif isinstance(d, date):
            active_dates.append(d)
        else:
            active_dates.append(d)

    # Calculate longest gap
    longest_gap = _calculate_longest_gap(
        active_dates, window_start.date(), window_end.date()
    )
    gap_normalized = min((longest_gap / total_days) * 100, 100) if total_days > 0 else 0

    # Calculate positive contribution (70 points max)
    positive = (
        0.20 * popularity_score
        + 0.15 * presence_score
        + 0.12 * aura_score
        + 0.10 * streak_normalized
        + 0.08 * humor_score
        + 0.05 * activity_score
    )

    # Calculate negative penalty (30 points max)
    negative = (
        0.12 * toxicity_pct
        + 0.07 * negative_reaction_ratio
        + 0.06 * negative_msg_ratio
        + 0.05 * gap_normalized
    )

    # Base score (before modifiers)
    base_score = positive - negative

    # Clamp to 0-100 (modifiers will be applied in generator.py)
    final_score = round(_clamp(base_score, 0, 100), 1)

    return StatResult(
        value={
            "score": final_score,
            "label": _overall_label(final_score, tiers),
            "positive_contribution": round(positive, 1),
            "negative_penalty": round(negative, 1),
            "longest_gap_days": longest_gap,
        }
    )


# =========================================
# CALCULATOR REGISTRY
# =========================================

# Registry of all stat calculators - add/remove entries to change what gets computed
CALCULATORS: dict[str, StatCalculator] = {
    "aura": calculate_aura,
    "activity": calculate_activity,
    "presence": calculate_presence,
    "humor": calculate_humor,
    "toxicity": calculate_toxicity,
    "popularity": calculate_popularity,
    "overall": calculate_overall_score,
}
