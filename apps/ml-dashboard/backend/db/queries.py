"""
Database query layer for ML Dashboard.

Design Principles:
1. Raw SQL with SQLAlchemy text() - no ORM
2. Parameterized queries with named parameters (:param)
3. Connection pooling via SQLAlchemy engine
"""

import logging
from typing import Any, Optional

from sqlalchemy import text
from sqlalchemy.engine import Engine

logger = logging.getLogger(__name__)


class DashboardQueries:
    """Database operations for ML Dashboard."""

    def __init__(self, engine: Engine):
        self._engine = engine

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

    # =========================================
    # STATS
    # =========================================

    def get_processing_stats(self, chat_id: int) -> dict:
        """Get ML processing statistics for a chat."""
        query = """
        SELECT
            (SELECT COUNT(*) FROM messages WHERE chat_id = :chat_id AND (text IS NOT NULL OR caption IS NOT NULL)) as total_with_text,
            (SELECT COUNT(DISTINCT message_id) FROM ml_processing_state WHERE chat_id = :chat_id) as processed,
            (SELECT COUNT(*) FROM ml_sentiment WHERE message_id IN (SELECT id FROM messages WHERE chat_id = :chat_id)) as sentiment_count,
            (SELECT COUNT(*) FROM ml_toxicity WHERE message_id IN (SELECT id FROM messages WHERE chat_id = :chat_id)) as toxicity_count,
            (SELECT COUNT(*) FROM ml_toxicity WHERE message_id IN (SELECT id FROM messages WHERE chat_id = :chat_id) AND is_toxic = true) as toxic_count,
            (SELECT COUNT(*) FROM ml_humor WHERE message_id IN (SELECT id FROM messages WHERE chat_id = :chat_id)) as humor_count,
            (SELECT COUNT(*) FROM ml_humor WHERE message_id IN (SELECT id FROM messages WHERE chat_id = :chat_id) AND is_humorous = true) as humorous_count,
            (SELECT COUNT(*) FROM ml_questions WHERE message_id IN (SELECT id FROM messages WHERE chat_id = :chat_id)) as questions_count,
            (SELECT COUNT(*) FROM ml_questions WHERE message_id IN (SELECT id FROM messages WHERE chat_id = :chat_id) AND is_question = true) as question_count,
            (SELECT COUNT(*) FROM ml_ner WHERE message_id IN (SELECT id FROM messages WHERE chat_id = :chat_id)) as entity_count,
            (SELECT COUNT(DISTINCT topic_id) FROM ml_message_topics WHERE message_id IN (SELECT id FROM messages WHERE chat_id = :chat_id) AND topic_id >= 0) as topic_count
        """
        return self._execute_single(query, {"chat_id": chat_id}) or {}

    def get_chats(self) -> list[dict]:
        """Get all chats with message counts."""
        query = """
        SELECT c.id, c.title, c.type, c.username,
               COUNT(m.id) as message_count,
               (SELECT COUNT(DISTINCT ps.message_id) FROM ml_processing_state ps
                JOIN messages m2 ON m2.id = ps.message_id WHERE m2.chat_id = c.id) as processed_count
        FROM chats c
        LEFT JOIN messages m ON m.chat_id = c.id
        GROUP BY c.id
        ORDER BY message_count DESC
        """
        return self._execute_many(query, {})

    # =========================================
    # MESSAGES
    # =========================================

    def get_messages(
        self,
        chat_id: int,
        limit: int = 50,
        offset: int = 0,
        user_id: Optional[int] = None,
        sentiment: Optional[str] = None,
        is_toxic: Optional[bool] = None,
        is_humorous: Optional[bool] = None,
        is_question: Optional[bool] = None,
        topic_id: Optional[int] = None,
        sort_by: str = "date",
        sort_order: str = "desc",
    ) -> tuple[list[dict], int]:
        """
        Get messages with ML results.

        Returns:
            Tuple of (messages, total_count)
        """
        # Build WHERE clause dynamically
        conditions = ["m.chat_id = :chat_id", "(m.text IS NOT NULL OR m.caption IS NOT NULL)"]
        params: dict[str, Any] = {"chat_id": chat_id, "limit": limit, "offset": offset}

        if user_id is not None:
            conditions.append("m.user_id = :user_id")
            params["user_id"] = user_id

        if sentiment is not None:
            conditions.append("s.label = :sentiment")
            params["sentiment"] = sentiment

        if is_toxic is not None:
            conditions.append("t.is_toxic = :is_toxic")
            params["is_toxic"] = is_toxic

        if is_humorous is not None:
            conditions.append("h.is_humorous = :is_humorous")
            params["is_humorous"] = is_humorous

        if is_question is not None:
            conditions.append("q.is_question = :is_question")
            params["is_question"] = is_question

        if topic_id is not None:
            conditions.append("mt.topic_id = :topic_id")
            params["topic_id"] = topic_id

        where_clause = " AND ".join(conditions)

        # Map sort_by to actual column
        sort_columns = {
            "date": "m.date",
            "toxicity_score": "t.score",
            "sentiment_score": "s.confidence",
        }
        order_col = sort_columns.get(sort_by, "m.date")
        order_dir = "DESC" if sort_order.lower() == "desc" else "ASC"

        # Count query
        count_query = f"""
        SELECT COUNT(DISTINCT m.id)
        FROM messages m
        LEFT JOIN ml_sentiment s ON s.message_id = m.id
        LEFT JOIN ml_toxicity t ON t.message_id = m.id
        LEFT JOIN ml_humor h ON h.message_id = m.id
        LEFT JOIN ml_questions q ON q.message_id = m.id
        LEFT JOIN ml_message_topics mt ON mt.message_id = m.id
        WHERE {where_clause}
        """
        count_result = self._execute_single(count_query, params)
        total = count_result["count"] if count_result else 0

        # Main query
        query = f"""
        SELECT
            m.id, m.message_id, m.chat_id, m.user_id,
            COALESCE(m.text, m.caption) as text,
            m.date,
            u.first_name, u.last_name, u.username,
            s.label as sentiment_label,
            s.score_positive, s.score_neutral, s.score_negative,
            s.confidence as sentiment_confidence,
            t.is_toxic, t.label as toxicity_label, t.score as toxicity_score,
            h.is_humorous, h.humor_type, h.score as humor_score,
            q.is_question, q.question_type, q.score as question_score,
            mt.topic_id, mt.similarity as topic_similarity
        FROM messages m
        LEFT JOIN users u ON u.id = m.user_id
        LEFT JOIN ml_sentiment s ON s.message_id = m.id
        LEFT JOIN ml_toxicity t ON t.message_id = m.id
        LEFT JOIN ml_humor h ON h.message_id = m.id
        LEFT JOIN ml_questions q ON q.message_id = m.id
        LEFT JOIN ml_message_topics mt ON mt.message_id = m.id
        WHERE {where_clause}
        ORDER BY {order_col} {order_dir} NULLS LAST
        LIMIT :limit OFFSET :offset
        """
        messages = self._execute_many(query, params)
        return messages, total

    def get_message_detail(self, message_id: int) -> dict | None:
        """Get a single message with all ML details."""
        query = """
        SELECT
            m.id, m.message_id, m.chat_id, m.user_id,
            COALESCE(m.text, m.caption) as text,
            m.date,
            u.first_name, u.last_name, u.username,
            s.label as sentiment_label,
            s.score_positive, s.score_neutral, s.score_negative,
            s.confidence as sentiment_confidence,
            t.is_toxic, t.label as toxicity_label, t.score as toxicity_score,
            h.is_humorous, h.humor_type, h.score as humor_score,
            q.is_question, q.question_type, q.score as question_score,
            mt.topic_id, mt.similarity as topic_similarity
        FROM messages m
        LEFT JOIN users u ON u.id = m.user_id
        LEFT JOIN ml_sentiment s ON s.message_id = m.id
        LEFT JOIN ml_toxicity t ON t.message_id = m.id
        LEFT JOIN ml_humor h ON h.message_id = m.id
        LEFT JOIN ml_questions q ON q.message_id = m.id
        LEFT JOIN ml_message_topics mt ON mt.message_id = m.id
        WHERE m.id = :message_id
        """
        return self._execute_single(query, {"message_id": message_id})

    def get_message_entities(self, message_id: int) -> list[dict]:
        """Get named entities for a message."""
        query = """
        SELECT entity_type, entity_text, start_pos, end_pos, confidence
        FROM ml_ner
        WHERE message_id = :message_id
        ORDER BY start_pos NULLS LAST
        """
        return self._execute_many(query, {"message_id": message_id})

    # =========================================
    # TOPICS
    # =========================================

    def get_topics(self, chat_id: int) -> list[dict]:
        """Get all topics for a chat with message counts."""
        query = """
        SELECT
            t.topic_id, t.keywords, t.message_count,
            (SELECT COUNT(*) FROM ml_message_topics mt
             JOIN messages m ON m.id = mt.message_id
             WHERE m.chat_id = :chat_id AND mt.topic_id = t.topic_id) as actual_count
        FROM ml_topics t
        WHERE t.chat_id = :chat_id AND t.topic_id >= 0
        ORDER BY t.message_count DESC
        """
        return self._execute_many(query, {"chat_id": chat_id})

    def get_topic_messages(
        self, chat_id: int, topic_id: int, limit: int = 50, offset: int = 0
    ) -> tuple[list[dict], int]:
        """Get messages for a specific topic."""
        count_query = """
        SELECT COUNT(*)
        FROM ml_message_topics mt
        JOIN messages m ON m.id = mt.message_id
        WHERE m.chat_id = :chat_id AND mt.topic_id = :topic_id
        """
        count_result = self._execute_single(
            count_query, {"chat_id": chat_id, "topic_id": topic_id}
        )
        total = count_result["count"] if count_result else 0

        query = """
        SELECT
            m.id, m.message_id, m.chat_id, m.user_id,
            COALESCE(m.text, m.caption) as text,
            m.date,
            u.first_name, u.last_name, u.username,
            mt.similarity
        FROM messages m
        JOIN ml_message_topics mt ON mt.message_id = m.id
        LEFT JOIN users u ON u.id = m.user_id
        WHERE m.chat_id = :chat_id AND mt.topic_id = :topic_id
        ORDER BY mt.similarity DESC
        LIMIT :limit OFFSET :offset
        """
        messages = self._execute_many(
            query, {"chat_id": chat_id, "topic_id": topic_id, "limit": limit, "offset": offset}
        )
        return messages, total

    def get_outlier_count(self, chat_id: int) -> int:
        """Get count of unclustered messages (topic_id = -1)."""
        query = """
        SELECT COUNT(*)
        FROM ml_message_topics mt
        JOIN messages m ON m.id = mt.message_id
        WHERE m.chat_id = :chat_id AND mt.topic_id = -1
        """
        result = self._execute_single(query, {"chat_id": chat_id})
        return result["count"] if result else 0

    # =========================================
    # USERS
    # =========================================

    def get_users_with_ml_stats(
        self, chat_id: int, limit: int = 50, offset: int = 0
    ) -> tuple[list[dict], int]:
        """Get users with aggregated ML statistics."""
        count_query = """
        SELECT COUNT(DISTINCT m.user_id)
        FROM messages m
        WHERE m.chat_id = :chat_id AND m.user_id IS NOT NULL
        """
        count_result = self._execute_single(count_query, {"chat_id": chat_id})
        total = count_result["count"] if count_result else 0

        query = """
        SELECT
            u.id as user_id, u.first_name, u.last_name, u.username,
            COUNT(m.id) as message_count,
            AVG(CASE s.label
                WHEN 'positive' THEN s.confidence
                WHEN 'negative' THEN -s.confidence
                ELSE 0
            END) as avg_sentiment,
            AVG(CASE WHEN t.is_toxic THEN 1.0 ELSE 0.0 END) as toxicity_rate,
            AVG(CASE WHEN h.is_humorous THEN 1.0 ELSE 0.0 END) as humor_rate,
            AVG(CASE WHEN q.is_question THEN 1.0 ELSE 0.0 END) as question_rate
        FROM users u
        JOIN messages m ON m.user_id = u.id
        LEFT JOIN ml_sentiment s ON s.message_id = m.id
        LEFT JOIN ml_toxicity t ON t.message_id = m.id
        LEFT JOIN ml_humor h ON h.message_id = m.id
        LEFT JOIN ml_questions q ON q.message_id = m.id
        WHERE m.chat_id = :chat_id
        GROUP BY u.id
        ORDER BY message_count DESC
        LIMIT :limit OFFSET :offset
        """
        users = self._execute_many(query, {"chat_id": chat_id, "limit": limit, "offset": offset})
        return users, total

    def get_user_sentiment_distribution(self, chat_id: int, user_id: int) -> dict:
        """Get sentiment distribution for a user."""
        query = """
        SELECT
            COUNT(*) as total,
            SUM(CASE WHEN s.label = 'positive' THEN 1 ELSE 0 END) as positive,
            SUM(CASE WHEN s.label = 'neutral' THEN 1 ELSE 0 END) as neutral,
            SUM(CASE WHEN s.label = 'negative' THEN 1 ELSE 0 END) as negative
        FROM messages m
        JOIN ml_sentiment s ON s.message_id = m.id
        WHERE m.chat_id = :chat_id AND m.user_id = :user_id
        """
        return self._execute_single(query, {"chat_id": chat_id, "user_id": user_id}) or {}

    def get_user_entity_mentions(
        self, chat_id: int, user_id: int, limit: int = 20
    ) -> list[dict]:
        """Get most frequent entity mentions by a user."""
        query = """
        SELECT n.entity_type, n.entity_text, COUNT(*) as count
        FROM ml_ner n
        JOIN messages m ON m.id = n.message_id
        WHERE m.chat_id = :chat_id AND m.user_id = :user_id
        GROUP BY n.entity_type, n.entity_text
        ORDER BY count DESC
        LIMIT :limit
        """
        return self._execute_many(query, {"chat_id": chat_id, "user_id": user_id, "limit": limit})

    def get_user_cards(self, chat_id: int, user_id: int) -> list[dict]:
        """Get user cards history."""
        query = """
        SELECT id, user_id, chat_id, week_start, week_end, stats, trends, image_url, created_at
        FROM ml_user_cards
        WHERE chat_id = :chat_id AND user_id = :user_id
        ORDER BY week_end DESC
        """
        return self._execute_many(query, {"chat_id": chat_id, "user_id": user_id})

    # =========================================
    # SEARCH HELPERS
    # =========================================

    def get_messages_by_ids(self, message_ids: list[int]) -> list[dict]:
        """Get messages by their database IDs."""
        if not message_ids:
            return []

        query = """
        SELECT
            m.id, m.message_id, m.chat_id, m.user_id,
            COALESCE(m.text, m.caption) as text,
            m.date,
            u.first_name, u.last_name, u.username,
            s.label as sentiment_label, s.confidence as sentiment_confidence,
            t.is_toxic, t.score as toxicity_score,
            h.is_humorous, h.score as humor_score
        FROM messages m
        LEFT JOIN users u ON u.id = m.user_id
        LEFT JOIN ml_sentiment s ON s.message_id = m.id
        LEFT JOIN ml_toxicity t ON t.message_id = m.id
        LEFT JOIN ml_humor h ON h.message_id = m.id
        WHERE m.id = ANY(:message_ids)
        """
        return self._execute_many(query, {"message_ids": message_ids})
