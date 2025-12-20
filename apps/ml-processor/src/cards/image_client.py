"""Client for card-image-generator service."""

import logging
from dataclasses import dataclass
from typing import Any

import httpx

logger = logging.getLogger(__name__)


@dataclass
class RenderResult:
    """Result from render request."""

    generated: int
    skipped: int
    failed: int
    results: list[dict[str, Any]]


class CardImageClient:
    """Client for card-image-generator service."""

    def __init__(
        self,
        base_url: str,
        api_key: str,
        timeout: float = 120.0,
    ):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.timeout = timeout

    def render_cards(
        self,
        chat_id: int,
        week_start: str,
        user_ids: list[int] | None = None,
        theme: str = "gaming",
        force_regenerate: bool = False,
    ) -> RenderResult:
        """
        Request card image rendering from the card-image-generator service.

        Args:
            chat_id: Chat ID
            week_start: Week start date (YYYY-MM-DD)
            user_ids: Optional list of specific user IDs
            theme: Template theme name
            force_regenerate: Regenerate even if images exist

        Returns:
            RenderResult with generation counts and details
        """
        with httpx.Client(timeout=self.timeout) as client:
            response = client.post(
                f"{self.base_url}/api/v1/render",
                json={
                    "chat_id": chat_id,
                    "week_start": week_start,
                    "user_ids": user_ids,
                    "theme": theme,
                    "force_regenerate": force_regenerate,
                },
                headers={"Authorization": f"Bearer {self.api_key}"},
            )
            response.raise_for_status()
            data = response.json()

            return RenderResult(
                generated=data.get("generated", 0),
                skipped=data.get("skipped", 0),
                failed=data.get("failed", 0),
                results=data.get("results", []),
            )

    def health_check(self) -> bool:
        """Check if the card-image-generator service is healthy."""
        try:
            with httpx.Client(timeout=5.0) as client:
                response = client.get(f"{self.base_url}/health")
                return response.status_code == 200
        except Exception as e:
            logger.warning(f"Card image generator health check failed: {e}")
            return False
