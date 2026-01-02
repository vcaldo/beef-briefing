"""
Database query layer for the leaderboard dashboard.

Design Principles:
1. Raw SQL with SQLAlchemy text() - no ORM
2. Parameterized queries with named parameters (:param)
3. Connection pooling via SQLAlchemy engine (passed from app.py)
4. Pandas integration via pd.read_sql() for chart data
5. Return patterns:
   - Single row -> dict | None
   - Multiple rows -> list[dict]
   - Chart data -> pd.DataFrame
"""

from datetime import date
from typing import Literal

import pandas as pd
from sqlalchemy import text
from sqlalchemy.engine import Engine


class DashboardQueries:
    """
    Query executor for the leaderboard dashboard.

    Uses materialized views for all-time stats and live queries
    for date-filtered analytics.
    """

    def __init__(self, engine: Engine):
        """
        Initialize with SQLAlchemy engine.

        Args:
            engine: SQLAlchemy engine with connection pooling
        """
        self._engine = engine

    # =========================================
    # PRIVATE HELPERS
    # =========================================

    def _execute_single(self, query: str, params: dict) -> dict | None:
        """Execute query expecting 0 or 1 row, return dict or None."""
        with self._engine.connect() as conn:
            result = conn.execute(text(query), params)
            row = result.mappings().fetchone()
            return dict(row) if row else None

    def _execute_many(self, query: str, params: dict) -> list[dict]:
        """Execute query expecting multiple rows, return list of dicts."""
        with self._engine.connect() as conn:
            result = conn.execute(text(query), params)
            return [dict(row) for row in result.mappings()]

    def _execute_df(self, query: str, params: dict) -> pd.DataFrame:
        """Execute query and return pandas DataFrame for charts."""
        with self._engine.connect() as conn:
            return pd.read_sql(text(query), conn, params=params)

    # =========================================
    # OVERVIEW METHODS
    # =========================================

    def get_overview_stats(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict:
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

    def get_overview_stats_comparison(
        self,
        chat_id: int,
        current_start: date,
        current_end: date,
        previous_start: date,
        previous_end: date,
    ) -> dict:
        """
        Get overview stats with comparison to previous period.

        Returns:
            {
                'current': {...stats for current period...},
                'previous': {...stats for previous period...},
                'changes': {...percentage changes...}
            }
        """
        current = self.get_overview_stats(chat_id, current_start, current_end)
        previous = self.get_overview_stats(chat_id, previous_start, previous_end)

        def calc_change(curr: float, prev: float) -> float | None:
            """Calculate percentage change."""
            if prev == 0:
                return None if curr == 0 else 100.0
            return round(((curr - prev) / prev) * 100, 1)

        changes = {
            "total_messages": calc_change(
                current["total_messages"], previous["total_messages"]
            ),
            "total_users": calc_change(current["total_users"], previous["total_users"]),
            "total_reactions": calc_change(
                current["total_reactions"], previous["total_reactions"]
            ),
            "total_media": calc_change(
                current["total_media"], previous["total_media"]
            ),
        }

        return {
            "current": current,
            "previous": previous,
            "changes": changes,
        }

    def get_chat_info(self, chat_id: int) -> dict | None:
        """
        Get detailed information about a chat.

        Returns:
            {
                'id': int,
                'title': str,
                'type': str,
                'username': str | None,
                'first_message': datetime,
                'last_message': datetime
            }
        """
        query = """
            SELECT
                c.id,
                COALESCE(c.title, '') as title,
                c.type::text as type,
                c.username,
                COALESCE(MIN(m.date), c.created_at) as first_message,
                COALESCE(MAX(m.date), c.created_at) as last_message
            FROM chats c
            LEFT JOIN messages m ON m.chat_id = c.id
            WHERE c.id = :chat_id
            GROUP BY c.id, c.title, c.type, c.username, c.created_at
        """
        return self._execute_single(query, {"chat_id": chat_id})

    def get_available_chats(self) -> list[dict]:
        """
        Get all available chats with basic stats.

        Returns list of:
            {
                'id': int,
                'title': str,
                'type': str,
                'username': str | None,
                'message_count': int,
                'user_count': int,
                'last_activity': datetime
            }
        """
        query = """
            SELECT
                c.id,
                COALESCE(c.title, '') as title,
                c.type::text as type,
                c.username,
                COUNT(DISTINCT m.id) as message_count,
                COUNT(DISTINCT m.user_id) as user_count,
                COALESCE(MAX(m.date), c.created_at) as last_activity
            FROM chats c
            LEFT JOIN messages m ON m.chat_id = c.id
            WHERE c.type IN ('group', 'supergroup')
            GROUP BY c.id, c.title, c.type, c.username, c.created_at
            ORDER BY last_activity DESC
        """
        return self._execute_many(query, {})

    def get_chats_with_stats(self, chat_ids: list[int] | None = None) -> list[dict]:
        """
        Get chats with statistics for landing page cards.

        Args:
            chat_ids: Optional list of chat IDs to filter. None returns all.

        Returns list of:
            {
                'id': int,
                'title': str,
                'type': str,
                'username': str | None,
                'message_count': int,
                'user_count': int,
                'last_activity': datetime,
                'first_activity': datetime,
                'avg_messages_per_day': float
            }
        """
        base_query = """
            WITH chat_stats AS (
                SELECT
                    c.id,
                    COALESCE(c.title, '') as title,
                    c.type::text as type,
                    c.username,
                    COUNT(DISTINCT m.id) as message_count,
                    COUNT(DISTINCT m.user_id) as user_count,
                    COALESCE(MAX(m.date), c.created_at) as last_activity,
                    COALESCE(MIN(m.date), c.created_at) as first_activity
                FROM chats c
                LEFT JOIN messages m ON m.chat_id = c.id
                WHERE c.type IN ('group', 'supergroup')
                {filter_clause}
                GROUP BY c.id, c.title, c.type, c.username, c.created_at
            )
            SELECT
                cs.*,
                CASE
                    WHEN EXTRACT(DAY FROM (cs.last_activity - cs.first_activity)) > 0
                    THEN ROUND(
                        cs.message_count::numeric /
                        NULLIF(EXTRACT(DAY FROM (cs.last_activity - cs.first_activity)), 0),
                        1
                    )
                    ELSE cs.message_count::numeric
                END as avg_messages_per_day
            FROM chat_stats cs
            ORDER BY cs.last_activity DESC
        """

        if chat_ids:
            query = base_query.format(filter_clause="AND c.id = ANY(:chat_ids)")
            return self._execute_many(query, {"chat_ids": chat_ids})
        else:
            query = base_query.format(filter_clause="")
            return self._execute_many(query, {})

    def get_chat_card_data(self, chat_id: int) -> dict | None:
        """
        Get data for a chat summary card.

        Returns:
            {
                'title': str,
                'message_count': int,
                'user_count': int,
                'top_user': str | None,
                'top_user_messages': int,
                'last_activity': datetime
            }
        """
        # Main stats
        query = """
            SELECT
                COALESCE(c.title, '') as title,
                COUNT(DISTINCT m.id) as message_count,
                COUNT(DISTINCT m.user_id) as user_count,
                MAX(m.date) as last_activity
            FROM chats c
            LEFT JOIN messages m ON m.chat_id = c.id
            WHERE c.id = :chat_id
            GROUP BY c.id, c.title
        """
        result = self._execute_single(query, {"chat_id": chat_id})
        if not result:
            return None

        # Top user from materialized view
        top_user_query = """
            SELECT first_name, message_count
            FROM mv_user_statistics
            WHERE chat_id = :chat_id AND is_bot = false
            ORDER BY message_count DESC
            LIMIT 1
        """
        top_user = self._execute_single(top_user_query, {"chat_id": chat_id})

        result["top_user"] = top_user.get("first_name") if top_user else None
        result["top_user_messages"] = top_user.get("message_count", 0) if top_user else 0

        return result

    def get_available_years(self, chat_id: int) -> list[int]:
        """
        Get all years that have messages for a chat.

        Returns:
            [2023, 2024, 2025] (sorted descending)
        """
        query = """
            SELECT DISTINCT EXTRACT(YEAR FROM date)::int as year
            FROM messages
            WHERE chat_id = :chat_id
            ORDER BY year DESC
        """
        rows = self._execute_many(query, {"chat_id": chat_id})
        return [row["year"] for row in rows]

    # =========================================
    # TIME SERIES METHODS (return DataFrames)
    # =========================================

    def get_daily_activity(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get daily message activity.

        Uses mv_daily_message_stats for all-time or date ranges.

        Returns DataFrame with columns:
            date, message_count, unique_users
        """
        if start_date is None and end_date is None:
            # All-time from MV
            query = """
                SELECT date, message_count, unique_users
                FROM mv_daily_message_stats
                WHERE chat_id = :chat_id
                ORDER BY date
            """
            return self._execute_df(query, {"chat_id": chat_id})
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
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_message_timeline(
        self,
        chat_id: int,
        start_date: date,
        end_date: date,
        granularity: Literal["hour", "day", "week", "month"] = "day",
    ) -> pd.DataFrame:
        """
        Get message timeline with configurable granularity.

        Args:
            granularity: 'hour', 'day', 'week', 'month'

        Returns DataFrame with columns:
            timestamp, message_count, unique_users, reaction_count
        """
        trunc_map = {
            "hour": "date_trunc('hour', m.date)",
            "day": "date_trunc('day', m.date)",
            "week": "date_trunc('week', m.date)",
            "month": "date_trunc('month', m.date)",
        }

        trunc_func = trunc_map[granularity]
        reaction_trunc = trunc_func.replace("m.date", "date")

        query = f"""
            WITH messages_agg AS (
                SELECT
                    {trunc_func} as timestamp,
                    COUNT(*) as message_count,
                    COUNT(DISTINCT m.user_id) as unique_users
                FROM messages m
                WHERE m.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                GROUP BY timestamp
            ),
            reactions_agg AS (
                SELECT
                    {reaction_trunc} as timestamp,
                    COUNT(*) as reaction_count
                FROM message_reactions
                WHERE chat_id = :chat_id
                    AND date >= :start_date
                    AND date < :end_date
                    AND is_removed = false
                GROUP BY timestamp
            )
            SELECT
                ma.timestamp,
                ma.message_count,
                ma.unique_users,
                COALESCE(ra.reaction_count, 0) as reaction_count
            FROM messages_agg ma
            LEFT JOIN reactions_agg ra ON ra.timestamp = ma.timestamp
            ORDER BY ma.timestamp
        """

        return self._execute_df(
            query,
            {
                "chat_id": chat_id,
                "start_date": start_date,
                "end_date": end_date,
            },
        )

    def get_hourly_activity_pattern(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get message distribution by hour of day.

        Uses mv_hourly_activity for all-time.

        Returns DataFrame with columns:
            hour (0-23), message_count
        """
        if start_date is None and end_date is None:
            query = """
                SELECT hour, SUM(message_count) as message_count
                FROM mv_hourly_activity
                WHERE chat_id = :chat_id
                GROUP BY hour
                ORDER BY hour
            """
            return self._execute_df(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    EXTRACT(HOUR FROM date)::int as hour,
                    COUNT(*) as message_count
                FROM messages
                WHERE chat_id = :chat_id
                    AND date >= :start_date
                    AND date < :end_date
                GROUP BY hour
                ORDER BY hour
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_day_of_week_pattern(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get message distribution by day of week.

        Uses mv_hourly_activity for all-time (aggregates hours).

        Returns DataFrame with columns:
            day_of_week (0=Sunday, 6=Saturday), message_count
        """
        if start_date is None and end_date is None:
            query = """
                SELECT day_of_week, SUM(message_count) as message_count
                FROM mv_hourly_activity
                WHERE chat_id = :chat_id
                GROUP BY day_of_week
                ORDER BY day_of_week
            """
            return self._execute_df(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    EXTRACT(DOW FROM date)::int as day_of_week,
                    COUNT(*) as message_count
                FROM messages
                WHERE chat_id = :chat_id
                    AND date >= :start_date
                    AND date < :end_date
                GROUP BY day_of_week
                ORDER BY day_of_week
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_hourly_heatmap_data(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get hour x day-of-week heatmap data.

        Uses mv_hourly_activity for all-time (already aggregated).

        Returns DataFrame with columns:
            day_of_week (0-6), hour (0-23), message_count
        """
        if start_date is None and end_date is None:
            query = """
                SELECT day_of_week, hour, message_count
                FROM mv_hourly_activity
                WHERE chat_id = :chat_id
                ORDER BY day_of_week, hour
            """
            return self._execute_df(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    EXTRACT(DOW FROM date)::int as day_of_week,
                    EXTRACT(HOUR FROM date)::int as hour,
                    COUNT(*) as message_count
                FROM messages
                WHERE chat_id = :chat_id
                    AND date >= :start_date
                    AND date < :end_date
                GROUP BY day_of_week, hour
                ORDER BY day_of_week, hour
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    # =========================================
    # USER ANALYTICS METHODS
    # =========================================

    def get_user_rankings(
        self,
        chat_id: int,
        metric: Literal[
            "message_count", "reactions_sent", "reactions_received", "active_days"
        ] = "message_count",
        limit: int = 50,
        offset: int = 0,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> list[dict]:
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
                'is_premium': bool,
                'score': int,
                'message_count': int,
                'reactions_sent': int,
                'reactions_received': int,
                'active_days': int
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
                    is_premium,
                    is_bot,
                    message_count,
                    reactions_sent,
                    reactions_received,
                    active_days,
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
                        u.is_premium,
                        u.is_bot,
                        COUNT(*) as message_count,
                        COUNT(DISTINCT DATE(m.date)) as active_days
                    FROM messages m
                    JOIN users u ON u.id = m.user_id
                    WHERE m.chat_id = :chat_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                    GROUP BY m.user_id, u.first_name, u.last_name, u.username, u.is_premium, u.is_bot
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
                    us.is_premium,
                    us.is_bot,
                    us.message_count,
                    COALESCE(rs.reactions_sent, 0) as reactions_sent,
                    COALESCE(rr.reactions_received, 0) as reactions_received,
                    us.active_days,
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

        return rows

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

    def get_user_active_chats(
        self,
        user_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> list[int]:
        """
        Get list of chat IDs where user has been active.

        Returns:
            List of chat IDs
        """
        if start_date is None and end_date is None:
            # All-time: use MV
            query = """
                SELECT DISTINCT chat_id
                FROM mv_user_statistics
                WHERE user_id = :user_id
                ORDER BY chat_id
            """
            rows = self._execute_many(query, {"user_id": user_id})
        else:
            # Date-filtered: live query with UNION (messages + reactions)
            query = """
                SELECT DISTINCT chat_id
                FROM (
                    SELECT chat_id FROM messages
                    WHERE user_id = :user_id
                        AND date >= :start_date
                        AND date < :end_date

                    UNION

                    SELECT chat_id FROM message_reactions
                    WHERE user_id = :user_id
                        AND date >= :start_date
                        AND date < :end_date
                        AND is_removed = false
                ) AS active_chats
                ORDER BY chat_id
            """
            rows = self._execute_many(
                query,
                {
                    "user_id": user_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

        return [row["chat_id"] for row in rows]

    # =========================================
    # CONTENT METHODS
    # =========================================

    def get_reaction_distribution(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get reaction emoji distribution.

        Uses mv_reaction_distribution for all-time, live query for date-filtered.

        Returns DataFrame with columns:
            emoji, reaction_type, count
        """
        if start_date is None and end_date is None:
            query = """
                SELECT emoji, reaction_type, count
                FROM mv_reaction_distribution
                WHERE chat_id = :chat_id
                ORDER BY count DESC
            """
            return self._execute_df(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    COALESCE(emoji_value, custom_emoji_id, '?') as emoji,
                    reaction_type::text as reaction_type,
                    COUNT(*) as count
                FROM message_reactions
                WHERE chat_id = :chat_id
                    AND date >= :start_date
                    AND date < :end_date
                    AND is_removed = false
                GROUP BY emoji, reaction_type
                ORDER BY count DESC
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_top_reactions(self, chat_id: int, limit: int = 10) -> list[dict]:
        """
        Get top N reactions by count.

        Uses mv_reaction_distribution.

        Returns list of:
            {'emoji': str, 'count': int}
        """
        query = """
            SELECT emoji, SUM(count) as count
            FROM mv_reaction_distribution
            WHERE chat_id = :chat_id
            GROUP BY emoji
            ORDER BY count DESC
            LIMIT :limit
        """
        return self._execute_many(query, {"chat_id": chat_id, "limit": limit})

    def get_media_distribution(self, chat_id: int) -> pd.DataFrame:
        """
        Get media type distribution.

        Uses mv_media_distribution.

        Returns DataFrame with columns:
            media_type, count, total_size
        """
        query = """
            SELECT media_type, count, total_size
            FROM mv_media_distribution
            WHERE chat_id = :chat_id
            ORDER BY count DESC
        """
        return self._execute_df(query, {"chat_id": chat_id})

    # =========================================
    # USER-SPECIFIC METHODS (for My Stats page)
    # =========================================

    def get_user_stats(
        self,
        chat_id: int,
        user_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict | None:
        """
        Get a specific user's statistics in a chat.

        Uses mv_user_statistics for all-time, live query for date-filtered.

        Returns:
            {
                'user_id': int,
                'first_name': str,
                'username': str | None,
                'is_premium': bool,
                'message_count': int,
                'reactions_sent': int,
                'reactions_received': int,
                'active_days': int
            }
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    user_id,
                    first_name,
                    username,
                    is_premium,
                    message_count,
                    reactions_sent,
                    reactions_received,
                    active_days
                FROM mv_user_statistics
                WHERE chat_id = :chat_id AND user_id = :user_id
            """
            return self._execute_single(query, {"chat_id": chat_id, "user_id": user_id})
        else:
            query = """
                WITH user_messages AS (
                    SELECT
                        COUNT(*) as message_count,
                        COUNT(DISTINCT DATE(date)) as active_days
                    FROM messages
                    WHERE chat_id = :chat_id
                        AND user_id = :user_id
                        AND date >= :start_date
                        AND date < :end_date
                ),
                reactions_sent AS (
                    SELECT COUNT(*) as reactions_sent
                    FROM message_reactions
                    WHERE chat_id = :chat_id
                        AND user_id = :user_id
                        AND date >= :start_date
                        AND date < :end_date
                        AND is_removed = false
                ),
                reactions_received AS (
                    SELECT COUNT(*) as reactions_received
                    FROM message_reactions mr
                    JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                    WHERE mr.chat_id = :chat_id
                        AND m.user_id = :user_id
                        AND mr.date >= :start_date
                        AND mr.date < :end_date
                        AND mr.is_removed = false
                )
                SELECT
                    u.id as user_id,
                    u.first_name,
                    u.username,
                    u.is_premium,
                    COALESCE(um.message_count, 0) as message_count,
                    COALESCE(rs.reactions_sent, 0) as reactions_sent,
                    COALESCE(rr.reactions_received, 0) as reactions_received,
                    COALESCE(um.active_days, 0) as active_days
                FROM users u
                CROSS JOIN user_messages um
                CROSS JOIN reactions_sent rs
                CROSS JOIN reactions_received rr
                WHERE u.id = :user_id
            """
            return self._execute_single(
                query,
                {
                    "chat_id": chat_id,
                    "user_id": user_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_user_rank(
        self,
        chat_id: int,
        user_id: int,
        metric: str = "message_count",
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> int | None:
        """
        Get a user's rank for a specific metric.

        Uses mv_user_statistics for all-time, live query for date-filtered.

        Returns:
            Rank (1-based) or None if user not found
        """
        if start_date is None and end_date is None:
            query = f"""
                WITH ranked AS (
                    SELECT
                        user_id,
                        ROW_NUMBER() OVER (ORDER BY {metric} DESC) as rank
                    FROM mv_user_statistics
                    WHERE chat_id = :chat_id AND is_bot = false
                )
                SELECT rank FROM ranked WHERE user_id = :user_id
            """
            result = self._execute_single(query, {"chat_id": chat_id, "user_id": user_id})
        else:
            # For date-filtered, we need to compute stats and rank
            query = f"""
                WITH user_stats AS (
                    SELECT
                        m.user_id,
                        COUNT(*) as message_count,
                        COUNT(DISTINCT DATE(m.date)) as active_days
                    FROM messages m
                    JOIN users u ON u.id = m.user_id
                    WHERE m.chat_id = :chat_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                        AND u.is_bot = false
                    GROUP BY m.user_id
                ),
                reactions_sent AS (
                    SELECT mr.user_id, COUNT(*) as reactions_sent
                    FROM message_reactions mr
                    JOIN users u ON u.id = mr.user_id
                    WHERE mr.chat_id = :chat_id
                        AND mr.date >= :start_date
                        AND mr.date < :end_date
                        AND mr.is_removed = false
                        AND u.is_bot = false
                    GROUP BY mr.user_id
                ),
                reactions_received AS (
                    SELECT m.user_id, COUNT(*) as reactions_received
                    FROM message_reactions mr
                    JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                    JOIN users u ON u.id = m.user_id
                    WHERE mr.chat_id = :chat_id
                        AND mr.date >= :start_date
                        AND mr.date < :end_date
                        AND mr.is_removed = false
                        AND u.is_bot = false
                    GROUP BY m.user_id
                ),
                combined AS (
                    SELECT
                        COALESCE(us.user_id, rs.user_id, rr.user_id) as user_id,
                        COALESCE(us.message_count, 0) as message_count,
                        COALESCE(us.active_days, 0) as active_days,
                        COALESCE(rs.reactions_sent, 0) as reactions_sent,
                        COALESCE(rr.reactions_received, 0) as reactions_received
                    FROM user_stats us
                    FULL OUTER JOIN reactions_sent rs ON rs.user_id = us.user_id
                    FULL OUTER JOIN reactions_received rr ON rr.user_id = COALESCE(us.user_id, rs.user_id)
                ),
                ranked AS (
                    SELECT
                        user_id,
                        ROW_NUMBER() OVER (ORDER BY {metric} DESC) as rank
                    FROM combined
                )
                SELECT rank FROM ranked WHERE user_id = :user_id
            """
            result = self._execute_single(
                query,
                {
                    "chat_id": chat_id,
                    "user_id": user_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )
        return result.get("rank") if result else None

    def get_group_averages(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict:
        """
        Get group average statistics per user.

        Uses mv_user_statistics for all-time, live query for date-filtered.

        Returns:
            {
                'avg_messages': float,
                'avg_reactions_sent': float,
                'avg_reactions_received': float,
                'avg_active_days': float,
                'total_users': int
            }
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    AVG(message_count) as avg_messages,
                    AVG(reactions_sent) as avg_reactions_sent,
                    AVG(reactions_received) as avg_reactions_received,
                    AVG(active_days) as avg_active_days,
                    COUNT(*) as total_users
                FROM mv_user_statistics
                WHERE chat_id = :chat_id AND is_bot = false
            """
            result = self._execute_single(query, {"chat_id": chat_id})
        else:
            query = """
                WITH user_stats AS (
                    SELECT
                        m.user_id,
                        COUNT(*) as message_count,
                        COUNT(DISTINCT DATE(m.date)) as active_days
                    FROM messages m
                    JOIN users u ON u.id = m.user_id
                    WHERE m.chat_id = :chat_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                        AND u.is_bot = false
                    GROUP BY m.user_id
                ),
                reactions_sent AS (
                    SELECT mr.user_id, COUNT(*) as reactions_sent
                    FROM message_reactions mr
                    JOIN users u ON u.id = mr.user_id
                    WHERE mr.chat_id = :chat_id
                        AND mr.date >= :start_date
                        AND mr.date < :end_date
                        AND mr.is_removed = false
                        AND u.is_bot = false
                    GROUP BY mr.user_id
                ),
                reactions_received AS (
                    SELECT m.user_id, COUNT(*) as reactions_received
                    FROM message_reactions mr
                    JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                    JOIN users u ON u.id = m.user_id
                    WHERE mr.chat_id = :chat_id
                        AND mr.date >= :start_date
                        AND mr.date < :end_date
                        AND mr.is_removed = false
                        AND u.is_bot = false
                    GROUP BY m.user_id
                ),
                combined AS (
                    SELECT
                        COALESCE(us.message_count, 0) as message_count,
                        COALESCE(us.active_days, 0) as active_days,
                        COALESCE(rs.reactions_sent, 0) as reactions_sent,
                        COALESCE(rr.reactions_received, 0) as reactions_received
                    FROM user_stats us
                    LEFT JOIN reactions_sent rs ON rs.user_id = us.user_id
                    LEFT JOIN reactions_received rr ON rr.user_id = us.user_id
                )
                SELECT
                    AVG(message_count) as avg_messages,
                    AVG(reactions_sent) as avg_reactions_sent,
                    AVG(reactions_received) as avg_reactions_received,
                    AVG(active_days) as avg_active_days,
                    COUNT(*) as total_users
                FROM combined
            """
            result = self._execute_single(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )
        return result or {
            "avg_messages": 0,
            "avg_reactions_sent": 0,
            "avg_reactions_received": 0,
            "avg_active_days": 0,
            "total_users": 0,
        }

    def get_user_daily_activity(
        self,
        chat_id: int,
        user_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get a user's daily message activity.

        Returns DataFrame with columns:
            date, message_count
        """
        if start_date is None and end_date is None:
            query = """
                SELECT DATE(date) as date, COUNT(*) as message_count
                FROM messages
                WHERE chat_id = :chat_id AND user_id = :user_id
                GROUP BY DATE(date)
                ORDER BY date
            """
            return self._execute_df(query, {"chat_id": chat_id, "user_id": user_id})
        else:
            query = """
                SELECT DATE(date) as date, COUNT(*) as message_count
                FROM messages
                WHERE chat_id = :chat_id
                    AND user_id = :user_id
                    AND date >= :start_date
                    AND date < :end_date
                GROUP BY DATE(date)
                ORDER BY date
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "user_id": user_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_user_reaction_distribution(
        self,
        chat_id: int,
        user_id: int,
        limit: int = 10,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> list[dict]:
        """
        Get distribution of reactions a user sends.

        Returns list of:
            {'emoji': str, 'count': int}
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    COALESCE(emoji_value, custom_emoji_id, '?') as emoji,
                    COUNT(*) as count
                FROM message_reactions
                WHERE chat_id = :chat_id
                    AND user_id = :user_id
                    AND is_removed = false
                GROUP BY COALESCE(emoji_value, custom_emoji_id, '?')
                ORDER BY count DESC
                LIMIT :limit
            """
            return self._execute_many(
                query,
                {"chat_id": chat_id, "user_id": user_id, "limit": limit},
            )
        else:
            query = """
                SELECT
                    COALESCE(emoji_value, custom_emoji_id, '?') as emoji,
                    COUNT(*) as count
                FROM message_reactions
                WHERE chat_id = :chat_id
                    AND user_id = :user_id
                    AND date >= :start_date
                    AND date < :end_date
                    AND is_removed = false
                GROUP BY COALESCE(emoji_value, custom_emoji_id, '?')
                ORDER BY count DESC
                LIMIT :limit
            """
            return self._execute_many(
                query,
                {
                    "chat_id": chat_id,
                    "user_id": user_id,
                    "start_date": start_date,
                    "end_date": end_date,
                    "limit": limit,
                },
            )

    def get_user_reply_stats(
        self,
        chat_id: int,
        user_id: int,
        limit: int = 5,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict:
        """
        Get user's reply statistics.

        Returns:
            {
                'replies_sent': int,
                'replies_received': int,
                'top_replied_to': [{'user_id': int, 'first_name': str, 'count': int}],
                'top_repliers': [{'user_id': int, 'first_name': str, 'count': int}]
            }
        """
        base_params = {"chat_id": chat_id, "user_id": user_id}
        date_filter = ""
        if start_date is not None and end_date is not None:
            date_filter = " AND m.date >= :start_date AND m.date < :end_date"
            base_params["start_date"] = start_date
            base_params["end_date"] = end_date

        # Replies sent (messages where this user replied to someone)
        replies_sent_query = f"""
            SELECT COUNT(*) as count
            FROM messages m
            WHERE m.chat_id = :chat_id
                AND m.user_id = :user_id
                AND m.reply_to_message_id IS NOT NULL
                {date_filter}
        """
        sent_result = self._execute_single(replies_sent_query, base_params)
        replies_sent = sent_result.get("count", 0) if sent_result else 0

        # Replies received (messages that reply to this user's messages)
        replies_received_query = f"""
            SELECT COUNT(*) as count
            FROM messages m
            JOIN messages original ON original.chat_id = m.chat_id AND original.message_id = m.reply_to_message_id
            WHERE m.chat_id = :chat_id
                AND original.user_id = :user_id
                AND m.user_id != :user_id
                {date_filter}
        """
        received_result = self._execute_single(replies_received_query, base_params)
        replies_received = received_result.get("count", 0) if received_result else 0

        # Top users this person replies to
        top_replied_to_query = f"""
            SELECT
                original.user_id,
                u.first_name,
                COUNT(*) as count
            FROM messages m
            JOIN messages original ON original.chat_id = m.chat_id AND original.message_id = m.reply_to_message_id
            JOIN users u ON u.id = original.user_id
            WHERE m.chat_id = :chat_id
                AND m.user_id = :user_id
                AND original.user_id != :user_id
                {date_filter}
            GROUP BY original.user_id, u.first_name
            ORDER BY count DESC
            LIMIT :limit
        """
        top_replied_to = self._execute_many(
            top_replied_to_query, {**base_params, "limit": limit}
        )

        # Top users who reply to this person
        top_repliers_query = f"""
            SELECT
                m.user_id,
                u.first_name,
                COUNT(*) as count
            FROM messages m
            JOIN messages original ON original.chat_id = m.chat_id AND original.message_id = m.reply_to_message_id
            JOIN users u ON u.id = m.user_id
            WHERE m.chat_id = :chat_id
                AND original.user_id = :user_id
                AND m.user_id != :user_id
                {date_filter}
            GROUP BY m.user_id, u.first_name
            ORDER BY count DESC
            LIMIT :limit
        """
        top_repliers = self._execute_many(
            top_repliers_query, {**base_params, "limit": limit}
        )

        return {
            "replies_sent": replies_sent,
            "replies_received": replies_received,
            "top_replied_to": top_replied_to,
            "top_repliers": top_repliers,
        }

    def get_user_first_message_date(self, chat_id: int, user_id: int) -> date | None:
        """
        Get the date of a user's first message in a chat.

        Returns:
            Date of first message or None if no messages
        """
        query = """
            SELECT MIN(DATE(date)) as first_date
            FROM messages
            WHERE chat_id = :chat_id AND user_id = :user_id
        """
        result = self._execute_single(query, {"chat_id": chat_id, "user_id": user_id})
        return result.get("first_date") if result else None

    # =========================================
    # SENTIMENT METHODS
    # =========================================

    def get_sentiment_stats(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict:
        """
        Get sentiment overview statistics for a chat.

        Returns:
            {
                'total_analyzed': int,
                'total_messages': int,
                'positive_count': int,
                'neutral_count': int,
                'negative_count': int,
                'positive_rate': float,
                'negative_rate': float,
                'avg_confidence': float
            }
        """
        if start_date is None and end_date is None:
            query = """
                WITH stats AS (
                    SELECT
                        COUNT(*) as total_analyzed,
                        SUM(CASE WHEN label = 'positive' THEN 1 ELSE 0 END) as positive_count,
                        SUM(CASE WHEN label = 'neutral' THEN 1 ELSE 0 END) as neutral_count,
                        SUM(CASE WHEN label = 'negative' THEN 1 ELSE 0 END) as negative_count,
                        AVG(confidence) as avg_confidence
                    FROM ml_sentiment
                    WHERE chat_id = :chat_id
                ),
                total AS (
                    SELECT COUNT(*) as total_messages
                    FROM messages
                    WHERE chat_id = :chat_id
                        AND (text IS NOT NULL OR caption IS NOT NULL)
                )
                SELECT
                    s.total_analyzed,
                    t.total_messages,
                    s.positive_count,
                    s.neutral_count,
                    s.negative_count,
                    CASE WHEN s.total_analyzed > 0
                        THEN ROUND(s.positive_count * 100.0 / s.total_analyzed, 1)
                        ELSE 0 END as positive_rate,
                    CASE WHEN s.total_analyzed > 0
                        THEN ROUND(s.negative_count * 100.0 / s.total_analyzed, 1)
                        ELSE 0 END as negative_rate,
                    COALESCE(s.avg_confidence, 0) as avg_confidence
                FROM stats s, total t
            """
            result = self._execute_single(query, {"chat_id": chat_id})
        else:
            query = """
                WITH stats AS (
                    SELECT
                        COUNT(*) as total_analyzed,
                        SUM(CASE WHEN ms.label = 'positive' THEN 1 ELSE 0 END) as positive_count,
                        SUM(CASE WHEN ms.label = 'neutral' THEN 1 ELSE 0 END) as neutral_count,
                        SUM(CASE WHEN ms.label = 'negative' THEN 1 ELSE 0 END) as negative_count,
                        AVG(ms.confidence) as avg_confidence
                    FROM ml_sentiment ms
                    JOIN messages m ON m.id = ms.message_id
                    WHERE ms.chat_id = :chat_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                ),
                total AS (
                    SELECT COUNT(*) as total_messages
                    FROM messages
                    WHERE chat_id = :chat_id
                        AND date >= :start_date
                        AND date < :end_date
                        AND (text IS NOT NULL OR caption IS NOT NULL)
                )
                SELECT
                    s.total_analyzed,
                    t.total_messages,
                    s.positive_count,
                    s.neutral_count,
                    s.negative_count,
                    CASE WHEN s.total_analyzed > 0
                        THEN ROUND(s.positive_count * 100.0 / s.total_analyzed, 1)
                        ELSE 0 END as positive_rate,
                    CASE WHEN s.total_analyzed > 0
                        THEN ROUND(s.negative_count * 100.0 / s.total_analyzed, 1)
                        ELSE 0 END as negative_rate,
                    COALESCE(s.avg_confidence, 0) as avg_confidence
                FROM stats s, total t
            """
            result = self._execute_single(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

        return result or {
            "total_analyzed": 0,
            "total_messages": 0,
            "positive_count": 0,
            "neutral_count": 0,
            "negative_count": 0,
            "positive_rate": 0,
            "negative_rate": 0,
            "avg_confidence": 0,
        }

    def get_sentiment_distribution(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get sentiment label distribution for charts.

        Returns DataFrame with columns:
            label, count, percentage
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    label,
                    COUNT(*) as count,
                    ROUND(COUNT(*) * 100.0 / NULLIF(SUM(COUNT(*)) OVER (), 0), 1) as percentage
                FROM ml_sentiment
                WHERE chat_id = :chat_id
                GROUP BY label
                ORDER BY
                    CASE label
                        WHEN 'positive' THEN 1
                        WHEN 'neutral' THEN 2
                        WHEN 'negative' THEN 3
                    END
            """
            return self._execute_df(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    ms.label,
                    COUNT(*) as count,
                    ROUND(COUNT(*) * 100.0 / NULLIF(SUM(COUNT(*)) OVER (), 0), 1) as percentage
                FROM ml_sentiment ms
                JOIN messages m ON m.id = ms.message_id
                WHERE ms.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                GROUP BY ms.label
                ORDER BY
                    CASE ms.label
                        WHEN 'positive' THEN 1
                        WHEN 'neutral' THEN 2
                        WHEN 'negative' THEN 3
                    END
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_sentiment_timeline(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get daily sentiment counts for timeline chart.

        Returns DataFrame with columns:
            date, positive, neutral, negative
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    DATE(m.date) as date,
                    SUM(CASE WHEN ms.label = 'positive' THEN 1 ELSE 0 END) as positive,
                    SUM(CASE WHEN ms.label = 'neutral' THEN 1 ELSE 0 END) as neutral,
                    SUM(CASE WHEN ms.label = 'negative' THEN 1 ELSE 0 END) as negative
                FROM ml_sentiment ms
                JOIN messages m ON m.id = ms.message_id
                WHERE ms.chat_id = :chat_id
                GROUP BY DATE(m.date)
                ORDER BY date
            """
            return self._execute_df(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    DATE(m.date) as date,
                    SUM(CASE WHEN ms.label = 'positive' THEN 1 ELSE 0 END) as positive,
                    SUM(CASE WHEN ms.label = 'neutral' THEN 1 ELSE 0 END) as neutral,
                    SUM(CASE WHEN ms.label = 'negative' THEN 1 ELSE 0 END) as negative
                FROM ml_sentiment ms
                JOIN messages m ON m.id = ms.message_id
                WHERE ms.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                GROUP BY DATE(m.date)
                ORDER BY date
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_hourly_sentiment_heatmap(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get average sentiment by hour and day of week for heatmap.

        Sentiment is converted to numeric: positive=1, neutral=0, negative=-1

        Returns DataFrame with columns:
            day_of_week (0-6), hour (0-23), avg_sentiment (-1 to 1), message_count
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    EXTRACT(DOW FROM m.date)::int as day_of_week,
                    EXTRACT(HOUR FROM m.date)::int as hour,
                    AVG(CASE
                        WHEN ms.label = 'positive' THEN 1
                        WHEN ms.label = 'neutral' THEN 0
                        WHEN ms.label = 'negative' THEN -1
                    END) as avg_sentiment,
                    COUNT(*) as message_count
                FROM ml_sentiment ms
                JOIN messages m ON m.id = ms.message_id
                WHERE ms.chat_id = :chat_id
                GROUP BY day_of_week, hour
                ORDER BY day_of_week, hour
            """
            return self._execute_df(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    EXTRACT(DOW FROM m.date)::int as day_of_week,
                    EXTRACT(HOUR FROM m.date)::int as hour,
                    AVG(CASE
                        WHEN ms.label = 'positive' THEN 1
                        WHEN ms.label = 'neutral' THEN 0
                        WHEN ms.label = 'negative' THEN -1
                    END) as avg_sentiment,
                    COUNT(*) as message_count
                FROM ml_sentiment ms
                JOIN messages m ON m.id = ms.message_id
                WHERE ms.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                GROUP BY day_of_week, hour
                ORDER BY day_of_week, hour
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_user_sentiment_rankings(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
        limit: int = 10,
        ascending: bool = False,
    ) -> list[dict]:
        """
        Get users ranked by sentiment with Bayesian smoothing.

        Uses Bayesian smoothing to adjust for sample size:
        - Users with few messages are pulled toward the global mean
        - Users with many messages keep their observed score
        - Formula: smoothed = (n * raw + k * global) / (n + k) where k=50

        Note: This uses a fixed k=50 for dashboard rankings. Card generation
        uses progressive k (30→5) - see ml-processor/src/cards/calculators.py.

        Args:
            chat_id: Chat ID
            start_date: Start date filter (inclusive)
            end_date: End date filter (exclusive)
            limit: Number of users to return
            ascending: If True, returns most negative users first

        Returns list of:
            {
                'user_id': int,
                'first_name': str,
                'username': str | None,
                'raw_sentiment': float,      # Original unweighted average
                'smoothed_sentiment': float, # Bayesian smoothed score
                'confidence': float,         # 0-1 confidence based on msg count
                'messages_analyzed': int
            }
        """
        order = "ASC" if ascending else "DESC"
        # Bayesian smoothing factor (k=50 means 50 messages = 50% weight on raw)
        k = 50

        # Build date filter clause
        date_filter = ""
        if start_date is not None and end_date is not None:
            date_filter = "AND m.date >= :start_date AND m.date < :end_date"

        query = f"""
            WITH global_stats AS (
                -- Calculate global average sentiment for the chat (within date range)
                SELECT COALESCE(AVG(
                    CASE ms.label
                        WHEN 'positive' THEN 1.0
                        WHEN 'neutral' THEN 0.0
                        WHEN 'negative' THEN -1.0
                        ELSE 0.0
                    END
                ), 0.0) as global_avg
                FROM ml_sentiment ms
                JOIN messages m ON m.id = ms.message_id AND m.chat_id = ms.chat_id
                JOIN users u ON u.id = m.user_id
                WHERE ms.chat_id = :chat_id
                    AND u.is_bot = false
                    {date_filter}
            ),
            user_stats AS (
                -- Calculate per-user raw sentiment
                SELECT
                    u.id as user_id,
                    u.first_name,
                    u.username,
                    AVG(
                        CASE ms.label
                            WHEN 'positive' THEN 1.0
                            WHEN 'neutral' THEN 0.0
                            WHEN 'negative' THEN -1.0
                            ELSE 0.0
                        END
                    ) as raw_sentiment,
                    COUNT(*) as messages_analyzed
                FROM ml_sentiment ms
                JOIN messages m ON m.id = ms.message_id AND m.chat_id = ms.chat_id
                JOIN users u ON u.id = m.user_id
                WHERE ms.chat_id = :chat_id
                    AND u.is_bot = false
                    {date_filter}
                GROUP BY u.id, u.first_name, u.username
                HAVING COUNT(*) >= 5
            )
            SELECT
                us.user_id,
                us.first_name,
                us.username,
                us.raw_sentiment,
                -- Bayesian smoothed score: (n * raw + k * global) / (n + k)
                (us.messages_analyzed * us.raw_sentiment + {k} * gs.global_avg)
                    / (us.messages_analyzed + {k}) as smoothed_sentiment,
                -- Confidence: exponential approach to 1 as messages increase
                1 - EXP(-us.messages_analyzed::float / {k}) as confidence,
                us.messages_analyzed
            FROM user_stats us
            CROSS JOIN global_stats gs
            ORDER BY smoothed_sentiment {order}
            LIMIT :limit
        """

        params = {"chat_id": chat_id, "limit": limit}
        if start_date is not None and end_date is not None:
            params["start_date"] = start_date
            params["end_date"] = end_date

        return self._execute_many(query, params)

    def get_toxicity_stats(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict:
        """
        Get toxicity statistics for a chat.

        Returns:
            {
                'total_analyzed': int,
                'toxic_count': int,
                'toxic_rate': float
            }
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    COUNT(*) as total_analyzed,
                    SUM(CASE WHEN is_toxic THEN 1 ELSE 0 END) as toxic_count,
                    CASE WHEN COUNT(*) > 0
                        THEN ROUND(SUM(CASE WHEN is_toxic THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 2)
                        ELSE 0 END as toxic_rate
                FROM ml_toxicity
                WHERE chat_id = :chat_id
            """
            result = self._execute_single(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    COUNT(*) as total_analyzed,
                    SUM(CASE WHEN mt.is_toxic THEN 1 ELSE 0 END) as toxic_count,
                    CASE WHEN COUNT(*) > 0
                        THEN ROUND(SUM(CASE WHEN mt.is_toxic THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 2)
                        ELSE 0 END as toxic_rate
                FROM ml_toxicity mt
                JOIN messages m ON m.id = mt.message_id
                WHERE mt.chat_id = :chat_id
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
            )

        return result or {
            "total_analyzed": 0,
            "toxic_count": 0,
            "toxic_rate": 0,
        }

    def get_toxicity_timeline(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get daily toxicity counts for timeline chart.

        Returns DataFrame with columns:
            date, toxic_count, total_count
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    DATE(m.date) as date,
                    SUM(CASE WHEN mt.is_toxic THEN 1 ELSE 0 END) as toxic_count,
                    COUNT(*) as total_count
                FROM ml_toxicity mt
                JOIN messages m ON m.id = mt.message_id
                WHERE mt.chat_id = :chat_id
                GROUP BY DATE(m.date)
                ORDER BY date
            """
            return self._execute_df(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    DATE(m.date) as date,
                    SUM(CASE WHEN mt.is_toxic THEN 1 ELSE 0 END) as toxic_count,
                    COUNT(*) as total_count
                FROM ml_toxicity mt
                JOIN messages m ON m.id = mt.message_id
                WHERE mt.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                GROUP BY DATE(m.date)
                ORDER BY date
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_user_toxicity_rankings(
        self,
        chat_id: int,
        limit: int = 10,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> list[dict]:
        """
        Get users ranked by toxicity rate (highest first).

        Args:
            chat_id: Chat ID
            limit: Number of users to return
            start_date: Start date filter (inclusive)
            end_date: End date filter (exclusive)

        Returns list of:
            {
                'user_id': int,
                'first_name': str,
                'username': str | None,
                'toxicity_rate': float,
                'messages_analyzed': int
            }
        """
        if start_date is None and end_date is None:
            # All-time: use pre-aggregated ml_user_profiles
            query = """
                SELECT
                    u.id as user_id,
                    u.first_name,
                    u.username,
                    mup.toxicity_rate,
                    mup.messages_analyzed
                FROM ml_user_profiles mup
                JOIN users u ON u.id = mup.user_id
                WHERE mup.chat_id = :chat_id
                    AND mup.messages_analyzed >= 5
                    AND mup.toxicity_rate > 0
                ORDER BY mup.toxicity_rate DESC
                LIMIT :limit
            """
            return self._execute_many(query, {"chat_id": chat_id, "limit": limit})
        else:
            # Date-filtered: compute from ml_toxicity + messages
            query = """
                SELECT
                    m.user_id,
                    u.first_name,
                    u.username,
                    ROUND(
                        SUM(CASE WHEN t.is_toxic THEN 1 ELSE 0 END) * 100.0 / COUNT(*),
                        2
                    ) as toxicity_rate,
                    COUNT(*) as messages_analyzed
                FROM ml_toxicity t
                JOIN messages m ON m.id = t.message_id
                JOIN users u ON u.id = m.user_id
                WHERE t.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                    AND u.is_bot = false
                GROUP BY m.user_id, u.first_name, u.username
                HAVING COUNT(*) >= 5
                    AND SUM(CASE WHEN t.is_toxic THEN 1 ELSE 0 END) > 0
                ORDER BY toxicity_rate DESC
                LIMIT :limit
            """
            return self._execute_many(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                    "limit": limit,
                },
            )

    def get_user_sentiment_stats(
        self,
        chat_id: int,
        user_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict:
        """
        Get sentiment statistics for a specific user compared to group average.

        Args:
            chat_id: Chat ID
            user_id: User ID
            start_date: Start date filter (inclusive)
            end_date: End date filter (exclusive)

        Returns:
            {
                'avg_sentiment': float (-1 to 1),
                'positive_count': int,
                'neutral_count': int,
                'negative_count': int,
                'messages_analyzed': int,
                'group_avg_sentiment': float,
                'sentiment_rank': int | None,
                'total_ranked_users': int
            }
        """
        if start_date is None and end_date is None:
            query = """
                WITH user_sentiment AS (
                    SELECT
                        AVG(
                            CASE ms.label
                                WHEN 'positive' THEN 1.0
                                WHEN 'neutral' THEN 0.0
                                WHEN 'negative' THEN -1.0
                            END
                        ) as avg_sentiment,
                        SUM(CASE WHEN ms.label = 'positive' THEN 1 ELSE 0 END) as positive_count,
                        SUM(CASE WHEN ms.label = 'neutral' THEN 1 ELSE 0 END) as neutral_count,
                        SUM(CASE WHEN ms.label = 'negative' THEN 1 ELSE 0 END) as negative_count,
                        COUNT(*) as messages_analyzed
                    FROM ml_sentiment ms
                    JOIN messages m ON m.id = ms.message_id AND m.chat_id = ms.chat_id
                    WHERE ms.chat_id = :chat_id
                        AND m.user_id = :user_id
                ),
                group_sentiment AS (
                    SELECT
                        AVG(
                            CASE ms.label
                                WHEN 'positive' THEN 1.0
                                WHEN 'neutral' THEN 0.0
                                WHEN 'negative' THEN -1.0
                            END
                        ) as group_avg
                    FROM ml_sentiment ms
                    WHERE ms.chat_id = :chat_id
                ),
                user_ranks AS (
                    SELECT
                        m.user_id,
                        AVG(
                            CASE ms.label
                                WHEN 'positive' THEN 1.0
                                WHEN 'neutral' THEN 0.0
                                WHEN 'negative' THEN -1.0
                            END
                        ) as avg_sentiment,
                        COUNT(*) as msg_count
                    FROM ml_sentiment ms
                    JOIN messages m ON m.id = ms.message_id AND m.chat_id = ms.chat_id
                    JOIN users u ON u.id = m.user_id
                    WHERE ms.chat_id = :chat_id
                        AND u.is_bot = false
                    GROUP BY m.user_id
                    HAVING COUNT(*) >= 5
                ),
                ranked AS (
                    SELECT
                        user_id,
                        ROW_NUMBER() OVER (ORDER BY avg_sentiment DESC) as rank,
                        COUNT(*) OVER () as total_users
                    FROM user_ranks
                )
                SELECT
                    us.avg_sentiment,
                    us.positive_count,
                    us.neutral_count,
                    us.negative_count,
                    us.messages_analyzed,
                    gs.group_avg as group_avg_sentiment,
                    r.rank as sentiment_rank,
                    r.total_users as total_ranked_users
                FROM user_sentiment us
                CROSS JOIN group_sentiment gs
                LEFT JOIN ranked r ON r.user_id = :user_id
            """
            result = self._execute_single(
                query, {"chat_id": chat_id, "user_id": user_id}
            )
        else:
            query = """
                WITH user_sentiment AS (
                    SELECT
                        AVG(
                            CASE ms.label
                                WHEN 'positive' THEN 1.0
                                WHEN 'neutral' THEN 0.0
                                WHEN 'negative' THEN -1.0
                            END
                        ) as avg_sentiment,
                        SUM(CASE WHEN ms.label = 'positive' THEN 1 ELSE 0 END) as positive_count,
                        SUM(CASE WHEN ms.label = 'neutral' THEN 1 ELSE 0 END) as neutral_count,
                        SUM(CASE WHEN ms.label = 'negative' THEN 1 ELSE 0 END) as negative_count,
                        COUNT(*) as messages_analyzed
                    FROM ml_sentiment ms
                    JOIN messages m ON m.id = ms.message_id AND m.chat_id = ms.chat_id
                    WHERE ms.chat_id = :chat_id
                        AND m.user_id = :user_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                ),
                group_sentiment AS (
                    SELECT
                        AVG(
                            CASE ms.label
                                WHEN 'positive' THEN 1.0
                                WHEN 'neutral' THEN 0.0
                                WHEN 'negative' THEN -1.0
                            END
                        ) as group_avg
                    FROM ml_sentiment ms
                    JOIN messages m ON m.id = ms.message_id
                    WHERE ms.chat_id = :chat_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                ),
                user_ranks AS (
                    SELECT
                        m.user_id,
                        AVG(
                            CASE ms.label
                                WHEN 'positive' THEN 1.0
                                WHEN 'neutral' THEN 0.0
                                WHEN 'negative' THEN -1.0
                            END
                        ) as avg_sentiment,
                        COUNT(*) as msg_count
                    FROM ml_sentiment ms
                    JOIN messages m ON m.id = ms.message_id AND m.chat_id = ms.chat_id
                    JOIN users u ON u.id = m.user_id
                    WHERE ms.chat_id = :chat_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                        AND u.is_bot = false
                    GROUP BY m.user_id
                    HAVING COUNT(*) >= 5
                ),
                ranked AS (
                    SELECT
                        user_id,
                        ROW_NUMBER() OVER (ORDER BY avg_sentiment DESC) as rank,
                        COUNT(*) OVER () as total_users
                    FROM user_ranks
                )
                SELECT
                    us.avg_sentiment,
                    us.positive_count,
                    us.neutral_count,
                    us.negative_count,
                    us.messages_analyzed,
                    gs.group_avg as group_avg_sentiment,
                    r.rank as sentiment_rank,
                    r.total_users as total_ranked_users
                FROM user_sentiment us
                CROSS JOIN group_sentiment gs
                LEFT JOIN ranked r ON r.user_id = :user_id
            """
            result = self._execute_single(
                query,
                {
                    "chat_id": chat_id,
                    "user_id": user_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

        return result or {
            "avg_sentiment": None,
            "positive_count": 0,
            "neutral_count": 0,
            "negative_count": 0,
            "messages_analyzed": 0,
            "group_avg_sentiment": None,
            "sentiment_rank": None,
            "total_ranked_users": 0,
        }

    # =========================================
    # TOPICS METHODS
    # =========================================

    def get_topics_overview(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict:
        """
        Get topics overview statistics.

        Returns:
            {
                'total_topics': int,
                'total_messages_with_topics': int,
                'total_messages': int,
                'coverage_rate': float
            }
        """
        if start_date is None and end_date is None:
            query = """
                WITH topic_stats AS (
                    SELECT COUNT(DISTINCT topic_id) as total_topics
                    FROM ml_topics
                    WHERE chat_id = :chat_id AND topic_id >= 0
                ),
                message_stats AS (
                    SELECT COUNT(*) as total_messages_with_topics
                    FROM ml_message_topics
                    WHERE chat_id = :chat_id AND topic_id >= 0
                ),
                total AS (
                    SELECT COUNT(*) as total_messages
                    FROM messages
                    WHERE chat_id = :chat_id
                        AND (text IS NOT NULL OR caption IS NOT NULL)
                )
                SELECT
                    ts.total_topics,
                    ms.total_messages_with_topics,
                    t.total_messages,
                    CASE WHEN t.total_messages > 0
                        THEN ROUND(ms.total_messages_with_topics * 100.0 / t.total_messages, 1)
                        ELSE 0 END as coverage_rate
                FROM topic_stats ts, message_stats ms, total t
            """
            result = self._execute_single(query, {"chat_id": chat_id})
        else:
            query = """
                WITH topic_stats AS (
                    SELECT COUNT(DISTINCT mt.topic_id) as total_topics
                    FROM ml_message_topics mt
                    JOIN messages m ON m.id = mt.message_id
                    WHERE mt.chat_id = :chat_id
                        AND mt.topic_id >= 0
                        AND m.date >= :start_date
                        AND m.date < :end_date
                ),
                message_stats AS (
                    SELECT COUNT(*) as total_messages_with_topics
                    FROM ml_message_topics mt
                    JOIN messages m ON m.id = mt.message_id
                    WHERE mt.chat_id = :chat_id
                        AND mt.topic_id >= 0
                        AND m.date >= :start_date
                        AND m.date < :end_date
                ),
                total AS (
                    SELECT COUNT(*) as total_messages
                    FROM messages
                    WHERE chat_id = :chat_id
                        AND date >= :start_date
                        AND date < :end_date
                        AND (text IS NOT NULL OR caption IS NOT NULL)
                )
                SELECT
                    ts.total_topics,
                    ms.total_messages_with_topics,
                    t.total_messages,
                    CASE WHEN t.total_messages > 0
                        THEN ROUND(ms.total_messages_with_topics * 100.0 / t.total_messages, 1)
                        ELSE 0 END as coverage_rate
                FROM topic_stats ts, message_stats ms, total t
            """
            result = self._execute_single(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

        return result or {
            "total_topics": 0,
            "total_messages_with_topics": 0,
            "total_messages": 0,
            "coverage_rate": 0,
        }

    def get_topic_distribution(
        self,
        chat_id: int,
        limit: int = 15,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get topic distribution for charts.

        Returns DataFrame with columns:
            topic_id, keywords, message_count
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    t.topic_id,
                    t.keywords,
                    t.message_count
                FROM ml_topics t
                WHERE t.chat_id = :chat_id AND t.topic_id >= 0
                ORDER BY t.message_count DESC
                LIMIT :limit
            """
            return self._execute_df(query, {"chat_id": chat_id, "limit": limit})
        else:
            query = """
                SELECT
                    t.topic_id,
                    t.keywords,
                    COUNT(*) as message_count
                FROM ml_message_topics mt
                JOIN ml_topics t ON t.chat_id = mt.chat_id AND t.topic_id = mt.topic_id
                JOIN messages m ON m.id = mt.message_id
                WHERE mt.chat_id = :chat_id
                    AND mt.topic_id >= 0
                    AND m.date >= :start_date
                    AND m.date < :end_date
                GROUP BY t.topic_id, t.keywords
                ORDER BY message_count DESC
                LIMIT :limit
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                    "limit": limit,
                },
            )

    def get_topic_timeline(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
        top_n: int = 5,
    ) -> pd.DataFrame:
        """
        Get topic trends over time for top N topics.

        Returns DataFrame with columns:
            date, topic_id, keywords, count
        """
        if start_date is None and end_date is None:
            query = """
                WITH top_topics AS (
                    SELECT topic_id, keywords
                    FROM ml_topics
                    WHERE chat_id = :chat_id AND topic_id >= 0
                    ORDER BY message_count DESC
                    LIMIT :top_n
                )
                SELECT
                    DATE(m.date) as date,
                    tt.topic_id,
                    tt.keywords,
                    COUNT(*) as count
                FROM ml_message_topics mt
                JOIN top_topics tt ON tt.topic_id = mt.topic_id
                JOIN messages m ON m.id = mt.message_id
                WHERE mt.chat_id = :chat_id
                GROUP BY DATE(m.date), tt.topic_id, tt.keywords
                ORDER BY date, tt.topic_id
            """
            return self._execute_df(query, {"chat_id": chat_id, "top_n": top_n})
        else:
            query = """
                WITH top_topics AS (
                    SELECT t.topic_id, t.keywords
                    FROM ml_topics t
                    WHERE t.chat_id = :chat_id AND t.topic_id >= 0
                    ORDER BY t.message_count DESC
                    LIMIT :top_n
                )
                SELECT
                    DATE(m.date) as date,
                    tt.topic_id,
                    tt.keywords,
                    COUNT(*) as count
                FROM ml_message_topics mt
                JOIN top_topics tt ON tt.topic_id = mt.topic_id
                JOIN messages m ON m.id = mt.message_id
                WHERE mt.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                GROUP BY DATE(m.date), tt.topic_id, tt.keywords
                ORDER BY date, tt.topic_id
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                    "top_n": top_n,
                },
            )

    def get_user_topic_interests(
        self,
        chat_id: int,
        limit: int = 10,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> list[dict]:
        """
        Get users with their most discussed topics.

        Returns list of:
            {
                'user_id': int,
                'first_name': str,
                'top_topic_keywords': list[str],
                'topic_message_count': int
            }
        """
        if start_date is None and end_date is None:
            query = """
                WITH user_topics AS (
                    SELECT
                        m.user_id,
                        mt.topic_id,
                        t.keywords,
                        COUNT(*) as topic_count,
                        ROW_NUMBER() OVER (
                            PARTITION BY m.user_id
                            ORDER BY COUNT(*) DESC
                        ) as rn
                    FROM ml_message_topics mt
                    JOIN messages m ON m.id = mt.message_id
                    JOIN ml_topics t ON t.chat_id = mt.chat_id AND t.topic_id = mt.topic_id
                    JOIN users u ON u.id = m.user_id
                    WHERE mt.chat_id = :chat_id
                        AND mt.topic_id >= 0
                        AND u.is_bot = false
                    GROUP BY m.user_id, mt.topic_id, t.keywords
                )
                SELECT
                    ut.user_id,
                    u.first_name,
                    ut.keywords as top_topic_keywords,
                    ut.topic_count as topic_message_count
                FROM user_topics ut
                JOIN users u ON u.id = ut.user_id
                WHERE ut.rn = 1
                ORDER BY ut.topic_count DESC
                LIMIT :limit
            """
            return self._execute_many(query, {"chat_id": chat_id, "limit": limit})
        else:
            query = """
                WITH user_topics AS (
                    SELECT
                        m.user_id,
                        mt.topic_id,
                        t.keywords,
                        COUNT(*) as topic_count,
                        ROW_NUMBER() OVER (
                            PARTITION BY m.user_id
                            ORDER BY COUNT(*) DESC
                        ) as rn
                    FROM ml_message_topics mt
                    JOIN messages m ON m.id = mt.message_id
                    JOIN ml_topics t ON t.chat_id = mt.chat_id AND t.topic_id = mt.topic_id
                    JOIN users u ON u.id = m.user_id
                    WHERE mt.chat_id = :chat_id
                        AND mt.topic_id >= 0
                        AND m.date >= :start_date
                        AND m.date < :end_date
                        AND u.is_bot = false
                    GROUP BY m.user_id, mt.topic_id, t.keywords
                )
                SELECT
                    ut.user_id,
                    u.first_name,
                    ut.keywords as top_topic_keywords,
                    ut.topic_count as topic_message_count
                FROM user_topics ut
                JOIN users u ON u.id = ut.user_id
                WHERE ut.rn = 1
                ORDER BY ut.topic_count DESC
                LIMIT :limit
            """
            return self._execute_many(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                    "limit": limit,
                },
            )

    def get_ner_overview(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict:
        """
        Get NER overview statistics.

        Returns:
            {
                'total_entities': int,
                'unique_entities': int,
                'top_entity_type': str | None
            }
        """
        if start_date is None and end_date is None:
            query = """
                WITH stats AS (
                    SELECT
                        COUNT(*) as total_entities,
                        COUNT(DISTINCT entity_text) as unique_entities
                    FROM ml_ner
                    WHERE chat_id = :chat_id
                ),
                top_type AS (
                    SELECT entity_type
                    FROM ml_ner
                    WHERE chat_id = :chat_id
                    GROUP BY entity_type
                    ORDER BY COUNT(*) DESC
                    LIMIT 1
                )
                SELECT
                    s.total_entities,
                    s.unique_entities,
                    tt.entity_type as top_entity_type
                FROM stats s
                LEFT JOIN top_type tt ON true
            """
            result = self._execute_single(query, {"chat_id": chat_id})
        else:
            query = """
                WITH stats AS (
                    SELECT
                        COUNT(*) as total_entities,
                        COUNT(DISTINCT n.entity_text) as unique_entities
                    FROM ml_ner n
                    JOIN messages m ON m.id = n.message_id
                    WHERE n.chat_id = :chat_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                ),
                top_type AS (
                    SELECT n.entity_type
                    FROM ml_ner n
                    JOIN messages m ON m.id = n.message_id
                    WHERE n.chat_id = :chat_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                    GROUP BY n.entity_type
                    ORDER BY COUNT(*) DESC
                    LIMIT 1
                )
                SELECT
                    s.total_entities,
                    s.unique_entities,
                    tt.entity_type as top_entity_type
                FROM stats s
                LEFT JOIN top_type tt ON true
            """
            result = self._execute_single(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

        return result or {
            "total_entities": 0,
            "unique_entities": 0,
            "top_entity_type": None,
        }

    def get_ner_distribution(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get named entity type distribution.

        Returns DataFrame with columns:
            entity_type, count, percentage
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    entity_type,
                    COUNT(*) as count,
                    ROUND(COUNT(*) * 100.0 / NULLIF(SUM(COUNT(*)) OVER (), 0), 1) as percentage
                FROM ml_ner
                WHERE chat_id = :chat_id
                GROUP BY entity_type
                ORDER BY count DESC
            """
            return self._execute_df(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    n.entity_type,
                    COUNT(*) as count,
                    ROUND(COUNT(*) * 100.0 / NULLIF(SUM(COUNT(*)) OVER (), 0), 1) as percentage
                FROM ml_ner n
                JOIN messages m ON m.id = n.message_id
                WHERE n.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                GROUP BY n.entity_type
                ORDER BY count DESC
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_top_entities(
        self,
        chat_id: int,
        entity_type: str | None = None,
        limit: int = 20,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> list[dict]:
        """
        Get most mentioned entities.

        Args:
            entity_type: Filter by type (PERSON, ORG, LOC, MISC) or None for all

        Returns list of:
            {'entity_text': str, 'entity_type': str, 'count': int}
        """
        type_filter = "AND entity_type = :entity_type" if entity_type else ""

        if start_date is None and end_date is None:
            query = f"""
                SELECT
                    entity_text,
                    entity_type,
                    COUNT(*) as count
                FROM ml_ner
                WHERE chat_id = :chat_id
                    {type_filter}
                GROUP BY entity_text, entity_type
                ORDER BY count DESC
                LIMIT :limit
            """
            params = {"chat_id": chat_id, "limit": limit}
            if entity_type:
                params["entity_type"] = entity_type
            return self._execute_many(query, params)
        else:
            query = f"""
                SELECT
                    n.entity_text,
                    n.entity_type,
                    COUNT(*) as count
                FROM ml_ner n
                JOIN messages m ON m.id = n.message_id
                WHERE n.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                    {type_filter.replace('entity_type', 'n.entity_type')}
                GROUP BY n.entity_text, n.entity_type
                ORDER BY count DESC
                LIMIT :limit
            """
            params = {
                "chat_id": chat_id,
                "start_date": start_date,
                "end_date": end_date,
                "limit": limit,
            }
            if entity_type:
                params["entity_type"] = entity_type
            return self._execute_many(query, params)

    # =========================================
    # HUMOR METHODS
    # =========================================

    def get_humor_stats(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict:
        """
        Get humor overview statistics.

        Returns:
            {
                'total_analyzed': int,
                'humorous_count': int,
                'humor_rate': float,
                'avg_score': float,
                'top_humor_type': str | None
            }
        """
        if start_date is None and end_date is None:
            query = """
                WITH stats AS (
                    SELECT
                        COUNT(*) as total_analyzed,
                        SUM(CASE WHEN is_humorous THEN 1 ELSE 0 END) as humorous_count,
                        AVG(CASE WHEN is_humorous THEN score ELSE NULL END) as avg_score
                    FROM ml_humor
                    WHERE chat_id = :chat_id
                ),
                top_type AS (
                    SELECT humor_type
                    FROM ml_humor
                    WHERE chat_id = :chat_id AND is_humorous = true
                    GROUP BY humor_type
                    ORDER BY COUNT(*) DESC
                    LIMIT 1
                )
                SELECT
                    s.total_analyzed,
                    s.humorous_count,
                    CASE WHEN s.total_analyzed > 0
                        THEN ROUND(s.humorous_count * 100.0 / s.total_analyzed, 1)
                        ELSE 0 END as humor_rate,
                    COALESCE(s.avg_score, 0) as avg_score,
                    tt.humor_type as top_humor_type
                FROM stats s
                LEFT JOIN top_type tt ON true
            """
            result = self._execute_single(query, {"chat_id": chat_id})
        else:
            query = """
                WITH stats AS (
                    SELECT
                        COUNT(*) as total_analyzed,
                        SUM(CASE WHEN h.is_humorous THEN 1 ELSE 0 END) as humorous_count,
                        AVG(CASE WHEN h.is_humorous THEN h.score ELSE NULL END) as avg_score
                    FROM ml_humor h
                    JOIN messages m ON m.id = h.message_id
                    WHERE h.chat_id = :chat_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                ),
                top_type AS (
                    SELECT h.humor_type
                    FROM ml_humor h
                    JOIN messages m ON m.id = h.message_id
                    WHERE h.chat_id = :chat_id
                        AND h.is_humorous = true
                        AND m.date >= :start_date
                        AND m.date < :end_date
                    GROUP BY h.humor_type
                    ORDER BY COUNT(*) DESC
                    LIMIT 1
                )
                SELECT
                    s.total_analyzed,
                    s.humorous_count,
                    CASE WHEN s.total_analyzed > 0
                        THEN ROUND(s.humorous_count * 100.0 / s.total_analyzed, 1)
                        ELSE 0 END as humor_rate,
                    COALESCE(s.avg_score, 0) as avg_score,
                    tt.humor_type as top_humor_type
                FROM stats s
                LEFT JOIN top_type tt ON true
            """
            result = self._execute_single(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

        return result or {
            "total_analyzed": 0,
            "humorous_count": 0,
            "humor_rate": 0,
            "avg_score": 0,
            "top_humor_type": None,
        }

    def get_humor_type_distribution(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get humor type breakdown.

        Returns DataFrame with columns:
            humor_type, count, percentage
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    humor_type,
                    COUNT(*) as count,
                    ROUND(COUNT(*) * 100.0 / NULLIF(SUM(COUNT(*)) OVER (), 0), 1) as percentage
                FROM ml_humor
                WHERE chat_id = :chat_id AND is_humorous = true
                GROUP BY humor_type
                ORDER BY count DESC
            """
            return self._execute_df(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    h.humor_type,
                    COUNT(*) as count,
                    ROUND(COUNT(*) * 100.0 / NULLIF(SUM(COUNT(*)) OVER (), 0), 1) as percentage
                FROM ml_humor h
                JOIN messages m ON m.id = h.message_id
                WHERE h.chat_id = :chat_id
                    AND h.is_humorous = true
                    AND m.date >= :start_date
                    AND m.date < :end_date
                GROUP BY h.humor_type
                ORDER BY count DESC
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_humor_timeline(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get daily humor counts.

        Returns DataFrame with columns:
            date, humorous_count, total_count
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    DATE(m.date) as date,
                    SUM(CASE WHEN h.is_humorous THEN 1 ELSE 0 END) as humorous_count,
                    COUNT(*) as total_count
                FROM ml_humor h
                JOIN messages m ON m.id = h.message_id
                WHERE h.chat_id = :chat_id
                GROUP BY DATE(m.date)
                ORDER BY date
            """
            return self._execute_df(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    DATE(m.date) as date,
                    SUM(CASE WHEN h.is_humorous THEN 1 ELSE 0 END) as humorous_count,
                    COUNT(*) as total_count
                FROM ml_humor h
                JOIN messages m ON m.id = h.message_id
                WHERE h.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                GROUP BY DATE(m.date)
                ORDER BY date
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_funniest_users(
        self,
        chat_id: int,
        limit: int = 10,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> list[dict]:
        """
        Get users ranked by humor rate.

        Returns list of:
            {
                'user_id': int,
                'first_name': str,
                'humor_rate': float,
                'humorous_count': int,
                'messages_analyzed': int
            }
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    m.user_id,
                    u.first_name,
                    ROUND(SUM(CASE WHEN h.is_humorous THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 1) as humor_rate,
                    SUM(CASE WHEN h.is_humorous THEN 1 ELSE 0 END) as humorous_count,
                    COUNT(*) as messages_analyzed
                FROM ml_humor h
                JOIN messages m ON m.id = h.message_id
                JOIN users u ON u.id = m.user_id
                WHERE h.chat_id = :chat_id
                    AND u.is_bot = false
                GROUP BY m.user_id, u.first_name
                HAVING COUNT(*) >= 10
                ORDER BY humor_rate DESC
                LIMIT :limit
            """
            return self._execute_many(query, {"chat_id": chat_id, "limit": limit})
        else:
            query = """
                SELECT
                    m.user_id,
                    u.first_name,
                    ROUND(SUM(CASE WHEN h.is_humorous THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 1) as humor_rate,
                    SUM(CASE WHEN h.is_humorous THEN 1 ELSE 0 END) as humorous_count,
                    COUNT(*) as messages_analyzed
                FROM ml_humor h
                JOIN messages m ON m.id = h.message_id
                JOIN users u ON u.id = m.user_id
                WHERE h.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                    AND u.is_bot = false
                GROUP BY m.user_id, u.first_name
                HAVING COUNT(*) >= 10
                ORDER BY humor_rate DESC
                LIMIT :limit
            """
            return self._execute_many(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                    "limit": limit,
                },
            )

    # =========================================
    # QUESTIONS METHODS
    # =========================================

    def get_questions_stats(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict:
        """
        Get questions overview statistics.

        Returns:
            {
                'total_analyzed': int,
                'question_count': int,
                'question_rate': float,
                'top_question_type': str | None
            }
        """
        if start_date is None and end_date is None:
            query = """
                WITH stats AS (
                    SELECT
                        COUNT(*) as total_analyzed,
                        SUM(CASE WHEN is_question THEN 1 ELSE 0 END) as question_count
                    FROM ml_questions
                    WHERE chat_id = :chat_id
                ),
                top_type AS (
                    SELECT question_type
                    FROM ml_questions
                    WHERE chat_id = :chat_id AND is_question = true
                    GROUP BY question_type
                    ORDER BY COUNT(*) DESC
                    LIMIT 1
                )
                SELECT
                    s.total_analyzed,
                    s.question_count,
                    CASE WHEN s.total_analyzed > 0
                        THEN ROUND(s.question_count * 100.0 / s.total_analyzed, 1)
                        ELSE 0 END as question_rate,
                    tt.question_type as top_question_type
                FROM stats s
                LEFT JOIN top_type tt ON true
            """
            result = self._execute_single(query, {"chat_id": chat_id})
        else:
            query = """
                WITH stats AS (
                    SELECT
                        COUNT(*) as total_analyzed,
                        SUM(CASE WHEN q.is_question THEN 1 ELSE 0 END) as question_count
                    FROM ml_questions q
                    JOIN messages m ON m.id = q.message_id
                    WHERE q.chat_id = :chat_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                ),
                top_type AS (
                    SELECT q.question_type
                    FROM ml_questions q
                    JOIN messages m ON m.id = q.message_id
                    WHERE q.chat_id = :chat_id
                        AND q.is_question = true
                        AND m.date >= :start_date
                        AND m.date < :end_date
                    GROUP BY q.question_type
                    ORDER BY COUNT(*) DESC
                    LIMIT 1
                )
                SELECT
                    s.total_analyzed,
                    s.question_count,
                    CASE WHEN s.total_analyzed > 0
                        THEN ROUND(s.question_count * 100.0 / s.total_analyzed, 1)
                        ELSE 0 END as question_rate,
                    tt.question_type as top_question_type
                FROM stats s
                LEFT JOIN top_type tt ON true
            """
            result = self._execute_single(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

        return result or {
            "total_analyzed": 0,
            "question_count": 0,
            "question_rate": 0,
            "top_question_type": None,
        }

    def get_question_type_distribution(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get question type breakdown.

        Returns DataFrame with columns:
            question_type, count, percentage
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    question_type,
                    COUNT(*) as count,
                    ROUND(COUNT(*) * 100.0 / NULLIF(SUM(COUNT(*)) OVER (), 0), 1) as percentage
                FROM ml_questions
                WHERE chat_id = :chat_id AND is_question = true
                GROUP BY question_type
                ORDER BY count DESC
            """
            return self._execute_df(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    q.question_type,
                    COUNT(*) as count,
                    ROUND(COUNT(*) * 100.0 / NULLIF(SUM(COUNT(*)) OVER (), 0), 1) as percentage
                FROM ml_questions q
                JOIN messages m ON m.id = q.message_id
                WHERE q.chat_id = :chat_id
                    AND q.is_question = true
                    AND m.date >= :start_date
                    AND m.date < :end_date
                GROUP BY q.question_type
                ORDER BY count DESC
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_questions_timeline(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get daily question counts.

        Returns DataFrame with columns:
            date, question_count, total_count
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    DATE(m.date) as date,
                    SUM(CASE WHEN q.is_question THEN 1 ELSE 0 END) as question_count,
                    COUNT(*) as total_count
                FROM ml_questions q
                JOIN messages m ON m.id = q.message_id
                WHERE q.chat_id = :chat_id
                GROUP BY DATE(m.date)
                ORDER BY date
            """
            return self._execute_df(query, {"chat_id": chat_id})
        else:
            query = """
                SELECT
                    DATE(m.date) as date,
                    SUM(CASE WHEN q.is_question THEN 1 ELSE 0 END) as question_count,
                    COUNT(*) as total_count
                FROM ml_questions q
                JOIN messages m ON m.id = q.message_id
                WHERE q.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                GROUP BY DATE(m.date)
                ORDER BY date
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                },
            )

    def get_most_inquisitive_users(
        self,
        chat_id: int,
        limit: int = 10,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> list[dict]:
        """
        Get users ranked by question frequency.

        Returns list of:
            {
                'user_id': int,
                'first_name': str,
                'question_rate': float,
                'question_count': int,
                'messages_analyzed': int
            }
        """
        if start_date is None and end_date is None:
            query = """
                SELECT
                    m.user_id,
                    u.first_name,
                    ROUND(SUM(CASE WHEN q.is_question THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 1) as question_rate,
                    SUM(CASE WHEN q.is_question THEN 1 ELSE 0 END) as question_count,
                    COUNT(*) as messages_analyzed
                FROM ml_questions q
                JOIN messages m ON m.id = q.message_id
                JOIN users u ON u.id = m.user_id
                WHERE q.chat_id = :chat_id
                    AND u.is_bot = false
                GROUP BY m.user_id, u.first_name
                HAVING COUNT(*) >= 10
                ORDER BY question_rate DESC
                LIMIT :limit
            """
            return self._execute_many(query, {"chat_id": chat_id, "limit": limit})
        else:
            query = """
                SELECT
                    m.user_id,
                    u.first_name,
                    ROUND(SUM(CASE WHEN q.is_question THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 1) as question_rate,
                    SUM(CASE WHEN q.is_question THEN 1 ELSE 0 END) as question_count,
                    COUNT(*) as messages_analyzed
                FROM ml_questions q
                JOIN messages m ON m.id = q.message_id
                JOIN users u ON u.id = m.user_id
                WHERE q.chat_id = :chat_id
                    AND m.date >= :start_date
                    AND m.date < :end_date
                    AND u.is_bot = false
                GROUP BY m.user_id, u.first_name
                HAVING COUNT(*) >= 10
                ORDER BY question_rate DESC
                LIMIT :limit
            """
            return self._execute_many(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                    "limit": limit,
                },
            )

    # =========================================
    # COMEDY METHODS
    # =========================================

    # Emojis that indicate humor/laughter reactions
    LAUGH_EMOJIS = [
        # Classic laughs
        "😂", "🤣", "😆", "😄", "😅", "😸", "😹",
        # "I'm dead" / melting
        "🫠", "💀", "☠️", "⚰️",
        # Crying (from laughing)
        "😭",
        # Loud reactions
        "📢", "🗣️",
        # Physical comedy reactions
        "🤸", "🏃", "💨", "🐒", "🤡",
        # Effects / emphasis
        "🪄", "💯", "✨", "💥",
        # Silly faces
        "🤪", "😜", "🤑", "🤭", "🫢", "🫣", "🤯",
    ]

    def get_comedy_leaderboard(
        self,
        chat_id: int,
        limit: int = 20,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> list[dict]:
        """
        Get users ranked by hybrid comedy score.

        Formula: comedy_score = (reaction_component * 0.7) + (ml_component * 0.3)
        - reaction_component: log2(1 + laugh_reactions) * (distinct_reactors / max_distinct) * 10
        - ml_component: (humorous_count / messages_analyzed) * avg_humor_score * 100

        Returns list of:
            {
                'user_id': int,
                'first_name': str,
                'comedy_score': float,
                'reaction_score': float,
                'ml_score': float,
                'laugh_reactions': int,
                'distinct_reactors': int,
                'humorous_count': int,
                'messages_analyzed': int
            }
        """
        laugh_emojis_tuple = tuple(self.LAUGH_EMOJIS)

        if start_date is None and end_date is None:
            query = """
                WITH user_humor AS (
                    SELECT
                        m.user_id,
                        COUNT(*) as messages_analyzed,
                        SUM(CASE WHEN h.is_humorous THEN 1 ELSE 0 END) as humorous_count,
                        AVG(CASE WHEN h.is_humorous THEN h.score ELSE NULL END) as avg_humor_score
                    FROM ml_humor h
                    JOIN messages m ON m.id = h.message_id
                    JOIN users u ON u.id = m.user_id
                    WHERE h.chat_id = :chat_id
                        AND u.is_bot = false
                    GROUP BY m.user_id
                    HAVING COUNT(*) >= 10
                ),
                user_laugh_reactions AS (
                    SELECT
                        m.user_id,
                        COUNT(*) as laugh_reactions,
                        COUNT(DISTINCT mr.user_id) as distinct_reactors
                    FROM message_reactions mr
                    JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                    JOIN users u ON u.id = m.user_id
                    WHERE mr.chat_id = :chat_id
                        AND mr.emoji_value = ANY(:laugh_emojis)
                        AND mr.is_removed = false
                        AND u.is_bot = false
                    GROUP BY m.user_id
                ),
                max_reactors AS (
                    SELECT COALESCE(MAX(distinct_reactors), 1) as max_distinct
                    FROM user_laugh_reactions
                ),
                scores AS (
                    SELECT
                        COALESCE(uh.user_id, ulr.user_id) as user_id,
                        COALESCE(uh.messages_analyzed, 0) as messages_analyzed,
                        COALESCE(uh.humorous_count, 0) as humorous_count,
                        COALESCE(uh.avg_humor_score, 0) as avg_humor_score,
                        COALESCE(ulr.laugh_reactions, 0) as laugh_reactions,
                        COALESCE(ulr.distinct_reactors, 0) as distinct_reactors,
                        CASE WHEN uh.messages_analyzed > 0 THEN
                            (uh.humorous_count::float / uh.messages_analyzed) * COALESCE(uh.avg_humor_score, 0) * 100
                        ELSE 0 END as ml_component,
                        CASE WHEN COALESCE(ulr.laugh_reactions, 0) > 0 THEN
                            LOG(2, 1 + ulr.laugh_reactions) * (ulr.distinct_reactors::float / mr.max_distinct) * 10
                        ELSE 0 END as reaction_component
                    FROM user_humor uh
                    FULL OUTER JOIN user_laugh_reactions ulr ON ulr.user_id = uh.user_id
                    CROSS JOIN max_reactors mr
                )
                SELECT
                    s.user_id,
                    u.first_name,
                    s.messages_analyzed,
                    s.humorous_count,
                    s.laugh_reactions,
                    s.distinct_reactors,
                    ROUND(s.ml_component::numeric, 2) as ml_score,
                    ROUND(s.reaction_component::numeric, 2) as reaction_score,
                    ROUND((s.reaction_component * 0.7 + s.ml_component * 0.3)::numeric, 2) as comedy_score
                FROM scores s
                JOIN users u ON u.id = s.user_id
                WHERE s.humorous_count > 0 OR s.laugh_reactions >= 3
                ORDER BY comedy_score DESC
                LIMIT :limit
            """
            return self._execute_many(
                query,
                {"chat_id": chat_id, "limit": limit, "laugh_emojis": list(laugh_emojis_tuple)},
            )
        else:
            query = """
                WITH user_humor AS (
                    SELECT
                        m.user_id,
                        COUNT(*) as messages_analyzed,
                        SUM(CASE WHEN h.is_humorous THEN 1 ELSE 0 END) as humorous_count,
                        AVG(CASE WHEN h.is_humorous THEN h.score ELSE NULL END) as avg_humor_score
                    FROM ml_humor h
                    JOIN messages m ON m.id = h.message_id
                    JOIN users u ON u.id = m.user_id
                    WHERE h.chat_id = :chat_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                        AND u.is_bot = false
                    GROUP BY m.user_id
                    HAVING COUNT(*) >= 10
                ),
                user_laugh_reactions AS (
                    SELECT
                        m.user_id,
                        COUNT(*) as laugh_reactions,
                        COUNT(DISTINCT mr.user_id) as distinct_reactors
                    FROM message_reactions mr
                    JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                    JOIN users u ON u.id = m.user_id
                    WHERE mr.chat_id = :chat_id
                        AND mr.emoji_value = ANY(:laugh_emojis)
                        AND mr.is_removed = false
                        AND mr.date >= :start_date
                        AND mr.date < :end_date
                        AND u.is_bot = false
                    GROUP BY m.user_id
                ),
                max_reactors AS (
                    SELECT COALESCE(MAX(distinct_reactors), 1) as max_distinct
                    FROM user_laugh_reactions
                ),
                scores AS (
                    SELECT
                        COALESCE(uh.user_id, ulr.user_id) as user_id,
                        COALESCE(uh.messages_analyzed, 0) as messages_analyzed,
                        COALESCE(uh.humorous_count, 0) as humorous_count,
                        COALESCE(uh.avg_humor_score, 0) as avg_humor_score,
                        COALESCE(ulr.laugh_reactions, 0) as laugh_reactions,
                        COALESCE(ulr.distinct_reactors, 0) as distinct_reactors,
                        CASE WHEN uh.messages_analyzed > 0 THEN
                            (uh.humorous_count::float / uh.messages_analyzed) * COALESCE(uh.avg_humor_score, 0) * 100
                        ELSE 0 END as ml_component,
                        CASE WHEN COALESCE(ulr.laugh_reactions, 0) > 0 THEN
                            LOG(2, 1 + ulr.laugh_reactions) * (ulr.distinct_reactors::float / mr.max_distinct) * 10
                        ELSE 0 END as reaction_component
                    FROM user_humor uh
                    FULL OUTER JOIN user_laugh_reactions ulr ON ulr.user_id = uh.user_id
                    CROSS JOIN max_reactors mr
                )
                SELECT
                    s.user_id,
                    u.first_name,
                    s.messages_analyzed,
                    s.humorous_count,
                    s.laugh_reactions,
                    s.distinct_reactors,
                    ROUND(s.ml_component::numeric, 2) as ml_score,
                    ROUND(s.reaction_component::numeric, 2) as reaction_score,
                    ROUND((s.reaction_component * 0.7 + s.ml_component * 0.3)::numeric, 2) as comedy_score
                FROM scores s
                JOIN users u ON u.id = s.user_id
                WHERE s.humorous_count > 0 OR s.laugh_reactions >= 3
                ORDER BY comedy_score DESC
                LIMIT :limit
            """
            return self._execute_many(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                    "limit": limit,
                    "laugh_emojis": list(laugh_emojis_tuple),
                },
            )

    def get_comedy_stats(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict:
        """
        Get group-level comedy statistics.

        Returns:
            {
                'total_laugh_reactions': int,
                'unique_reactors': int,
                'messages_with_laughs': int,
                'humorous_count': int,
                'total_analyzed': int,
                'humor_rate': float
            }
        """
        laugh_emojis_tuple = tuple(self.LAUGH_EMOJIS)

        if start_date is None and end_date is None:
            query = """
                WITH humor_stats AS (
                    SELECT
                        COUNT(*) as total_analyzed,
                        SUM(CASE WHEN is_humorous THEN 1 ELSE 0 END) as humorous_count
                    FROM ml_humor h
                    JOIN messages m ON m.id = h.message_id
                    WHERE h.chat_id = :chat_id
                ),
                laugh_stats AS (
                    SELECT
                        COUNT(*) as total_laugh_reactions,
                        COUNT(DISTINCT user_id) as unique_reactors,
                        COUNT(DISTINCT message_id) as messages_with_laughs
                    FROM message_reactions
                    WHERE chat_id = :chat_id
                        AND emoji_value = ANY(:laugh_emojis)
                        AND is_removed = false
                )
                SELECT
                    hs.total_analyzed,
                    hs.humorous_count,
                    CASE WHEN hs.total_analyzed > 0
                        THEN ROUND(hs.humorous_count * 100.0 / hs.total_analyzed, 1)
                        ELSE 0 END as humor_rate,
                    ls.total_laugh_reactions,
                    ls.unique_reactors,
                    ls.messages_with_laughs
                FROM humor_stats hs, laugh_stats ls
            """
            return self._execute_single(
                query,
                {"chat_id": chat_id, "laugh_emojis": list(laugh_emojis_tuple)},
            ) or {}
        else:
            query = """
                WITH humor_stats AS (
                    SELECT
                        COUNT(*) as total_analyzed,
                        SUM(CASE WHEN is_humorous THEN 1 ELSE 0 END) as humorous_count
                    FROM ml_humor h
                    JOIN messages m ON m.id = h.message_id
                    WHERE h.chat_id = :chat_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                ),
                laugh_stats AS (
                    SELECT
                        COUNT(*) as total_laugh_reactions,
                        COUNT(DISTINCT user_id) as unique_reactors,
                        COUNT(DISTINCT message_id) as messages_with_laughs
                    FROM message_reactions
                    WHERE chat_id = :chat_id
                        AND emoji_value = ANY(:laugh_emojis)
                        AND is_removed = false
                        AND date >= :start_date
                        AND date < :end_date
                )
                SELECT
                    hs.total_analyzed,
                    hs.humorous_count,
                    CASE WHEN hs.total_analyzed > 0
                        THEN ROUND(hs.humorous_count * 100.0 / hs.total_analyzed, 1)
                        ELSE 0 END as humor_rate,
                    ls.total_laugh_reactions,
                    ls.unique_reactors,
                    ls.messages_with_laughs
                FROM humor_stats hs, laugh_stats ls
            """
            return self._execute_single(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                    "laugh_emojis": list(laugh_emojis_tuple),
                },
            ) or {}

    def get_top_funny_messages(
        self,
        chat_id: int,
        limit: int = 5,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> list[dict]:
        """
        Get top funny messages by combined score.

        Returns list of:
            {
                'message_id': int,
                'text': str,
                'date': datetime,
                'author_name': str,
                'author_id': int,
                'humor_type': str,
                'ml_score': float,
                'laugh_reactions': int,
                'combined_score': float
            }
        """
        laugh_emojis_tuple = tuple(self.LAUGH_EMOJIS)

        if start_date is None and end_date is None:
            query = """
                SELECT
                    m.id as message_id,
                    m.text,
                    m.date,
                    u.first_name as author_name,
                    m.user_id as author_id,
                    h.humor_type,
                    ROUND(h.score::numeric, 2) as ml_score,
                    COALESCE(lr.laugh_count, 0) as laugh_reactions,
                    ROUND((h.score * 0.3 + COALESCE(LOG(2, 1 + lr.laugh_count) / 10, 0) * 0.7)::numeric, 3) as combined_score
                FROM ml_humor h
                JOIN messages m ON m.id = h.message_id
                JOIN users u ON u.id = m.user_id
                LEFT JOIN (
                    SELECT chat_id, message_id, COUNT(*) as laugh_count
                    FROM message_reactions
                    WHERE emoji_value = ANY(:laugh_emojis)
                        AND is_removed = false
                    GROUP BY chat_id, message_id
                ) lr ON lr.chat_id = m.chat_id AND lr.message_id = m.message_id
                WHERE h.chat_id = :chat_id
                    AND h.is_humorous = true
                    AND u.is_bot = false
                ORDER BY combined_score DESC
                LIMIT :limit
            """
            return self._execute_many(
                query,
                {"chat_id": chat_id, "limit": limit, "laugh_emojis": list(laugh_emojis_tuple)},
            )
        else:
            query = """
                SELECT
                    m.id as message_id,
                    m.text,
                    m.date,
                    u.first_name as author_name,
                    m.user_id as author_id,
                    h.humor_type,
                    ROUND(h.score::numeric, 2) as ml_score,
                    COALESCE(lr.laugh_count, 0) as laugh_reactions,
                    ROUND((h.score * 0.3 + COALESCE(LOG(2, 1 + lr.laugh_count) / 10, 0) * 0.7)::numeric, 3) as combined_score
                FROM ml_humor h
                JOIN messages m ON m.id = h.message_id
                JOIN users u ON u.id = m.user_id
                LEFT JOIN (
                    SELECT chat_id, message_id, COUNT(*) as laugh_count
                    FROM message_reactions
                    WHERE emoji_value = ANY(:laugh_emojis)
                        AND is_removed = false
                        AND date >= :start_date
                        AND date < :end_date
                    GROUP BY chat_id, message_id
                ) lr ON lr.chat_id = m.chat_id AND lr.message_id = m.message_id
                WHERE h.chat_id = :chat_id
                    AND h.is_humorous = true
                    AND u.is_bot = false
                    AND m.date >= :start_date
                    AND m.date < :end_date
                ORDER BY combined_score DESC
                LIMIT :limit
            """
            return self._execute_many(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                    "limit": limit,
                    "laugh_emojis": list(laugh_emojis_tuple),
                },
            )

    def get_comedy_timeline(
        self,
        chat_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> pd.DataFrame:
        """
        Get daily comedy stats (laugh reactions + humorous messages).

        Returns DataFrame with columns:
            date, laugh_reactions, humorous_count
        """
        laugh_emojis_tuple = tuple(self.LAUGH_EMOJIS)

        if start_date is None and end_date is None:
            query = """
                WITH dates AS (
                    SELECT DISTINCT DATE(date) as date
                    FROM messages
                    WHERE chat_id = :chat_id
                ),
                laugh_by_date AS (
                    SELECT DATE(date) as date, COUNT(*) as laugh_reactions
                    FROM message_reactions
                    WHERE chat_id = :chat_id
                        AND emoji_value = ANY(:laugh_emojis)
                        AND is_removed = false
                    GROUP BY DATE(date)
                ),
                humor_by_date AS (
                    SELECT DATE(m.date) as date, COUNT(*) as humorous_count
                    FROM ml_humor h
                    JOIN messages m ON m.id = h.message_id
                    WHERE h.chat_id = :chat_id
                        AND h.is_humorous = true
                    GROUP BY DATE(m.date)
                )
                SELECT
                    d.date,
                    COALESCE(l.laugh_reactions, 0) as laugh_reactions,
                    COALESCE(h.humorous_count, 0) as humorous_count
                FROM dates d
                LEFT JOIN laugh_by_date l ON l.date = d.date
                LEFT JOIN humor_by_date h ON h.date = d.date
                WHERE COALESCE(l.laugh_reactions, 0) > 0 OR COALESCE(h.humorous_count, 0) > 0
                ORDER BY d.date
            """
            return self._execute_df(
                query,
                {"chat_id": chat_id, "laugh_emojis": list(laugh_emojis_tuple)},
            )
        else:
            query = """
                WITH dates AS (
                    SELECT DISTINCT DATE(date) as date
                    FROM messages
                    WHERE chat_id = :chat_id
                        AND date >= :start_date
                        AND date < :end_date
                ),
                laugh_by_date AS (
                    SELECT DATE(date) as date, COUNT(*) as laugh_reactions
                    FROM message_reactions
                    WHERE chat_id = :chat_id
                        AND emoji_value = ANY(:laugh_emojis)
                        AND is_removed = false
                        AND date >= :start_date
                        AND date < :end_date
                    GROUP BY DATE(date)
                ),
                humor_by_date AS (
                    SELECT DATE(m.date) as date, COUNT(*) as humorous_count
                    FROM ml_humor h
                    JOIN messages m ON m.id = h.message_id
                    WHERE h.chat_id = :chat_id
                        AND h.is_humorous = true
                        AND m.date >= :start_date
                        AND m.date < :end_date
                    GROUP BY DATE(m.date)
                )
                SELECT
                    d.date,
                    COALESCE(l.laugh_reactions, 0) as laugh_reactions,
                    COALESCE(h.humorous_count, 0) as humorous_count
                FROM dates d
                LEFT JOIN laugh_by_date l ON l.date = d.date
                LEFT JOIN humor_by_date h ON h.date = d.date
                WHERE COALESCE(l.laugh_reactions, 0) > 0 OR COALESCE(h.humorous_count, 0) > 0
                ORDER BY d.date
            """
            return self._execute_df(
                query,
                {
                    "chat_id": chat_id,
                    "start_date": start_date,
                    "end_date": end_date,
                    "laugh_emojis": list(laugh_emojis_tuple),
                },
            )

    def get_user_comedy_stats(
        self,
        chat_id: int,
        user_id: int,
        start_date: date | None = None,
        end_date: date | None = None,
    ) -> dict:
        """
        Get personal comedy stats for comparison.

        Returns:
            {
                'messages_analyzed': int,
                'humorous_count': int,
                'humor_rate': float,
                'laugh_reactions_received': int,
                'distinct_reactors': int,
                'comedy_score': float,
                'group_avg_comedy_score': float
            }
        """
        laugh_emojis_tuple = tuple(self.LAUGH_EMOJIS)

        if start_date is None and end_date is None:
            query = """
                WITH user_humor AS (
                    SELECT
                        COUNT(*) as messages_analyzed,
                        SUM(CASE WHEN h.is_humorous THEN 1 ELSE 0 END) as humorous_count,
                        AVG(CASE WHEN h.is_humorous THEN h.score ELSE NULL END) as avg_humor_score
                    FROM ml_humor h
                    JOIN messages m ON m.id = h.message_id
                    WHERE h.chat_id = :chat_id
                        AND m.user_id = :user_id
                ),
                user_reactions AS (
                    SELECT
                        COUNT(*) as laugh_reactions_received,
                        COUNT(DISTINCT mr.user_id) as distinct_reactors
                    FROM message_reactions mr
                    JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                    WHERE mr.chat_id = :chat_id
                        AND m.user_id = :user_id
                        AND mr.emoji_value = ANY(:laugh_emojis)
                        AND mr.is_removed = false
                ),
                max_reactors AS (
                    SELECT COALESCE(MAX(cnt), 1) as max_distinct
                    FROM (
                        SELECT COUNT(DISTINCT mr.user_id) as cnt
                        FROM message_reactions mr
                        JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                        WHERE mr.chat_id = :chat_id
                            AND mr.emoji_value = ANY(:laugh_emojis)
                            AND mr.is_removed = false
                        GROUP BY m.user_id
                    ) sub
                ),
                group_scores AS (
                    SELECT AVG(comedy_score) as avg_score
                    FROM (
                        SELECT
                            (CASE WHEN uh.messages_analyzed > 0 THEN
                                (uh.humorous_count::float / uh.messages_analyzed) * COALESCE(uh.avg_humor_score, 0) * 100
                            ELSE 0 END * 0.3) +
                            (CASE WHEN COALESCE(ulr.laugh_reactions, 0) > 0 THEN
                                LOG(2, 1 + ulr.laugh_reactions) * (ulr.distinct_reactors::float / mr.max_distinct) * 10
                            ELSE 0 END * 0.7) as comedy_score
                        FROM (
                            SELECT m.user_id, COUNT(*) as messages_analyzed,
                                SUM(CASE WHEN h.is_humorous THEN 1 ELSE 0 END) as humorous_count,
                                AVG(CASE WHEN h.is_humorous THEN h.score ELSE NULL END) as avg_humor_score
                            FROM ml_humor h
                            JOIN messages m ON m.id = h.message_id
                            JOIN users u ON u.id = m.user_id
                            WHERE h.chat_id = :chat_id AND u.is_bot = false
                            GROUP BY m.user_id
                            HAVING COUNT(*) >= 10
                        ) uh
                        LEFT JOIN (
                            SELECT m.user_id, COUNT(*) as laugh_reactions, COUNT(DISTINCT mr.user_id) as distinct_reactors
                            FROM message_reactions mr
                            JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                            WHERE mr.chat_id = :chat_id AND mr.emoji_value = ANY(:laugh_emojis) AND mr.is_removed = false
                            GROUP BY m.user_id
                        ) ulr ON ulr.user_id = uh.user_id
                        CROSS JOIN (SELECT COALESCE(MAX(distinct_reactors), 1) as max_distinct FROM (
                            SELECT COUNT(DISTINCT mr.user_id) as distinct_reactors
                            FROM message_reactions mr
                            JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                            WHERE mr.chat_id = :chat_id AND mr.emoji_value = ANY(:laugh_emojis) AND mr.is_removed = false
                            GROUP BY m.user_id
                        ) sub) mr
                    ) scores
                )
                SELECT
                    uh.messages_analyzed,
                    uh.humorous_count,
                    CASE WHEN uh.messages_analyzed > 0
                        THEN ROUND(uh.humorous_count * 100.0 / uh.messages_analyzed, 1)
                        ELSE 0 END as humor_rate,
                    ur.laugh_reactions_received,
                    ur.distinct_reactors,
                    ROUND((
                        (CASE WHEN uh.messages_analyzed > 0 THEN
                            (uh.humorous_count::float / uh.messages_analyzed) * COALESCE(uh.avg_humor_score, 0) * 100
                        ELSE 0 END * 0.3) +
                        (CASE WHEN ur.laugh_reactions_received > 0 THEN
                            LOG(2, 1 + ur.laugh_reactions_received) * (ur.distinct_reactors::float / mr.max_distinct) * 10
                        ELSE 0 END * 0.7)
                    )::numeric, 2) as comedy_score,
                    ROUND(gs.avg_score::numeric, 2) as group_avg_comedy_score
                FROM user_humor uh, user_reactions ur, max_reactors mr, group_scores gs
            """
            return self._execute_single(
                query,
                {"chat_id": chat_id, "user_id": user_id, "laugh_emojis": list(laugh_emojis_tuple)},
            ) or {}
        else:
            query = """
                WITH user_humor AS (
                    SELECT
                        COUNT(*) as messages_analyzed,
                        SUM(CASE WHEN h.is_humorous THEN 1 ELSE 0 END) as humorous_count,
                        AVG(CASE WHEN h.is_humorous THEN h.score ELSE NULL END) as avg_humor_score
                    FROM ml_humor h
                    JOIN messages m ON m.id = h.message_id
                    WHERE h.chat_id = :chat_id
                        AND m.user_id = :user_id
                        AND m.date >= :start_date
                        AND m.date < :end_date
                ),
                user_reactions AS (
                    SELECT
                        COUNT(*) as laugh_reactions_received,
                        COUNT(DISTINCT mr.user_id) as distinct_reactors
                    FROM message_reactions mr
                    JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                    WHERE mr.chat_id = :chat_id
                        AND m.user_id = :user_id
                        AND mr.emoji_value = ANY(:laugh_emojis)
                        AND mr.is_removed = false
                        AND mr.date >= :start_date
                        AND mr.date < :end_date
                ),
                max_reactors AS (
                    SELECT COALESCE(MAX(cnt), 1) as max_distinct
                    FROM (
                        SELECT COUNT(DISTINCT mr.user_id) as cnt
                        FROM message_reactions mr
                        JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                        WHERE mr.chat_id = :chat_id
                            AND mr.emoji_value = ANY(:laugh_emojis)
                            AND mr.is_removed = false
                            AND mr.date >= :start_date
                            AND mr.date < :end_date
                        GROUP BY m.user_id
                    ) sub
                ),
                group_scores AS (
                    SELECT AVG(comedy_score) as avg_score
                    FROM (
                        SELECT
                            (CASE WHEN uh.messages_analyzed > 0 THEN
                                (uh.humorous_count::float / uh.messages_analyzed) * COALESCE(uh.avg_humor_score, 0) * 100
                            ELSE 0 END * 0.3) +
                            (CASE WHEN COALESCE(ulr.laugh_reactions, 0) > 0 THEN
                                LOG(2, 1 + ulr.laugh_reactions) * (ulr.distinct_reactors::float / mr.max_distinct) * 10
                            ELSE 0 END * 0.7) as comedy_score
                        FROM (
                            SELECT m.user_id, COUNT(*) as messages_analyzed,
                                SUM(CASE WHEN h.is_humorous THEN 1 ELSE 0 END) as humorous_count,
                                AVG(CASE WHEN h.is_humorous THEN h.score ELSE NULL END) as avg_humor_score
                            FROM ml_humor h
                            JOIN messages m ON m.id = h.message_id
                            JOIN users u ON u.id = m.user_id
                            WHERE h.chat_id = :chat_id AND u.is_bot = false
                                AND m.date >= :start_date AND m.date < :end_date
                            GROUP BY m.user_id
                            HAVING COUNT(*) >= 10
                        ) uh
                        LEFT JOIN (
                            SELECT m.user_id, COUNT(*) as laugh_reactions, COUNT(DISTINCT mr.user_id) as distinct_reactors
                            FROM message_reactions mr
                            JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                            WHERE mr.chat_id = :chat_id AND mr.emoji_value = ANY(:laugh_emojis) AND mr.is_removed = false
                                AND mr.date >= :start_date AND mr.date < :end_date
                            GROUP BY m.user_id
                        ) ulr ON ulr.user_id = uh.user_id
                        CROSS JOIN (SELECT COALESCE(MAX(distinct_reactors), 1) as max_distinct FROM (
                            SELECT COUNT(DISTINCT mr.user_id) as distinct_reactors
                            FROM message_reactions mr
                            JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                            WHERE mr.chat_id = :chat_id AND mr.emoji_value = ANY(:laugh_emojis) AND mr.is_removed = false
                                AND mr.date >= :start_date AND mr.date < :end_date
                            GROUP BY m.user_id
                        ) sub) mr
                    ) scores
                )
                SELECT
                    uh.messages_analyzed,
                    uh.humorous_count,
                    CASE WHEN uh.messages_analyzed > 0
                        THEN ROUND(uh.humorous_count * 100.0 / uh.messages_analyzed, 1)
                        ELSE 0 END as humor_rate,
                    ur.laugh_reactions_received,
                    ur.distinct_reactors,
                    ROUND((
                        (CASE WHEN uh.messages_analyzed > 0 THEN
                            (uh.humorous_count::float / uh.messages_analyzed) * COALESCE(uh.avg_humor_score, 0) * 100
                        ELSE 0 END * 0.3) +
                        (CASE WHEN ur.laugh_reactions_received > 0 THEN
                            LOG(2, 1 + ur.laugh_reactions_received) * (ur.distinct_reactors::float / mr.max_distinct) * 10
                        ELSE 0 END * 0.7)
                    )::numeric, 2) as comedy_score,
                    ROUND(gs.avg_score::numeric, 2) as group_avg_comedy_score
                FROM user_humor uh, user_reactions ur, max_reactors mr, group_scores gs
            """
            return self._execute_single(
                query,
                {
                    "chat_id": chat_id,
                    "user_id": user_id,
                    "start_date": start_date,
                    "end_date": end_date,
                    "laugh_emojis": list(laugh_emojis_tuple),
                },
            ) or {}
