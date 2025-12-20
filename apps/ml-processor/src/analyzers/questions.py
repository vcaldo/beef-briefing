"""
Question detection implementations.

Provides local heuristic + zero-shot and API-based question classifiers.
"""

from __future__ import annotations

import logging
import re
from typing import TYPE_CHECKING, Any

from src.analyzers.base import Analyzer, AnalysisType

if TYPE_CHECKING:
    from src.ratelimit import OpenAIRateLimiter

logger = logging.getLogger(__name__)


class LocalQuestionClassifier(Analyzer):
    """
    Question classifier using heuristics + zero-shot classification.

    Detection logic:
    1. If text ends with '?' -> is_question=True, score=0.95 (no model call)
    2. If text starts with question word -> is_question=True, score=0.85 (no model call)
    3. Otherwise -> run facebook/bart-large-mnli zero-shot classification

    This saves model inference for obvious cases while handling ambiguous text.
    """

    def __init__(
        self,
        model_name: str = "facebook/bart-large-mnli",
        device: str = "cuda",
    ):
        """
        Initialize the question classifier.

        Args:
            model_name: HuggingFace model ID for zero-shot classification
            device: Device to run on ('cuda' or 'cpu')
        """
        self.model_name = model_name
        self.device = device
        self._pipeline = None  # Lazy loaded

        # Portuguese question starters
        self.question_starters = [
            "quem",
            "qual",
            "quais",
            "quando",
            "onde",
            "como",
            "por que",
            "porque",
            "porquê",
            "o que",
            "o q",
            "oq",
            "quanto",
            "quantos",
            "quantas",
            "será",
            "sera",
            "cadê",
            "cade",
            "alguém",
            "alguem",
            "algum",
            "alguma",
        ]

        # Question type keywords for classification
        self.question_type_patterns = {
            "yes_no": re.compile(
                r"^(é|e|está|esta|foi|será|sera|tem|pode|vai|quer|sabe|conhece)\s",
                re.IGNORECASE,
            ),
            "factual": re.compile(
                r"^(quem|qual|quais|quando|onde|quanto|quantos|quantas)\s",
                re.IGNORECASE,
            ),
            "opinion": re.compile(
                r"(acha|acham|pensa|pensam|opina|opinam|prefere|preferem)\b",
                re.IGNORECASE,
            ),
            "rhetorical": re.compile(
                r"(né|ne|hein|ein|não é|nao e)\s*\??$",
                re.IGNORECASE,
            ),
            "clarification": re.compile(
                r"^(como assim|o que você quis dizer|não entendi|nao entendi|hã|ha)\b",
                re.IGNORECASE,
            ),
        }

    @property
    def analysis_type(self) -> AnalysisType:
        return AnalysisType.QUESTIONS

    def _load_model(self):
        """Lazy load the zero-shot classification model."""
        if self._pipeline is None:
            from transformers import pipeline

            self._pipeline = pipeline(
                "zero-shot-classification",
                model=self.model_name,
                device=0 if self.device == "cuda" else -1,
            )
            logger.info(f"Question classifier model loaded: {self.model_name}")

    def _classify_question_type(self, text: str) -> str:
        """Classify the type of question based on patterns."""
        for qtype, pattern in self.question_type_patterns.items():
            if pattern.search(text):
                return qtype
        return "factual"  # Default

    def _detect_single(self, text: str, use_model: bool = True) -> dict:
        """Detect if a single text is a question."""
        text_stripped = text.strip()

        # Heuristic 1: Ends with question mark (high confidence)
        if text_stripped.endswith("?"):
            return {
                "is_question": True,
                "question_type": self._classify_question_type(text),
                "score": 0.95,
            }

        # Heuristic 2: Starts with question word (high confidence)
        text_lower = text_stripped.lower()
        for starter in self.question_starters:
            if text_lower.startswith(starter + " ") or text_lower.startswith(
                starter + ","
            ):
                return {
                    "is_question": True,
                    "question_type": self._classify_question_type(text),
                    "score": 0.85,
                }

        # Heuristic 3: Use zero-shot model for ambiguous cases
        if use_model:
            self._load_model()

            result = self._pipeline(
                text_stripped,
                candidate_labels=["question", "statement"],
                hypothesis_template="This text is a {}.",
            )

            is_question = result["labels"][0] == "question"
            score = result["scores"][0] if is_question else 1.0 - result["scores"][0]

            return {
                "is_question": is_question and score > 0.5,
                "question_type": self._classify_question_type(text) if is_question else None,
                "score": round(score, 4),
            }

        # No model, not detected as question
        return {
            "is_question": False,
            "question_type": None,
            "score": 0.0,
        }

    def analyze(
        self, texts: list[str], batch_size: int = 32, **kwargs
    ) -> list[dict]:
        """
        Analyze if texts are questions.

        Args:
            texts: List of texts to analyze
            batch_size: Processing batch size

        Returns:
            List of dicts with:
                - is_question: bool
                - question_type: str | None ('factual', 'opinion', 'rhetorical', etc.)
                - score: float (0-1)
        """
        if not texts:
            return []

        results = []
        for text in texts:
            results.append(self._detect_single(text, use_model=True))

        return results

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
            logger.info("Question classifier model unloaded")


class OpenAIQuestionClassifier(Analyzer):
    """
    Question classifier using OpenAI API.

    Uses gpt-4o-mini for question detection and classification.
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
        return AnalysisType.QUESTIONS

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
        Analyze if texts are questions using OpenAI API.

        Args:
            texts: List of texts to analyze

        Returns:
            List of question classification dicts
        """
        if not texts:
            return []

        client = self._get_client()

        # Batch texts into a single prompt for efficiency
        prompt = """Analyze if each message is a question.
Return a JSON object with a "results" array.
Each result should have:
- is_question: boolean (true if the message is asking something)
- question_type: string or null ("factual", "opinion", "rhetorical", "clarification", "yes_no", or null)
- score: float 0-1 (confidence of classification)

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
                    "is_question": result.get("is_question", False),
                    "question_type": result.get("question_type"),
                    "score": float(result.get("score", 0.0)),
                }
            )

        # Pad if API returned fewer results
        while len(processed) < len(texts):
            processed.append(
                {
                    "is_question": False,
                    "question_type": None,
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
