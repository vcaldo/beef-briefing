"""
HTTP client for communicating with the api-service.
"""

import logging
from dataclasses import dataclass
from typing import Optional

import httpx

logger = logging.getLogger(__name__)


@dataclass
class Message:
    """Represents a message from the API."""

    id: int
    message_id: int
    chat_id: int
    user_id: Optional[int]
    text: str


@dataclass
class MessagesResponse:
    """Response from GET /api/v1/ml/messages."""

    messages: list[Message]
    has_more: bool


@dataclass
class SentimentResult:
    """Sentiment analysis result for a message."""

    label: str
    scores: dict[str, float]


@dataclass
class ToxicityResult:
    """Toxicity detection result for a message."""

    is_toxic: bool
    label: str
    score: float


@dataclass
class MLResult:
    """Combined ML result for a message."""

    message_id: int
    chat_id: int
    sentiment: Optional[SentimentResult] = None
    toxicity: Optional[ToxicityResult] = None


class APIClient:
    """Client for the api-service ML endpoints."""

    def __init__(self, base_url: str, api_key: str, timeout: float = 30.0):
        """
        Initialize the API client.

        Args:
            base_url: Base URL of the api-service (e.g., http://localhost:8080)
            api_key: API key for authentication
            timeout: Request timeout in seconds
        """
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout
        self._client = httpx.Client(
            base_url=self.base_url,
            headers={"Authorization": f"Bearer {api_key}"},
            timeout=timeout,
        )

    def close(self):
        """Close the HTTP client."""
        self._client.close()

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.close()

    def get_messages(self, limit: int = 500) -> MessagesResponse:
        """
        Fetch unprocessed messages from the API.

        Args:
            limit: Maximum number of messages to fetch (max 1000)

        Returns:
            MessagesResponse with messages and has_more flag
        """
        response = self._client.get(
            "/api/v1/ml/messages",
            params={"limit": min(limit, 1000)},
        )
        response.raise_for_status()

        data = response.json()
        messages = [
            Message(
                id=m["id"],
                message_id=m["message_id"],
                chat_id=m["chat_id"],
                user_id=m.get("user_id"),
                text=m["text"],
            )
            for m in data.get("messages", [])
        ]

        return MessagesResponse(
            messages=messages,
            has_more=data.get("has_more", False),
        )

    def post_results(
        self, results: list[MLResult], processor_version: str
    ) -> dict:
        """
        Submit ML analysis results to the API.

        Args:
            results: List of ML results to submit
            processor_version: Version identifier for the processor

        Returns:
            Response dict with status and count
        """
        payload = {
            "results": [
                {
                    "message_id": r.message_id,
                    "chat_id": r.chat_id,
                    "sentiment": (
                        {"label": r.sentiment.label, "scores": r.sentiment.scores}
                        if r.sentiment
                        else None
                    ),
                    "toxicity": (
                        {
                            "is_toxic": r.toxicity.is_toxic,
                            "label": r.toxicity.label,
                            "score": r.toxicity.score,
                        }
                        if r.toxicity
                        else None
                    ),
                }
                for r in results
            ],
            "processor_version": processor_version,
        }

        response = self._client.post("/api/v1/ml/results", json=payload)
        response.raise_for_status()

        return response.json()

    def get_status(self) -> dict:
        """
        Get ML processing statistics.

        Returns:
            Dict with processing stats
        """
        response = self._client.get("/api/v1/ml/status")
        response.raise_for_status()

        return response.json()
