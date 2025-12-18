"""
Toxicity detection implementations.

Provides local HuggingFace-based and API-based toxicity analyzers.
"""

import logging
from typing import Any

from src.analyzers.base import Analyzer, AnalysisType

logger = logging.getLogger(__name__)


class LocalToxicityAnalyzer(Analyzer):
    """
    Toxicity detector using HuggingFace transformers.

    Default model: ruanchaves/bert-base-portuguese-cased-hatebr
    - Languages: Portuguese
    - Memory: ~440MB GPU
    - Output: Hateful/Non-hateful classification
    - Quality: Trained on HateBR Brazilian Portuguese hate speech corpus
    """

    # Labels considered toxic (handles both named labels and LABEL_X format)
    TOXIC_LABELS = {
        "hate speech",
        "offensive",
        "hate",
        "toxic",
        "hateful",
        "label_1",
        "1",  # HateBR uses LABEL_1 for hateful
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
        self._pipeline = None  # Lazy loaded

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.TOXICITY

    def _load_model(self):
        """Lazy load the model on first use."""
        if self._pipeline is None:
            from transformers import pipeline

            self._pipeline = pipeline(
                "text-classification",
                model=self.model_name,
                device=0 if self.device == "cuda" else -1,
            )
            logger.info(f"Toxicity model loaded: {self.model_name}")

    def analyze(self, texts: list[str], batch_size: int = 32, **kwargs) -> list[dict]:
        """
        Detect toxicity for a batch of texts.

        Args:
            texts: List of texts to analyze
            batch_size: Processing batch size

        Returns:
            List of dicts with:
                - is_toxic: bool
                - label: str
                - score: float
        """
        if not texts:
            return []

        self._load_model()

        results = self._pipeline(
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
                {
                    "is_toxic": is_toxic,
                    "label": label,
                    "score": result["score"],
                }
            )

        return processed

    def is_available(self) -> bool:
        """Check if transformers is available."""
        try:
            import transformers

            return True
        except ImportError:
            return False

    def cleanup(self) -> None:
        """Release model resources."""
        if self._pipeline is not None:
            del self._pipeline
            self._pipeline = None
            logger.info("Toxicity model unloaded")


class PerspectiveToxicityAnalyzer(Analyzer):
    """
    Toxicity analyzer using Google Perspective API.

    Purpose-built for toxicity detection, free tier available.
    """

    ENDPOINT = "https://commentanalyzer.googleapis.com/v1alpha1/comments:analyze"

    def __init__(self, api_key: str | None = None):
        """
        Initialize with Perspective API key.

        Args:
            api_key: Google Perspective API key (required)
        """
        self.api_key = api_key

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.TOXICITY

    def analyze(self, texts: list[str], **kwargs) -> list[dict]:
        """
        Detect toxicity using Perspective API.

        Args:
            texts: List of texts to analyze

        Returns:
            List of toxicity dicts
        """
        if not texts:
            return []

        import httpx

        results = []
        with httpx.Client(timeout=30.0) as client:
            for text in texts:
                try:
                    response = client.post(
                        self.ENDPOINT,
                        params={"key": self.api_key},
                        json={
                            "comment": {"text": text[:3000]},  # API limit
                            "languages": ["pt", "en"],
                            "requestedAttributes": {
                                "TOXICITY": {},
                                "SEVERE_TOXICITY": {},
                                "INSULT": {},
                                "THREAT": {},
                                "IDENTITY_ATTACK": {},
                            },
                        },
                    )

                    if response.status_code == 200:
                        data = response.json()
                        scores = data.get("attributeScores", {})

                        toxicity = (
                            scores.get("TOXICITY", {})
                            .get("summaryScore", {})
                            .get("value", 0)
                        )
                        results.append(
                            {
                                "is_toxic": toxicity > 0.5,
                                "label": "toxic" if toxicity > 0.5 else "non-toxic",
                                "score": toxicity,
                            }
                        )
                    else:
                        # Return neutral on API error
                        logger.warning(f"Perspective API error: {response.status_code}")
                        results.append(
                            {
                                "is_toxic": False,
                                "label": "unknown",
                                "score": 0.0,
                            }
                        )

                except Exception as e:
                    logger.warning(f"Perspective API request failed: {e}")
                    results.append(
                        {
                            "is_toxic": False,
                            "label": "unknown",
                            "score": 0.0,
                        }
                    )

        return results

    def is_available(self) -> bool:
        """Check if API key is available."""
        return bool(self.api_key)

    def cleanup(self) -> None:
        """No resources to release."""
        pass


class OpenAIModerationAnalyzer(Analyzer):
    """
    Toxicity analyzer using OpenAI Moderation API.

    Free with OpenAI API key, supports batching.
    """

    def __init__(self, api_key: str | None = None):
        """
        Initialize with OpenAI API key.

        Args:
            api_key: OpenAI API key (required)
        """
        self.api_key = api_key
        self._client = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.TOXICITY

    def _get_client(self):
        """Lazy load the OpenAI client."""
        if self._client is None:
            from openai import OpenAI

            self._client = OpenAI(api_key=self.api_key)
        return self._client

    def analyze(self, texts: list[str], **kwargs) -> list[dict]:
        """
        Detect toxicity using OpenAI Moderation API.

        Args:
            texts: List of texts to analyze

        Returns:
            List of toxicity dicts
        """
        if not texts:
            return []

        client = self._get_client()

        # OpenAI moderation supports batching
        response = client.moderations.create(input=texts)

        results = []
        for result in response.results:
            # Aggregate categories into toxicity score
            scores = result.category_scores
            toxicity = max(
                scores.hate,
                scores.harassment,
                scores.violence,
                scores.self_harm,
            )

            results.append(
                {
                    "is_toxic": result.flagged,
                    "label": "flagged" if result.flagged else "safe",
                    "score": toxicity,
                }
            )

        return results

    def is_available(self) -> bool:
        """Check if OpenAI API key is available."""
        return bool(self.api_key)

    def cleanup(self) -> None:
        """Release client resources."""
        self._client = None
