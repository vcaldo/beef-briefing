"""
Session management database operations.

Uses the existing dashboard_sessions table from 002_dashboard.sql migration.
"""

from datetime import datetime

from sqlalchemy import text
from sqlalchemy.engine import Engine


class SessionQueries:
    """Database operations for dashboard sessions."""

    def __init__(self, engine: Engine):
        """Initialize with SQLAlchemy engine."""
        self._engine = engine

    def create_session(
        self,
        session_id: str,
        user_id: int,
        username: str | None,
        first_name: str,
        photo_url: str | None,
        allowed_chat_ids: list[int],
        expires_at: datetime,
    ) -> None:
        """
        Create a new session in dashboard_sessions table.

        Args:
            session_id: Unique session identifier
            user_id: Telegram user ID
            username: Telegram username (optional)
            first_name: User's first name
            photo_url: URL to user's profile photo (optional)
            allowed_chat_ids: List of chat IDs user can access
            expires_at: Session expiration timestamp
        """
        # Convert list to comma-separated string for storage
        chat_ids_str = ",".join(str(cid) for cid in allowed_chat_ids) if allowed_chat_ids else None

        query = """
            INSERT INTO dashboard_sessions (
                session_id, user_id, username, first_name, photo_url,
                allowed_chat_ids, expires_at, created_at, last_accessed_at
            ) VALUES (
                :session_id, :user_id, :username, :first_name, :photo_url,
                :allowed_chat_ids, :expires_at, NOW(), NOW()
            )
        """
        with self._engine.connect() as conn:
            conn.execute(
                text(query),
                {
                    "session_id": session_id,
                    "user_id": user_id,
                    "username": username,
                    "first_name": first_name,
                    "photo_url": photo_url,
                    "allowed_chat_ids": chat_ids_str,
                    "expires_at": expires_at,
                },
            )
            conn.commit()

    def get_session(self, session_id: str) -> dict | None:
        """
        Get session by ID and update last_accessed_at.

        Returns session dict if valid (not expired), None otherwise.
        Also updates last_accessed_at timestamp on successful retrieval.
        """
        # First get the session
        query = """
            SELECT
                session_id, user_id, username, first_name, photo_url,
                allowed_chat_ids, created_at, expires_at, last_accessed_at
            FROM dashboard_sessions
            WHERE session_id = :session_id
                AND expires_at > NOW()
        """
        with self._engine.connect() as conn:
            result = conn.execute(text(query), {"session_id": session_id})
            row = result.mappings().fetchone()

            if not row:
                return None

            # Update last_accessed_at
            update_query = """
                UPDATE dashboard_sessions
                SET last_accessed_at = NOW()
                WHERE session_id = :session_id
            """
            conn.execute(text(update_query), {"session_id": session_id})
            conn.commit()

            session = dict(row)
            # Parse allowed_chat_ids back to list
            if session.get("allowed_chat_ids"):
                session["allowed_chat_ids"] = [
                    int(cid) for cid in session["allowed_chat_ids"].split(",")
                ]
            else:
                session["allowed_chat_ids"] = []

            return session

    def delete_session(self, session_id: str) -> None:
        """Delete a session by ID."""
        query = """
            DELETE FROM dashboard_sessions
            WHERE session_id = :session_id
        """
        with self._engine.connect() as conn:
            conn.execute(text(query), {"session_id": session_id})
            conn.commit()

    def delete_user_sessions(self, user_id: int) -> None:
        """Delete all sessions for a user."""
        query = """
            DELETE FROM dashboard_sessions
            WHERE user_id = :user_id
        """
        with self._engine.connect() as conn:
            conn.execute(text(query), {"user_id": user_id})
            conn.commit()

    def cleanup_expired(self) -> int:
        """
        Delete all expired sessions.

        Returns the number of sessions deleted.
        """
        query = """
            DELETE FROM dashboard_sessions
            WHERE expires_at <= NOW()
        """
        with self._engine.connect() as conn:
            result = conn.execute(text(query))
            conn.commit()
            return result.rowcount
