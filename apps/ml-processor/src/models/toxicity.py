"""
Toxicity detection model wrapper.
"""

import logging
from dataclasses import dataclass

from transformers import pipeline

logger = logging.getLogger(__name__)


@dataclass
class ToxicityResult:
    """Result of toxicity detection."""

    is_toxic: bool
    label: str
    score: float


class ToxicityDetector:
    """
    Wrapper for Portuguese hate speech/toxicity detection.

    Default model: ruanchaves/bert-base-portuguese-cased-hatebr
    - Languages: Portuguese
    - Memory: ~440MB
    - Output: Hateful/Non-hateful classification
    - Quality: Trained on HateBR Brazilian Portuguese hate speech corpus
    """

    # Labels considered toxic (handles both named labels and LABEL_X format)
    TOXIC_LABELS = {
        "hate speech", "offensive", "hate", "toxic", "hateful",
        "label_1", "1",  # HateBR uses LABEL_1 for hateful
    }

    def __init__(
        self,
        model_name: str = "ruanchaves/bert-base-portuguese-cased-hatebr",
        device: str = "cuda",
    ):
        """
        Initialize the toxicity detector.

        Args:
            model_name: HuggingFace model ID
            device: Device to run on ('cuda' or 'cpu')
        """
        self.model_name = model_name
        self.device = device

        self.pipeline = pipeline(
            "text-classification",
            model=model_name,
            device=0 if device == "cuda" else -1,
        )

        logger.info(f"Toxicity detector initialized: {model_name}")

    def analyze(self, texts: list[str], batch_size: int = 32) -> list[ToxicityResult]:
        """
        Detect toxicity for a batch of texts.

        Args:
            texts: List of texts to analyze
            batch_size: Processing batch size

        Returns:
            List of ToxicityResult objects
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
            raw_label = result["label"]

            # Handle boolean or string labels (HateBR returns bool)
            if isinstance(raw_label, bool):
                is_toxic = raw_label
                label = "hateful" if raw_label else "non-hateful"
            else:
                label = str(raw_label).lower()
                is_toxic = label in self.TOXIC_LABELS

            processed.append(
                ToxicityResult(
                    is_toxic=is_toxic,
                    label=label,
                    score=result["score"],
                )
            )

        return processed
