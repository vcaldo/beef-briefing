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

    def get_model_name(self) -> str:
        """Return the sentence-transformers model name."""
        return self.model_name

    def cleanup(self) -> None:
        """Release model resources."""
        if self._model is not None:
            del self._model
            self._model = None
            logger.info("Embedding model unloaded")


class OpenAIEmbeddingEncoder(Analyzer):
    """
    Embedding encoder using OpenAI API.

    Default model: text-embedding-3-small
    - Languages: Multilingual support
    - Output: 1536-dimensional embeddings (small) or 3072 (large)
    - Quality: High quality, production-ready
    - Cost: ~$0.02 per 1M tokens (small)

    Available models:
    - text-embedding-3-small: 1536 dims, cheaper, good for most use cases
    - text-embedding-3-large: 3072 dims, higher quality, more expensive
    - text-embedding-ada-002: 1536 dims, legacy model
    """

    # Embedding dimensions for each model
    MODEL_DIMS = {
        "text-embedding-3-small": 1536,
        "text-embedding-3-large": 3072,
        "text-embedding-ada-002": 1536,
    }

    def __init__(
        self,
        api_key: str | None = None,
        model_name: str = "text-embedding-3-small",
        max_retries: int = 5,
        timeout: float = 60.0,
    ):
        """
        Initialize with OpenAI API key.

        Args:
            api_key: OpenAI API key (required)
            model_name: OpenAI embedding model name
            max_retries: Maximum retry attempts for rate limits/errors
            timeout: Request timeout in seconds
        """
        self.api_key = api_key
        self.model_name = model_name
        self.max_retries = max_retries
        self.timeout = timeout
        self._client = None
        self._last_usage: dict | None = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.EMBEDDINGS

    def _get_client(self):
        """Lazy load the OpenAI client with retry configuration."""
        if self._client is None:
            from openai import OpenAI

            self._client = OpenAI(
                api_key=self.api_key,
                max_retries=self.max_retries,
                timeout=self.timeout,
            )
        return self._client

    def analyze(self, texts: list[str], batch_size: int = 100, **kwargs) -> list[dict]:
        """
        Generate embeddings for a batch of texts.

        Args:
            texts: List of texts to encode
            batch_size: Processing batch size (max 2048 for OpenAI)

        Returns:
            List of dicts with:
                - embedding: list[float]
        """
        if not texts:
            return []

        embeddings = self.encode(texts, batch_size=batch_size)

        return [{"embedding": emb.tolist()} for emb in embeddings]

    def encode(self, texts: list[str], batch_size: int = 100) -> np.ndarray:
        """
        Generate embeddings as numpy array.

        Args:
            texts: List of texts to encode
            batch_size: Processing batch size

        Returns:
            Numpy array of shape (n_texts, embedding_dim)
        """
        if not texts:
            return np.array([])

        client = self._get_client()

        all_embeddings = []

        # Process in batches
        for i in range(0, len(texts), batch_size):
            batch = texts[i : i + batch_size]

            # Replace empty strings with a space (OpenAI doesn't accept empty)
            batch = [t if t.strip() else " " for t in batch]

            try:
                response = client.embeddings.create(
                    model=self.model_name,
                    input=batch,
                )

                # Capture token usage for monitoring (accumulate across batches)
                if response.usage:
                    if self._last_usage is None:
                        self._last_usage = {
                            "prompt_tokens": 0,
                            "completion_tokens": 0,
                            "total_tokens": 0,
                            "model": self.model_name,
                        }
                    self._last_usage["prompt_tokens"] += response.usage.prompt_tokens
                    self._last_usage["total_tokens"] += response.usage.total_tokens

                # Sort by index to ensure correct order
                sorted_data = sorted(response.data, key=lambda x: x.index)
                batch_embeddings = [item.embedding for item in sorted_data]
                all_embeddings.extend(batch_embeddings)

            except Exception as e:
                logger.error(f"OpenAI embedding failed for batch {i}: {e}")
                # Return zero vectors for failed batch
                dim = self.embedding_dim
                all_embeddings.extend([[0.0] * dim for _ in batch])

        return np.array(all_embeddings)

    @property
    def embedding_dim(self) -> int:
        """Return the embedding dimension for the current model."""
        return self.MODEL_DIMS.get(self.model_name, 1536)

    def is_available(self) -> bool:
        """Check if OpenAI API key is available."""
        return bool(self.api_key)

    def get_model_name(self) -> str:
        """Return the OpenAI embedding model name."""
        return self.model_name

    def is_local_provider(self) -> bool:
        """Return False - this uses OpenAI API."""
        return False

    def cleanup(self) -> None:
        """Release client resources."""
        self._client = None
        self._last_usage = None
