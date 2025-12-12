"""Client for api-service endpoints."""
import logging
from datetime import datetime, timedelta
from typing import List, Optional

import requests

logger = logging.getLogger(__name__)


class ApiServiceClient:
    """Client for interacting with the api-service."""

    def __init__(self, base_url: str, api_key: Optional[str] = None):
        """
        Initialize the API client.

        Args:
            base_url: Base URL of the api-service (e.g., http://api-service:8080)
            api_key: Optional API key for authentication
        """
        self.base_url = base_url.rstrip('/')
        self.api_key = api_key

    def _get_headers(self) -> dict:
        """Get request headers including authentication if configured."""
        headers = {"Content-Type": "application/json"}
        if self.api_key:
            headers["X-API-Key"] = self.api_key
        return headers

    def get_user_active_chats(
        self,
        user_id: int,
        days: int = 15
    ) -> List[int]:
        """
        Get list of chat IDs where user has been active.

        Uses: GET /api/v1/analytics/users/{user_id}/active-chats

        Args:
            user_id: Telegram user ID
            days: Number of days to look back for activity (default: 15)

        Returns:
            List of chat IDs where the user has been active

        Response format:
            {
                "data": [-1001234567890, -1009876543210],
                "metadata": {
                    "user_id": 987654321,
                    "start_date": "2024-01-01T00:00:00Z",
                    "end_date": "2024-12-31T23:59:59Z",
                    "generated_at": "2024-12-13T10:30:00Z",
                    "total_count": 2
                }
            }
        """
        end_date = datetime.utcnow()
        start_date = end_date - timedelta(days=days)

        url = f"{self.base_url}/api/v1/analytics/users/{user_id}/active-chats"
        params = {
            "start_date": start_date.strftime("%Y-%m-%dT%H:%M:%SZ"),
            "end_date": end_date.strftime("%Y-%m-%dT%H:%M:%SZ"),
        }

        try:
            response = requests.get(
                url,
                params=params,
                headers=self._get_headers(),
                timeout=10,
            )
            response.raise_for_status()
            data = response.json()

            chat_ids = data.get("data", [])
            logger.info(
                "Fetched active chats for user",
                extra={
                    "user_id": user_id,
                    "days": days,
                    "chat_count": len(chat_ids),
                }
            )
            return chat_ids

        except requests.exceptions.Timeout:
            logger.error(
                "API request timed out",
                extra={"user_id": user_id, "url": url}
            )
            return []

        except requests.exceptions.RequestException as e:
            logger.error(
                f"API request failed: {e}",
                extra={"user_id": user_id, "url": url}
            )
            return []

        except (KeyError, ValueError) as e:
            logger.error(
                f"Error parsing API response: {e}",
                extra={"user_id": user_id}
            )
            return []
