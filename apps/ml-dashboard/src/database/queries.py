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

    # =========================================
    # HUMOR METHODS
    # =========================================

    def get_humor_stats(self, chat_id: int) -> dict:
        """
        Get humor statistics for a chat.

        Returns:
            {
                'total_analyzed': int,
                'humorous_count': int,
                'humor_rate': float,
                'avg_score': float
            }
        """
        query = """
            SELECT
                COUNT(*) as total_analyzed,
                SUM(CASE WHEN is_humorous THEN 1 ELSE 0 END) as humorous_count,
                CASE WHEN COUNT(*) > 0
                    THEN ROUND(SUM(CASE WHEN is_humorous THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 2)
                    ELSE 0 END as humor_rate,
                AVG(CASE WHEN is_humorous THEN score ELSE NULL END) as avg_score
            FROM ml_humor
            WHERE chat_id = :chat_id
        """
        result = self._execute_single(query, {"chat_id": chat_id})
        return result or {
            "total_analyzed": 0,
            "humorous_count": 0,
            "humor_rate": 0,
            "avg_score": 0,
        }

    def get_humor_type_distribution(self, chat_id: int) -> pd.DataFrame:
        """
        Get distribution of humor types.

        Returns DataFrame with columns:
            humor_type, count, percentage
        """
        query = """
            SELECT
                COALESCE(humor_type, 'unknown') as humor_type,
                COUNT(*) as count,
                ROUND(COUNT(*) * 100.0 / NULLIF(SUM(COUNT(*)) OVER (), 0), 1) as percentage
            FROM ml_humor
            WHERE chat_id = :chat_id
                AND is_humorous = true
            GROUP BY humor_type
            ORDER BY count DESC
        """
        return self._execute_df(query, {"chat_id": chat_id})

    def get_humor_timeline(self, chat_id: int) -> pd.DataFrame:
        """
        Get daily humor counts.

        Returns DataFrame with columns:
            date, humorous_count, total_count, humor_rate
        """
        query = """
            SELECT
                DATE(m.date) as date,
                SUM(CASE WHEN mh.is_humorous THEN 1 ELSE 0 END) as humorous_count,
                COUNT(*) as total_count,
                ROUND(
                    SUM(CASE WHEN mh.is_humorous THEN 1 ELSE 0 END) * 100.0 /
                    NULLIF(COUNT(*), 0),
                    2
                ) as humor_rate
            FROM ml_humor mh
            JOIN messages m ON m.id = mh.message_id AND m.chat_id = mh.chat_id
            WHERE mh.chat_id = :chat_id
            GROUP BY DATE(m.date)
            ORDER BY date
        """
        return self._execute_df(query, {"chat_id": chat_id})

    def get_top_humorous_messages(self, chat_id: int, limit: int = 15) -> list[dict]:
        """
        Get most humorous messages ranked by score.

        Returns list of:
            {
                'message_id': int,
                'user_id': int,
                'first_name': str,
                'username': str | None,
                'text': str,
                'date': datetime,
                'humor_type': str,
                'score': float
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
                COALESCE(mh.humor_type, 'unknown') as humor_type,
                mh.score
            FROM ml_humor mh
            JOIN messages m ON m.id = mh.message_id AND m.chat_id = mh.chat_id
            JOIN users u ON u.id = m.user_id
            WHERE mh.chat_id = :chat_id
                AND mh.is_humorous = true
                AND COALESCE(m.text, m.caption, '') != ''
            ORDER BY mh.score DESC
            LIMIT :limit
        """
        return self._execute_many(query, {"chat_id": chat_id, "limit": limit})

    def get_user_humor_rankings(self, chat_id: int, limit: int = 10) -> list[dict]:
        """
        Get users ranked by humor rate (funniest first).

        Returns list of:
            {
                'user_id': int,
                'first_name': str,
                'username': str | None,
                'humor_rate': float,
                'humorous_count': int,
                'messages_analyzed': int
            }
        """
        query = """
            SELECT
                u.id as user_id,
                u.first_name,
                u.username,
                ROUND(SUM(CASE WHEN mh.is_humorous THEN 1 ELSE 0 END) * 100.0 /
                      NULLIF(COUNT(*), 0), 2) as humor_rate,
                SUM(CASE WHEN mh.is_humorous THEN 1 ELSE 0 END)::int as humorous_count,
                COUNT(*)::int as messages_analyzed
            FROM ml_humor mh
            JOIN messages m ON m.id = mh.message_id AND m.chat_id = mh.chat_id
            JOIN users u ON u.id = m.user_id
            WHERE mh.chat_id = :chat_id
                AND u.is_bot = false
            GROUP BY u.id, u.first_name, u.username
            HAVING COUNT(*) >= 5
            ORDER BY humor_rate DESC
            LIMIT :limit
        """
        return self._execute_many(query, {"chat_id": chat_id, "limit": limit})

    # =========================================
    # QUESTION METHODS
    # =========================================

    def get_question_stats(self, chat_id: int) -> dict:
        """
        Get question statistics for a chat.

        Returns:
            {
                'total_analyzed': int,
                'question_count': int,
                'question_rate': float,
                'avg_score': float
            }
        """
        query = """
            SELECT
                COUNT(*) as total_analyzed,
                SUM(CASE WHEN is_question THEN 1 ELSE 0 END) as question_count,
                CASE WHEN COUNT(*) > 0
                    THEN ROUND(SUM(CASE WHEN is_question THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 2)
                    ELSE 0 END as question_rate,
                AVG(CASE WHEN is_question THEN score ELSE NULL END) as avg_score
            FROM ml_questions
            WHERE chat_id = :chat_id
        """
        result = self._execute_single(query, {"chat_id": chat_id})
        return result or {
            "total_analyzed": 0,
            "question_count": 0,
            "question_rate": 0,
            "avg_score": 0,
        }

    def get_question_type_distribution(self, chat_id: int) -> pd.DataFrame:
        """
        Get distribution of question types.

        Returns DataFrame with columns:
            question_type, count, percentage
        """
        query = """
            SELECT
                COALESCE(question_type, 'unknown') as question_type,
                COUNT(*) as count,
                ROUND(COUNT(*) * 100.0 / NULLIF(SUM(COUNT(*)) OVER (), 0), 1) as percentage
            FROM ml_questions
            WHERE chat_id = :chat_id
                AND is_question = true
            GROUP BY question_type
            ORDER BY count DESC
        """
        return self._execute_df(query, {"chat_id": chat_id})

    def get_question_timeline(self, chat_id: int) -> pd.DataFrame:
        """
        Get daily question counts.

        Returns DataFrame with columns:
            date, question_count, total_count, question_rate
        """
        query = """
            SELECT
                DATE(m.date) as date,
                SUM(CASE WHEN mq.is_question THEN 1 ELSE 0 END) as question_count,
                COUNT(*) as total_count,
                ROUND(
                    SUM(CASE WHEN mq.is_question THEN 1 ELSE 0 END) * 100.0 /
                    NULLIF(COUNT(*), 0),
                    2
                ) as question_rate
            FROM ml_questions mq
            JOIN messages m ON m.id = mq.message_id AND m.chat_id = mq.chat_id
            WHERE mq.chat_id = :chat_id
            GROUP BY DATE(m.date)
            ORDER BY date
        """
        return self._execute_df(query, {"chat_id": chat_id})

    def get_user_question_rankings(self, chat_id: int, limit: int = 10) -> list[dict]:
        """
        Get users ranked by question rate (most curious first).

        Returns list of:
            {
                'user_id': int,
                'first_name': str,
                'username': str | None,
                'question_rate': float,
                'question_count': int,
                'messages_analyzed': int
            }
        """
        query = """
            SELECT
                u.id as user_id,
                u.first_name,
                u.username,
                ROUND(SUM(CASE WHEN mq.is_question THEN 1 ELSE 0 END) * 100.0 /
                      NULLIF(COUNT(*), 0), 2) as question_rate,
                SUM(CASE WHEN mq.is_question THEN 1 ELSE 0 END)::int as question_count,
                COUNT(*)::int as messages_analyzed
            FROM ml_questions mq
            JOIN messages m ON m.id = mq.message_id AND m.chat_id = mq.chat_id
            JOIN users u ON u.id = m.user_id
            WHERE mq.chat_id = :chat_id
                AND u.is_bot = false
            GROUP BY u.id, u.first_name, u.username
            HAVING COUNT(*) >= 5
            ORDER BY question_rate DESC
            LIMIT :limit
        """
        return self._execute_many(query, {"chat_id": chat_id, "limit": limit})

    # =========================================
    # NAMED ENTITY RECOGNITION METHODS
    # =========================================

    def get_ner_stats(self, chat_id: int) -> dict:
        """
        Get NER statistics for a chat.

        Returns:
            {
                'total_entities': int,
                'unique_entities': int,
                'messages_with_entities': int,
                'avg_confidence': float
            }
        """
        query = """
            SELECT
                COUNT(*) as total_entities,
                COUNT(DISTINCT entity_text) as unique_entities,
                COUNT(DISTINCT message_id) as messages_with_entities,
                AVG(confidence) as avg_confidence
            FROM ml_ner
            WHERE chat_id = :chat_id
        """
        result = self._execute_single(query, {"chat_id": chat_id})
        return result or {
            "total_entities": 0,
            "unique_entities": 0,
            "messages_with_entities": 0,
            "avg_confidence": 0,
        }

    def get_entity_type_distribution(self, chat_id: int) -> pd.DataFrame:
        """
        Get distribution of entity types.

        Returns DataFrame with columns:
            entity_type, count, unique_count, percentage
        """
        query = """
            SELECT
                entity_type,
                COUNT(*) as count,
                COUNT(DISTINCT entity_text) as unique_count,
                ROUND(COUNT(*) * 100.0 / NULLIF(SUM(COUNT(*)) OVER (), 0), 1) as percentage
            FROM ml_ner
            WHERE chat_id = :chat_id
            GROUP BY entity_type
            ORDER BY count DESC
        """
        return self._execute_df(query, {"chat_id": chat_id})

    def get_top_entities(
        self, chat_id: int, entity_type: str | None = None, limit: int = 20
    ) -> list[dict]:
        """
        Get most frequently mentioned entities.

        Args:
            chat_id: Chat to query
            entity_type: Optional filter by type ('PERSON', 'ORG', 'LOC', 'MISC')
            limit: Max entities to return

        Returns list of:
            {
                'entity_text': str,
                'entity_type': str,
                'mention_count': int,
                'avg_confidence': float
            }
        """
        query = """
            SELECT
                entity_text,
                entity_type,
                COUNT(*) as mention_count,
                AVG(confidence) as avg_confidence
            FROM ml_ner
            WHERE chat_id = :chat_id
                AND (:entity_type IS NULL OR entity_type = :entity_type)
            GROUP BY entity_text, entity_type
            ORDER BY mention_count DESC
            LIMIT :limit
        """
        return self._execute_many(
            query, {"chat_id": chat_id, "entity_type": entity_type, "limit": limit}
        )

    def get_entity_timeline(
        self, chat_id: int, entity_type: str | None = None
    ) -> pd.DataFrame:
        """
        Get entity mentions over time.

        Returns DataFrame with columns:
            date, mention_count, unique_entities
        """
        query = """
            SELECT
                DATE(m.date) as date,
                COUNT(*) as mention_count,
                COUNT(DISTINCT ne.entity_text) as unique_entities
            FROM ml_ner ne
            JOIN messages m ON m.id = ne.message_id AND m.chat_id = ne.chat_id
            WHERE ne.chat_id = :chat_id
                AND (:entity_type IS NULL OR ne.entity_type = :entity_type)
            GROUP BY DATE(m.date)
            ORDER BY date
        """
        return self._execute_df(query, {"chat_id": chat_id, "entity_type": entity_type})

    def get_entity_cooccurrence(self, chat_id: int, limit: int = 50) -> pd.DataFrame:
        """
        Get entities that frequently appear together in messages.

        Returns DataFrame with columns:
            entity1, entity2, cooccurrence_count
        """
        query = """
            WITH entity_pairs AS (
                SELECT
                    LEAST(a.entity_text, b.entity_text) as entity1,
                    GREATEST(a.entity_text, b.entity_text) as entity2,
                    a.message_id
                FROM ml_ner a
                JOIN ml_ner b ON a.message_id = b.message_id
                    AND a.chat_id = b.chat_id
                    AND a.entity_text < b.entity_text
                WHERE a.chat_id = :chat_id
            )
            SELECT
                entity1,
                entity2,
                COUNT(DISTINCT message_id) as cooccurrence_count
            FROM entity_pairs
            GROUP BY entity1, entity2
            HAVING COUNT(DISTINCT message_id) >= 2
            ORDER BY cooccurrence_count DESC
            LIMIT :limit
        """
        return self._execute_df(query, {"chat_id": chat_id, "limit": limit})

    def get_user_entity_mentions(self, chat_id: int, limit: int = 10) -> list[dict]:
        """
        Get users ranked by entity mentions.

        Returns list of:
            {
                'user_id': int,
                'first_name': str,
                'username': str | None,
                'entity_mentions': int,
                'unique_entities': int
            }
        """
        query = """
            SELECT
                u.id as user_id,
                u.first_name,
                u.username,
                COUNT(*)::int as entity_mentions,
                COUNT(DISTINCT ne.entity_text)::int as unique_entities
            FROM ml_ner ne
            JOIN messages m ON m.id = ne.message_id AND m.chat_id = ne.chat_id
            JOIN users u ON u.id = m.user_id
            WHERE ne.chat_id = :chat_id
                AND u.is_bot = false
            GROUP BY u.id, u.first_name, u.username
            ORDER BY entity_mentions DESC
            LIMIT :limit
        """
        return self._execute_many(query, {"chat_id": chat_id, "limit": limit})

    # =========================================
    # USER CARDS METHODS
    # =========================================

    def get_user_cards_weeks(self, chat_id: int) -> list[dict]:
        """
        Get available weeks for user cards.

        Returns list of:
            {'week_start': date, 'week_end': date, 'user_count': int}
        """
        query = """
            SELECT
                week_start,
                week_end,
                COUNT(DISTINCT user_id) as user_count
            FROM ml_user_cards
            WHERE chat_id = :chat_id
            GROUP BY week_start, week_end
            ORDER BY week_start DESC
        """
        return self._execute_many(query, {"chat_id": chat_id})

    def get_user_card(
        self, user_id: int, chat_id: int, week_start
    ) -> dict | None:
        """
        Get a specific user card.

        Returns:
            {
                'user_id': int,
                'chat_id': int,
                'week_start': date,
                'week_end': date,
                'stats': dict,
                'trends': dict | None,
                'daily_breakdown': dict | None,
                'messages_analyzed': int
            }
        """
        query = """
            SELECT
                user_id,
                chat_id,
                week_start,
                week_end,
                stats,
                trends,
                daily_breakdown,
                messages_analyzed
            FROM ml_user_cards
            WHERE user_id = :user_id
                AND chat_id = :chat_id
                AND week_start = :week_start
        """
        return self._execute_single(
            query, {"user_id": user_id, "chat_id": chat_id, "week_start": week_start}
        )

    def get_weekly_user_cards(self, chat_id: int, week_start) -> list[dict]:
        """
        Get all user cards for a specific week.

        Returns list of:
            {
                'user_id': int,
                'first_name': str,
                'username': str | None,
                'stats': dict,
                'messages_analyzed': int
            }
        """
        query = """
            SELECT
                uc.user_id,
                u.first_name,
                u.username,
                uc.stats,
                uc.messages_analyzed
            FROM ml_user_cards uc
            JOIN users u ON u.id = uc.user_id
            WHERE uc.chat_id = :chat_id
                AND uc.week_start = :week_start
                AND u.is_bot = false
            ORDER BY uc.messages_analyzed DESC
        """
        return self._execute_many(query, {"chat_id": chat_id, "week_start": week_start})

    def get_user_card_history(
        self, user_id: int, chat_id: int, limit: int = 12
    ) -> list[dict]:
        """
        Get a user's card history over multiple weeks.

        Returns list of:
            {
                'week_start': date,
                'week_end': date,
                'stats': dict,
                'messages_analyzed': int
            }
        """
        query = """
            SELECT
                week_start,
                week_end,
                stats,
                messages_analyzed
            FROM ml_user_cards
            WHERE user_id = :user_id
                AND chat_id = :chat_id
            ORDER BY week_start DESC
            LIMIT :limit
        """
        return self._execute_many(
            query, {"user_id": user_id, "chat_id": chat_id, "limit": limit}
        )

    def get_weekly_leaderboard(
        self, chat_id: int, week_start, stat_key: str
    ) -> list[dict]:
        """
        Get leaderboard for a specific stat in a week.

        Args:
            chat_id: Chat to query
            week_start: Week to query
            stat_key: Key in stats JSONB (mood, volatility, toxicity, activity, etc.)

        Returns list of:
            {
                'user_id': int,
                'first_name': str,
                'username': str | None,
                'value': float,
                'messages_analyzed': int
            }
        """
        query = """
            SELECT
                uc.user_id,
                u.first_name,
                u.username,
                (uc.stats->:stat_key)::real as value,
                uc.messages_analyzed
            FROM ml_user_cards uc
            JOIN users u ON u.id = uc.user_id
            WHERE uc.chat_id = :chat_id
                AND uc.week_start = :week_start
                AND u.is_bot = false
                AND uc.stats ? :stat_key
            ORDER BY value DESC
            LIMIT 10
        """
        return self._execute_many(
            query, {"chat_id": chat_id, "week_start": week_start, "stat_key": stat_key}
        )
