"""
Emoji sentiment classification using emosent-py.

Based on research by Kralj Novak et al. (2015).
Provides sentiment scores for 751 emojis.

Thresholds:
- Positive: sentiment_score > 0.2
- Negative: sentiment_score < -0.2
- Neutral: -0.2 to 0.2
"""

import logging
from functools import lru_cache

from emosent import get_emoji_sentiment_rank

logger = logging.getLogger(__name__)


@lru_cache(maxsize=1024)
def get_emoji_sentiment(emoji: str) -> float | None:
    """
    Return sentiment score for an emoji (-1 to 1), or None if not found.

    Uses emosent-py library which provides sentiment rankings for 751 emojis
    based on crowdsourced annotations.

    Args:
        emoji: A single emoji character

    Returns:
        Sentiment score between -1 (negative) and 1 (positive), or None if unknown
    """
    try:
        result = get_emoji_sentiment_rank(emoji)
        if result and "sentiment_score" in result:
            return float(result["sentiment_score"])
    except Exception as e:
        logger.debug(f"Failed to get sentiment for emoji '{emoji}': {e}")
    return None


def classify_emoji_sentiment(emoji: str) -> str:
    """
    Classify an emoji as 'positive', 'negative', or 'neutral'.

    Args:
        emoji: A single emoji character

    Returns:
        'positive' if score > 0.2
        'negative' if score < -0.2
        'neutral' otherwise or if unknown
    """
    score = get_emoji_sentiment(emoji)
    if score is None:
        return "neutral"
    if score > 0.2:
        return "positive"
    elif score < -0.2:
        return "negative"
    return "neutral"


def is_positive_reaction(emoji: str) -> bool:
    """
    Check if an emoji reaction is positive (sentiment > 0.2).

    Args:
        emoji: A single emoji character

    Returns:
        True if the emoji has a positive sentiment score > 0.2
    """
    score = get_emoji_sentiment(emoji)
    return score is not None and score > 0.2


def is_negative_reaction(emoji: str) -> bool:
    """
    Check if an emoji reaction is negative (sentiment < -0.2).

    Args:
        emoji: A single emoji character

    Returns:
        True if the emoji has a negative sentiment score < -0.2
    """
    score = get_emoji_sentiment(emoji)
    return score is not None and score < -0.2


def classify_reaction_list(emojis: list[str]) -> dict[str, int]:
    """
    Classify a list of emoji reactions into positive, negative, and neutral counts.

    Args:
        emojis: List of emoji characters

    Returns:
        Dictionary with 'positive', 'negative', and 'neutral' counts
    """
    counts = {"positive": 0, "negative": 0, "neutral": 0}
    for emoji in emojis:
        sentiment = classify_emoji_sentiment(emoji)
        counts[sentiment] += 1
    return counts
