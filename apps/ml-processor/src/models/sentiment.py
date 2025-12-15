"""
Sentiment analysis model wrapper.
"""

import logging
from dataclasses import dataclass

from transformers import pipeline

logger = logging.getLogger(__name__)


@dataclass
class SentimentResult:
    """Result of sentiment analysis."""

    label: str
    score_positive: float
    score_neutral: float
    score_negative: float
    confidence: float


class SentimentAnalyzer:
    """
    Wrapper for multilingual sentiment analysis.

    Default model: lxyuan/distilbert-base-multilingual-cased-sentiments-student
    - Languages: 6 languages including Portuguese
    - Memory: ~270MB
    - Output: Positive, Neutral, Negative with confidence scores
    """

    def __init__(
        self,
        model_name: str = "lxyuan/distilbert-base-multilingual-cased-sentiments-student",
        device: str = "cuda",
    ):
        """
        Initialize the sentiment analyzer.

        Args:
            model_name: HuggingFace model ID
            device: Device to run on ('cuda' or 'cpu')
        """
        self.model_name = model_name
        self.device = device

        self.pipeline = pipeline(
            "text-classification",
            model=model_name,
            top_k=None,  # Return all scores
            device=0 if device == "cuda" else -1,
        )

        logger.info(f"Sentiment analyzer initialized: {model_name}")

    def analyze(self, texts: list[str], batch_size: int = 32) -> list[SentimentResult]:
        """
        Analyze sentiment for a batch of texts.

        Args:
            texts: List of texts to analyze
            batch_size: Processing batch size

        Returns:
            List of SentimentResult objects
        """
        if not texts:
            return []

        results = self.pipeline(
            texts,
            batch_size=batch_size,
            truncation=True,
            max_length=512,
        )

        processed = []
        for result in results:
            # Convert list of {label, score} to dict
            scores = {r["label"].lower(): r["score"] for r in result}

            # Get the highest scoring label
            label = max(scores, key=scores.get)

            processed.append(
                SentimentResult(
                    label=label,
                    score_positive=scores.get("positive", 0.0),
                    score_neutral=scores.get("neutral", 0.0),
                    score_negative=scores.get("negative", 0.0),
                    confidence=max(scores.values()),
                )
            )

        return processed
