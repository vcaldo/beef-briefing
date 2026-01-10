"""Users API - User analytics and cards."""

from fastapi import APIRouter, Depends, HTTPException, Query

from db import DashboardQueries

router = APIRouter(prefix="/api/users", tags=["users"])


def get_queries() -> DashboardQueries:
    """Dependency to get database queries instance."""
    from main import queries
    return queries


@router.get("")
def list_users(
    chat_id: int = Query(..., description="Chat ID"),
    limit: int = Query(50, ge=1, le=200),
    offset: int = Query(0, ge=0),
    queries: DashboardQueries = Depends(get_queries),
):
    """
    List users with aggregated ML statistics.

    Returns users with:
    - message_count: Total messages in chat
    - avg_sentiment: Average sentiment score (-1 to 1)
    - toxicity_rate: Percentage of toxic messages (0 to 1)
    - humor_rate: Percentage of humorous messages (0 to 1)
    - question_rate: Percentage of questions (0 to 1)
    """
    users, total = queries.get_users_with_ml_stats(
        chat_id=chat_id,
        limit=limit,
        offset=offset,
    )

    return {
        "users": users,
        "total": total,
        "page_info": {
            "limit": limit,
            "offset": offset,
            "has_more": offset + len(users) < total,
        },
    }


@router.get("/{user_id}/profile")
def get_user_profile(
    user_id: int,
    chat_id: int = Query(..., description="Chat ID"),
    queries: DashboardQueries = Depends(get_queries),
):
    """
    Get detailed ML profile for a user.

    Returns:
    - Basic user info
    - Sentiment distribution (positive/neutral/negative counts)
    - Top entity mentions
    """
    # Get user stats first to verify user exists in chat
    users, _ = queries.get_users_with_ml_stats(chat_id=chat_id, limit=1, offset=0)

    # Get sentiment distribution
    sentiment_dist = queries.get_user_sentiment_distribution(chat_id, user_id)

    # Get entity mentions
    entity_mentions = queries.get_user_entity_mentions(chat_id, user_id, limit=20)

    # Calculate percentages
    total = sentiment_dist.get("total", 0) or 0
    sentiment_pct = {}
    if total > 0:
        sentiment_pct = {
            "positive": round((sentiment_dist.get("positive", 0) or 0) / total * 100, 1),
            "neutral": round((sentiment_dist.get("neutral", 0) or 0) / total * 100, 1),
            "negative": round((sentiment_dist.get("negative", 0) or 0) / total * 100, 1),
        }

    return {
        "user_id": user_id,
        "sentiment_distribution": {
            "counts": sentiment_dist,
            "percentages": sentiment_pct,
        },
        "entity_mentions": entity_mentions,
    }


@router.get("/{user_id}/cards")
def get_user_cards(
    user_id: int,
    chat_id: int = Query(..., description="Chat ID"),
    queries: DashboardQueries = Depends(get_queries),
):
    """Get card history for a user."""
    cards = queries.get_user_cards(chat_id, user_id)

    return {
        "user_id": user_id,
        "cards": cards,
        "total": len(cards),
    }
