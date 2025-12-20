"""
Humor detection implementations.

Provides local heuristic-based and API-based humor detectors for Portuguese text.
"""

from __future__ import annotations

import logging
import re
from typing import TYPE_CHECKING, Any

from src.analyzers.base import Analyzer, AnalysisType

if TYPE_CHECKING:
    from src.ratelimit import OpenAIRateLimiter

logger = logging.getLogger(__name__)


class LocalHumorDetector(Analyzer):
    """
    Humor detector using heuristic signals.

    Detects humor in Brazilian Portuguese using:
    - Laugh patterns (kkkk, hahaha, rsrsrs, huahua)
    - Laugh emojis
    - Combined signal scoring

    This is a lightweight approach that doesn't require ML models.
    """

    def __init__(self):
        """Initialize the humor detector with pattern matchers."""
        # Brazilian Portuguese laugh patterns
        self.laugh_patterns = [
            (re.compile(r"k{3,}", re.IGNORECASE), "laugh_pattern"),
            (re.compile(r"(ha){2,}", re.IGNORECASE), "laugh_pattern"),
            (re.compile(r"(rs){2,}", re.IGNORECASE), "laugh_pattern"),
            (re.compile(r"(hua){2,}", re.IGNORECASE), "laugh_pattern"),
            (re.compile(r"(he){3,}", re.IGNORECASE), "laugh_pattern"),
            (re.compile(r"(hi){3,}", re.IGNORECASE), "laugh_pattern"),
        ]

        # Laugh emojis
        self.laugh_emojis = frozenset(["😂", "🤣", "😆", "😹", "🤭", "😁"])

        # Weights for different signals
        self.weights = {
            "laugh_pattern": 0.4,
            "laugh_emoji": 0.3,
            "multiple_emojis": 0.2,
        }

        # Threshold for is_humorous
        self.threshold = 0.4

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.HUMOR

    def _detect_single(self, text: str) -> dict:
        """Detect humor in a single text."""
        score = 0.0
        signals = []
        humor_type = None

        # Signal 1: Laugh patterns in text
        for pattern, signal_type in self.laugh_patterns:
            if pattern.search(text):
                score += self.weights["laugh_pattern"]
                signals.append(signal_type)
                humor_type = "laugh_pattern"
                break  # Only count first pattern match

        # Signal 2: Laugh emojis
        emoji_count = sum(1 for char in text if char in self.laugh_emojis)
        if emoji_count > 0:
            score += min(self.weights["laugh_emoji"], emoji_count * 0.15)
            signals.append("laugh_emoji")
            if not humor_type:
                humor_type = "emoji"

        # Signal 3: Multiple laugh emojis (bonus)
        if emoji_count >= 2:
            score += self.weights["multiple_emojis"]
            signals.append("multiple_emojis")

        # Cap score at 1.0
        score = min(1.0, score)

        # Determine is_humorous
        is_humorous = score >= self.threshold

        # Default humor_type if humorous but no type assigned
        if is_humorous and not humor_type:
            humor_type = "unknown"

        return {
            "is_humorous": is_humorous,
            "humor_type": humor_type,
            "score": round(score, 4),
        }

    def analyze(self, texts: list[str], **kwargs) -> list[dict]:
        """
        Analyze humor for a batch of texts.

        Args:
            texts: List of texts to analyze

        Returns:
            List of dicts with:
                - is_humorous: bool
                - humor_type: str | None ('laugh_pattern', 'emoji', etc.)
                - score: float (0-1)
        """
        if not texts:
            return []

        return [self._detect_single(text) for text in texts]

    def is_available(self) -> bool:
        """Always available since we use only heuristics."""
        return True

    def get_model_name(self) -> str:
        """Return the model name (heuristic-based, no ML model)."""
        return "heuristic"

    def cleanup(self) -> None:
        """No resources to clean up."""
        pass


class OpenAIHumorDetector(Analyzer):
    """
    Humor detector using OpenAI API.

    Uses gpt-4o-mini for humor detection in Portuguese text.
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
        return AnalysisType.HUMOR

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
        Analyze humor using OpenAI API.

        Args:
            texts: List of texts to analyze

        Returns:
            List of humor dicts
        """
        if not texts:
            return []

        client = self._get_client()

        # Batch texts into a single prompt for efficiency
        prompt = """Analyze if each message is humorous (contains jokes, wit, or intentional humor).
Return a JSON object with a "results" array.
Each result should have:
- is_humorous: boolean (true if the message is intentionally funny)
- humor_type: string or null ("joke", "sarcasm", "wordplay", "irony", "absurd", or null)
- score: float 0-1 (confidence of humor detection)

Messages (in Portuguese):
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
                    "is_humorous": result.get("is_humorous", False),
                    "humor_type": result.get("humor_type"),
                    "score": float(result.get("score", 0.0)),
                }
            )

        # Pad if API returned fewer results
        while len(processed) < len(texts):
            processed.append(
                {
                    "is_humorous": False,
                    "humor_type": None,
                    "score": 0.0,
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
