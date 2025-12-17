"""
Embedding generation implementations.

Provides sentence embedding encoders for semantic similarity,
clustering, and vector search.
"""

import logging
from typing import Any

import numpy as np

from src.analyzers.base import Analyzer, AnalysisType

logger = logging.getLogger(__name__)


class LocalEmbeddingEncoder(Analyzer):
    """
    Embedding encoder using sentence-transformers.

    Default model: sentence-transformers/paraphrase-multilingual-mpnet-base-v2
    - Languages: 50+ languages including Portuguese
    - Memory: ~420MB GPU
    - Output: 768-dimensional embeddings
    - Quality: Best multilingual sentence embeddings available
    """

    def __init__(
        self,
        model_name: str = "sentence-transformers/paraphrase-multilingual-mpnet-base-v2",
        device: str = "cuda",
    ):
        """
        Initialize the embedding encoder.

        Args:
            model_name: HuggingFace model ID
            device: Device to run on ('cuda' or 'cpu')
        """
        self.model_name = model_name
        self.device = device
        self._model = None  # Lazy loaded

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.EMBEDDINGS

    def _load_model(self):
        """Lazy load the model on first use."""
        if self._model is None:
            from sentence_transformers import SentenceTransformer

            self._model = SentenceTransformer(self.model_name, device=self.device)
            logger.info(f"Embedding model loaded: {self.model_name}")

    def analyze(self, texts: list[str], batch_size: int = 64, **kwargs) -> list[dict]:
        """
        Generate embeddings for a batch of texts.

        Note: This returns embeddings wrapped in dicts for API consistency,
        but you may want to use encode() directly for efficiency.

        Args:
            texts: List of texts to encode
            batch_size: Processing batch size

        Returns:
            List of dicts with:
                - embedding: list[float] (768-dimensional vector)
        """
        if not texts:
            return []

        embeddings = self.encode(texts, batch_size=batch_size)

        return [{"embedding": emb.tolist()} for emb in embeddings]

    def encode(self, texts: list[str], batch_size: int = 64) -> np.ndarray:
        """
        Generate embeddings as numpy array.

        More efficient than analyze() when you need raw vectors.

        Args:
            texts: List of texts to encode
            batch_size: Processing batch size

        Returns:
            Numpy array of shape (n_texts, embedding_dim)
        """
        if not texts:
            return np.array([])

        self._load_model()

        embeddings = self._model.encode(
            texts,
            batch_size=batch_size,
            show_progress_bar=False,
            convert_to_numpy=True,
        )

        return embeddings

    @property
    def embedding_dim(self) -> int:
        """Return the embedding dimension."""
        self._load_model()
        return self._model.get_sentence_embedding_dimension()

    def is_available(self) -> bool:
        """Check if sentence-transformers is available."""
        try:
            import sentence_transformers

            return True
        except ImportError:
            return False

    def cleanup(self) -> None:
        """Release model resources."""
        if self._model is not None:
            del self._model
            self._model = None
            logger.info("Embedding model unloaded")
