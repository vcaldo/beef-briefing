"""
Sentiment analysis implementations.

Provides local HuggingFace-based and API-based sentiment analyzers.
"""

from __future__ import annotations

import logging
from typing import TYPE_CHECKING, Any

from src.analyzers.base import Analyzer, AnalysisType

if TYPE_CHECKING:
    from src.ratelimit import OpenAIRateLimiter

logger = logging.getLogger(__name__)


class LocalSentimentAnalyzer(Analyzer):
    """
    Sentiment analyzer using HuggingFace transformers.

    Default model: lxyuan/distilbert-base-multilingual-cased-sentiments-student
    - Languages: 6 languages including Portuguese
    - Memory: ~270MB GPU
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
        self._pipeline = None  # Lazy loaded

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.SENTIMENT

    def _load_model(self):
        """Lazy load the model on first use."""
        if self._pipeline is None:
            from transformers import pipeline

            self._pipeline = pipeline(
                "text-classification",
                model=self.model_name,
                top_k=None,  # Return all scores
                device=0 if self.device == "cuda" else -1,
            )
            logger.info(f"Sentiment model loaded: {self.model_name}")

    def analyze(self, texts: list[str], batch_size: int = 32, **kwargs) -> list[dict]:
        """
        Analyze sentiment for a batch of texts.

        Args:
            texts: List of texts to analyze
            batch_size: Processing batch size

        Returns:
            List of dicts with:
                - label: str ('positive', 'neutral', 'negative')
                - score_positive: float
                - score_neutral: float
                - score_negative: float
                - confidence: float
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
            # Convert list of {label, score} to dict
            scores = {r["label"].lower(): r["score"] for r in result}

            # Get the highest scoring label
            label = max(scores, key=scores.get)

            processed.append(
                {
                    "label": label,
                    "score_positive": scores.get("positive", 0.0),
                    "score_neutral": scores.get("neutral", 0.0),
                    "score_negative": scores.get("negative", 0.0),
                    "confidence": max(scores.values()),
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

    def get_model_name(self) -> str:
        """Return the HuggingFace model name."""
        return self.model_name

    def cleanup(self) -> None:
        """Release model resources."""
        if self._pipeline is not None:
            del self._pipeline
            self._pipeline = None
            logger.info("Sentiment model unloaded")


class OpenAISentimentAnalyzer(Analyzer):
    """
    Sentiment analyzer using OpenAI API.

    Uses gpt-4o-mini for cost-effective batch sentiment analysis.
    """

    MODEL_NAME = "gpt-4o-mini"

    def __init__(
        self,
        api_key: str | None = None,
        max_retries: int = 5,
        timeout: float = 60.0,
        rate_limiter: OpenAIRateLimiter | None = None,
        rate_limit_timeout: float = 120.0,
    ):
        """
        Initialize with OpenAI API key.

        Args:
            api_key: OpenAI API key (required)
            max_retries: Maximum retry attempts for rate limits/errors
            timeout: Request timeout in seconds
            rate_limiter: Shared rate limiter for OpenAI requests
            rate_limit_timeout: Max time to wait for rate limit capacity
        """
        self.api_key = api_key
        self.max_retries = max_retries
        self.timeout = timeout
        self._rate_limiter = rate_limiter
        self._rate_limit_timeout = rate_limit_timeout
        self._client = None
        self._last_usage: dict | None = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.SENTIMENT

    def _get_client(self):
        """Lazy load the OpenAI client, optionally with rate limiting."""
        if self._client is None:
            from openai import OpenAI

            base_client = OpenAI(
                api_key=self.api_key,
                max_retries=self.max_retries,
                timeout=self.timeout,
            )

            if self._rate_limiter:
                from src.ratelimit import RateLimitedOpenAI

                self._client = RateLimitedOpenAI(
                    base_client,
                    self._rate_limiter,
                    self.MODEL_NAME,
                    self._rate_limit_timeout,
                )
            else:
                self._client = base_client

        return self._client

    def analyze(self, texts: list[str], **kwargs) -> list[dict]:
        """
        Analyze sentiment using OpenAI API.

        Args:
            texts: List of texts to analyze

        Returns:
            List of sentiment dicts
        """
        if not texts:
            return []

        client = self._get_client()

        # Batch texts into a single prompt for efficiency
        prompt = """Analyze the sentiment of each message. Return a JSON object with a "results" array.
Each result should have:
- positive: float 0-1
- negative: float 0-1
- neutral: float 0-1
- label: "positive" | "negative" | "neutral"

Messages:
"""
        for i, text in enumerate(texts):
            prompt += f"{i + 1}. {text[:500]}\n"  # Truncate long texts

        messages = [{"role": "user", "content": prompt}]

        # Use wrapper method if rate-limited, otherwise direct client
        if hasattr(client, "chat_completions_create"):
            response = client.chat_completions_create(
                model=self.MODEL_NAME,
                messages=messages,
                response_format={"type": "json_object"},
                temperature=0,
            )
        else:
            response = client.chat.completions.create(
                model=self.MODEL_NAME,
                messages=messages,
                response_format={"type": "json_object"},
                temperature=0,
            )

        # Capture token usage for monitoring
        if response.usage:
            self._last_usage = {
                "prompt_tokens": response.usage.prompt_tokens,
                "completion_tokens": response.usage.completion_tokens,
                "total_tokens": response.usage.total_tokens,
                "model": self.MODEL_NAME,
            }

        import json

        data = json.loads(response.choices[0].message.content)
        results = data.get("results", [])

        # Normalize to our format
        processed = []
        for result in results:
            processed.append(
                {
                    "label": result.get("label", "neutral"),
                    "score_positive": result.get("positive", 0.0),
                    "score_neutral": result.get("neutral", 0.0),
                    "score_negative": result.get("negative", 0.0),
                    "confidence": max(
                        result.get("positive", 0),
                        result.get("neutral", 0),
                        result.get("negative", 0),
                    ),
                }
            )

        # Pad if API returned fewer results
        while len(processed) < len(texts):
            processed.append(
                {
                    "label": "neutral",
                    "score_positive": 0.0,
                    "score_neutral": 1.0,
                    "score_negative": 0.0,
                    "confidence": 1.0,
                }
            )

        return processed

    def is_available(self) -> bool:
        """Check if OpenAI API key is available."""
        return bool(self.api_key)

    def get_model_name(self) -> str:
        """Return the OpenAI model name."""
        return self.MODEL_NAME

    def is_local_provider(self) -> bool:
        """Return False - this uses OpenAI API."""
        return False

    def cleanup(self) -> None:
        """Release client resources."""
        self._client = None
        self._last_usage = None


class AnthropicSentimentAnalyzer(Analyzer):
    """
    Sentiment analyzer using Anthropic Claude API.

    Uses claude-3-haiku for cost-effective batch sentiment analysis.
    """

    MODEL_NAME = "claude-3-haiku-20240307"

    def __init__(self, api_key: str | None = None):
        """
        Initialize with Anthropic API key.

        Args:
            api_key: Anthropic API key (required)
        """
        self.api_key = api_key
        self._client = None
        self._last_usage: dict | None = None

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.SENTIMENT

    def _get_client(self):
        """Lazy load the Anthropic client."""
        if self._client is None:
            from anthropic import Anthropic

            self._client = Anthropic(api_key=self.api_key)
        return self._client

    def analyze(self, texts: list[str], **kwargs) -> list[dict]:
        """
        Analyze sentiment using Anthropic API.

        Args:
            texts: List of texts to analyze

        Returns:
            List of sentiment dicts
        """
        if not texts:
            return []

        client = self._get_client()

        prompt = """Analyze the sentiment of each message. Return a JSON array where each object has:
- positive: float between 0 and 1
- negative: float between 0 and 1
- neutral: float between 0 and 1
- label: the dominant sentiment ("positive", "negative", or "neutral")

Messages to analyze:
"""
        for i, text in enumerate(texts):
            prompt += f"{i + 1}. {text[:500]}\n"

        response = client.messages.create(
            model=self.MODEL_NAME,
            max_tokens=1024,
            messages=[{"role": "user", "content": prompt}],
        )

        # Capture token usage for monitoring
        if response.usage:
            self._last_usage = {
                "prompt_tokens": response.usage.input_tokens,
                "completion_tokens": response.usage.output_tokens,
                "total_tokens": response.usage.input_tokens + response.usage.output_tokens,
                "model": self.MODEL_NAME,
            }

        import json

        content = response.content[0].text
        # Find JSON array in response
        start = content.find("[")
        end = content.rfind("]") + 1

        if start >= 0 and end > start:
            results = json.loads(content[start:end])
        else:
            results = []

        # Normalize to our format
        processed = []
        for result in results:
            processed.append(
                {
                    "label": result.get("label", "neutral"),
                    "score_positive": result.get("positive", 0.0),
                    "score_neutral": result.get("neutral", 0.0),
                    "score_negative": result.get("negative", 0.0),
                    "confidence": max(
                        result.get("positive", 0),
                        result.get("neutral", 0),
                        result.get("negative", 0),
                    ),
                }
            )

        # Pad if API returned fewer results
        while len(processed) < len(texts):
            processed.append(
                {
                    "label": "neutral",
                    "score_positive": 0.0,
                    "score_neutral": 1.0,
                    "score_negative": 0.0,
                    "confidence": 1.0,
                }
            )

        return processed

    def is_available(self) -> bool:
        """Check if Anthropic API key is available."""
        return bool(self.api_key)

    def get_model_name(self) -> str:
        """Return the Anthropic model name."""
        return self.MODEL_NAME

    def is_local_provider(self) -> bool:
        """Return False - this uses Anthropic API."""
        return False

    def cleanup(self) -> None:
        """Release client resources."""
        self._client = None
        self._last_usage = None
