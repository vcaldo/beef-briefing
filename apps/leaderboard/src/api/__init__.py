"""
API clients for fetching data from api-service.

Provides:
- PhotoClient: Fetch user and chat profile photos with presigned URLs
- GalleryClient: Fetch card gallery images from card-image-generator
"""

import logging

import httpx

from src.api.gallery_client import GalleryClient

logger = logging.getLogger(__name__)


class PhotoClient:
    """
    Client for fetching presigned photo URLs from api-service.

    Handles individual and batch lookups with graceful error handling.
    Returns None for missing photos instead of raising exceptions.
    """

    def __init__(self, base_url: str, api_key: str, timeout: float = 5.0):
        """
        Initialize the PhotoClient.

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

    def get_user_photo(self, user_id: int, size: str = "small") -> str | None:
        """
        Get presigned URL for a user's profile photo.

        Args:
            user_id: Telegram user ID
            size: Photo size (small, medium, large). Defaults to small.

        Returns:
            Presigned URL string or None if photo not found or on error.
        """
        if not self._api_key or not self._base_url:
            return None

        try:
            client = self._get_client()
            response = client.get(
                f"/api/v1/users/{user_id}/photo",
                params={"size": size},
            )
            if response.status_code == 404:
                return None
            response.raise_for_status()
            return response.json().get("url")
        except httpx.HTTPStatusError as e:
            logger.warning(f"HTTP error fetching user photo for {user_id}: {e.response.status_code}")
            return None
        except httpx.RequestError as e:
            logger.warning(f"Request error fetching user photo for {user_id}: {e}")
            return None

    def get_chat_photo(self, chat_id: int, size: str = "small") -> str | None:
        """
        Get presigned URL for a chat's profile photo.

        Args:
            chat_id: Telegram chat ID
            size: Photo size (small, medium, large). Defaults to small.

        Returns:
            Presigned URL string or None if photo not found or on error.
        """
        if not self._api_key or not self._base_url:
            return None

        try:
            client = self._get_client()
            response = client.get(
                f"/api/v1/chats/{chat_id}/photo",
                params={"size": size},
            )
            if response.status_code == 404:
                return None
            response.raise_for_status()
            return response.json().get("url")
        except httpx.HTTPStatusError as e:
            logger.warning(f"HTTP error fetching chat photo for {chat_id}: {e.response.status_code}")
            return None
        except httpx.RequestError as e:
            logger.warning(f"Request error fetching chat photo for {chat_id}: {e}")
            return None

    def get_user_photos_batch(
        self, user_ids: list[int], size: str = "small"
    ) -> dict[int, str | None]:
        """
        Fetch photos for multiple users.

        Args:
            user_ids: List of Telegram user IDs
            size: Photo size (small, medium, large). Defaults to small.

        Returns:
            Dict mapping user_id -> presigned URL (or None if not found).
        """
        result = {}
        for user_id in user_ids:
            result[user_id] = self.get_user_photo(user_id, size)
        return result

    def get_chat_photos_batch(
        self, chat_ids: list[int], size: str = "small"
    ) -> dict[int, str | None]:
        """
        Fetch photos for multiple chats.

        Args:
            chat_ids: List of Telegram chat IDs
            size: Photo size (small, medium, large). Defaults to small.

        Returns:
            Dict mapping chat_id -> presigned URL (or None if not found).
        """
        result = {}
        for chat_id in chat_ids:
            result[chat_id] = self.get_chat_photo(chat_id, size)
        return result

    def close(self):
        """Close the HTTP client and release resources."""
        if self._client:
            self._client.close()
            self._client = None


__all__ = ["PhotoClient", "GalleryClient"]
