"""
ML model wrappers package.
"""

import logging
from dataclasses import dataclass
from typing import Optional

import torch

logger = logging.getLogger(__name__)


@dataclass
class ModelManager:
    """Manages GPU memory for ML models."""

    device: str = "cuda" if torch.cuda.is_available() else "cpu"

    sentiment_analyzer: Optional["SentimentAnalyzer"] = None
    toxicity_detector: Optional["ToxicityDetector"] = None
    embedding_encoder: Optional["EmbeddingEncoder"] = None

    def load_all(
        self,
        sentiment_model: str,
        toxicity_model: str,
        embedding_model: str,
    ):
        """
        Load all models into GPU memory.

        Args:
            sentiment_model: HuggingFace model ID for sentiment
            toxicity_model: HuggingFace model ID for toxicity
            embedding_model: HuggingFace model ID for embeddings
        """
        from .sentiment import SentimentAnalyzer
        from .toxicity import ToxicityDetector
        from .embeddings import EmbeddingEncoder

        logger.info(f"Loading models to device: {self.device}")

        logger.info(f"Loading sentiment model: {sentiment_model}")
        self.sentiment_analyzer = SentimentAnalyzer(
            model_name=sentiment_model,
            device=self.device,
        )

        logger.info(f"Loading toxicity model: {toxicity_model}")
        self.toxicity_detector = ToxicityDetector(
            model_name=toxicity_model,
            device=self.device,
        )

        logger.info(f"Loading embedding model: {embedding_model}")
        self.embedding_encoder = EmbeddingEncoder(
            model_name=embedding_model,
            device=self.device,
        )

        logger.info("All models loaded successfully")

    def unload_all(self):
        """Free GPU memory by unloading all models."""
        self.sentiment_analyzer = None
        self.toxicity_detector = None
        self.embedding_encoder = None

        if torch.cuda.is_available():
            torch.cuda.empty_cache()

        logger.info("All models unloaded, GPU memory freed")
