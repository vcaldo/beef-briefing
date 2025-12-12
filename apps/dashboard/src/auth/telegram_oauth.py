"""
Telegram OAuth authentication using the Telegram Login Widget.
Validates authentication data received from Telegram.
"""

import hashlib
import hmac
import logging
import time
from dataclasses import dataclass
from typing import Any, Dict, Optional

logger = logging.getLogger(__name__)

# Maximum allowed age for auth_date (24 hours in seconds)
AUTH_DATE_MAX_AGE = 24 * 60 * 60


@dataclass
class TelegramAuthData:
    """Data received from Telegram Login Widget."""
    id: int
    first_name: str
    last_name: Optional[str]
    username: Optional[str]
    photo_url: Optional[str]
    auth_date: int
    hash: str

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "TelegramAuthData":
        """Create TelegramAuthData from a dictionary."""
        return cls(
            id=int(data["id"]),
            first_name=data["first_name"],
            last_name=data.get("last_name"),
            username=data.get("username"),
            photo_url=data.get("photo_url"),
            auth_date=int(data["auth_date"]),
            hash=data["hash"],
        )

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary (without hash)."""
        result = {
            "id": self.id,
            "first_name": self.first_name,
            "auth_date": self.auth_date,
        }
        if self.last_name:
            result["last_name"] = self.last_name
        if self.username:
            result["username"] = self.username
        if self.photo_url:
            result["photo_url"] = self.photo_url
        return result

    @property
    def display_name(self) -> str:
        """Get user's display name."""
        if self.last_name:
            return f"{self.first_name} {self.last_name}"
        return self.first_name


def validate_telegram_auth(
    data: Dict[str, Any],
    bot_token: str,
    max_age: int = AUTH_DATE_MAX_AGE
) -> Optional[TelegramAuthData]:
    """
    Validate Telegram Login Widget authentication data.

    The validation follows Telegram's specification:
    https://core.telegram.org/widgets/login#checking-authorization

    Args:
        data: Dictionary containing auth data from Telegram widget
        bot_token: The Telegram bot token
        max_age: Maximum allowed age of auth_date in seconds (default: 24 hours)

    Returns:
        TelegramAuthData if validation succeeds, None otherwise
    """
    try:
        # Extract required fields
        if "hash" not in data or "id" not in data or "auth_date" not in data:
            logger.warning("Missing required fields in Telegram auth data")
            return None

        received_hash = data["hash"]

        # Check auth_date freshness
        auth_date = int(data["auth_date"])
        current_time = int(time.time())

        if current_time - auth_date > max_age:
            logger.warning(
                "Telegram auth data expired",
                extra={
                    "auth_date": auth_date,
                    "current_time": current_time,
                    "age": current_time - auth_date,
                    "max_age": max_age,
                }
            )
            return None

        # Build data-check-string
        # 1. Sort all key-value pairs alphabetically by key (excluding hash)
        # 2. Concatenate as "key=value\n" pairs
        check_data = {k: v for k, v in data.items() if k != "hash"}
        data_check_string = "\n".join(
            f"{k}={v}" for k, v in sorted(check_data.items())
        )

        # Compute secret key: SHA256(bot_token)
        secret_key = hashlib.sha256(bot_token.encode()).digest()

        # Compute hash: HMAC-SHA256(data_check_string, secret_key)
        computed_hash = hmac.new(
            secret_key,
            data_check_string.encode(),
            hashlib.sha256
        ).hexdigest()

        # Compare hashes (constant-time comparison)
        if not hmac.compare_digest(computed_hash, received_hash):
            logger.warning(
                "Telegram auth hash validation failed",
                extra={"user_id": data.get("id")}
            )
            return None

        # Validation successful
        auth_data = TelegramAuthData.from_dict(data)
        logger.info(
            "Telegram auth validated successfully",
            extra={
                "user_id": auth_data.id,
                "username": auth_data.username,
            }
        )
        return auth_data

    except (KeyError, ValueError, TypeError) as e:
        logger.error(f"Error validating Telegram auth data: {e}")
        return None


def generate_telegram_widget_html(
    bot_username: str,
    callback_url: str,
    button_size: str = "large",
    corner_radius: int = 10,
    request_access: str = "write"
) -> str:
    """
    Generate HTML for Telegram Login Widget.

    Args:
        bot_username: The bot's username (without @)
        callback_url: URL to redirect after authentication
        button_size: Button size - "large", "medium", or "small"
        corner_radius: Button corner radius in pixels
        request_access: "write" to request write access to user's PM

    Returns:
        HTML string for the login widget
    """
    return f'''
    <script async src="https://telegram.org/js/telegram-widget.js?22"
        data-telegram-login="{bot_username}"
        data-size="{button_size}"
        data-radius="{corner_radius}"
        data-auth-url="{callback_url}"
        data-request-access="{request_access}">
    </script>
    '''
