"""
Humor detection implementations.

Provides local heuristic-based and API-based humor detectors for Portuguese text.
"""

import logging
import re
from typing import Any

from src.analyzers.base import Analyzer, AnalysisType

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

    def cleanup(self) -> None:
        """No resources to clean up."""
        pass


class OpenAIHumorDetector(Analyzer):
    """
    Humor detector using OpenAI API.

    Uses gpt-4o-mini for humor detection in Portuguese text.
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
        return AnalysisType.HUMOR

    def _get_client(self):
        """Lazy load the OpenAI client."""
        if self._client is None:
            from openai import OpenAI

            self._client = OpenAI(api_key=self.api_key)
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

        response = client.chat.completions.create(
            model="gpt-4o-mini",
            messages=[{"role": "user", "content": prompt}],
            response_format={"type": "json_object"},
            temperature=0,
        )

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

    def cleanup(self) -> None:
        """Release client resources."""
        self._client = None
