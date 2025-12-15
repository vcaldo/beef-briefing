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
