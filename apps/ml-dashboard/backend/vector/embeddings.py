"""
Embedding service for semantic search.
Uses the same model as ml-processor for consistency.
"""

import logging
from typing import Optional

logger = logging.getLogger(__name__)


class EmbeddingService:
    """
    Service for generating text embeddings.

    Lazy-loads the model on first use to avoid slow startup.
    """

    def __init__(self, model_name: str = "sentence-transformers/paraphrase-multilingual-mpnet-base-v2"):
        self._model_name = model_name
        self._model = None

    @property
    def model(self):
        """Lazy-load the sentence transformer model."""
        if self._model is None:
            logger.info(f"Loading embedding model: {self._model_name}")
            from sentence_transformers import SentenceTransformer

            self._model = SentenceTransformer(self._model_name)
            logger.info("Embedding model loaded")
        return self._model

    def embed(self, text: str) -> list[float]:
        """
        Generate embedding for a text string.

        Args:
            text: Input text to embed

        Returns:
            768-dimensional embedding vector
        """
        return self.model.encode(text).tolist()

    def embed_batch(self, texts: list[str]) -> list[list[float]]:
        """
        Generate embeddings for multiple texts.

        Args:
            texts: List of input texts

        Returns:
            List of 768-dimensional embedding vectors
        """
        return self.model.encode(texts).tolist()

    def is_loaded(self) -> bool:
        """Check if the model is already loaded."""
        return self._model is not None
