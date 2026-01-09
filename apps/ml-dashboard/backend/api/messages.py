"""Messages API - Browse ML results."""

from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Query

from db import DashboardQueries

router = APIRouter(prefix="/api/messages", tags=["messages"])


def get_queries() -> DashboardQueries:
    """Dependency to get database queries instance."""
    from main import queries
    return queries


@router.get("")
def list_messages(
    chat_id: int = Query(..., description="Chat ID to filter by"),
    limit: int = Query(50, ge=1, le=200),
    offset: int = Query(0, ge=0),
    user_id: Optional[int] = Query(None),
    sentiment: Optional[str] = Query(None, pattern="^(positive|neutral|negative)$"),
    is_toxic: Optional[bool] = Query(None),
    is_humorous: Optional[bool] = Query(None),
    is_question: Optional[bool] = Query(None),
    topic_id: Optional[int] = Query(None),
    sort_by: str = Query("date", pattern="^(date|toxicity_score|sentiment_score)$"),
    sort_order: str = Query("desc", pattern="^(asc|desc)$"),
    queries: DashboardQueries = Depends(get_queries),
):
    """
    List messages with ML results.

    Supports filtering by:
    - user_id: Filter by specific user
    - sentiment: positive, neutral, or negative
    - is_toxic: true/false
    - is_humorous: true/false
    - is_question: true/false
    - topic_id: Filter by topic cluster
    """
    messages, total = queries.get_messages(
        chat_id=chat_id,
        limit=limit,
        offset=offset,
        user_id=user_id,
        sentiment=sentiment,
        is_toxic=is_toxic,
        is_humorous=is_humorous,
        is_question=is_question,
        topic_id=topic_id,
        sort_by=sort_by,
        sort_order=sort_order,
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


@router.get("/{message_id}")
def get_message(
    message_id: int,
    queries: DashboardQueries = Depends(get_queries),
):
    """Get a single message with all ML details including entities."""
    message = queries.get_message_detail(message_id)
    if not message:
        raise HTTPException(status_code=404, detail="Message not found")

    entities = queries.get_message_entities(message_id)

    return {
        "message": message,
        "entities": entities,
    }
