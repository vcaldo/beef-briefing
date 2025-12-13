"""
Analytics queries for Beef Dashboard.
Contains SQL queries for dashboard visualizations.
"""

import logging
from datetime import datetime, timedelta
from typing import Any, Dict, List, Optional, Tuple

import pandas as pd
from sqlalchemy import text
from sqlalchemy.engine import Engine

logger = logging.getLogger(__name__)


class DashboardQueries:
    """Database queries for dashboard analytics."""

    def __init__(self, engine: Engine):
        self.engine = engine

    def get_overview_stats(
        self,
        chat_id: int,
        start_date: datetime,
        end_date: datetime
    ) -> Dict[str, Any]:
        """
        Get overview statistics for a chat within a date range.
        Returns: total_messages, active_users, total_reactions, media_count
        """
        query = text("""
            WITH period_messages AS (
                SELECT
                    COUNT(*) as total_messages,
                    COUNT(DISTINCT user_id) as active_users
                FROM messages
                WHERE chat_id = :chat_id
                  AND date >= :start_date
                  AND date < :end_date
            ),
            period_reactions AS (
                SELECT COUNT(*) as total_reactions
                FROM message_reactions
                WHERE chat_id = :chat_id
                  AND date >= :start_date
                  AND date < :end_date
                  AND is_removed = FALSE
            ),
            period_media AS (
                SELECT COUNT(DISTINCT mf.id) as media_count
                FROM media_files mf
                JOIN messages m ON m.id = mf.message_id
                WHERE m.chat_id = :chat_id
                  AND m.date >= :start_date
                  AND m.date < :end_date
            )
            SELECT
                pm.total_messages,
                pm.active_users,
                pr.total_reactions,
                pmed.media_count
            FROM period_messages pm
            CROSS JOIN period_reactions pr
            CROSS JOIN period_media pmed
        """)

        with self.engine.connect() as conn:
            result = conn.execute(query, {
                "chat_id": chat_id,
                "start_date": start_date,
                "end_date": end_date,
            }).fetchone()

        if result:
            return {
                "total_messages": result.total_messages or 0,
                "active_users": result.active_users or 0,
                "total_reactions": result.total_reactions or 0,
                "media_count": result.media_count or 0,
            }
        return {
            "total_messages": 0,
            "active_users": 0,
            "total_reactions": 0,
            "media_count": 0,
        }

    def get_daily_activity(
        self,
        chat_id: int,
        start_date: datetime,
        end_date: datetime
    ) -> pd.DataFrame:
        """
        Get daily message counts for activity heatmap.
        Returns DataFrame with columns: date, message_count
        """
        query = text("""
            SELECT
                DATE(date) as date,
                COUNT(*) as message_count
            FROM messages
            WHERE chat_id = :chat_id
              AND date >= :start_date
              AND date < :end_date
            GROUP BY DATE(date)
            ORDER BY date
        """)

        with self.engine.connect() as conn:
            df = pd.read_sql(query, conn, params={
                "chat_id": chat_id,
                "start_date": start_date,
                "end_date": end_date,
            })

        return df

    def get_message_timeline(
        self,
        chat_id: int,
        start_date: datetime,
        end_date: datetime,
        granularity: str = "day"
    ) -> pd.DataFrame:
        """
        Get message counts over time with specified granularity.
        granularity: 'hour', 'day', 'week', 'month'
        Returns DataFrame with columns: period, message_count, user_count
        """
        # Build date truncation based on granularity
        trunc_map = {
            "hour": "hour",
            "day": "day",
            "week": "week",
            "month": "month",
        }
        trunc = trunc_map.get(granularity, "day")

        query = text(f"""
            SELECT
                DATE_TRUNC('{trunc}', date) as period,
                COUNT(*) as message_count,
                COUNT(DISTINCT user_id) as user_count
            FROM messages
            WHERE chat_id = :chat_id
              AND date >= :start_date
              AND date < :end_date
            GROUP BY DATE_TRUNC('{trunc}', date)
            ORDER BY period
        """)

        with self.engine.connect() as conn:
            df = pd.read_sql(query, conn, params={
                "chat_id": chat_id,
                "start_date": start_date,
                "end_date": end_date,
            })

        return df

    def get_user_rankings(
        self,
        chat_id: int,
        start_date: datetime,
        end_date: datetime,
        limit: int = 20
    ) -> pd.DataFrame:
        """
        Get user leaderboard with message counts and statistics.
        Returns DataFrame with user stats sorted by message count.
        """
        query = text("""
            WITH user_messages AS (
                SELECT
                    m.user_id,
                    COUNT(*) as message_count,
                    AVG(LENGTH(COALESCE(m.text, ''))) as avg_message_length,
                    COUNT(DISTINCT DATE(m.date)) as active_days
                FROM messages m
                WHERE m.chat_id = :chat_id
                  AND m.date >= :start_date
                  AND m.date < :end_date
                  AND m.user_id IS NOT NULL
                GROUP BY m.user_id
            ),
            user_reactions_sent AS (
                SELECT
                    mr.user_id,
                    COUNT(*) as reactions_sent
                FROM message_reactions mr
                WHERE mr.chat_id = :chat_id
                  AND mr.date >= :start_date
                  AND mr.date < :end_date
                  AND mr.is_removed = FALSE
                  AND mr.user_id IS NOT NULL
                GROUP BY mr.user_id
            ),
            user_reactions_received AS (
                SELECT
                    m.user_id,
                    COUNT(*) as reactions_received
                FROM message_reactions mr
                JOIN messages m ON m.chat_id = mr.chat_id AND m.message_id = mr.message_id
                WHERE mr.chat_id = :chat_id
                  AND mr.date >= :start_date
                  AND mr.date < :end_date
                  AND mr.is_removed = FALSE
                  AND m.user_id IS NOT NULL
                GROUP BY m.user_id
            )
            SELECT
                u.id as user_id,
                u.first_name,
                u.last_name,
                u.username,
                u.is_premium,
                COALESCE(um.message_count, 0) as message_count,
                COALESCE(um.avg_message_length, 0) as avg_message_length,
                COALESCE(um.active_days, 0) as active_days,
                COALESCE(urs.reactions_sent, 0) as reactions_sent,
                COALESCE(urr.reactions_received, 0) as reactions_received
            FROM users u
            JOIN user_messages um ON u.id = um.user_id
            LEFT JOIN user_reactions_sent urs ON u.id = urs.user_id
            LEFT JOIN user_reactions_received urr ON u.id = urr.user_id
            WHERE u.is_bot = FALSE
            ORDER BY um.message_count DESC
            LIMIT :limit
        """)

        with self.engine.connect() as conn:
            df = pd.read_sql(query, conn, params={
                "chat_id": chat_id,
                "start_date": start_date,
                "end_date": end_date,
                "limit": limit,
            })

        return df

    def get_reaction_distribution(
        self,
        chat_id: int,
        start_date: datetime,
        end_date: datetime,
        limit: int = 20
    ) -> pd.DataFrame:
        """
        Get distribution of reaction types.
        Returns DataFrame with columns: emoji_value, count
        """
        query = text("""
            SELECT
                COALESCE(emoji_value, custom_emoji_id, 'paid') as emoji_value,
                reaction_type,
                COUNT(*) as count
            FROM message_reactions
            WHERE chat_id = :chat_id
              AND date >= :start_date
              AND date < :end_date
              AND is_removed = FALSE
            GROUP BY emoji_value, custom_emoji_id, reaction_type
            ORDER BY count DESC
            LIMIT :limit
        """)

        with self.engine.connect() as conn:
            df = pd.read_sql(query, conn, params={
                "chat_id": chat_id,
                "start_date": start_date,
                "end_date": end_date,
                "limit": limit,
            })

        return df

    def get_media_distribution(
        self,
        chat_id: int,
        start_date: datetime,
        end_date: datetime
    ) -> pd.DataFrame:
        """
        Get distribution of media types.
        Returns DataFrame with columns: media_type, count
        """
        query = text("""
            SELECT
                mf.media_type::text as media_type,
                COUNT(*) as count
            FROM media_files mf
            JOIN messages m ON m.id = mf.message_id
            WHERE m.chat_id = :chat_id
              AND m.date >= :start_date
              AND m.date < :end_date
            GROUP BY mf.media_type
            ORDER BY count DESC
        """)

        with self.engine.connect() as conn:
            df = pd.read_sql(query, conn, params={
                "chat_id": chat_id,
                "start_date": start_date,
                "end_date": end_date,
            })

        # Also count photos separately (they're in a different table)
        photo_query = text("""
            SELECT COUNT(*) as count
            FROM photos p
            JOIN messages m ON m.id = p.message_id
            WHERE m.chat_id = :chat_id
              AND m.date >= :start_date
              AND m.date < :end_date
        """)

        with self.engine.connect() as conn:
            photo_result = conn.execute(photo_query, {
                "chat_id": chat_id,
                "start_date": start_date,
                "end_date": end_date,
            }).fetchone()

        if photo_result and photo_result.count > 0:
            photo_row = pd.DataFrame([{"media_type": "photo", "count": photo_result.count}])
            df = pd.concat([df, photo_row], ignore_index=True)

        return df

    def get_hourly_activity_pattern(
        self,
        chat_id: int,
        start_date: datetime,
        end_date: datetime
    ) -> pd.DataFrame:
        """
        Get message distribution by hour of day.
        Returns DataFrame with columns: hour, message_count
        """
        query = text("""
            SELECT
                EXTRACT(HOUR FROM date) as hour,
                COUNT(*) as message_count
            FROM messages
            WHERE chat_id = :chat_id
              AND date >= :start_date
              AND date < :end_date
            GROUP BY EXTRACT(HOUR FROM date)
            ORDER BY hour
        """)

        with self.engine.connect() as conn:
            df = pd.read_sql(query, conn, params={
                "chat_id": chat_id,
                "start_date": start_date,
                "end_date": end_date,
            })

        return df

    def get_day_of_week_pattern(
        self,
        chat_id: int,
        start_date: datetime,
        end_date: datetime
    ) -> pd.DataFrame:
        """
        Get message distribution by day of week.
        Returns DataFrame with columns: day_of_week, message_count
        Day 0 = Sunday, 6 = Saturday
        """
        query = text("""
            SELECT
                EXTRACT(DOW FROM date) as day_of_week,
                COUNT(*) as message_count
            FROM messages
            WHERE chat_id = :chat_id
              AND date >= :start_date
              AND date < :end_date
            GROUP BY EXTRACT(DOW FROM date)
            ORDER BY day_of_week
        """)

        with self.engine.connect() as conn:
            df = pd.read_sql(query, conn, params={
                "chat_id": chat_id,
                "start_date": start_date,
                "end_date": end_date,
            })

        return df

    def get_hourly_heatmap_data(
        self,
        chat_id: int,
        start_date: datetime,
        end_date: datetime
    ) -> pd.DataFrame:
        """
        Get message counts by hour and day of week for heatmap.
        Returns DataFrame with columns: day_of_week, hour, message_count
        """
        query = text("""
            SELECT
                EXTRACT(DOW FROM date) as day_of_week,
                EXTRACT(HOUR FROM date) as hour,
                COUNT(*) as message_count
            FROM messages
            WHERE chat_id = :chat_id
              AND date >= :start_date
              AND date < :end_date
            GROUP BY EXTRACT(DOW FROM date), EXTRACT(HOUR FROM date)
            ORDER BY day_of_week, hour
        """)

        with self.engine.connect() as conn:
            df = pd.read_sql(query, conn, params={
                "chat_id": chat_id,
                "start_date": start_date,
                "end_date": end_date,
            })

        return df

    def get_chat_info(self, chat_id: int) -> Optional[Dict[str, Any]]:
        """Get basic chat information."""
        query = text("""
            SELECT
                id,
                type::text as chat_type,
                title,
                username
            FROM chats
            WHERE id = :chat_id
        """)

        with self.engine.connect() as conn:
            result = conn.execute(query, {"chat_id": chat_id}).fetchone()

        if result:
            return {
                "id": result.id,
                "type": result.chat_type,
                "title": result.title,
                "username": result.username,
            }
        return None

    def get_available_chats(self) -> List[Dict[str, Any]]:
        """Get list of available chats with message counts."""
        query = text("""
            SELECT
                c.id,
                c.type::text as chat_type,
                c.title,
                c.username,
                COUNT(m.id) as message_count
            FROM chats c
            LEFT JOIN messages m ON c.id = m.chat_id
            WHERE c.type IN ('group', 'supergroup')
            GROUP BY c.id, c.type, c.title, c.username
            ORDER BY message_count DESC
        """)

        with self.engine.connect() as conn:
            result = conn.execute(query).fetchall()

        return [
            {
                "id": row.id,
                "type": row.chat_type,
                "title": row.title,
                "username": row.username,
                "message_count": row.message_count,
            }
            for row in result
        ]

    def get_chat_card_data(
        self,
        chat_ids: Optional[List[int]] = None
    ) -> List[Dict[str, Any]]:
        """
        Get enriched chat data for welcome page cards.

        Args:
            chat_ids: Optional list of chat IDs to filter by.
                      If None, returns all groups/supergroups.

        Returns:
            List of chat data dictionaries with:
            - id, type, title
            - message_count
            - user_count (distinct users who posted)
            - last_activity (max message date)
            - avg_messages_per_day
        """
        # Build the WHERE clause for chat filtering
        chat_filter = "c.type IN ('group', 'supergroup')"
        params = {}

        if chat_ids:
            chat_filter += " AND c.id = ANY(:chat_ids)"
            params["chat_ids"] = chat_ids

        query = text(f"""
            WITH chat_stats AS (
                SELECT
                    m.chat_id,
                    COUNT(*) as message_count,
                    COUNT(DISTINCT m.user_id) as user_count,
                    MAX(m.date) as last_activity,
                    MIN(m.date) as first_activity
                FROM messages m
                GROUP BY m.chat_id
            )
            SELECT
                c.id,
                c.type::text as chat_type,
                c.title,
                c.username,
                COALESCE(cs.message_count, 0) as message_count,
                COALESCE(cs.user_count, 0) as user_count,
                cs.last_activity,
                cs.first_activity,
                CASE
                    WHEN cs.first_activity IS NOT NULL
                         AND cs.last_activity IS NOT NULL
                         AND cs.last_activity > cs.first_activity
                    THEN ROUND(
                        cs.message_count::numeric /
                        GREATEST(
                            EXTRACT(EPOCH FROM (cs.last_activity - cs.first_activity)) / 86400,
                            1
                        ),
                        1
                    )
                    ELSE cs.message_count
                END as avg_messages_per_day
            FROM chats c
            LEFT JOIN chat_stats cs ON c.id = cs.chat_id
            WHERE {chat_filter}
            ORDER BY cs.message_count DESC NULLS LAST
        """)

        with self.engine.connect() as conn:
            result = conn.execute(query, params).fetchall()

        return [
            {
                "id": row.id,
                "type": row.chat_type,
                "title": row.title,
                "username": row.username,
                "message_count": row.message_count,
                "user_count": row.user_count,
                "last_activity": row.last_activity,
                "avg_messages_per_day": float(row.avg_messages_per_day) if row.avg_messages_per_day else 0.0,
            }
            for row in result
        ]
