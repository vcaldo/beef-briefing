"""
Telegram OAuth authentication service.

Handles HMAC-SHA256 validation of Telegram Login Widget data
and session management.
"""

import hashlib
import hmac
import logging
import secrets
import time
from datetime import datetime, timedelta, timezone

from config import Config
from src.database import DashboardQueries, SessionQueries

logger = logging.getLogger(__name__)

# Session expiration in days
SESSION_MAX_AGE_DAYS = 7

# Max age for auth_date validation (1 hour)
AUTH_DATE_MAX_AGE_SECONDS = 3600


class TelegramAuthService:
    """Handles Telegram OAuth validation and session management."""

    def __init__(
        self,
        config: Config,
        session_queries: SessionQueries,
        dashboard_queries: DashboardQueries,
    ):
        """
        Initialize auth service.

        Args:
            config: Application configuration
            session_queries: Session database operations
            dashboard_queries: Dashboard database operations (for user chat lookup)
        """
        self._config = config
        self._sessions = session_queries
        self._dashboard = dashboard_queries

    def validate_telegram_auth(self, auth_data: dict) -> bool:
        """
        Validate Telegram OAuth data using HMAC-SHA256.

        The Telegram Login Widget sends these parameters:
        - id: User ID
        - first_name: User's first name
        - last_name: (optional) User's last name
        - username: (optional) Username
        - photo_url: (optional) Profile photo URL
        - auth_date: Unix timestamp of authentication
        - hash: HMAC-SHA256 hash for verification

        Validation steps:
        1. Extract 'hash' from auth_data
        2. Sort remaining params alphabetically
        3. Create data_check_string as "key=value\\n..."
        4. Calculate secret_key = SHA256(bot_token)
        5. Calculate HMAC-SHA256(data_check_string, secret_key)
        6. Compare with provided hash using constant-time comparison
        7. Verify auth_date is within acceptable range

        Args:
            auth_data: Dictionary of auth parameters from Telegram widget

        Returns:
            True if valid, False otherwise
        """
        received_hash = auth_data.get("hash")
        if not received_hash:
            logger.warning("Auth validation failed: missing hash")
            return False

        # Verify auth_date freshness
        auth_date = auth_data.get("auth_date")
        if not auth_date:
            logger.warning("Auth validation failed: missing auth_date")
            return False

        try:
            auth_timestamp = int(auth_date)
        except (ValueError, TypeError):
            logger.warning("Auth validation failed: invalid auth_date format")
            return False

        current_timestamp = int(time.time())
        if current_timestamp - auth_timestamp > AUTH_DATE_MAX_AGE_SECONDS:
            logger.warning(
                "Auth validation failed: auth_date too old",
                extra={"auth_age_seconds": current_timestamp - auth_timestamp},
            )
            return False

        # Build data check string (sorted alphabetically, excluding 'hash')
        data_check_arr = []
        for key in sorted(auth_data.keys()):
            if key != "hash":
                value = auth_data[key]
                if value is not None:
                    data_check_arr.append(f"{key}={value}")
        data_check_string = "\n".join(data_check_arr)

        # Calculate secret key (SHA256 of bot token)
        secret_key = hashlib.sha256(self._config.telegram_bot_token.encode()).digest()

        # Calculate expected hash
        calculated_hash = hmac.new(
            secret_key, data_check_string.encode(), hashlib.sha256
        ).hexdigest()

        # Constant-time comparison to prevent timing attacks
        if not hmac.compare_digest(calculated_hash, received_hash):
            logger.warning("Auth validation failed: hash mismatch")
            return False

        logger.info(
            "Auth validation successful",
            extra={"user_id": auth_data.get("id")},
        )
        return True

    def create_session(self, user_data: dict) -> str:
        """
        Create a new session after successful authentication.

        Args:
            user_data: Validated auth data from Telegram
                - id: User ID
                - first_name: User's first name
                - username: (optional) Username
                - photo_url: (optional) Profile photo URL

        Returns:
            Session ID string
        """
        session_id = self._generate_session_id()
        user_id = int(user_data["id"])
        first_name = user_data.get("first_name", "User")
        username = user_data.get("username")
        photo_url = user_data.get("photo_url")

        # Determine allowed chat IDs
        if self._config.is_admin(user_id):
            # Admins can access all chats - we'll handle this in the app layer
            # by checking is_admin rather than allowed_chat_ids
            allowed_chat_ids = []
        else:
            # Regular users can only access chats where they have messages
            allowed_chat_ids = self._dashboard.get_user_active_chats(user_id)

        expires_at = datetime.now(timezone.utc) + timedelta(days=SESSION_MAX_AGE_DAYS)

        self._sessions.create_session(
            session_id=session_id,
            user_id=user_id,
            username=username,
            first_name=first_name,
            photo_url=photo_url,
            allowed_chat_ids=allowed_chat_ids,
            expires_at=expires_at,
        )

        logger.info(
            "Session created",
            extra={
                "user_id": user_id,
                "is_admin": self._config.is_admin(user_id),
                "allowed_chats_count": len(allowed_chat_ids),
            },
        )

        return session_id

    def create_dev_session(self, user_id: int) -> str:
        """
        Create a session for development mode auto-login.

        Args:
            user_id: Admin user ID to create session for

        Returns:
            Session ID string
        """
        session_id = self._generate_session_id()
        expires_at = datetime.now(timezone.utc) + timedelta(days=SESSION_MAX_AGE_DAYS)

        self._sessions.create_session(
            session_id=session_id,
            user_id=user_id,
            username=None,
            first_name="Dev Admin",
            photo_url=None,
            allowed_chat_ids=[],  # Admin can access all
            expires_at=expires_at,
        )

        logger.info("Dev session created", extra={"user_id": user_id})
        return session_id

    def get_session(self, session_id: str) -> dict | None:
        """
        Get and validate a session.

        Args:
            session_id: Session ID from cookie

        Returns:
            Session dict with user info if valid, None otherwise.
            Adds 'is_admin' field based on user_id.
        """
        session = self._sessions.get_session(session_id)
        if session:
            # Add is_admin flag
            session["is_admin"] = self._config.is_admin(session["user_id"])
        return session

    def logout(self, session_id: str) -> None:
        """
        Invalidate a session.

        Args:
            session_id: Session ID to invalidate
        """
        self._sessions.delete_session(session_id)
        logger.info("Session invalidated", extra={"session_id": session_id[:8] + "..."})

    def _generate_session_id(self) -> str:
        """Generate a cryptographically secure session ID."""
        return secrets.token_urlsafe(48)
