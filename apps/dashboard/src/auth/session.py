"""
PostgreSQL-backed session management for Beef Dashboard.
"""

import hashlib
import logging
import secrets
import time
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from typing import Any, Dict, Optional

from sqlalchemy import text
from sqlalchemy.engine import Engine

logger = logging.getLogger(__name__)

# Session ID length in bytes (32 bytes = 64 hex chars)
SESSION_ID_BYTES = 32


@dataclass
class SessionData:
    """Session data stored in the database."""
    session_id: str
    user_id: int
    username: Optional[str]
    first_name: str
    photo_url: Optional[str]
    allowed_chat_ids: str  # Comma-separated list
    created_at: datetime
    expires_at: datetime
    last_accessed_at: datetime

    def is_expired(self) -> bool:
        """Check if session has expired."""
        return datetime.now(timezone.utc) > self.expires_at

    def get_allowed_chat_ids(self) -> list[int]:
        """Get list of allowed chat IDs."""
        if not self.allowed_chat_ids:
            return []
        return [int(cid) for cid in self.allowed_chat_ids.split(",") if cid]


class SessionManager:
    """Manages user sessions in PostgreSQL."""

    def __init__(self, engine: Engine, session_lifetime_days: int = 7):
        self.engine = engine
        self.session_lifetime = timedelta(days=session_lifetime_days)

    def _generate_session_id(self) -> str:
        """Generate a cryptographically secure session ID."""
        return secrets.token_hex(SESSION_ID_BYTES)

    def create_session(
        self,
        user_id: int,
        username: Optional[str],
        first_name: str,
        photo_url: Optional[str],
        allowed_chat_ids: list[int],
    ) -> str:
        """
        Create a new session for a user.

        Args:
            user_id: Telegram user ID
            username: Telegram username
            first_name: User's first name
            photo_url: URL to user's profile photo
            allowed_chat_ids: List of chat IDs the user can access

        Returns:
            The session ID
        """
        session_id = self._generate_session_id()
        now = datetime.now(timezone.utc)
        expires_at = now + self.session_lifetime
        chat_ids_str = ",".join(str(cid) for cid in allowed_chat_ids)

        query = text("""
            INSERT INTO dashboard_sessions (
                session_id, user_id, username, first_name, photo_url,
                allowed_chat_ids, created_at, expires_at, last_accessed_at
            ) VALUES (
                :session_id, :user_id, :username, :first_name, :photo_url,
                :allowed_chat_ids, :created_at, :expires_at, :last_accessed_at
            )
        """)

        with self.engine.connect() as conn:
            conn.execute(query, {
                "session_id": session_id,
                "user_id": user_id,
                "username": username,
                "first_name": first_name,
                "photo_url": photo_url,
                "allowed_chat_ids": chat_ids_str,
                "created_at": now,
                "expires_at": expires_at,
                "last_accessed_at": now,
            })
            conn.commit()

        logger.info(
            "Session created",
            extra={
                "user_id": user_id,
                "username": username,
                "expires_at": expires_at.isoformat(),
            }
        )

        return session_id

    def get_session(self, session_id: str) -> Optional[SessionData]:
        """
        Retrieve a session by ID.

        Returns None if session doesn't exist or is expired.
        """
        query = text("""
            SELECT
                session_id, user_id, username, first_name, photo_url,
                allowed_chat_ids, created_at, expires_at, last_accessed_at
            FROM dashboard_sessions
            WHERE session_id = :session_id
        """)

        with self.engine.connect() as conn:
            result = conn.execute(query, {"session_id": session_id}).fetchone()

        if result is None:
            return None

        session = SessionData(
            session_id=result.session_id,
            user_id=result.user_id,
            username=result.username,
            first_name=result.first_name,
            photo_url=result.photo_url,
            allowed_chat_ids=result.allowed_chat_ids,
            created_at=result.created_at,
            expires_at=result.expires_at,
            last_accessed_at=result.last_accessed_at,
        )

        # Check expiration
        if session.is_expired():
            self.delete_session(session_id)
            return None

        # Update last accessed time
        self._update_last_accessed(session_id)

        return session

    def _update_last_accessed(self, session_id: str) -> None:
        """Update the last accessed timestamp for a session."""
        query = text("""
            UPDATE dashboard_sessions
            SET last_accessed_at = :now
            WHERE session_id = :session_id
        """)

        with self.engine.connect() as conn:
            conn.execute(query, {
                "session_id": session_id,
                "now": datetime.now(timezone.utc),
            })
            conn.commit()

    def delete_session(self, session_id: str) -> bool:
        """Delete a session."""
        query = text("""
            DELETE FROM dashboard_sessions
            WHERE session_id = :session_id
        """)

        with self.engine.connect() as conn:
            result = conn.execute(query, {"session_id": session_id})
            conn.commit()
            deleted = result.rowcount > 0

        if deleted:
            logger.info("Session deleted", extra={"session_id": session_id[:16] + "..."})

        return deleted

    def delete_user_sessions(self, user_id: int) -> int:
        """Delete all sessions for a user."""
        query = text("""
            DELETE FROM dashboard_sessions
            WHERE user_id = :user_id
        """)

        with self.engine.connect() as conn:
            result = conn.execute(query, {"user_id": user_id})
            conn.commit()
            count = result.rowcount

        logger.info(
            "User sessions deleted",
            extra={"user_id": user_id, "count": count}
        )

        return count

    def cleanup_expired_sessions(self) -> int:
        """Delete all expired sessions."""
        query = text("""
            DELETE FROM dashboard_sessions
            WHERE expires_at < :now
        """)

        with self.engine.connect() as conn:
            result = conn.execute(query, {"now": datetime.now(timezone.utc)})
            conn.commit()
            count = result.rowcount

        if count > 0:
            logger.info(
                "Expired sessions cleaned up",
                extra={"count": count}
            )

        return count

    def get_active_session_count(self) -> int:
        """Get count of active (non-expired) sessions."""
        query = text("""
            SELECT COUNT(*) as count
            FROM dashboard_sessions
            WHERE expires_at > :now
        """)

        with self.engine.connect() as conn:
            result = conn.execute(query, {"now": datetime.now(timezone.utc)}).fetchone()
            return result.count if result else 0


def create_session_table_sql() -> str:
    """
    Return SQL to create the dashboard_sessions table.
    This is included in the migration file.
    """
    return """
    CREATE TABLE IF NOT EXISTS dashboard_sessions (
        session_id VARCHAR(64) PRIMARY KEY,
        user_id BIGINT NOT NULL,
        username VARCHAR(255),
        first_name VARCHAR(255) NOT NULL,
        photo_url TEXT,
        allowed_chat_ids TEXT,
        created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
        expires_at TIMESTAMPTZ NOT NULL,
        last_accessed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_dashboard_sessions_user_id
    ON dashboard_sessions(user_id);

    CREATE INDEX IF NOT EXISTS idx_dashboard_sessions_expires_at
    ON dashboard_sessions(expires_at);
    """
