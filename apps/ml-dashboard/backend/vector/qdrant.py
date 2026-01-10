"""
Qdrant vector database client for semantic search.
Reuses the same collection as ml-processor.
"""

import logging
from typing import Optional

from qdrant_client import QdrantClient
from qdrant_client.http import models
from qdrant_client.http.exceptions import UnexpectedResponse

logger = logging.getLogger(__name__)

COLLECTION_NAME = "message_embeddings"


class QdrantSearcher:
    """Client for searching message embeddings in Qdrant."""

    def __init__(self, host: str = "localhost", port: int = 6333):
        self._host = host
        self._port = port
        self._client: Optional[QdrantClient] = None

    @property
    def client(self) -> QdrantClient:
        """Lazy-load the Qdrant client."""
        if self._client is None:
            self._client = QdrantClient(host=self._host, port=self._port)
        return self._client

    def is_available(self) -> bool:
        """Check if Qdrant is available and the collection exists."""
        try:
            self.client.get_collection(COLLECTION_NAME)
            return True
        except Exception as e:
            logger.warning(f"Qdrant not available: {e}")
            return False

    def search(
        self,
        query_vector: list[float],
        chat_id: Optional[int] = None,
        user_id: Optional[int] = None,
        limit: int = 20,
    ) -> list[dict]:
        """
        Search for similar messages.

        Args:
            query_vector: Query embedding vector
            chat_id: Optional chat ID to filter by
            user_id: Optional user ID to filter by
            limit: Maximum number of results

        Returns:
            List of dicts with message_id, score, and payload
        """
        query_filter = None
        conditions = []

        if chat_id is not None:
            conditions.append(
                models.FieldCondition(
                    key="chat_id",
                    match=models.MatchValue(value=chat_id),
                )
            )

        if user_id is not None:
            conditions.append(
                models.FieldCondition(
                    key="user_id",
                    match=models.MatchValue(value=user_id),
                )
            )

        if conditions:
            query_filter = models.Filter(must=conditions)

        try:
            results = self.client.search(
                collection_name=COLLECTION_NAME,
                query_vector=query_vector,
                query_filter=query_filter,
                limit=limit,
            )

            return [
                {
                    "message_id": r.id,
                    "score": r.score,
                    "chat_id": r.payload.get("chat_id"),
                    "user_id": r.payload.get("user_id"),
                    "text_preview": r.payload.get("text_preview"),
                }
                for r in results
            ]
        except Exception as e:
            logger.error(f"Qdrant search error: {e}")
            return []

    def get_collection_info(self) -> dict:
        """Get collection statistics."""
        try:
            info = self.client.get_collection(COLLECTION_NAME)
            points_count = getattr(info, "points_count", None)
            if points_count is None:
                points_count = getattr(info, "vectors_count", 0)
            return {
                "available": True,
                "points_count": points_count,
                "status": info.status.value if hasattr(info.status, "value") else str(info.status),
            }
        except (UnexpectedResponse, Exception) as e:
            logger.warning(f"Could not get Qdrant collection info: {e}")
            return {"available": False, "points_count": 0, "status": "unavailable"}
