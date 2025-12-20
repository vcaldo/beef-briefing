"""
API client for fetching card gallery images from card-image-generator.

Provides GalleryClient class for:
- Fetching available weeks with generated card images
- Fetching all card images for a chat/week with presigned URLs
- Getting presigned URLs for individual images
"""

import logging

import httpx

logger = logging.getLogger(__name__)


class GalleryClient:
    """
    Client for card image gallery operations.

    Connects to the card-image-generator service to fetch
    generated card images and their presigned URLs.
    """

    def __init__(self, base_url: str, api_key: str, timeout: float = 10.0):
        """
        Initialize the GalleryClient.

        Args:
            base_url: Base URL of the card-image-generator service
            api_key: API key for authentication
            timeout: Request timeout in seconds (default 10s for image operations)
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

    def get_available_weeks(self, chat_id: int) -> list[str] | None:
        """
        Get list of weeks with generated card images for a chat.

        Args:
            chat_id: Telegram chat ID

        Returns:
            List of week_start dates (YYYY-MM-DD) in descending order,
            or None on error.
        """
        if not self._api_key or not self._base_url:
            return None

        try:
            client = self._get_client()
            response = client.get(
                "/api/v1/weeks",
                params={"chat_id": chat_id},
            )
            if response.status_code == 404:
                return []
            response.raise_for_status()
            data = response.json()
            return data.get("weeks", [])
        except httpx.HTTPStatusError as e:
            logger.warning(f"HTTP error fetching weeks for chat {chat_id}: {e.response.status_code}")
            return None
        except httpx.RequestError as e:
            logger.warning(f"Request error fetching weeks for chat {chat_id}: {e}")
            return None

    def get_gallery_images(
        self,
        chat_id: int,
        week_start: str,
        theme: str | None = None,
    ) -> list[dict] | None:
        """
        Get all card images for a chat/week with presigned URLs.

        Args:
            chat_id: Telegram chat ID
            week_start: Week start date (YYYY-MM-DD)
            theme: Optional theme filter

        Returns:
            List of image dicts with id, user_id, url, user info, etc.
            or None on error.
        """
        if not self._api_key or not self._base_url:
            return None

        try:
            client = self._get_client()
            params = {"chat_id": chat_id, "week_start": week_start}
            if theme:
                params["theme"] = theme

            response = client.get("/api/v1/images", params=params)
            if response.status_code == 404:
                return []
            response.raise_for_status()
            data = response.json()
            images = data.get("images", [])

            # Fetch presigned URLs for each image
            for img in images:
                url = self.get_image_url(img["id"])
                img["url"] = url

            return images
        except httpx.HTTPStatusError as e:
            logger.warning(
                f"HTTP error fetching gallery images for chat {chat_id}, week {week_start}: {e.response.status_code}"
            )
            return None
        except httpx.RequestError as e:
            logger.warning(
                f"Request error fetching gallery images for chat {chat_id}, week {week_start}: {e}"
            )
            return None

    def get_image_url(
        self,
        image_id: int,
        expires: int = 3600,
    ) -> str | None:
        """
        Get presigned URL for a specific card image.

        Args:
            image_id: Image ID
            expires: URL expiration time in seconds (default 1 hour)

        Returns:
            Presigned URL string or None on error.
        """
        if not self._api_key or not self._base_url:
            return None

        try:
            client = self._get_client()
            response = client.get(
                f"/api/v1/image/{image_id}",
                params={"expires": expires},
            )
            if response.status_code == 404:
                return None
            response.raise_for_status()
            data = response.json()
            return data.get("url")
        except httpx.HTTPStatusError as e:
            logger.warning(f"HTTP error fetching image URL for image {image_id}: {e.response.status_code}")
            return None
        except httpx.RequestError as e:
            logger.warning(f"Request error fetching image URL for image {image_id}: {e}")
            return None

    def close(self):
        """Close the HTTP client and release resources."""
        if self._client:
            self._client.close()
            self._client = None


__all__ = ["GalleryClient"]
