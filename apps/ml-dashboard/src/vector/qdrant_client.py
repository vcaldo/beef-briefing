"""
Qdrant vector database client for embedding retrieval.
"""

import logging
from typing import Optional

import numpy as np
from qdrant_client import QdrantClient
from qdrant_client.http import models

logger = logging.getLogger(__name__)

COLLECTION_NAME = "message_embeddings"
VECTOR_SIZE = 768  # Size of paraphrase-multilingual-mpnet-base-v2 embeddings


class EmbeddingsClient:
    """Client for retrieving message embeddings from Qdrant."""

    def __init__(self, host: str = "localhost", port: int = 6333):
        """
        Initialize the Qdrant client.

        Args:
            host: Qdrant server host
            port: Qdrant server port
        """
        self.client = QdrantClient(host=host, port=port)

    def get_embeddings_for_chat(
        self,
        chat_id: int,
        limit: int = 10000,
    ) -> tuple[np.ndarray, list[dict]]:
        """
        Fetch embeddings and metadata for a chat.

        Args:
            chat_id: Chat ID to fetch embeddings for
            limit: Maximum number of embeddings to fetch

        Returns:
            Tuple of:
                embeddings: (N, 768) numpy array
                metadata: list of dicts with message_id, user_id, text_preview
        """
        points = []
        offset = None

        while True:
            result = self.client.scroll(
                collection_name=COLLECTION_NAME,
                scroll_filter=models.Filter(
                    must=[
                        models.FieldCondition(
                            key="chat_id",
                            match=models.MatchValue(value=chat_id),
                        )
                    ]
                ),
                limit=1000,
                offset=offset,
                with_payload=True,
                with_vectors=True,
            )
            points.extend(result[0])
            offset = result[1]
            if offset is None or len(points) >= limit:
                break

        if not points:
            return np.array([]), []

        embeddings = np.array([p.vector for p in points])
        metadata = [
            {
                "message_id": p.id,
                "user_id": p.payload.get("user_id"),
                "text_preview": p.payload.get("text_preview", ""),
                "chat_id": p.payload.get("chat_id"),
            }
            for p in points
        ]

        logger.info(f"Fetched {len(points)} embeddings for chat {chat_id}")
        return embeddings, metadata

    def get_embeddings_for_user(
        self,
        chat_id: int,
        user_id: int,
        limit: int = 5000,
    ) -> tuple[np.ndarray, list[dict]]:
        """
        Fetch embeddings for a specific user in a chat.

        Args:
            chat_id: Chat ID
            user_id: User ID to filter by
            limit: Maximum number of embeddings

        Returns:
            Tuple of embeddings array and metadata list
        """
        points = []
        offset = None

        while True:
            result = self.client.scroll(
                collection_name=COLLECTION_NAME,
                scroll_filter=models.Filter(
                    must=[
                        models.FieldCondition(
                            key="chat_id",
                            match=models.MatchValue(value=chat_id),
                        ),
                        models.FieldCondition(
                            key="user_id",
                            match=models.MatchValue(value=user_id),
                        ),
                    ]
                ),
                limit=1000,
                offset=offset,
                with_payload=True,
                with_vectors=True,
            )
            points.extend(result[0])
            offset = result[1]
            if offset is None or len(points) >= limit:
                break

        if not points:
            return np.array([]), []

        embeddings = np.array([p.vector for p in points])
        metadata = [
            {
                "message_id": p.id,
                "user_id": p.payload.get("user_id"),
                "text_preview": p.payload.get("text_preview", ""),
            }
            for p in points
        ]

        return embeddings, metadata

    def get_collection_info(self) -> dict:
        """Get collection statistics."""
        try:
            info = self.client.get_collection(COLLECTION_NAME)
            points_count = getattr(info, "points_count", None)
            if points_count is None:
                points_count = getattr(info, "vectors_count", 0)
            return {
                "points_count": points_count,
                "status": (
                    info.status.value
                    if hasattr(info.status, "value")
                    else str(info.status)
                ),
            }
        except Exception as e:
            logger.warning(f"Could not get collection info: {e}")
            return {"points_count": 0, "status": "unknown"}

    def is_available(self) -> bool:
        """Check if Qdrant is available and has the collection."""
        try:
            self.client.get_collection(COLLECTION_NAME)
            return True
        except Exception:
            return False
