"""Topics API - Explore topic clusters."""

from fastapi import APIRouter, Depends, Query

from db import DashboardQueries

router = APIRouter(prefix="/api/topics", tags=["topics"])


def get_queries() -> DashboardQueries:
    """Dependency to get database queries instance."""
    from main import queries
    return queries


@router.get("")
def list_topics(
    chat_id: int = Query(..., description="Chat ID"),
    queries: DashboardQueries = Depends(get_queries),
):
    """
    List all topics for a chat.

    Returns topics with their keywords and message counts.
    Also includes count of unclustered (outlier) messages.
    """
    topics = queries.get_topics(chat_id)
    outlier_count = queries.get_outlier_count(chat_id)

    return {
        "topics": topics,
        "total_topics": len(topics),
        "outlier_count": outlier_count,
    }


@router.get("/{topic_id}/messages")
def get_topic_messages(
    topic_id: int,
    chat_id: int = Query(..., description="Chat ID"),
    limit: int = Query(50, ge=1, le=200),
    offset: int = Query(0, ge=0),
    queries: DashboardQueries = Depends(get_queries),
):
    """Get messages assigned to a specific topic."""
    messages, total = queries.get_topic_messages(
        chat_id=chat_id,
        topic_id=topic_id,
        limit=limit,
        offset=offset,
    )

    return {
        "messages": messages,
        "total": total,
        "page_info": {
            "limit": limit,
            "offset": offset,
            "has_more": offset + len(messages) < total,
        },
    }
