"""Search API - Semantic search using Qdrant embeddings."""

import time
from typing import Optional

from fastapi import APIRouter, Depends, HTTPException, Query
from pydantic import BaseModel

from db import DashboardQueries
from vector import EmbeddingService, QdrantSearcher

router = APIRouter(prefix="/api/search", tags=["search"])


class SearchRequest(BaseModel):
    """Search request body."""

    query: str
    chat_id: Optional[int] = None
    user_id: Optional[int] = None
    limit: int = 20


def get_dependencies():
    """Get all dependencies."""
    from main import queries, qdrant, embeddings
    return queries, qdrant, embeddings


@router.get("/status")
def search_status():
    """Check if semantic search is available."""
    from main import qdrant, embeddings

    qdrant_info = qdrant.get_collection_info() if qdrant else {"available": False}
    model_loaded = embeddings.is_loaded() if embeddings else False

    return {
        "qdrant": qdrant_info,
        "embedding_model_loaded": model_loaded,
        "search_available": qdrant_info.get("available", False),
    }


@router.post("")
def search_messages(request: SearchRequest):
    """
    Semantic search for similar messages.

    Converts the query text to an embedding and searches Qdrant
    for similar messages.
    """
    from main import queries, qdrant, embeddings

    if not qdrant or not qdrant.is_available():
        raise HTTPException(
            status_code=503,
            detail="Semantic search unavailable. Qdrant not connected (dev environment only).",
        )

    # Generate embedding for query
    start_time = time.time()
    query_vector = embeddings.embed(request.query)
    embedding_time = (time.time() - start_time) * 1000

    # Search Qdrant
    search_start = time.time()
    results = qdrant.search(
        query_vector=query_vector,
        chat_id=request.chat_id,
        user_id=request.user_id,
        limit=request.limit,
    )
    search_time = (time.time() - search_start) * 1000

    # Enrich results with full message data
    message_ids = [r["message_id"] for r in results]
    messages_map = {}
    if message_ids:
        messages = queries.get_messages_by_ids(message_ids)
        messages_map = {m["id"]: m for m in messages}

    enriched_results = []
    for r in results:
        msg = messages_map.get(r["message_id"])
        enriched_results.append({
            "message_id": r["message_id"],
            "score": r["score"],
            "text_preview": r["text_preview"],
            "message": msg,
        })

    return {
        "results": enriched_results,
        "query": request.query,
        "timing": {
            "embedding_ms": round(embedding_time, 2),
            "search_ms": round(search_time, 2),
        },
    }
