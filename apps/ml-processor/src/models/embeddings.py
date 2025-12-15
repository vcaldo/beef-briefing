"""
Sentence embedding model wrapper.
"""

import logging

import numpy as np
from sentence_transformers import SentenceTransformer

logger = logging.getLogger(__name__)


class EmbeddingEncoder:
    """
    Wrapper for multilingual sentence embeddings.

    Default model: sentence-transformers/paraphrase-multilingual-mpnet-base-v2
    - Languages: 50+ languages including Portuguese
    - Memory: ~420MB
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

        self.model = SentenceTransformer(model_name, device=device)

        logger.info(f"Embedding encoder initialized: {model_name}")

    def encode(self, texts: list[str], batch_size: int = 64) -> np.ndarray:
        """
        Generate embeddings for a batch of texts.

        Args:
            texts: List of texts to encode
            batch_size: Processing batch size

        Returns:
            Numpy array of shape (n_texts, 768)
        """
        if not texts:
            return np.array([])

        embeddings = self.model.encode(
            texts,
            batch_size=batch_size,
            show_progress_bar=False,
            convert_to_numpy=True,
        )

        return embeddings

    @property
    def embedding_dim(self) -> int:
        """Return the embedding dimension."""
        return self.model.get_sentence_embedding_dimension()
