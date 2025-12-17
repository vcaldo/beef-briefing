"""
Database query layer for the ML Dashboard.

Design Principles:
1. Raw SQL with SQLAlchemy text() - no ORM
2. Parameterized queries with named parameters (:param)
3. Connection pooling via SQLAlchemy engine
4. Pandas integration via pd.read_sql() for chart data
5. Return patterns:
   - Single row -> dict | None
   - Multiple rows -> list[dict]
   - Chart data -> pd.DataFrame
"""

import pandas as pd
from sqlalchemy import text
from sqlalchemy.engine import Engine


class MLDashboardQueries:
    """
    Query executor for the ML analytics dashboard.

    Provides queries for sentiment, toxicity, and user behavior analysis.
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
    # CHAT METHODS
    # =========================================

    def get_available_chats(self) -> list[dict]:
        """
        Get all chats that have ML-processed data.

        Returns list of:
            {
                'id': int,
                'title': str,
                'message_count': int,
                'analyzed_count': int
            }
        """
        query = """
            SELECT
                c.id,
                COALESCE(c.title, 'Unknown') as title,
                (SELECT COUNT(*) FROM messages WHERE chat_id = c.id) as message_count,
                (SELECT COUNT(*) FROM ml_sentiment WHERE chat_id = c.id) as analyzed_count
            FROM chats c
            WHERE c.type IN ('group', 'supergroup')
                AND EXISTS (SELECT 1 FROM ml_sentiment WHERE chat_id = c.id)
            ORDER BY analyzed_count DESC
        """
        return self._execute_many(query, {})

    def get_chat_info(self, chat_id: int) -> dict | None:
        """Get basic info about a chat."""
        query = """
            SELECT
                c.id,
                COALESCE(c.title, 'Unknown') as title,
                c.type::text as type
            FROM chats c
            WHERE c.id = :chat_id
        """
        return self._execute_single(query, {"chat_id": chat_id})

    # =========================================
    # USER BEHAVIOR QUADRANT
    # =========================================

    def get_user_behavior_quadrant(
        self,
        chat_id: int,
        start_date: "date | None" = None,
        end_date: "date | None" = None,
    ) -> pd.DataFrame:
        """
        Get user data for behavior quadrant scatter plot.

        Args:
            chat_id: Chat to query
            start_date: Optional start date filter (inclusive)
            end_date: Optional end date filter (inclusive)

        Returns DataFrame with columns:
            user_id, first_name, username, avg_sentiment, toxicity_rate,
            messages_analyzed
        """
        # When date filtering is needed, calculate aggregates dynamically
        if start_date is not None or end_date is not None:
            query = """
                SELECT
                    u.id as user_id,
                    u.first_name,
                    u.username,
                    AVG(CASE
                        WHEN ms.label = 'positive' THEN 1.0
                        WHEN ms.label = 'neutral' THEN 0.0
                        WHEN ms.label = 'negative' THEN -1.0
                    END) as avg_sentiment,
                    COALESCE(
                        SUM(CASE WHEN mt.is_toxic THEN 1 ELSE 0 END)::real
                        / NULLIF(COUNT(mt.message_id), 0),
                        0
                    ) as toxicity_rate,
                    COUNT(ms.message_id) as messages_analyzed
                FROM ml_sentiment ms
                JOIN messages m ON m.id = ms.message_id AND m.chat_id = ms.chat_id
                JOIN users u ON u.id = m.user_id
                LEFT JOIN ml_toxicity mt
                    ON mt.message_id = ms.message_id AND mt.chat_id = ms.chat_id
                WHERE ms.chat_id = :chat_id
                    AND u.is_bot = false
                    AND (:start_date::date IS NULL OR m.date >= :start_date::date)
                    AND (:end_date::date IS NULL OR m.date < :end_date::date + interval '1 day')
                GROUP BY u.id, u.first_name, u.username
                HAVING COUNT(ms.message_id) >= 5
                ORDER BY COUNT(ms.message_id) DESC
            """
            return self._execute_df(
                query,
                {"chat_id": chat_id, "start_date": start_date, "end_date": end_date},
            )

        # No date filter: use pre-computed ml_user_profiles for performance
        query = """
            SELECT
                u.id as user_id,
                u.first_name,
                u.username,
                mup.avg_sentiment,
                mup.toxicity_rate,
                mup.messages_analyzed
            FROM ml_user_profiles mup
            JOIN users u ON u.id = mup.user_id
            WHERE mup.chat_id = :chat_id
                AND mup.messages_analyzed >= 5
                AND u.is_bot = false
            ORDER BY mup.messages_analyzed DESC
        """
        return self._execute_df(query, {"chat_id": chat_id})

    def get_user_details(self, user_id: int, chat_id: int) -> dict | None:
        """
        Get detailed user profile for a specific user.

        Returns:
            {
                'user_id': int,
                'first_name': str,
                'username': str | None,
                'avg_sentiment': float,
                'sentiment_variance': float,
                'toxicity_rate': float,
                'messages_analyzed': int,
                'total_messages': int
            }
        """
        query = """
            SELECT
                u.id as user_id,
                u.first_name,
                u.username,
                mup.avg_sentiment,
                mup.sentiment_variance,
                mup.toxicity_rate,
                mup.messages_analyzed,
                (SELECT COUNT(*) FROM messages m
                 WHERE m.user_id = u.id AND m.chat_id = :chat_id) as total_messages
            FROM ml_user_profiles mup
            JOIN users u ON u.id = mup.user_id
            WHERE mup.user_id = :user_id
                AND mup.chat_id = :chat_id
        """
        return self._execute_single(
            query, {"user_id": user_id, "chat_id": chat_id}
        )

    # =========================================
    # SENTIMENT METHODS
    # =========================================

    def get_sentiment_stats(self, chat_id: int) -> dict:
        """
        Get sentiment overview statistics for a chat.

        Returns:
            {
                'total_analyzed': int,
                'positive_count': int,
                'neutral_count': int,
                'negative_count': int,
                'avg_confidence': float
            }
        """
        query = """
            SELECT
                COUNT(*) as total_analyzed,
                SUM(CASE WHEN label = 'positive' THEN 1 ELSE 0 END) as positive_count,
                SUM(CASE WHEN label = 'neutral' THEN 1 ELSE 0 END) as neutral_count,
                SUM(CASE WHEN label = 'negative' THEN 1 ELSE 0 END) as negative_count,
                AVG(confidence) as avg_confidence
            FROM ml_sentiment
            WHERE chat_id = :chat_id
        """
        result = self._execute_single(query, {"chat_id": chat_id})
        return result or {
            "total_analyzed": 0,
            "positive_count": 0,
            "neutral_count": 0,
            "negative_count": 0,
            "avg_confidence": 0,
        }

    def get_sentiment_distribution(self, chat_id: int) -> pd.DataFrame:
        """
        Get sentiment label distribution.

        Returns DataFrame with columns:
            label, count, percentage
        """
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

    # =========================================
    # TOXICITY METHODS
    # =========================================

    def get_toxicity_stats(self, chat_id: int) -> dict:
        """
        Get toxicity statistics for a chat.

        Returns:
            {
                'total_analyzed': int,
                'toxic_count': int,
                'toxic_rate': float
            }
        """
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
        return result or {
            "total_analyzed": 0,
            "toxic_count": 0,
            "toxic_rate": 0,
        }

    def get_toxicity_timeline(self, chat_id: int) -> pd.DataFrame:
        """
        Get daily toxicity counts.

        Returns DataFrame with columns:
            date, toxic_count, total_count, toxic_rate
        """
        query = """
            SELECT
                DATE(m.date) as date,
                SUM(CASE WHEN mt.is_toxic THEN 1 ELSE 0 END) as toxic_count,
                COUNT(*) as total_count,
                ROUND(
                    SUM(CASE WHEN mt.is_toxic THEN 1 ELSE 0 END) * 100.0 /
                    NULLIF(COUNT(*), 0),
                    2
                ) as toxic_rate
            FROM ml_toxicity mt
            JOIN messages m ON m.id = mt.message_id
            WHERE mt.chat_id = :chat_id
            GROUP BY DATE(m.date)
            ORDER BY date
        """
        return self._execute_df(query, {"chat_id": chat_id})

    def get_top_toxic_messages(self, chat_id: int, limit: int = 10) -> list[dict]:
        """
        Get most toxic messages in a chat, ranked by toxicity score.

        Returns list of:
            {
                'message_id': int,
                'user_id': int,
                'first_name': str,
                'username': str | None,
                'text': str,
                'date': datetime,
                'toxicity_score': float
            }
        """
        query = """
            SELECT
                m.id as message_id,
                m.user_id,
                u.first_name,
                u.username,
                COALESCE(m.text, m.caption, '') as text,
                m.date,
                mt.score as toxicity_score
            FROM ml_toxicity mt
            JOIN messages m ON m.id = mt.message_id AND m.chat_id = mt.chat_id
            JOIN users u ON u.id = m.user_id
            WHERE mt.chat_id = :chat_id
                AND mt.is_toxic = true
                AND COALESCE(m.text, m.caption, '') != ''
            ORDER BY mt.score DESC
            LIMIT :limit
        """
        return self._execute_many(query, {"chat_id": chat_id, "limit": limit})

    def get_user_toxicity_rankings(self, chat_id: int, limit: int = 10) -> list[dict]:
        """
        Get users ranked by toxicity rate (highest first).

        Returns list of:
            {
                'user_id': int,
                'first_name': str,
                'username': str | None,
                'toxicity_rate': float,
                'messages_analyzed': int
            }
        """
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
                AND u.is_bot = false
            ORDER BY mup.toxicity_rate DESC
            LIMIT :limit
        """
        return self._execute_many(query, {"chat_id": chat_id, "limit": limit})

    # =========================================
    # USER DRILLDOWN METHODS
    # =========================================

    def get_user_sentiment_timeline(
        self, user_id: int, chat_id: int
    ) -> pd.DataFrame:
        """
        Get daily sentiment for a specific user.

        Returns DataFrame with columns:
            date, positive, neutral, negative, avg_sentiment
        """
        query = """
            SELECT
                DATE(m.date) as date,
                SUM(CASE WHEN ms.label = 'positive' THEN 1 ELSE 0 END) as positive,
                SUM(CASE WHEN ms.label = 'neutral' THEN 1 ELSE 0 END) as neutral,
                SUM(CASE WHEN ms.label = 'negative' THEN 1 ELSE 0 END) as negative,
                AVG(CASE
                    WHEN ms.label = 'positive' THEN 1
                    WHEN ms.label = 'neutral' THEN 0
                    WHEN ms.label = 'negative' THEN -1
                END) as avg_sentiment
            FROM ml_sentiment ms
            JOIN messages m ON m.id = ms.message_id AND m.chat_id = ms.chat_id
            WHERE ms.chat_id = :chat_id
                AND m.user_id = :user_id
            GROUP BY DATE(m.date)
            ORDER BY date
        """
        return self._execute_df(
            query, {"user_id": user_id, "chat_id": chat_id}
        )

    def get_user_vs_group_comparison(
        self, user_id: int, chat_id: int
    ) -> dict:
        """
        Compare user's sentiment/toxicity to group average.

        Returns:
            {
                'user_avg_sentiment': float,
                'group_avg_sentiment': float,
                'user_toxicity_rate': float,
                'group_toxicity_rate': float,
                'sentiment_percentile': float,
                'toxicity_percentile': float
            }
        """
        query = """
            WITH user_stats AS (
                SELECT
                    avg_sentiment as user_avg,
                    toxicity_rate as user_toxic
                FROM ml_user_profiles
                WHERE user_id = :user_id AND chat_id = :chat_id
            ),
            group_stats AS (
                SELECT
                    AVG(avg_sentiment) as group_avg,
                    AVG(toxicity_rate) as group_toxic
                FROM ml_user_profiles
                WHERE chat_id = :chat_id
            ),
            sentiment_rank AS (
                SELECT
                    COUNT(*) FILTER (WHERE avg_sentiment <= (
                        SELECT avg_sentiment FROM ml_user_profiles
                        WHERE user_id = :user_id AND chat_id = :chat_id
                    )) * 100.0 / NULLIF(COUNT(*), 0) as percentile
                FROM ml_user_profiles
                WHERE chat_id = :chat_id AND messages_analyzed >= 5
            ),
            toxicity_rank AS (
                SELECT
                    COUNT(*) FILTER (WHERE toxicity_rate >= (
                        SELECT toxicity_rate FROM ml_user_profiles
                        WHERE user_id = :user_id AND chat_id = :chat_id
                    )) * 100.0 / NULLIF(COUNT(*), 0) as percentile
                FROM ml_user_profiles
                WHERE chat_id = :chat_id AND messages_analyzed >= 5
            )
            SELECT
                us.user_avg as user_avg_sentiment,
                gs.group_avg as group_avg_sentiment,
                us.user_toxic as user_toxicity_rate,
                gs.group_toxic as group_toxicity_rate,
                sr.percentile as sentiment_percentile,
                tr.percentile as toxicity_percentile
            FROM user_stats us
            CROSS JOIN group_stats gs
            CROSS JOIN sentiment_rank sr
            CROSS JOIN toxicity_rank tr
        """
        result = self._execute_single(
            query, {"user_id": user_id, "chat_id": chat_id}
        )
        return result or {
            "user_avg_sentiment": 0,
            "group_avg_sentiment": 0,
            "user_toxicity_rate": 0,
            "group_toxicity_rate": 0,
            "sentiment_percentile": 0,
            "toxicity_percentile": 0,
        }

    def get_user_sample_messages(
        self, user_id: int, chat_id: int, limit: int = 20
    ) -> list[dict]:
        """
        Get sample messages from a user with sentiment and toxicity labels.

        Returns list of:
            {
                'message_id': int,
                'text': str,
                'date': datetime,
                'sentiment_label': str,
                'score_negative': float,
                'is_toxic': bool
            }
        """
        query = """
            SELECT
                m.id as message_id,
                COALESCE(m.text, m.caption, '') as text,
                m.date,
                ms.label as sentiment_label,
                ms.score_negative,
                COALESCE(mt.is_toxic, false) as is_toxic
            FROM messages m
            JOIN ml_sentiment ms ON ms.message_id = m.id AND ms.chat_id = m.chat_id
            LEFT JOIN ml_toxicity mt ON mt.message_id = m.id AND mt.chat_id = m.chat_id
            WHERE m.user_id = :user_id
                AND m.chat_id = :chat_id
                AND COALESCE(m.text, m.caption, '') != ''
            ORDER BY m.date DESC
            LIMIT :limit
        """
        return self._execute_many(
            query, {"user_id": user_id, "chat_id": chat_id, "limit": limit}
        )

    # =========================================
    # EMBEDDING EXPLORER METHODS
    # =========================================

    def get_messages_with_sentiment(
        self, chat_id: int, limit: int = 10000
    ) -> pd.DataFrame:
        """
        Get messages with their sentiment labels for embedding visualization.

        Returns DataFrame with columns:
            message_id, user_id, first_name, text_preview, sentiment_label, date
        """
        query = """
            SELECT
                m.id as message_id,
                m.user_id,
                u.first_name,
                LEFT(COALESCE(m.text, m.caption, ''), 100) as text_preview,
                ms.label as sentiment_label,
                m.date
            FROM ml_sentiment ms
            JOIN messages m ON m.id = ms.message_id AND m.chat_id = ms.chat_id
            JOIN users u ON u.id = m.user_id
            WHERE ms.chat_id = :chat_id
            ORDER BY m.date DESC
            LIMIT :limit
        """
        return self._execute_df(query, {"chat_id": chat_id, "limit": limit})

    def get_group_users(self, chat_id: int) -> list[dict]:
        """
        Get all users in a group that have ML-analyzed messages.

        Returns list of:
            {'user_id': int, 'first_name': str, 'username': str | None}
        """
        query = """
            SELECT DISTINCT
                u.id as user_id,
                u.first_name,
                u.username
            FROM ml_user_profiles mup
            JOIN users u ON u.id = mup.user_id
            WHERE mup.chat_id = :chat_id
                AND u.is_bot = false
            ORDER BY u.first_name
        """
        return self._execute_many(query, {"chat_id": chat_id})
