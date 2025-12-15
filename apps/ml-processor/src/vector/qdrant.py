"""
Qdrant vector database client wrapper.
"""

import logging
from typing import Optional

import numpy as np
from qdrant_client import QdrantClient
from qdrant_client.http import models
from qdrant_client.http.exceptions import UnexpectedResponse

logger = logging.getLogger(__name__)

COLLECTION_NAME = "message_embeddings"
VECTOR_SIZE = 768  # Size of paraphrase-multilingual-mpnet-base-v2 embeddings


class QdrantWrapper:
    """Wrapper for Qdrant vector database operations."""

    def __init__(self, host: str = "localhost", port: int = 6333):
        """
        Initialize the Qdrant client.

        Args:
            host: Qdrant server host
            port: Qdrant server port
        """
        self.client = QdrantClient(host=host, port=port)
        self._ensure_collection()

    def _ensure_collection(self):
        """Create the collection if it doesn't exist."""
        try:
            self.client.get_collection(COLLECTION_NAME)
            logger.info(f"Collection '{COLLECTION_NAME}' exists")
        except (UnexpectedResponse, Exception):
            logger.info(f"Creating collection '{COLLECTION_NAME}'")
            self.client.create_collection(
                collection_name=COLLECTION_NAME,
                vectors_config=models.VectorParams(
                    size=VECTOR_SIZE,
                    distance=models.Distance.COSINE,
                ),
            )
            # Create payload index for filtering
            self.client.create_payload_index(
                collection_name=COLLECTION_NAME,
                field_name="chat_id",
                field_schema=models.PayloadSchemaType.INTEGER,
            )
            self.client.create_payload_index(
                collection_name=COLLECTION_NAME,
                field_name="user_id",
                field_schema=models.PayloadSchemaType.INTEGER,
            )
            logger.info(f"Collection '{COLLECTION_NAME}' created with indexes")

    def upsert_embeddings(
        self,
        message_ids: list[int],
        chat_ids: list[int],
        user_ids: list[Optional[int]],
        texts: list[str],
        embeddings: np.ndarray,
    ) -> int:
        """
        Insert or update message embeddings.

        Args:
            message_ids: Database message IDs (used as point IDs)
            chat_ids: Chat IDs for filtering
            user_ids: User IDs for filtering (can be None)
            texts: Original message texts (stored as preview)
            embeddings: Numpy array of embeddings (n_messages, 768)

        Returns:
            Number of points upserted
        """
        if len(message_ids) == 0:
            return 0

        points = []
        for i, (msg_id, chat_id, user_id, text) in enumerate(
            zip(message_ids, chat_ids, user_ids, texts)
        ):
            # Truncate text preview to 100 chars
            text_preview = text[:100] if len(text) > 100 else text

            payload = {
                "message_id": msg_id,
                "chat_id": chat_id,
                "text_preview": text_preview,
            }

            # Only add user_id if not None
            if user_id is not None:
                payload["user_id"] = user_id

            points.append(
                models.PointStruct(
                    id=msg_id,  # Use message_id as point ID
                    vector=embeddings[i].tolist(),
                    payload=payload,
                )
            )

        self.client.upsert(
            collection_name=COLLECTION_NAME,
            points=points,
        )

        logger.debug(f"Upserted {len(points)} embeddings to Qdrant")
        return len(points)

    def search_similar(
        self,
        query_vector: np.ndarray,
        chat_id: Optional[int] = None,
        limit: int = 10,
    ) -> list[dict]:
        """
        Search for similar messages.

        Args:
            query_vector: 768-dimensional query vector
            chat_id: Optional chat ID to filter by
            limit: Maximum number of results

        Returns:
            List of dicts with message_id, score, and payload
        """
        query_filter = None
        if chat_id is not None:
            query_filter = models.Filter(
                must=[
                    models.FieldCondition(
                        key="chat_id",
                        match=models.MatchValue(value=chat_id),
                    )
                ]
            )

        results = self.client.search(
            collection_name=COLLECTION_NAME,
            query_vector=query_vector.tolist(),
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

    def get_collection_info(self) -> dict:
        """Get collection statistics."""
        info = self.client.get_collection(COLLECTION_NAME)
        # API changed in newer qdrant-client versions
        points_count = getattr(info, "points_count", None)
        if points_count is None:
            # Try alternative attribute names
            points_count = getattr(info, "vectors_count", 0)
        return {
            "points_count": points_count,
            "status": info.status.value if hasattr(info.status, "value") else str(info.status),
        }

    def delete_by_chat(self, chat_id: int) -> int:
        """
        Delete all embeddings for a chat.

        Args:
            chat_id: Chat ID to delete embeddings for

        Returns:
            Estimated number of deleted points
        """
        # Get count before deletion
        info_before = self.get_collection_info()

        self.client.delete(
            collection_name=COLLECTION_NAME,
            points_selector=models.FilterSelector(
                filter=models.Filter(
                    must=[
                        models.FieldCondition(
                            key="chat_id",
                            match=models.MatchValue(value=chat_id),
                        )
                    ]
                )
            ),
        )

        # Get count after deletion
        info_after = self.get_collection_info()
        deleted = (info_before.get("points_count", 0) or 0) - (info_after.get("points_count", 0) or 0)

        logger.info(f"Deleted ~{deleted} embeddings for chat {chat_id}")
        return deleted
