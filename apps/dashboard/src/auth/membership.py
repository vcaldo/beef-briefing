"""
Telegram group membership verification using the Bot API.
"""

import logging
import time
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Set

import requests

logger = logging.getLogger(__name__)

# Cache TTL in seconds (1 hour)
MEMBERSHIP_CACHE_TTL = 60 * 60

# Telegram Bot API base URL
TELEGRAM_API_BASE = "https://api.telegram.org/bot{token}/{method}"

# Valid member statuses that grant access
VALID_MEMBER_STATUSES = {"creator", "administrator", "member", "restricted"}


@dataclass
class MembershipResult:
    """Result of a membership check."""
    is_member: bool
    status: Optional[str] = None
    chat_id: Optional[int] = None
    error: Optional[str] = None


@dataclass
class CacheEntry:
    """Cache entry for membership status."""
    is_member: bool
    status: str
    timestamp: float


class MembershipCache:
    """In-memory cache for membership status."""

    def __init__(self, ttl: int = MEMBERSHIP_CACHE_TTL):
        self.ttl = ttl
        self._cache: Dict[str, CacheEntry] = {}

    def _make_key(self, user_id: int, chat_id: int) -> str:
        return f"{user_id}:{chat_id}"

    def get(self, user_id: int, chat_id: int) -> Optional[CacheEntry]:
        """Get cached membership status if not expired."""
        key = self._make_key(user_id, chat_id)
        entry = self._cache.get(key)

        if entry is None:
            return None

        # Check if expired
        if time.time() - entry.timestamp > self.ttl:
            del self._cache[key]
            return None

        return entry

    def set(self, user_id: int, chat_id: int, is_member: bool, status: str) -> None:
        """Cache membership status."""
        key = self._make_key(user_id, chat_id)
        self._cache[key] = CacheEntry(
            is_member=is_member,
            status=status,
            timestamp=time.time(),
        )

    def invalidate(self, user_id: int, chat_id: int) -> None:
        """Invalidate cache entry."""
        key = self._make_key(user_id, chat_id)
        self._cache.pop(key, None)

    def clear(self) -> None:
        """Clear all cache entries."""
        self._cache.clear()


# Global cache instance
_membership_cache = MembershipCache()


def get_membership_cache() -> MembershipCache:
    """Get the global membership cache instance."""
    return _membership_cache


def verify_group_membership(
    user_id: int,
    chat_id: int,
    bot_token: str,
    cache: Optional[MembershipCache] = None,
    skip_cache: bool = False
) -> MembershipResult:
    """
    Verify if a user is a member of a Telegram group.

    Uses the Telegram Bot API getChatMember method.

    Args:
        user_id: The Telegram user ID
        chat_id: The Telegram chat ID (group/supergroup)
        bot_token: The bot token for API authentication
        cache: Optional membership cache instance
        skip_cache: If True, bypass cache and always call API

    Returns:
        MembershipResult with verification status
    """
    if cache is None:
        cache = _membership_cache

    # Check cache first
    if not skip_cache:
        cached = cache.get(user_id, chat_id)
        if cached is not None:
            logger.debug(
                "Membership check cache hit",
                extra={
                    "user_id": user_id,
                    "chat_id": chat_id,
                    "is_member": cached.is_member,
                    "status": cached.status,
                }
            )
            return MembershipResult(
                is_member=cached.is_member,
                status=cached.status,
                chat_id=chat_id,
            )

    # Call Telegram API
    url = TELEGRAM_API_BASE.format(token=bot_token, method="getChatMember")

    try:
        response = requests.post(
            url,
            json={"chat_id": chat_id, "user_id": user_id},
            timeout=10,
        )
        response.raise_for_status()
        data = response.json()

        if not data.get("ok"):
            error_description = data.get("description", "Unknown error")
            logger.warning(
                "Telegram API returned error",
                extra={
                    "user_id": user_id,
                    "chat_id": chat_id,
                    "error": error_description,
                }
            )
            return MembershipResult(
                is_member=False,
                error=error_description,
            )

        # Extract member status
        result = data.get("result", {})
        status = result.get("status", "unknown")
        is_member = status in VALID_MEMBER_STATUSES

        # Cache the result
        cache.set(user_id, chat_id, is_member, status)

        logger.info(
            "Membership verification completed",
            extra={
                "user_id": user_id,
                "chat_id": chat_id,
                "status": status,
                "is_member": is_member,
            }
        )

        return MembershipResult(
            is_member=is_member,
            status=status,
            chat_id=chat_id,
        )

    except requests.exceptions.Timeout:
        logger.error(
            "Telegram API timeout",
            extra={"user_id": user_id, "chat_id": chat_id}
        )
        return MembershipResult(
            is_member=False,
            error="API request timed out",
        )

    except requests.exceptions.RequestException as e:
        logger.error(
            f"Telegram API request failed: {e}",
            extra={"user_id": user_id, "chat_id": chat_id}
        )
        return MembershipResult(
            is_member=False,
            error=str(e),
        )

    except (KeyError, ValueError) as e:
        logger.error(
            f"Error parsing Telegram API response: {e}",
            extra={"user_id": user_id, "chat_id": chat_id}
        )
        return MembershipResult(
            is_member=False,
            error=f"Invalid API response: {e}",
        )


def verify_membership_any_chat(
    user_id: int,
    allowed_chat_ids: List[int],
    bot_token: str,
    cache: Optional[MembershipCache] = None
) -> MembershipResult:
    """
    Verify if a user is a member of any of the allowed chats.

    Args:
        user_id: The Telegram user ID
        allowed_chat_ids: List of allowed chat IDs
        bot_token: The bot token for API authentication
        cache: Optional membership cache instance

    Returns:
        MembershipResult for the first chat where user is a member
    """
    if not allowed_chat_ids:
        logger.warning("No allowed chat IDs configured")
        return MembershipResult(
            is_member=False,
            error="No allowed chats configured",
        )

    for chat_id in allowed_chat_ids:
        result = verify_group_membership(user_id, chat_id, bot_token, cache)
        if result.is_member:
            return result

    logger.info(
        "User is not a member of any allowed chat",
        extra={
            "user_id": user_id,
            "checked_chats": allowed_chat_ids,
        }
    )

    return MembershipResult(
        is_member=False,
        error="Not a member of any allowed group",
    )
