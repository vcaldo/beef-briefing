"""
OpenAI rate limiting for Tier 1 limits.

Implements per-model token bucket rate limiting for:
- gpt-4o-mini: 200,000 TPM, 500 RPM
- text-embedding-3-small: 1,000,000 TPM, 3,000 RPM
- omni-moderation-latest: 10,000 TPM, 500 RPM
"""

from __future__ import annotations

import logging
import time
from dataclasses import dataclass, field
from threading import Lock
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from openai import OpenAI

logger = logging.getLogger(__name__)


class RateLimitExceededError(Exception):
    """Raised when rate limit timeout is exceeded."""

    pass


@dataclass
class ModelLimits:
    """Rate limits for a specific OpenAI model."""

    model_name: str
    tokens_per_minute: int
    requests_per_minute: int


# Default Tier 1 limits
DEFAULT_MODEL_LIMITS = {
    "gpt-4o-mini": ModelLimits(
        model_name="gpt-4o-mini",
        tokens_per_minute=200_000,
        requests_per_minute=500,
    ),
    "text-embedding-3-small": ModelLimits(
        model_name="text-embedding-3-small",
        tokens_per_minute=1_000_000,
        requests_per_minute=3_000,
    ),
    "omni-moderation-latest": ModelLimits(
        model_name="omni-moderation-latest",
        tokens_per_minute=10_000,
        requests_per_minute=500,
    ),
}


@dataclass
class TokenBucket:
    """
    Thread-safe token bucket for rate limiting.

    Tokens refill continuously based on elapsed time.
    """

    capacity: int
    tokens: float = field(init=False)
    refill_rate: float = field(init=False)
    last_refill: float = field(init=False)
    lock: Lock = field(default_factory=Lock, repr=False)

    def __post_init__(self):
        self.tokens = float(self.capacity)
        self.refill_rate = self.capacity / 60.0  # Tokens per second
        self.last_refill = time.monotonic()

    def _refill(self) -> None:
        """Refill tokens based on elapsed time. Must be called with lock held."""
        now = time.monotonic()
        elapsed = now - self.last_refill
        self.tokens = min(self.capacity, self.tokens + elapsed * self.refill_rate)
        self.last_refill = now

    def acquire(self, count: int, timeout: float = 60.0) -> bool:
        """
        Acquire tokens, waiting if necessary.

        Args:
            count: Number of tokens to acquire
            timeout: Maximum time to wait in seconds

        Returns:
            True if tokens acquired, False if timeout exceeded
        """
        if count > self.capacity:
            logger.warning(
                f"Requested {count} tokens exceeds bucket capacity {self.capacity}. "
                "Request may not complete."
            )

        deadline = time.monotonic() + timeout

        while True:
            with self.lock:
                self._refill()

                if self.tokens >= count:
                    self.tokens -= count
                    return True

                # Calculate wait time for enough tokens
                needed = count - self.tokens
                wait_time = needed / self.refill_rate

            # Check timeout
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                return False

            # Wait (outside lock) in small chunks
            sleep_time = min(wait_time, remaining, 1.0)
            time.sleep(sleep_time)

    def consume(self, count: int) -> None:
        """
        Consume tokens after a request completes.

        Used for post-facto token accounting based on actual usage.
        """
        with self.lock:
            self.tokens = max(0, self.tokens - count)

    def refund(self, count: int) -> None:
        """Refund tokens (e.g., if request failed)."""
        with self.lock:
            self.tokens = min(self.capacity, self.tokens + count)


class OpenAIRateLimiter:
    """
    Centralized rate limiter for OpenAI models.

    Coordinates rate limiting across all analyzers sharing the same model.
    Thread-safe for potential concurrent use.
    """

    def __init__(self, limits: dict[str, ModelLimits] | None = None):
        """
        Initialize the rate limiter.

        Args:
            limits: Custom model limits. Uses Tier 1 defaults if not provided.
        """
        self._limits = limits or DEFAULT_MODEL_LIMITS
        self._rpm_buckets: dict[str, TokenBucket] = {}
        self._tpm_buckets: dict[str, TokenBucket] = {}
        self._lock = Lock()

    def _get_buckets(self, model: str) -> tuple[TokenBucket, TokenBucket]:
        """Get or create token buckets for a model."""
        with self._lock:
            if model not in self._rpm_buckets:
                limits = self._limits.get(model)
                if not limits:
                    # Unknown model - use conservative defaults
                    logger.warning(
                        f"Unknown model '{model}', using conservative limits "
                        "(10,000 TPM, 100 RPM)"
                    )
                    limits = ModelLimits(model, 10_000, 100)

                self._rpm_buckets[model] = TokenBucket(limits.requests_per_minute)
                self._tpm_buckets[model] = TokenBucket(limits.tokens_per_minute)
                logger.info(
                    f"Created rate limiter for {model}: "
                    f"{limits.tokens_per_minute:,} TPM, {limits.requests_per_minute} RPM"
                )

            return self._rpm_buckets[model], self._tpm_buckets[model]

    def acquire(
        self,
        model: str,
        estimated_tokens: int,
        timeout: float = 60.0,
    ) -> bool:
        """
        Acquire capacity for a request.

        Args:
            model: OpenAI model name
            estimated_tokens: Estimated total tokens for the request
            timeout: Maximum time to wait for capacity

        Returns:
            True if capacity acquired, False if timeout
        """
        start = time.monotonic()
        rpm_bucket, tpm_bucket = self._get_buckets(model)

        # First acquire RPM (always 1 request)
        if not rpm_bucket.acquire(1, timeout=timeout):
            logger.warning(f"Rate limit timeout (RPM) for {model}")
            return False

        # Calculate remaining timeout
        elapsed = time.monotonic() - start
        remaining_timeout = max(0, timeout - elapsed)

        # Then acquire TPM
        if not tpm_bucket.acquire(estimated_tokens, timeout=remaining_timeout):
            # Return the RPM token if TPM fails
            rpm_bucket.refund(1)
            logger.warning(f"Rate limit timeout (TPM) for {model}")
            return False

        wait_time = time.monotonic() - start
        if wait_time > 0.1:  # Log waits > 100ms
            logger.info(
                f"Rate limit wait: model={model}, tokens={estimated_tokens}, "
                f"wait_ms={wait_time * 1000:.0f}"
            )

        return True

    def record_usage(
        self, model: str, actual_tokens: int, estimated_tokens: int
    ) -> None:
        """
        Record actual token usage after request completes.

        Adjusts TPM bucket if actual usage differed from estimate.
        """
        _, tpm_bucket = self._get_buckets(model)

        diff = actual_tokens - estimated_tokens
        if diff > 0:
            # Used more than estimated - consume additional
            tpm_bucket.consume(diff)
        elif diff < 0:
            # Used less than estimated - refund
            tpm_bucket.refund(abs(diff))


class RateLimitedOpenAI:
    """
    Wrapper for OpenAI client that enforces rate limits.

    Wraps common API methods with pre-request rate limiting and
    post-request usage tracking.
    """

    def __init__(
        self,
        client: "OpenAI",
        rate_limiter: OpenAIRateLimiter,
        default_model: str,
        timeout: float = 120.0,
    ):
        """
        Initialize the rate-limited wrapper.

        Args:
            client: Underlying OpenAI client
            rate_limiter: Shared rate limiter instance
            default_model: Default model for this wrapper
            timeout: Max time to wait for rate limit capacity
        """
        self._client = client
        self._rate_limiter = rate_limiter
        self._default_model = default_model
        self._timeout = timeout

    @staticmethod
    def _estimate_chat_tokens(messages: list[dict]) -> int:
        """Estimate token count for chat messages."""
        # Rough estimation: 1 token per 4 characters
        total_chars = sum(len(str(m.get("content", ""))) for m in messages)
        # Add overhead for message structure and estimated response
        return (total_chars // 4) + (len(messages) * 4) + 200

    @staticmethod
    def _estimate_text_tokens(texts: list[str]) -> int:
        """Estimate token count for a list of texts."""
        return sum(len(text) // 4 + 1 for text in texts)

    def chat_completions_create(
        self,
        *,
        messages: list[dict],
        model: str | None = None,
        **kwargs,
    ):
        """Rate-limited chat completions."""
        model = model or self._default_model
        estimated_tokens = self._estimate_chat_tokens(messages)

        if not self._rate_limiter.acquire(model, estimated_tokens, self._timeout):
            raise RateLimitExceededError(
                f"Timeout waiting for {model} capacity "
                f"(estimated {estimated_tokens} tokens)"
            )

        try:
            response = self._client.chat.completions.create(
                messages=messages,
                model=model,
                **kwargs,
            )

            # Record actual usage
            if response.usage:
                self._rate_limiter.record_usage(
                    model,
                    response.usage.total_tokens,
                    estimated_tokens,
                )

            return response

        except Exception:
            # On error, refund estimated tokens
            self._rate_limiter.record_usage(model, 0, estimated_tokens)
            raise

    def embeddings_create(
        self, *, input: list[str], model: str | None = None, **kwargs
    ):
        """Rate-limited embeddings."""
        model = model or self._default_model
        estimated_tokens = self._estimate_text_tokens(input)

        if not self._rate_limiter.acquire(model, estimated_tokens, self._timeout):
            raise RateLimitExceededError(
                f"Timeout waiting for {model} capacity "
                f"(estimated {estimated_tokens} tokens)"
            )

        try:
            response = self._client.embeddings.create(
                input=input, model=model, **kwargs
            )

            if response.usage:
                self._rate_limiter.record_usage(
                    model,
                    response.usage.total_tokens,
                    estimated_tokens,
                )

            return response

        except Exception:
            self._rate_limiter.record_usage(model, 0, estimated_tokens)
            raise

    def moderations_create(self, *, input: list[str], model: str | None = None, **kwargs):
        """Rate-limited moderations."""
        model = model or "omni-moderation-latest"
        estimated_tokens = self._estimate_text_tokens(input)

        if not self._rate_limiter.acquire(model, estimated_tokens, self._timeout):
            raise RateLimitExceededError(
                f"Timeout waiting for {model} capacity "
                f"(estimated {estimated_tokens} tokens)"
            )

        try:
            return self._client.moderations.create(input=input, model=model, **kwargs)

        except Exception:
            self._rate_limiter.record_usage(model, 0, estimated_tokens)
            raise
