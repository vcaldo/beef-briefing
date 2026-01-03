"""Database queries for the leaderboard mini-app."""

from datetime import date, timedelta
from typing import Any, Literal

from sqlalchemy import text
from sqlalchemy.engine import Engine

from .utils import rows_to_dicts


# Period definitions matching the leaderboard app
PERIODS = [
    {"value": "24h", "days": 1},
    {"value": "7d", "days": 7},
    {"value": "30d", "days": 30},
    {"value": "90d", "days": 90},
    {"value": "180d", "days": 180},
    {"value": "365d", "days": 365},
    {"value": "ytd", "days": None},
    {"value": "max", "days": None},
]

DEFAULT_PERIOD = "30d"


def get_period_dates(period: str) -> tuple[date | None, date | None]:
    """
    Get start and end dates for a period.

    Args:
        period: Period value (e.g., "7d", "30d", "ytd", "max")

    Returns:
        Tuple of (start_date, end_date) where None means no limit
    """
    today = date.today()
    end_date = today + timedelta(days=1)  # Include today

    if period == "max":
        return None, None

    if period == "ytd":
        start_date = date(today.year, 1, 1)
        return start_date, end_date

    # Find period definition
    for p in PERIODS:
        if p["value"] == period and p["days"] is not None:
            start_date = today - timedelta(days=p["days"])
            return start_date, end_date

    # Default to 30 days if unknown period
    return today - timedelta(days=30), end_date


class MiniAppQueries:
    """
    Queries for the leaderboard mini-app.

    Uses materialized views for all-time stats and live queries
    for date-filtered analytics.
    """

    def __init__(self, engine: Engine):
        self.engine = engine

    def _execute_single(self, query: str, params: dict) -> dict | None:
        """Execute query expecting 0 or 1 row, return dict or None."""
        with self.engine.connect() as conn:
            result = conn.execute(text(query), params)
            row = result.mappings().fetchone()
            return dict(row) if row else None

    def _execute_many(self, query: str, params: dict) -> list[dict]:
        """Execute query expecting multiple rows, return list of dicts."""
        with self.engine.connect() as conn:
            result = conn.execute(text(query), params)
            return [dict(row) for row in result.mappings()]

    def get_overview_stats(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict[str, Any]:
        """
        Get overview statistics for a chat.

        For all-time stats (no dates), uses materialized views.
        For date-filtered stats, uses live queries.

        Returns:
            {
                'total_messages': int,
                'total_users': int,
                'total_reactions': int,
                'total_media': int,
                'messages_per_day': float
            }
        """
        if start_date is None and end_date is None:
            # All-time: use materialized views
            query = """
                SELECT
                    COALESCE(SUM(message_count), 0) as total_messages,
                    COUNT(DISTINCT user_id) as total_users
                FROM mv_user_statistics
                WHERE chat_id = :chat_id
            """
            result = self._execute_single(query, {"chat_id": chat_id}) or {}

            # Get reactions from MV
            reaction_query = """
                SELECT COALESCE(SUM(count), 0) as total_reactions
                FROM mv_reaction_distribution
                WHERE chat_id = :chat_id
            """
            reaction_result = (
                self._execute_single(reaction_query, {"chat_id": chat_id}) or {}
            )

            # Get media from MV
            media_query = """
                SELECT COALESCE(SUM(count), 0) as total_media
                FROM mv_media_distribution
                WHERE chat_id = :chat_id
            """
            media_result = self._execute_single(media_query, {"chat_id": chat_id}) or {}

            # Calculate messages per day from daily stats
            days_query = """
                SELECT COUNT(*) as active_days
                FROM mv_daily_message_stats
                WHERE chat_id = :chat_id
            """
            days_result = self._execute_single(days_query, {"chat_id": chat_id}) or {}

            total_msgs = result.get("total_messages", 0)
            active_days = days_result.get("active_days", 1) or 1

            return {
                "total_messages": total_msgs,
                "total_users": result.get("total_users", 0),
                "total_reactions": reaction_result.get("total_reactions", 0),
                "total_media": media_result.get("total_media", 0),
                "messages_per_day": round(total_msgs / active_days, 2),
            }
        else:
            # Date-filtered: use live queries
            query = """
                SELECT
                    COUNT(DISTINCT m.id) as total_messages,
                    COUNT(DISTINCT m.user_id) as total_users,
                    COUNT(DISTINCT mf.id) as total_media
                FROM messages m
                LEFT JOIN media_files mf ON mf.message_id = m.id
                WHERE m.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
            """
            result = self._execute_single(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            ) or {}

            # Reactions with date filter
            reaction_query = """
                SELECT COUNT(*) as total_reactions
                FROM message_reactions
                WHERE chat_id = :chat_id
                    AND date >= :start_date
                    AND date < :end_date
                    AND is_removed = false
            """
            reaction_result = (
                self._execute_single(
                    reaction_query,
                    {
                        "chat_id": chat_id,
                        "start_date": start_date,
                        "end_date": end_date,
                    },
                )
                or {}
            )

            total_msgs = result.get("total_messages", 0)
            days = ((end_date - start_date).days if end_date and start_date else 1) or 1

            return {
                "total_messages": total_msgs,
                "total_users": result.get("total_users", 0),
                "total_reactions": reaction_result.get("total_reactions", 0),
                "total_media": result.get("total_media", 0),
                "messages_per_day": round(total_msgs / days, 2),
            }

    def get_daily_activity(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> list[dict[str, Any]]:
        """
        Get daily message activity.

        Uses mv_daily_message_stats for all-time or date ranges.

        Returns list of:
            {'date': 'YYYY-MM-DD', 'messages': int, 'users': int}
        """
        if start_date is None and end_date is None:
            # All-time from MV
            query = """
                SELECT date, message_count, unique_users
                FROM mv_daily_message_stats
                WHERE chat_id = :chat_id
                ORDER BY date
            """
            rows = self._execute_many(query, {"chat_id": chat_id})
        else:
            # Date-filtered from MV
            query = """
                SELECT date, message_count, unique_users
                FROM mv_daily_message_stats
                WHERE chat_id = :chat_id
                    AND date >= :start_date
                    AND date < :end_date
                ORDER BY date
            """
            rows = self._execute_many(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

        return [
            {
                "date": row["date"].isoformat(),
                "messages": row["message_count"],
                "users": row["unique_users"],
            }
            for row in rows
        ]

    def get_user_rankings(
        self,
        chat_id: int,
        metric: Literal[
            "message_count", "reactions_sent", "reactions_received", "active_days"
        ] = "message_count",
        limit: int = 20,
        offset: int = 0,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> list[dict[str, Any]]:
        """
        Get user rankings for leaderboard.

        Uses mv_user_statistics for all-time, live queries for date-filtered.

        Args:
            metric: 'message_count', 'reactions_sent', 'reactions_received', 'active_days'
            start_date: Optional start date for filtering
            end_date: Optional end date for filtering

        Returns list of:
            {
                'rank': int,
                'user_id': int,
                'first_name': str,
                'last_name': str | None,
                'username': str | None,
                'score': int
            }
        """
        if start_date is None and end_date is None:
            # All-time: use materialized view
            query = f"""
                SELECT
                    user_id,
                    first_name,
                    last_name,
                    username,
                    {metric} as score,
                    ROW_NUMBER() OVER (ORDER BY {metric} DESC) as rank
                FROM mv_user_statistics
                WHERE chat_id = :chat_id
                    AND is_bot = false
                ORDER BY {metric} DESC
                LIMIT :limit OFFSET :offset
            """
            rows = self._execute_many(
                query,
                {
                    "chat_id": chat_id,
                    "limit": limit,
                    "offset": offset,
                },
            )
        else:
            # Date-filtered: use live query
            query = f"""
                WITH user_stats AS (
                    SELECT
                        m.user_id,
                        u.first_name,
                        u.last_name,
                        u.username,
                        u.is_bot,
                        COUNT(*) as message_count,
                        COUNT(DISTINCT DATE(m.date)) as active_days
                    FROM messages m
                    JOIN users u ON u.id = m.user_id
                    WHERE m.chat_id = :chat_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                    GROUP BY m.user_id, u.first_name, u.last_name, u.username, u.is_bot
                ),
                reactions_sent AS (
                    SELECT user_id, COUNT(*) as reactions_sent
                    FROM message_reactions
                    WHERE chat_id = :chat_id
                        AND date >= :start_date
                        AND date < :end_date
                        AND is_removed = false
                    GROUP BY user_id
                ),
                reactions_received AS (
                    SELECT m.user_id, COUNT(*) as reactions_received
                    FROM message_reactions mr
                    JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                    WHERE mr.chat_id = :chat_id
                        AND mr.date >= :start_date
                        AND mr.date < :end_date
                        AND mr.is_removed = false
                    GROUP BY m.user_id
                )
                SELECT
                    us.user_id,
                    us.first_name,
                    us.last_name,
                    us.username,
                    COALESCE(us.message_count, 0) as message_count,
                    COALESCE(rs.reactions_sent, 0) as reactions_sent,
                    COALESCE(rr.reactions_received, 0) as reactions_received,
                    COALESCE(us.active_days, 0) as active_days,
                    {metric} as score,
                    ROW_NUMBER() OVER (ORDER BY {metric} DESC) as rank
                FROM user_stats us
                LEFT JOIN reactions_sent rs ON rs.user_id = us.user_id
                LEFT JOIN reactions_received rr ON rr.user_id = us.user_id
                WHERE us.is_bot = false
                ORDER BY {metric} DESC
                LIMIT :limit OFFSET :offset
            """
            rows = self._execute_many(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                    "limit": limit,
                    "offset": offset,
                },
            )

        # Adjust rank for offset
        for i, row in enumerate(rows):
            row["rank"] = offset + i + 1

        return [
            {
                "rank": row["rank"],
                "user_id": row["user_id"],
                "first_name": row["first_name"],
                "last_name": row.get("last_name"),
                "username": row.get("username"),
                "score": row["score"],
            }
            for row in rows
        ]

    def get_user_rankings_total(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> int:
        """
        Get total count of users for pagination.

        Uses mv_user_statistics for all-time, live query for date-filtered.

        Returns:
            Total number of non-bot users
        """
        if start_date is None and end_date is None:
            query = """
                SELECT COUNT(*) as total
                FROM mv_user_statistics
                WHERE chat_id = :chat_id
                    AND is_bot = false
            """
            result = self._execute_single(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT COUNT(DISTINCT m.user_id) as total
                FROM messages m
                JOIN users u ON u.id = m.user_id
                WHERE m.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                    AND u.is_bot = false
            """
            result = self._execute_single(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )
        return result.get("total", 0) if result else 0
