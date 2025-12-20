"""
API client for fetching user cards from api-service.

Provides CardClient class for fetching weekly user personality cards
with stats, trends, and batch support for admin dropdown.
"""

import logging
from typing import Optional

import httpx

logger = logging.getLogger(__name__)


class CardClient:
    """
    Client for fetching user cards from api-service.

    Handles individual and batch card lookups with graceful error handling.
    Returns None for missing cards instead of raising exceptions.
    """

    def __init__(self, base_url: str, api_key: str, timeout: float = 5.0):
        """
        Initialize the CardClient.

        Args:
            base_url: Base URL of the API service (e.g., http://api-service:8080)
            api_key: API key for authentication
            timeout: Request timeout in seconds
        """
        self._base_url = base_url.rstrip("/") if base_url else ""
        self._api_key = api_key
        self._timeout = timeout
        self._client: httpx.Client | None = None

    def _get_client(self) -> httpx.Client:
        """Get or create the HTTP client."""
        if self._client is None:
            self._client = httpx.Client(
                base_url=self._base_url,
                headers={"Authorization": f"Bearer {self._api_key}"},
                timeout=self._timeout,
            )
        return self._client

    def get_user_card(
        self, user_id: int, chat_id: int, week: str | None = None
    ) -> Optional[dict]:
        """
        Get a user's card for a specific week.

        Args:
            user_id: Telegram user ID
            chat_id: Telegram chat ID
            week: Week start date (YYYY-MM-DD). Defaults to latest week.

        Returns:
            Card data dict or None if not found or on error.
            Structure: {
                "card": {
                    "user_id": int,
                    "week_start": str,
                    "week_end": str,
                    "stats": {...},
                    "trends": {...},
                    "messages_analyzed": int
                },
                "user": {
                    "id": int,
                    "first_name": str,
                    "last_name": str,
                    "username": str
                }
            }
        """
        if not self._api_key or not self._base_url:
            return None

        try:
            client = self._get_client()
            params = {"chat_id": chat_id}
            if week:
                params["week"] = week

            response = client.get(
                f"/api/v1/cards/{user_id}",
                params=params,
            )
            if response.status_code == 404:
                return None
            response.raise_for_status()
            return response.json()
        except Exception as e:
            logger.warning(f"Failed to fetch card for user {user_id}: {e}")
            return None

    def get_chat_cards(
        self,
        chat_id: int,
        sort_by: str = "mood",
        order: str = "desc",
        limit: int = 100,
    ) -> Optional[dict]:
        """
        Get all user cards for a chat (for admin dropdown).

        Args:
            chat_id: Telegram chat ID
            sort_by: Sort field (mood, influence, activity, reactions)
            order: Sort order (asc, desc)
            limit: Maximum number of cards to return

        Returns:
            Response dict with cards list or None on error.
            Structure: {
                "cards": [{
                    "user_id": int,
                    "user": {...},
                    "week_start": str,
                    "stats": {...},
                    "trends": {...},
                    "rank": int
                }, ...],
                "pagination": {...},
                "metadata": {...}
            }
        """
        if not self._api_key or not self._base_url:
            return None

        try:
            client = self._get_client()
            response = client.get(
                "/api/v1/cards",
                params={
                    "chat_id": chat_id,
                    "sort_by": sort_by,
                    "order": order,
                    "limit": limit,
                },
            )
            if response.status_code == 404:
                return None
            response.raise_for_status()
            return response.json()
        except Exception as e:
            logger.warning(f"Failed to fetch cards for chat {chat_id}: {e}")
            return None

    def close(self):
        """Close the HTTP client and release resources."""
        if self._client:
            self._client.close()
            self._client = None


__all__ = ["CardClient"]
